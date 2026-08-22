package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fixedDecisionLimiter struct {
	decision Decision
	err      error
}

func (l fixedDecisionLimiter) Allow(context.Context, string) (Decision, error) {
	return l.decision, l.err
}

func TestDecisionLimiterSuppliesCompleteQuotaHeaders(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	mw, err := New(Options{
		Clock: headerClock{now: now},
		Key:   func(*http.Request) string { return "client-1" },
		DecisionLimiter: fixedDecisionLimiter{decision: Decision{
			Allowed:    false,
			Limit:      10,
			Remaining:  0,
			Reset:      now.Add(time.Minute),
			RetryAfter: time.Second,
		}},
		HeaderConfig: DefaultHeaderConfig(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler must not run for denied decision")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	for name, want := range map[string]string{
		"RateLimit-Limit":     "10",
		"RateLimit-Remaining": "0",
		"RateLimit-Reset":     "1060",
		"Retry-After":         "1",
	} {
		if got := recorder.Header().Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestDecisionLimiterSuppliesHeadersForAllowedDecision(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	mw, err := New(Options{
		Clock: headerClock{now: now},
		DecisionLimiter: fixedDecisionLimiter{decision: Decision{
			Allowed:   true,
			Limit:     10,
			Remaining: 9,
			Reset:     now.Add(time.Minute),
		}},
		HeaderConfig: DefaultHeaderConfig(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	for name, want := range map[string]string{
		"RateLimit-Limit":     "10",
		"RateLimit-Remaining": "9",
		"RateLimit-Reset":     "1060",
	} {
		if got := recorder.Header().Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestQuotaFromDecisionPreservesLegacyQuota(t *testing.T) {
	reset := time.Unix(1_060, 0).UTC()
	quota := QuotaFromDecision(Decision{
		Quota:      Quota{Limit: 10, Remaining: 3, Reset: reset},
		RetryAfter: time.Second,
	})

	if quota.Limit != 10 || quota.Remaining != 3 || !quota.Reset.Equal(reset) || quota.RetryAfter != time.Second {
		t.Fatalf("QuotaFromDecision() = %#v, want preserved legacy quota", quota)
	}
}

func TestQuotaFromDecisionPreservesLegacyNestedRetryAfter(t *testing.T) {
	quota := QuotaFromDecision(Decision{
		Quota:      Quota{RetryAfter: 2 * time.Second},
		RetryAfter: time.Second,
	})
	if quota.RetryAfter != 2*time.Second {
		t.Fatalf("RetryAfter = %s, want legacy nested value", quota.RetryAfter)
	}
}

func TestCleanupProcessesOnlyFixedBatch(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	mw, err := New(Options{
		Clock:           headerClock{now: now},
		StateTTL:        time.Second,
		CleanupInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for i := 0; i < maxCleanupPerRequest*2; i++ {
		key := "expired-" + itoa(i)
		bucket := newBucket(now.Add(-2 * time.Second))
		bucket.key = key
		mw.m[key] = bucket
		mw.trackBucket(bucket)
	}

	mw.cleanup(now)
	if got, want := len(mw.m), maxCleanupPerRequest; got != want {
		t.Fatalf("remaining buckets after bounded cleanup = %d, want %d", got, want)
	}
	mw.cleanup(now)
	if got := len(mw.m); got != 0 {
		t.Fatalf("remaining buckets after pending cleanup = %d, want 0", got)
	}
}

func TestCleanupPreservesFreshState(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	mw, err := New(Options{
		Clock:           headerClock{now: now},
		StateTTL:        time.Second,
		CleanupInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	bucket := newBucket(now)
	bucket.key = "fresh"
	mw.m[bucket.key] = bucket
	mw.trackBucket(bucket)

	mw.cleanup(now)
	if got := mw.m[bucket.key]; got != bucket {
		t.Fatalf("fresh bucket = %p, want %p", got, bucket)
	}
}
