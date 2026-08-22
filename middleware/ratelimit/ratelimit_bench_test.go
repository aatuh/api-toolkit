package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkRateLimit(b *testing.B) {
	mw, err := New(Options{
		Capacity:        1e9,
		RefillRate:      1e6,
		Key:             func(*http.Request) string { return "bench" },
		StateTTL:        time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		b.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/bench", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkRateLimitHighCardinalityCleanup(b *testing.B) {
	clock := &benchmarkClock{now: time.Unix(1_000, 0).UTC()}
	mw, err := New(Options{
		Capacity:        1e9,
		RefillRate:      1e6,
		Clock:           clock,
		Key:             func(r *http.Request) string { return r.RemoteAddr },
		StateTTL:        time.Millisecond,
		CleanupInterval: time.Millisecond,
	})
	if err != nil {
		b.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/bench", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.RemoteAddr = "198.51.100." + itoa(i%255+1) + ":" + itoa(10_000+i)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
	}
}

type benchmarkClock struct{ now time.Time }

func (c *benchmarkClock) Now() time.Time {
	c.now = c.now.Add(time.Millisecond)
	return c.now
}
