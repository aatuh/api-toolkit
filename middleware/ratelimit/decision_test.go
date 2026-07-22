package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDecisionLimiterSuppliesCompleteHeaders(t *testing.T) {
	reset := time.Unix(1_700_000_000, 0).UTC()
	limiter := &decisionTestLimiter{decision: Decision{
		Allowed:    true,
		Limit:      20,
		Remaining:  7,
		Reset:      reset,
		RetryAfter: 0,
	}}
	middleware, err := New(Options{
		DecisionLimiter: limiter,
		Key:             func(*http.Request) string { return "customer-42" },
		HeaderConfig:    DefaultHeaderConfig(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	recorder := httptest.NewRecorder()
	middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("RateLimit-Limit"); got != "20" {
		t.Fatalf("RateLimit-Limit = %q, want 20", got)
	}
	if got := recorder.Header().Get("RateLimit-Remaining"); got != "7" {
		t.Fatalf("RateLimit-Remaining = %q, want 7", got)
	}
	if got := recorder.Header().Get("RateLimit-Reset"); got != "1700000000" {
		t.Fatalf("RateLimit-Reset = %q, want 1700000000", got)
	}
}

func TestDecisionLimiterDenialUsesDecisionRetryAfter(t *testing.T) {
	limiter := &decisionTestLimiter{decision: Decision{
		Allowed:    false,
		Limit:      10,
		Remaining:  0,
		Reset:      time.Unix(1_700_000_010, 0).UTC(),
		RetryAfter: 1500 * time.Millisecond,
	}}
	middleware, err := New(Options{
		DecisionLimiter: limiter,
		HeaderConfig:    DefaultHeaderConfig(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	recorder := httptest.NewRecorder()
	middleware.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("denied request reached next handler")
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if got := recorder.Header().Get("RateLimit-Limit"); got != "10" {
		t.Fatalf("RateLimit-Limit = %q, want 10", got)
	}
	if got := recorder.Header().Get("RateLimit-Remaining"); got != "0" {
		t.Fatalf("RateLimit-Remaining = %q, want 0", got)
	}
	if got := recorder.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("Retry-After = %q, want 2", got)
	}
}

func TestNewRejectsBothLegacyAndDecisionLimiters(t *testing.T) {
	_, err := New(Options{
		Limiter:         fixedLimiter{allowed: true},
		DecisionLimiter: &decisionTestLimiter{decision: Decision{Allowed: true}},
	})
	if err == nil {
		t.Fatal("New() accepted both limiter contracts")
	}
}

func TestDecisionLimiterUsesAnonymousKeyForEmptyInput(t *testing.T) {
	limiter := &decisionTestLimiter{decision: Decision{Allowed: true}}
	middleware, err := New(Options{
		DecisionLimiter: limiter,
		Key:             func(*http.Request) string { return " \t " },
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	middleware.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if limiter.key != anonymousRateLimitKey {
		t.Fatalf("decision limiter key = %q, want %q", limiter.key, anonymousRateLimitKey)
	}
}

func TestCleanupRemovesAtMostConfiguredBatch(t *testing.T) {
	clock := &decisionTestClock{now: time.Unix(1_700_000_000, 0).UTC()}
	middleware, err := New(Options{
		Capacity:         10,
		RefillRate:       1,
		Clock:            clock,
		Key:              func(r *http.Request) string { return r.URL.Query().Get("key") },
		StateTTL:         time.Second,
		CleanupInterval:  time.Second,
		CleanupBatchSize: 3,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for i := 0; i < 10; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?key=%d", i), nil))
	}

	clock.Advance(2 * time.Second)
	middleware.mu.Lock()
	removed := middleware.cleanup(clock.Now())
	remaining := len(middleware.m)
	middleware.mu.Unlock()
	if removed != 3 {
		t.Fatalf("cleanup removed %d buckets, want bounded batch of 3", removed)
	}
	if remaining != 7 {
		t.Fatalf("buckets after bounded cleanup = %d, want 7", remaining)
	}
}

func TestExpiredBucketStartsWithFreshCapacity(t *testing.T) {
	clock := &decisionTestClock{now: time.Unix(1_700_000_000, 0).UTC()}
	middleware, err := New(Options{
		Capacity:        1,
		RefillRate:      0,
		Clock:           clock,
		Key:             func(*http.Request) string { return "customer-42" },
		StateTTL:        time.Second,
		CleanupInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/", nil))
	if denied.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", denied.Code)
	}

	clock.Advance(2 * time.Second)
	fresh := httptest.NewRecorder()
	handler.ServeHTTP(fresh, httptest.NewRequest(http.MethodGet, "/", nil))
	if fresh.Code != http.StatusNoContent {
		t.Fatalf("expired bucket request status = %d, want %d", fresh.Code, http.StatusNoContent)
	}
}

func TestMiddlewareConcurrentHighCardinalityState(t *testing.T) {
	middleware, err := New(Options{
		Capacity:        10,
		RefillRate:      1,
		Key:             func(r *http.Request) string { return r.URL.Query().Get("key") },
		StateTTL:        time.Second,
		CleanupInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	var wait sync.WaitGroup
	for i := 0; i < 256; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("/?key=client-%d", i), nil))
		}(i)
	}
	wait.Wait()
}

type decisionTestLimiter struct {
	decision Decision
	err      error
	key      string
}

func (l *decisionTestLimiter) Allow(_ context.Context, key string) (Decision, error) {
	l.key = key
	return l.decision, l.err
}

type decisionTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *decisionTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *decisionTestClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()
}
