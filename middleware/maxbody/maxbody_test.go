package maxbody

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRequiresPositiveLimit(t *testing.T) {
	if _, err := New(Options{MaxBytes: 0}); err == nil {
		t.Fatal("expected error for zero max bytes")
	}
}

func TestNilMiddlewareHandler(t *testing.T) {
	var mw *Middleware
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
