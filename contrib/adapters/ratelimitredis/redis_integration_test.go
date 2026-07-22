//go:build redis

package ratelimitredis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testredis"
)

func TestLimiterUsesRealRedisLuaConcurrencyAndIsolation(t *testing.T) {
	h := testredis.New(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	limiter := New(h.Client, Options{Capacity: 4, RefillRate: 0.01, StateTTL: 2 * time.Second, KeyPrefix: h.Key("ratelimit:"), Clock: func() time.Time { return now }})

	start := make(chan struct{})
	results := make(chan bool, 16)
	var wg sync.WaitGroup
	for i := 0; i < cap(results); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			allowed, _, err := limiter.Allow(ctx, "tenant-a:client-a")
			if err != nil {
				t.Errorf("Allow() error = %v", err)
				return
			}
			results <- allowed
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	allowed := 0
	for result := range results {
		if result {
			allowed++
		}
	}
	if allowed != 4 {
		t.Fatalf("atomic Lua allowances = %d, want 4", allowed)
	}
	if allowed, _, err := limiter.Allow(ctx, "tenant-b:client-a"); err != nil || !allowed {
		t.Fatalf("tenant-isolated Allow() = (%t, %v)", allowed, err)
	}
	if allowed, _, err := limiter.Allow(ctx, ""); err != nil || !allowed {
		t.Fatalf("empty-key Allow() = (%t, %v)", allowed, err)
	}
	if ttl, err := h.Client.TTL(ctx, h.Key("ratelimit:tenant-a:client-a")).Result(); err != nil || ttl <= 0 || ttl > 2*time.Second {
		t.Fatalf("state TTL = (%v, %v)", ttl, err)
	}
	if err := h.Client.HSet(ctx, h.Key("ratelimit:malformed"), "tokens", "not-a-number", "ts", now.Unix()).Err(); err != nil {
		t.Fatalf("store malformed limiter state: %v", err)
	}
	if allowed, _, err := limiter.Allow(ctx, "malformed"); err != nil || !allowed {
		t.Fatalf("malformed state fallback = (%t, %v)", allowed, err)
	}
	if _, _, err := limiter.Allow(h.CanceledContext(t), "canceled"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Allow() canceled context error = %v", err)
	}
}
