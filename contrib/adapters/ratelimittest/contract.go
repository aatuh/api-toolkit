// Package ratelimittest provides reusable rate limiter adapter contract tests.
package ratelimittest

import (
	"context"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/middleware/ratelimit"
)

// Config contains deterministic limiter settings for a contract test run.
type Config struct {
	Capacity   float64
	RefillRate float64
	StateTTL   time.Duration
	KeyPrefix  string
	Clock      func() time.Time
}

// LimiterFactory builds a fresh limiter for one contract test run.
type LimiterFactory func(testing.TB, Config) ratelimit.Limiter

// AssertLimiterContract verifies rate limiter behavior shared by supported
// adapters.
func AssertLimiterContract(t testing.TB, newLimiter LimiterFactory) {
	t.Helper()

	now := time.Unix(1_700_000_000, 0).UTC()
	limiter := newLimiter(t, Config{
		Capacity:   2,
		RefillRate: 2,
		StateTTL:   time.Minute,
		KeyPrefix:  "ratelimit-contract:",
		Clock: func() time.Time {
			return now
		},
	})
	if limiter == nil {
		t.Fatal("newLimiter returned nil")
	}

	ctx := context.Background()
	assertAllowed(t, limiter, ctx, "")

	primaryKey := "tenant-a:user-1"
	assertAllowed(t, limiter, ctx, primaryKey)
	assertAllowed(t, limiter, ctx, primaryKey)
	firstRetryAfter := assertDenied(t, limiter, ctx, primaryKey, 500*time.Millisecond)

	assertAllowed(t, limiter, ctx, "tenant-b:user-1")
	assertAllowed(t, limiter, ctx, "tenant-a:user-2")

	now = now.Add(250 * time.Millisecond)
	secondRetryAfter := assertDenied(t, limiter, ctx, primaryKey, 250*time.Millisecond)
	if secondRetryAfter >= firstRetryAfter {
		t.Fatalf("retry-after after partial refill = %v, want less than initial %v", secondRetryAfter, firstRetryAfter)
	}

	now = now.Add(250 * time.Millisecond)
	assertAllowed(t, limiter, ctx, primaryKey)
}

func assertAllowed(t testing.TB, limiter ratelimit.Limiter, ctx context.Context, key string) {
	t.Helper()

	allowed, retryAfter, err := limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow(%q) error = %v", key, err)
	}
	if !allowed || retryAfter != 0 {
		t.Fatalf("Allow(%q) = (%v, %v), want (true, 0)", key, allowed, retryAfter)
	}
}

func assertDenied(t testing.TB, limiter ratelimit.Limiter, ctx context.Context, key string, wantRetryAfter time.Duration) time.Duration {
	t.Helper()

	allowed, retryAfter, err := limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow(%q) error = %v", key, err)
	}
	if allowed {
		t.Fatalf("Allow(%q) = true, want false", key)
	}
	assertDurationNear(t, retryAfter, wantRetryAfter, 20*time.Millisecond)
	return retryAfter
}

func assertDurationNear(t testing.TB, got, want, tolerance time.Duration) {
	t.Helper()
	if got < want-tolerance || got > want+tolerance {
		t.Fatalf("duration = %v, want %v +/- %v", got, want, tolerance)
	}
}
