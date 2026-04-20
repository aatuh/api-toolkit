package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoadConfigAndHelpers(t *testing.T) {
	t.Setenv("API_HOST", "http://api.example")
	t.Setenv("SKIP_API_WAIT", "true")
	t.Setenv("API_WAIT_TIMEOUT", "45s")
	t.Setenv("PKG", "./internal/...")
	t.Setenv("TEST_PATTERN", "TestOnly")
	t.Setenv("FLAGS", "-race -count=1")
	t.Setenv("FAST", "true")
	t.Setenv("TEST_CACHE_ENABLED", "false")
	t.Setenv("TEST_CACHE_TTL", "42")
	t.Setenv("TEST_CACHE_DIR", ".cache-tests")

	cfg := LoadConfig()
	if cfg.APIHost != "http://api.example" {
		t.Fatalf("APIHost = %q, want %q", cfg.APIHost, "http://api.example")
	}
	if !cfg.SkipAPIWait || !cfg.FastMode {
		t.Fatal("expected skip wait and fast mode to be true")
	}
	if cfg.APIWaitTimeout != 45*time.Second {
		t.Fatalf("APIWaitTimeout = %v, want %v", cfg.APIWaitTimeout, 45*time.Second)
	}
	if cfg.PackagePattern != "./internal/..." || cfg.TestPattern != "TestOnly" {
		t.Fatalf("unexpected package/test pattern config: %#v", cfg)
	}
	if cfg.Flags != "-race -count=1" {
		t.Fatalf("Flags = %q, want %q", cfg.Flags, "-race -count=1")
	}
	if cfg.CacheEnabled {
		t.Fatal("expected cache to be disabled")
	}
	if cfg.CacheTTL != 42 || cfg.CacheDir != ".cache-tests" {
		t.Fatalf("unexpected cache config: %#v", cfg)
	}

	keyA := computeCacheKey("./...", "TestFoo", "-race")
	keyB := computeCacheKey("./...", "TestFoo", "-race")
	if keyA != keyB {
		t.Fatal("expected cache keys to be deterministic")
	}
	if got := splitFlags("-race -count=1"); len(got) != 2 || got[0] != "-race" || got[1] != "-count=1" {
		t.Fatalf("splitFlags() = %#v", got)
	}
	if got := sanitizeForFilename("pkg/path.test"); got != "pkg--path--test" {
		t.Fatalf("sanitizeForFilename() = %q, want %q", got, "pkg--path--test")
	}
}

func TestWaitForAPISucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path = %q, want /health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := waitForAPI(context.Background(), server.URL, 5*time.Second); err != nil {
		t.Fatalf("waitForAPI() error = %v", err)
	}
}

func TestWaitForAPITimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	err := waitForAPI(context.Background(), server.URL, 1200*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if got := err.Error(); got != "timed out waiting for API after 1.2s" {
		t.Fatalf("unexpected timeout error: %q", got)
	}
}

func TestRunGoListHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runGoList(ctx, "./...")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestRunGoCmdStreamingHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runGoCmdStreaming(ctx, "version")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestRunTestsWithCacheHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runTestsWithCache(ctx, "./...", "", "", &Config{
		CacheEnabled: false,
		CacheDir:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
