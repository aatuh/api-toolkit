package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func TestMiddlewareConcurrentInMemoryBucketsRace(t *testing.T) {
	mw, err := New(Options{
		Capacity:   128,
		RefillRate: 1,
		Key:        func(*http.Request) string { return "client-1" },
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var passed atomic.Int32
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		passed.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))

	errs := make(chan error, 64)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			req.RemoteAddr = fmt.Sprintf("198.51.100.%d:443", i%32+1)
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent && rec.Code != http.StatusTooManyRequests {
				errs <- fmt.Errorf("request %d status = %d", i, rec.Code)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
	if passed.Load() == 0 {
		t.Fatal("expected at least one request to pass through the shared bucket")
	}
}
