package redis_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	referenceredis "example.com/reference-saas-api/internal/adapters/redis"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/cacheredis"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/idempotencyredis"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/ratelimitredis"
	"github.com/aatuh/api-toolkit/contrib/v4/testredis"
	"github.com/aatuh/api-toolkit/v4/middleware/idempotency"
)

// TestRealRedisSupportedAdapters proves the supported Redis adapters and the
// generated reference-service constructors against Redis 7 rather than
// miniredis. Each subtest owns a cryptographically random key prefix.
func TestRealRedisSupportedAdapters(t *testing.T) {
	requireRedis(t)

	t.Run("cache TTL values isolation cancellation and reconnect", testCache)
	t.Run("idempotency atomic reservation token release and malformed data", testIdempotency)
	t.Run("rate limit Lua concurrency TTL and tenant isolation", testRateLimit)
	t.Run("dependency failures propagate without implicit fail open", testFailurePolicy)
	t.Run("generated reference service paths use real Redis", testGeneratedPaths)
}

func requireRedis(t *testing.T) {
	t.Helper()
	if os.Getenv(testredis.EnableEnv) != "1" {
		t.Skipf("requires %s=1; run make test-redis", testredis.EnableEnv)
	}
	if os.Getenv(testredis.URLEnv) == "" {
		t.Fatalf("%s is required when %s=1", testredis.URLEnv, testredis.EnableEnv)
	}
}

func testCache(t *testing.T) {
	h := testredis.New(t)
	ctx := context.Background()
	store := cacheredis.New(h.Client(), cacheredis.Options{
		KeyPrefix:  h.Key("cache:"),
		DefaultTTL: 75 * time.Millisecond,
	})

	if err := store.Set(ctx, "empty", []byte{}, 0); err != nil {
		t.Fatalf("Set(empty) error = %v", err)
	}
	if value, found, err := store.Get(ctx, "empty"); err != nil || !found || len(value) != 0 {
		t.Fatalf("Get(empty) = length %d, found %t, error %v", len(value), found, err)
	}
	large := bytes.Repeat([]byte("x"), 1<<20)
	if err := store.Set(ctx, "large", large, time.Second); err != nil {
		t.Fatalf("Set(large) error = %v", err)
	}
	if value, found, err := store.Get(ctx, "large"); err != nil || !found || !bytes.Equal(value, large) {
		t.Fatalf("Get(large) = length %d, found %t, error %v", len(value), found, err)
	}
	if err := store.Set(ctx, "expires", []byte("value"), 0); err != nil {
		t.Fatalf("Set(expires) error = %v", err)
	}
	eventuallyRedis(t, 3*time.Second, func() (bool, error) {
		_, found, err := store.Get(ctx, "expires")
		return !found, err
	})

	other := cacheredis.New(h.Client(), cacheredis.Options{
		KeyPrefix:  h.Key("other-cache:"),
		DefaultTTL: time.Second,
	})
	if _, found, err := other.Get(ctx, "large"); err != nil || found {
		t.Fatalf("other-prefix Get() = found %t, error %v", found, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.Set(canceled, "canceled", []byte("value"), time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("Set() canceled context error = %v", err)
	}

	reconnecting, err := h.NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = reconnecting.Close() })
	reconnectingStore := cacheredis.New(reconnecting, cacheredis.Options{
		KeyPrefix:  h.Key("reconnect:"),
		DefaultTTL: time.Second,
	})
	if err := reconnectingStore.Set(ctx, "value", []byte("ok"), time.Second); err != nil {
		t.Fatalf("Set(reconnect) error = %v", err)
	}
	if err := h.InterruptClient(ctx, reconnecting); err != nil {
		t.Fatalf("InterruptClient() error = %v", err)
	}
	if value, found, err := reconnectingStore.Get(ctx, "value"); err != nil || !found || string(value) != "ok" {
		t.Fatalf("Get() after reconnect = (%q, %t, %v)", value, found, err)
	}
}

