package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestNewDefaultRouterCanBeConstructedRepeatedly(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "")
	t.Setenv("TRUSTED_PROXIES", "")

	for i := 0; i < 2; i++ {
		router, err := NewDefaultRouter(ports.NopLogger{})
		if err != nil {
			t.Fatalf("router error on iteration %d: %v", i, err)
		}
		if router == nil {
			t.Fatalf("expected router on iteration %d", i)
		}
	}
}

func TestNewDefaultRouterReturnsErrorForInvalidTrustedProxies(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "")
	t.Setenv("TRUSTED_PROXIES", "definitely-not-a-proxy")

	router, err := NewDefaultRouter(ports.NopLogger{})
	if err == nil {
		t.Fatal("expected invalid trusted proxies error")
	}
	if router != nil {
		t.Fatal("expected nil router on invalid trusted proxies")
	}
}

func TestNewDefaultRouterWithConfigIgnoresAmbientEnvironment(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "x-env-header")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "true")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "true")
	t.Setenv("TRUSTED_PROXIES", "definitely-not-a-proxy")

	prefixes, err := identity.ParseTrustedProxies([]string{"127.0.0.1/32"})
	if err != nil {
		t.Fatalf("trusted proxies: %v", err)
	}

	router, err := NewDefaultRouterWithConfig(ports.NopLogger{}, DefaultRouterConfig{
		TrustedProxies: prefixes,
	})
	if err != nil {
		t.Fatalf("router error: %v", err)
	}
	if router == nil {
		t.Fatal("expected router")
	}
}

func TestNewDefaultRouterAppliesStrictMiddlewareStack(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "")
	t.Setenv("TRUSTED_PROXIES", "")
	prevProvider := otel.GetTracerProvider()
	prevPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(prevProvider)
		otel.SetTextMapPropagator(prevPropagator)
	})

	router, err := NewDefaultRouter(ports.NopLogger{})
	if err != nil {
		t.Fatalf("router error: %v", err)
	}
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("X-Correlation-ID", "corr-123")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "corr-123" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	if got := rec.Header().Get("X-Trace-ID"); got == "" {
		t.Fatal("expected X-Trace-ID response header")
	}
	if got := rec.Header().Get("traceparent"); got == "" {
		t.Fatal("expected traceparent response header")
	}
}

func TestDefaultRouterConfigFromEnvParsesTrustedProxies(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "x-test-bypass")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "true")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "true")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32,::1/128")

	cfg, err := DefaultRouterConfigFromEnv(nil)
	if err != nil {
		t.Fatalf("config from env: %v", err)
	}

	if !cfg.RateLimit.SkipEnabled {
		t.Fatal("expected rate-limit skip enabled")
	}
	if got := cfg.RateLimit.SkipHeader; got != "x-test-bypass" {
		t.Fatalf("skip header = %q", got)
	}
	if !cfg.RateLimit.AllowDangerousDevBypasses {
		t.Fatal("expected dangerous dev bypasses enabled")
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("expected 2 trusted proxies, got %d", len(cfg.TrustedProxies))
	}
	if want := netip.MustParsePrefix("127.0.0.1/32"); cfg.TrustedProxies[0] != want {
		t.Fatalf("trusted proxy[0] = %v", cfg.TrustedProxies[0])
	}
}
