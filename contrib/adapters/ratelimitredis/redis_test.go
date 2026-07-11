package ratelimitredis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/ratelimittest"
	"github.com/aatuh/api-toolkit/v3/middleware/ratelimit"
)

func TestLimiterContract(t *testing.T) {
	t.Parallel()

	ratelimittest.AssertLimiterContract(t, func(t testing.TB, cfg ratelimittest.Config) ratelimit.Limiter {
		t.Helper()
		mini := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
		t.Cleanup(func() {
			_ = client.Close()
		})
		return New(client, Options{
			Capacity:   cfg.Capacity,
			RefillRate: cfg.RefillRate,
			StateTTL:   cfg.StateTTL,
			KeyPrefix:  cfg.KeyPrefix,
			Clock:      cfg.Clock,
		})
	})
}

func TestLimiterAllowTracksRetryAfterAndTTL(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	now := time.Unix(1_700_000_000, 0).UTC()
	limiter := New(client, Options{
		Capacity:   2,
		RefillRate: 2,
		StateTTL:   3 * time.Second,
		KeyPrefix:  "rl:",
		Clock: func() time.Time {
			return now
		},
	})

	ctx := context.Background()
	key := "customer:42"

	allowed, retryAfter, err := limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow() first call error = %v", err)
	}
	if !allowed || retryAfter != 0 {
		t.Fatalf("Allow() first call = (%v, %v), want (true, 0)", allowed, retryAfter)
	}
	if got := mini.TTL("rl:" + key); got != 3*time.Second {
		t.Fatalf("Allow() first TTL = %v, want %v", got, 3*time.Second)
	}

	allowed, retryAfter, err = limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow() second call error = %v", err)
	}
	if !allowed || retryAfter != 0 {
		t.Fatalf("Allow() second call = (%v, %v), want (true, 0)", allowed, retryAfter)
	}

	allowed, retryAfter, err = limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow() third call error = %v", err)
	}
	if allowed {
		t.Fatal("Allow() third call = true, want false")
	}
	assertDurationNear(t, retryAfter, 500*time.Millisecond, 20*time.Millisecond)
	if got := mini.TTL("rl:" + key); got != 3*time.Second {
		t.Fatalf("Allow() denied TTL = %v, want %v", got, 3*time.Second)
	}

	now = now.Add(250 * time.Millisecond)
	allowed, retryAfter, err = limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow() fourth call error = %v", err)
	}
	if allowed {
		t.Fatal("Allow() fourth call = true, want false")
	}
	assertDurationNear(t, retryAfter, 250*time.Millisecond, 20*time.Millisecond)

	now = now.Add(250 * time.Millisecond)
	allowed, retryAfter, err = limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow() fifth call error = %v", err)
	}
	if !allowed || retryAfter != 0 {
		t.Fatalf("Allow() fifth call = (%v, %v), want (true, 0)", allowed, retryAfter)
	}
}

func TestParseLimiterResultCoversStringAndErrorCases(t *testing.T) {
	t.Parallel()

	allowed, retryAfter, err := parseLimiterResult([]any{"1", "0.5"})
	if err != nil {
		t.Fatalf("parseLimiterResult() string values error = %v", err)
	}
	if !allowed || retryAfter != 0.5 {
		t.Fatalf("parseLimiterResult() string values = (%v, %v), want (true, 0.5)", allowed, retryAfter)
	}

	if _, _, err := parseLimiterResult([]any{int64(1)}); err == nil {
		t.Fatal("parseLimiterResult() short response error = nil, want error")
	}
	if _, _, err := parseLimiterResult(struct{}{}); err == nil {
		t.Fatal("parseLimiterResult() unexpected type error = nil, want error")
	}
	if _, err := toFloat(struct{}{}); err == nil {
		t.Fatal("toFloat() unexpected type error = nil, want error")
	}
}

func assertDurationNear(t *testing.T, got, want, tolerance time.Duration) {
	t.Helper()
	if got < want-tolerance || got > want+tolerance {
		t.Fatalf("duration = %v, want %v +/- %v", got, want, tolerance)
	}
}
