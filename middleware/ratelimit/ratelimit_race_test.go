package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

type incrementingClock struct {
	epoch time.Time
	step  atomic.Int64
}

func (c *incrementingClock) Now() time.Time {
	return c.epoch.Add(time.Duration(c.step.Add(1)) * time.Millisecond)
}

func TestMiddlewareConcurrentHighCardinalityCleanupRace(t *testing.T) {
	clock := &incrementingClock{epoch: time.Unix(1_000, 0).UTC()}
	mw, err := New(Options{
		Capacity:        1,
		RefillRate:      1,
		Clock:           clock,
		StateTTL:        time.Millisecond,
		CleanupInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	var passed atomic.Int32
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		passed.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))

	const requestCount = 512
	errs := make(chan error, requestCount)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			recorder := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			req.RemoteAddr = fmt.Sprintf("198.51.%d.%d:443", i/255+1, i%255+1)
			handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusNoContent {
				errs <- fmt.Errorf("request %d status = %d", i, recorder.Code)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
	if got := passed.Load(); got != requestCount {
		t.Fatalf("passed = %d, want %d", got, requestCount)
	}
	mw.mu.Lock()
	expiredAt := clock.Now().Add(time.Hour)
	for cleanup := 0; cleanup <= requestCount/maxCleanupPerRequest; cleanup++ {
		mw.cleanup(expiredAt)
	}
	remaining := len(mw.m)
	mw.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("retained buckets after high-cardinality cleanup = %d, want 0", remaining)
	}
}
