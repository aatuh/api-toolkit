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

	"github.com/aatuh/api-toolkit/v3/httpx"
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
