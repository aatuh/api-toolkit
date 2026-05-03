package oteltrace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNilMiddlewareAndSpanNameFallbacks(t *testing.T) {
	var mw *Middleware
	called := false
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("nil middleware called=%v status=%d", called, rec.Code)
	}
	if got := spanName("", ""); got != "unknown" {
		t.Fatalf("spanName empty = %q", got)
	}
	if got := spanName("GET", "/widgets"); got != "GET /widgets" {
		t.Fatalf("spanName route = %q", got)
	}
	if got := chiRoutePattern(nil); got != "" {
		t.Fatalf("chiRoutePattern(nil) = %q", got)
	}
}

func TestMiddlewareUsesCustomRouteAndPreservesExistingRequestID(t *testing.T) {
	mw, err := New(Options{RoutePattern: func(*http.Request) string { return "/widgets/{id}" }})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context() == context.Background() {
			t.Fatal("expected request context to be replaced with trace context")
		}
		w.Header().Set(headerRequestID, "existing")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets/1", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get(headerRequestID); got != "existing" {
		t.Fatalf("request id = %q", got)
	}
}

func TestTraceResponseRecorderFallbacks(t *testing.T) {
	var nilRecorder *responseRecorder
	if nilRecorder.Status() != 0 || nilRecorder.BytesWritten() != 0 || nilRecorder.Committed() || nilRecorder.Unwrap() != nil {
		t.Fatal("nil response recorder should expose zero values")
	}
	if _, err := nilRecorder.Write([]byte("x")); err == nil {
		t.Fatal("expected nil write error")
	}
	if _, _, err := nilRecorder.Hijack(); err == nil {
		t.Fatal("expected nil hijack error")
	}
	if err := nilRecorder.Push("/", nil); err == nil {
		t.Fatal("expected nil push error")
	}
	if _, err := nilRecorder.ReadFrom(nil); err == nil {
		t.Fatal("expected nil readfrom error")
	}
}
