package timeout

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRequiresPositiveTimeout(t *testing.T) {
	if _, err := NewPropagator(Options{Timeout: 0}); err == nil {
		t.Fatal("expected error for zero timeout")
	}
	if _, err := NewHard(Options{Timeout: time.Millisecond, MaxCaptureBytes: -1}); err == nil {
		t.Fatal("expected error for negative hard-timeout capture limit")
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

func TestHardTimeoutWritesProblemAndDiscardsLateHandlerResponse(t *testing.T) {
	mw, err := NewHard(Options{Timeout: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	done := make(chan struct{})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("expected request deadline")
		}
		time.Sleep(30 * time.Millisecond)
		w.Header().Set("X-Late", "true")
		if _, err := w.Write([]byte("late")); err == nil {
			t.Error("expected late write to fail after hard timeout")
		}
		close(done)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/problem+json") {
		t.Fatalf("expected problem content type, got %q", got)
	}
	if rec.Header().Get("X-Late") != "" {
		t.Fatalf("late handler header leaked: %q", rec.Header().Get("X-Late"))
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if body["status"] != float64(http.StatusGatewayTimeout) {
		t.Fatalf("problem status = %#v, want 504", body["status"])
	}
	<-done
}

func TestHardTimeoutPreservesFastHandlerStatusHeadersAndBody(t *testing.T) {
	mw, err := NewHard(Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Result", "fast")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if rec.Header().Get("X-Result") != "fast" {
		t.Fatalf("expected fast header, got %q", rec.Header().Get("X-Result"))
	}
	if rec.Body.String() != "created" {
		t.Fatalf("expected body %q, got %q", "created", rec.Body.String())
	}
}

func TestHardTimeoutRejectsOversizedCapturedResponse(t *testing.T) {
	mw, err := NewHard(Options{Timeout: time.Second, MaxCaptureBytes: 4})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	writeErr := make(chan error, 1)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Result", "oversized")
		w.WriteHeader(http.StatusAccepted)
		_, err := w.Write([]byte("too large"))
		writeErr <- err
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	handler.ServeHTTP(rec, req)

	if !errors.Is(<-writeErr, ErrHardTimeoutCaptureLimitExceeded) {
		t.Fatal("expected handler write to receive capture-limit error")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on capture overflow, got %d", rec.Code)
	}
	if rec.Header().Get("X-Result") != "" {
		t.Fatalf("oversized handler header leaked: %q", rec.Header().Get("X-Result"))
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if body["status"] != float64(http.StatusInternalServerError) {
		t.Fatalf("problem status = %#v, want 500", body["status"])
	}
}

func TestHardTimeoutDefaultCaptureLimitAllowsSmallResponses(t *testing.T) {
	mw, err := NewHard(Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("small"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "small" {
		t.Fatalf("expected body %q, got %q", "small", rec.Body.String())
	}
}
