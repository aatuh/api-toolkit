package maxbody

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestNewStoresPositiveLimit(t *testing.T) {
	mw, err := New(Options{MaxBytes: 8})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw.MaxBytes != 8 {
		t.Fatalf("MaxBytes = %d, want 8", mw.MaxBytes)
	}
}

func TestMiddlewareAdapterWrapsHandler(t *testing.T) {
	mw, err := New(Options{MaxBytes: 4})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected body read to exceed limit")
		}
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("12345"))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestHandlerLeavesNilBodyAlone(t *testing.T) {
	mw, err := New(Options{MaxBytes: 4})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			t.Fatal("expected nil body to remain nil")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Body = nil
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}
