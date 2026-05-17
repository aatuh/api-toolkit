package cacheredis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/contrib/v3/cache"
	"github.com/aatuh/api-toolkit/contrib/v3/cache/cachetest"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestStoreContract(t *testing.T) {
	t.Parallel()

	var mini *miniredis.Miniredis
	cachetest.AssertStoreContract(t, func(t testing.TB) cache.Store {
		t.Helper()
		mini = miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mini.Addr(), MaxRetries: 0})
		t.Cleanup(func() {
			_ = client.Close()
		})
		return New(client, Options{KeyPrefix: "cache-contract:"})
	}, func(d time.Duration) {
		// miniredis uses deterministic time and only expires keys when advanced.
		mini.FastForward(d)
	})
}

func TestStoreUsesPrefixAndDefaultTTL(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr(), MaxRetries: 0})
	t.Cleanup(func() {
		_ = client.Close()
	})
	store := New(client, Options{KeyPrefix: "cache:", DefaultTTL: time.Second})

	if err := store.Set(context.Background(), "widget:1", []byte("value"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if got, err := mini.Get("cache:widget:1"); err != nil || got != "value" {
		t.Fatalf("redis value = %q err=%v", got, err)
	}
	if got := mini.TTL("cache:widget:1"); got != time.Second {
		t.Fatalf("TTL = %v, want %v", got, time.Second)
	}
}

func TestHealthChecker(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	checker := HealthChecker(client)
	result := checker.Check(context.Background())
	if result.Status != ports.HealthStatusHealthy {
		t.Fatalf("healthy status = %q message=%q", result.Status, result.Message)
	}

	mini.Close()
	result = checker.Check(context.Background())
	if result.Status != ports.HealthStatusUnhealthy {
		t.Fatalf("closed status = %q message=%q", result.Status, result.Message)
	}
}
