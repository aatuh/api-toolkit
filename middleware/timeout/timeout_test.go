package timeout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRequiresPositiveTimeout(t *testing.T) {
	if _, err := NewPropagator(Options{Timeout: 0}); err == nil {
		t.Fatal("expected error for zero timeout")
	}
}

func TestNewRemainsBackwardCompatible(t *testing.T) {
	mw, err := New(Options{Timeout: time.Millisecond})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw == nil {
		t.Fatal("expected propagator")
	}
}

func TestPropagatorHandlerAppliesContextDeadline(t *testing.T) {
	mw, err := NewPropagator(Options{Timeout: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	done := make(chan error, 1)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Fatal("expected request deadline")
		}
		<-r.Context().Done()
		done <- r.Context().Err()
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestPropagatorHandlerDoesNotForceTimeoutResponse(t *testing.T) {
	mw, err := NewPropagator(Options{Timeout: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
