package testredis

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseConfigRejectsUnsafeURLWithoutEchoingIt(t *testing.T) {
	const secret = "must-not-appear"
	cases := []struct {
		name     string
		endpoint string
	}{
		{name: "missing", endpoint: ""},
		{name: "remote host", endpoint: "redis://cache.example.test:6379/15"},
		{name: "credentials", endpoint: "redis://user:" + secret + "@127.0.0.1:6379/15"},
		{name: "production database", endpoint: "redis://127.0.0.1:6379/0"},
		{name: "missing port", endpoint: "redis://127.0.0.1/15"},
		{name: "unexpected query", endpoint: "redis://127.0.0.1:6379/15?client_name=" + secret},
		{name: "fragment", endpoint: "redis://127.0.0.1:6379/15#" + secret},
		{name: "tls endpoint", endpoint: "rediss://127.0.0.1:6379/15"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseConfig(tc.endpoint)
			if err == nil {
				t.Fatal("parseConfig() error = nil, want rejected test URL")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("parseConfig() error leaked Redis URL secret: %v", err)
			}
		})
	}
}

func TestParseConfigAcceptsDedicatedLocalAndServiceEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"redis://127.0.0.1:6379/15",
		"redis://localhost:56379/15",
		"redis://redis:6379/15",
		"redis://[::1]:6379/15",
	} {
		cfg, err := parseConfig(endpoint)
		if err != nil {
			t.Fatalf("parseConfig(%q) error = %v", endpoint, err)
		}
		if cfg.endpoint != endpoint || cfg.database != 15 || cfg.addr == "" {
			t.Fatalf("parseConfig(%q) = %#v", endpoint, cfg)
		}
	}
}

func TestConfigFromEnvRequiresExplicitOptInAndURL(t *testing.T) {
	t.Setenv(EnableEnv, "")
	t.Setenv(URLEnv, "redis://127.0.0.1:6379/15")
	if _, err := configFromEnv(); err == nil {
		t.Fatal("configFromEnv() error = nil, want explicit opt-in error")
	}

	t.Setenv(EnableEnv, "1")
	t.Setenv(URLEnv, "")
	if _, err := configFromEnv(); err == nil {
		t.Fatal("configFromEnv() error = nil, want explicit URL error")
	}
}

func TestOpenRejectsUnavailableRedisWithoutEchoingEndpoint(t *testing.T) {
	if os.Getenv(EnableEnv) == "1" {
		t.Skip("do not replace the configured service during a real Redis integration run")
	}
	t.Setenv(EnableEnv, "1")
	t.Setenv(URLEnv, "redis://127.0.0.1:1/15")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := Open(ctx); err == nil {
		t.Fatal("Open() error = nil, want unavailable-service error")
	} else if strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("Open() error leaked Redis endpoint: %v", err)
	}
}

func TestHarnessLifecycleAndPrefixCleanup(t *testing.T) {
	requireRedis(t)
	h := New(t)
	ctx := context.Background()

	if h.ServerMajorVersion() != 7 {
		t.Fatalf("Redis major version = %d, want declared test major 7", h.ServerMajorVersion())
	}
	key := h.Key("cleanup")
	if err := h.Client().Set(ctx, key, "value", 0).Err(); err != nil {
		t.Fatalf("store cleanup fixture: %v", err)
	}
	monitor, err := h.NewClientForDatabase(15)
	if err != nil {
		t.Fatalf("NewClientForDatabase() error = %v", err)
	}
	t.Cleanup(func() { _ = monitor.Close() })
	if err := h.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if count, err := monitor.Exists(ctx, key).Result(); err != nil || count != 0 {
		t.Fatalf("fixture after Close() = count %d, error %v", count, err)
	}
}

func TestHarnessSupportsCancellationInterruptionAndReconnect(t *testing.T) {
	requireRedis(t)
	h := New(t)
	ctx := context.Background()
	target, err := h.NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = target.Close() })

	key := h.Key("reconnect")
	if err := target.Set(ctx, key, "value", time.Minute).Err(); err != nil {
		t.Fatalf("store reconnect fixture: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := target.Get(canceled, key).Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Get() error = %v, want context canceled", err)
	}
	if err := h.InterruptClient(ctx, target); err != nil {
		t.Fatalf("InterruptClient() error = %v", err)
	}
	value, err := target.Get(ctx, key).Result()
	if err != nil || value != "value" {
		t.Fatalf("Get() after reconnect = (%q, %v)", value, err)
	}
}

func requireRedis(t *testing.T) {
	t.Helper()
	if os.Getenv(EnableEnv) != "1" {
		t.Skip("set API_TOOLKIT_TEST_REDIS=1 through make test-redis to run real Redis harness tests")
	}
	if os.Getenv(URLEnv) == "" {
		t.Fatal("API_TOOLKIT_TEST_REDIS_URL is required when API_TOOLKIT_TEST_REDIS=1")
	}
}
