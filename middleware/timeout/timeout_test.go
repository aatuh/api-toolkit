package timeout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		<-r.Context().Done()
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

	writeErr := make(chan error, 1)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("expected request deadline")
		}
		<-r.Context().Done()
		w.Header().Set("X-Late", "true")
		_, err := w.Write([]byte("late"))
		writeErr <- err
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
	if err := <-writeErr; !errors.Is(err, http.ErrHandlerTimeout) {
		t.Fatalf("late write error = %v, want %v", err, http.ErrHandlerTimeout)
	}
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

func TestHardTimeoutGlobalCompositionBreaksLargeStreamingRouteAndOptOutPreservesIt(t *testing.T) {
	mw, err := NewHard(Options{Timeout: time.Second, MaxCaptureBytes: 4})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}
	largeStreamingRoute := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasFlusher := w.(http.Flusher)
		w.Header().Set("X-Has-Flusher", strconv.FormatBool(hasFlusher))
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte(strings.Repeat("x", 128)))
	})

	globalRec := httptest.NewRecorder()
	mw.Handler(largeStreamingRoute).ServeHTTP(globalRec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/download", nil))
	if globalRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected unsafe global hard timeout to fail on large response, got %d", globalRec.Code)
	}
	if !strings.Contains(globalRec.Body.String(), "timeout-capture-overflow") {
		t.Fatalf("expected capture overflow problem, got %q", globalRec.Body.String())
	}

	mux := http.NewServeMux()
	mux.Handle("/finite", mw.Handler(largeStreamingRoute))
	mux.Handle("/download", largeStreamingRoute)

	finiteRec := httptest.NewRecorder()
	mux.ServeHTTP(finiteRec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/finite", nil))
	if finiteRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected finite route wrapped with hard timeout to fail on oversized response, got %d", finiteRec.Code)
	}

	streamRec := httptest.NewRecorder()
	mux.ServeHTTP(streamRec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/download", nil))
	if streamRec.Code != http.StatusOK {
		t.Fatalf("expected route-level opt-out to preserve response, got %d", streamRec.Code)
	}
	if streamRec.Body.Len() != 128 {
		t.Fatalf("expected full large response, got %d bytes", streamRec.Body.Len())
	}
	if got := streamRec.Header().Get("X-Has-Flusher"); got != "true" {
		t.Fatalf("expected opt-out route to preserve flusher, got %q", got)
	}
	if !streamRec.Flushed {
		t.Fatal("expected opt-out route to flush through the original writer")
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

func TestHardTimeoutContainsPanicBeforeTimeout(t *testing.T) {
	mw, err := NewHard(Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if !strings.Contains(body["type"].(string), "timeout-panic") {
		t.Fatalf("problem type = %#v, want timeout panic", body["type"])
	}
}

func TestHardTimeoutContainsPanicAfterTimeout(t *testing.T) {
	mw, err := NewHard(Options{Timeout: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	done := make(chan struct{})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		<-r.Context().Done()
		panic("late boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
	<-done

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", rec.Code)
	}
}

func TestHardTimeoutContainsPanicAfterPartialCapture(t *testing.T) {
	mw, err := NewHard(Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Partial", "true")
		_, _ = w.Write([]byte("partial"))
		panic("after partial")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if rec.Header().Get("X-Partial") != "" {
		t.Fatalf("partial handler header leaked: %q", rec.Header().Get("X-Partial"))
	}
}

func TestHardTimeoutContainsChildPanicBeforeOuterRecover(t *testing.T) {
	mw, err := NewHard(Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	outerRecovered := false
	outerRecover := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recover() != nil {
					outerRecovered = true
					http.Error(w, "outer recovered", http.StatusTeapot)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
	handler := outerRecover(mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("child boom")
	})))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if outerRecovered {
		t.Fatal("outer recover middleware should not observe child goroutine panic")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected hard-timeout panic response, got %d", rec.Code)
	}
}

func TestHardTimeoutEmitsBoundedTimeoutEvent(t *testing.T) {
	events := make(chan HardTimeoutEvent, 1)
	mw, err := NewHard(Options{
		Timeout: 5 * time.Millisecond,
		EventHooks: &HardTimeoutEventHooks{
			OnEvent: func(event HardTimeoutEvent) {
				events <- event
			},
		},
	})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		_, _ = w.Write([]byte("late"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets?token=secret", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", rec.Code)
	}
	event := <-events
	if event.Outcome != HardTimeoutOutcomeTimeout {
		t.Fatalf("outcome = %q, want %q", event.Outcome, HardTimeoutOutcomeTimeout)
	}
	if event.Method != http.MethodGet {
		t.Fatalf("method = %q, want GET", event.Method)
	}
	if event.Status != http.StatusGatewayTimeout || !event.TimedOut || event.Panicked || event.CaptureOverflow {
		t.Fatalf("event = %#v", event)
	}
	if event.Timeout != 5*time.Millisecond {
		t.Fatalf("timeout = %v, want 5ms", event.Timeout)
	}
	if event.Duration <= 0 {
		t.Fatalf("duration = %v, want positive", event.Duration)
	}
	if event.CaptureLimit != defaultHardTimeoutMaxCaptureBytes {
		t.Fatalf("capture limit = %d, want default", event.CaptureLimit)
	}
}

func TestHardTimeoutEmitsBoundedPanicAndOverflowEvents(t *testing.T) {
	t.Run("panic", func(t *testing.T) {
		var got HardTimeoutEvent
		mw, err := NewHard(Options{
			Timeout: time.Second,
			EventHooks: &HardTimeoutEventHooks{
				OnEvent: func(event HardTimeoutEvent) {
					got = event
				},
			},
		})
		if err != nil {
			t.Fatalf("new hard timeout: %v", err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", nil)
		mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("secret panic detail")
		})).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if got.Outcome != HardTimeoutOutcomePanic || !got.Panicked || got.Status != http.StatusInternalServerError {
			t.Fatalf("event = %#v", got)
		}
		if strings.Contains(fmt.Sprintf("%#v", got), "secret panic detail") {
			t.Fatalf("event leaked panic value: %#v", got)
		}
	})

	t.Run("capture overflow", func(t *testing.T) {
		var got HardTimeoutEvent
		mw, err := NewHard(Options{
			Timeout:         time.Second,
			MaxCaptureBytes: 4,
			EventHooks: &HardTimeoutEventHooks{
				OnEvent: func(event HardTimeoutEvent) {
					got = event
				},
			},
		})
		if err != nil {
			t.Fatalf("new hard timeout: %v", err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", nil)
		mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("too large"))
		})).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if got.Outcome != HardTimeoutOutcomeCaptureOverflow || !got.CaptureOverflow || got.Status != http.StatusInternalServerError {
			t.Fatalf("event = %#v", got)
		}
		if got.CaptureLimit != 4 {
			t.Fatalf("capture limit = %d, want 4", got.CaptureLimit)
		}
	})
}

func TestHardTimeoutEventHookPanicDoesNotChangeResponse(t *testing.T) {
	mw, err := NewHard(Options{
		Timeout: 5 * time.Millisecond,
		EventHooks: &HardTimeoutEventHooks{
			OnEvent: func(HardTimeoutEvent) {
				panic("hook failed")
			},
		},
	})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", rec.Code)
	}
}
