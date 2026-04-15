package oteltrace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestMiddlewareSetsCorrelationHeaders(t *testing.T) {
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

	mw, err := New(Options{})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("X-Correlation-ID", "corr-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get(headerRequestID); got != "corr-123" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	if got := rec.Header().Get(headerTraceID); got == "" {
		t.Fatal("expected X-Trace-ID response header")
	}
	if got := rec.Header().Get("traceparent"); got == "" {
		t.Fatal("expected traceparent response header")
	}
}
