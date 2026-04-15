package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

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
