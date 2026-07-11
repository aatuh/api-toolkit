package idempotency

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestMiddlewareConcurrentSameKeyRace(t *testing.T) {
	mw, err := New(Options{
		Store:        newMemoryStore(),
		MaxBodyBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls atomic.Int32
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": string(body)})
	}))

	errs := make(chan error, 32)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
			req.Header.Set("Idempotency-Key", "shared-key")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated && rec.Code != http.StatusConflict {
				errs <- fmt.Errorf("request %d status = %d body=%s", i, rec.Code, rec.Body.String())
			}
		}(i)
	}
	close(start)

	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("timed out waiting for first idempotent request to enter handler")
	}
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("handler calls = %d, want 1 for shared idempotency key", calls.Load())
	}
}

func TestMiddlewareConcurrentSameKeyHashMismatchRace(t *testing.T) {
	mw, err := New(Options{
		Store:        newMemoryStore(),
		MaxBodyBytes: 1024,
		InFlightTTL:  90 * time.Second,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var calls atomic.Int32
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": string(body)})
	}))

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("alpha"))
		req.Header.Set("Idempotency-Key", "shared-mismatch-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		firstDone <- rec
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("timed out waiting for first request to reserve key")
	}

	secondReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charge", strings.NewReader("beta"))
	secondReq.Header.Set("Idempotency-Key", "shared-mismatch-key")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	close(release)

	firstRec := <-firstDone
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("first status = %d body=%s", firstRec.Code, firstRec.Body.String())
	}
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("second status = %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	if got := secondRec.Header().Get("Retry-After"); got != "" {
		t.Fatalf("hash mismatch should not be reported as a retryable lock timeout, Retry-After=%q", got)
	}
	if !strings.Contains(secondRec.Body.String(), "idempotency key reuse with different request") {
		t.Fatalf("expected hash mismatch problem detail, got %q", secondRec.Body.String())
	}
	if strings.Contains(secondRec.Body.String(), "beta") || strings.Contains(secondRec.Body.String(), "alpha") {
		t.Fatalf("conflict response leaked request body: %q", secondRec.Body.String())
	}
	if calls.Load() != 1 {
		t.Fatalf("handler calls = %d, want 1 for mismatched shared idempotency key", calls.Load())
	}
}