func testIdempotency(t *testing.T) {
	h := testredis.New(t)
	ctx := context.Background()
	prefix := h.Key("idempotency:")
	store := idempotencyredis.New(h.Client(), idempotencyredis.Options{KeyPrefix: prefix})
	key := "tenant-a:request-1"
	record := idempotency.Record{
		State:            idempotency.StateInFlight,
		RequestHash:      "request-hash",
		ReservationToken: "token-a",
		CreatedAt:        time.Unix(1_700_000_000, 0).UTC(),
	}

	const contenders = 12
	start := make(chan struct{})
	results := make(chan bool, contenders)
	errorsSeen := make(chan error, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, err := store.TryBegin(ctx, key, record, time.Second)
			if err != nil {
				errorsSeen <- err
				return
			}
			results <- ok
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Errorf("concurrent TryBegin() error = %v", err)
	}
	reserved := 0
	for ok := range results {
		if ok {
			reserved++
		}
	}
	if reserved != 1 {
		t.Fatalf("atomic reservations = %d, want 1", reserved)
	}
	if err := store.ReleaseReservation(ctx, key, "wrong-token"); !errors.Is(err, idempotency.ErrLegacyInFlightTokenMismatch) {
		t.Fatalf("ReleaseReservation(wrong token) error = %v", err)
	}
	if err := store.ReleaseReservation(ctx, key, record.ReservationToken); err != nil {
		t.Fatalf("ReleaseReservation() Lua script error = %v", err)
	}
	if ok, err := store.TryBegin(ctx, key, record, time.Second); err != nil || !ok {
		t.Fatalf("TryBegin() retry after release = (%t, %v)", ok, err)
	}
	if err := store.ReleaseReservation(ctx, key, record.ReservationToken); err != nil {
		t.Fatalf("release retried reservation: %v", err)
	}

	if ok, err := store.TryBegin(ctx, "expires", record, 75*time.Millisecond); err != nil || !ok {
		t.Fatalf("TryBegin(expiring) = (%t, %v)", ok, err)
	}
	eventuallyRedis(t, 3*time.Second, func() (bool, error) {
		_, found, err := store.Get(ctx, "expires")
		return !found, err
	})
	if err := h.Client().Set(ctx, prefix+"malformed", "{not-json", 0).Err(); err != nil {
		t.Fatalf("store malformed record: %v", err)
	}
	if _, _, err := store.Get(ctx, "malformed"); err == nil {
		t.Fatal("Get(malformed) error = nil")
	}

	other := idempotencyredis.New(h.Client(), idempotencyredis.Options{KeyPrefix: h.Key("other-idempotency:")})
	if _, found, err := other.Get(ctx, key); err != nil || found {
		t.Fatalf("other-prefix Get() = found %t, error %v", found, err)
	}
	if ok, err := store.TryBegin(ctx, "tenant-b:request-1", record, time.Second); err != nil || !ok {
		t.Fatalf("tenant-isolated TryBegin() = (%t, %v)", ok, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.TryBegin(canceled, "canceled", record, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("TryBegin() canceled context error = %v", err)
	}
}

func testRateLimit(t *testing.T) {
	h := testredis.New(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	prefix := h.Key("ratelimit:")
	limiter := ratelimitredis.New(h.Client(), ratelimitredis.Options{
		Capacity: 4, RefillRate: 0.01, StateTTL: 2 * time.Second,
		KeyPrefix: prefix, Clock: func() time.Time { return now },
	})

	const contenders = 16
	start := make(chan struct{})
	results := make(chan bool, contenders)
	errorsSeen := make(chan error, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			allowed, _, err := limiter.Allow(ctx, "tenant-a:client-a")
			if err != nil {
				errorsSeen <- err
				return
			}
			results <- allowed
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Errorf("concurrent Allow() error = %v", err)
	}
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
	if ttl, err := h.Client().TTL(ctx, prefix+"tenant-a:client-a").Result(); err != nil || ttl <= 0 || ttl > 2*time.Second {
		t.Fatalf("state TTL = (%v, %v)", ttl, err)
	}
	if err := h.Client().HSet(ctx, prefix+"malformed", "tokens", "not-a-number", "ts", now.Unix()).Err(); err != nil {
		t.Fatalf("store malformed limiter state: %v", err)
	}
	if allowed, _, err := limiter.Allow(ctx, "malformed"); err != nil || !allowed {
		t.Fatalf("malformed state fallback = (%t, %v)", allowed, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := limiter.Allow(canceled, "canceled"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Allow() canceled context error = %v", err)
	}
}

func testFailurePolicy(t *testing.T) {
	h := testredis.New(t)
	closed, err := h.NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close failure-policy client: %v", err)
	}
	ctx := context.Background()

	cache := cacheredis.New(closed, cacheredis.Options{KeyPrefix: h.Key("failure-cache:")})
	if _, _, err := cache.Get(ctx, "key"); err == nil {
		t.Fatal("cache Get() dependency failure = nil")
	}
	idem := idempotencyredis.New(closed, idempotencyredis.Options{KeyPrefix: h.Key("failure-idempotency:")})
	if ok, err := idem.TryBegin(ctx, "key", idempotency.Record{State: idempotency.StateInFlight}, time.Second); err == nil || ok {
		t.Fatalf("idempotency TryBegin() dependency failure = (%t, %v)", ok, err)
	}
	limiter := ratelimitredis.New(closed, ratelimitredis.Options{KeyPrefix: h.Key("failure-ratelimit:")})
	if allowed, _, err := limiter.Allow(ctx, "key"); err == nil || allowed {
		t.Fatalf("rate-limit Allow() dependency failure = (%t, %v)", allowed, err)
	}
}

func testGeneratedPaths(t *testing.T) {
	h := testredis.New(t)
	ctx := context.Background()
	cleanup, err := h.NewClientForDatabase(0)
	if err != nil {
		t.Fatalf("open generated-path cleanup client: %v", err)
	}
	t.Cleanup(func() { _ = cleanup.Close() })

	cacheKey := h.Key("generated-cache")
	idempotencyPrefix := h.Key("generated-idempotency:")
	rateLimitPrefix := h.Key("generated-ratelimit:")
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cleanup.Del(cleanupCtx,
			"api:"+cacheKey,
			idempotencyPrefix+"request",
			rateLimitPrefix+"tenant-a",
		).Err()
	})

	cache, err := referenceredis.OpenCache(ctx, h.Addr())
	if err != nil {
		t.Fatalf("OpenCache() error = %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	if err := cache.Store.Set(ctx, cacheKey, []byte("value"), time.Second); err != nil {
		t.Fatalf("generated cache Set(): %v", err)
	}
	if value, found, err := cache.Store.Get(ctx, cacheKey); err != nil || !found || string(value) != "value" {
		t.Fatalf("generated cache Get() = (%q, %t, %v)", value, found, err)
	}

	idem, err := referenceredis.OpenIdempotencyStore(ctx, h.Addr(), idempotencyPrefix)
	if err != nil {
		t.Fatalf("OpenIdempotencyStore() error = %v", err)
	}
	t.Cleanup(func() { _ = idem.Close() })
	if ok, err := idem.Store.TryBegin(ctx, "request", idempotency.Record{
		State: idempotency.StateInFlight, ReservationToken: "token",
	}, time.Second); err != nil || !ok {
		t.Fatalf("generated idempotency TryBegin() = (%t, %v)", ok, err)
	}

	rate, err := referenceredis.OpenRateLimiter(ctx, h.Addr(), rateLimitPrefix, 1, 1)
	if err != nil {
		t.Fatalf("OpenRateLimiter() error = %v", err)
	}
	t.Cleanup(func() { _ = rate.Close() })
	if allowed, _, err := rate.Limiter.Allow(ctx, "tenant-a"); err != nil || !allowed {
		t.Fatalf("generated rate-limit Allow() = (%t, %v)", allowed, err)
	}
}

func eventuallyRedis(t *testing.T, timeout time.Duration, condition func() (bool, error)) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var lastErr error
	for {
		ok, err := condition()
		if err != nil {
			lastErr = err
		} else if ok {
			return
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatalf("timed out waiting for real Redis condition; last error: %v", lastErr)
		}
	}
}
