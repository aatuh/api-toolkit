//go:build redis

package idempotencyredis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testredis"
	"github.com/aatuh/api-toolkit/v4/middleware/idempotency"
)

func TestStoreUsesRealRedisAtomicReservationsScriptsAndMalformedData(t *testing.T) {
	h := testredis.New(t)
	ctx := context.Background()
	store := New(h.Client, Options{KeyPrefix: h.Key("idempotency:")})
	key := "tenant-a:request-1"
	record := idempotency.Record{State: idempotency.StateInFlight, RequestHash: "request-hash", ReservationToken: "token-a", CreatedAt: time.Unix(1_700_000_000, 0).UTC()}

	start := make(chan struct{})
	results := make(chan bool, 12)
	var wg sync.WaitGroup
	for i := 0; i < cap(results); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, err := store.TryBegin(ctx, key, record, time.Second)
			if err != nil {
				t.Errorf("TryBegin() error = %v", err)
				return
			}
			results <- ok
		}()
	}
	close(start)
	wg.Wait()
	close(results)
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
		t.Fatalf("ReleaseReservation() script error = %v", err)
	}
	if _, found, err := store.Get(ctx, key); err != nil || found {
		t.Fatalf("Get() after release = found %t, error %v", found, err)
	}
	if ok, err := store.TryBegin(ctx, "expires", record, 75*time.Millisecond); err != nil || !ok {
		t.Fatalf("TryBegin(expiring) = (%t, %v)", ok, err)
	}
	eventuallyExpiredReservation(t, 3*time.Second, func() bool {
		_, found, err := store.Get(ctx, "expires")
		return err == nil && !found
	})
	if err := h.Client.Set(ctx, h.Key("idempotency:malformed"), "{not-json", 0).Err(); err != nil {
		t.Fatalf("store malformed record: %v", err)
	}
	if _, _, err := store.Get(ctx, "malformed"); err == nil {
		t.Fatal("Get(malformed) error = nil")
	}
	other := New(h.Client, Options{KeyPrefix: h.Key("other-idempotency:")})
	if _, found, err := other.Get(ctx, key); err != nil || found {
		t.Fatalf("other-prefix Get() = found %t, error %v", found, err)
	}
	if _, err := store.TryBegin(h.CanceledContext(t), "canceled", record, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("TryBegin() canceled context error = %v", err)
	}
}

func eventuallyExpiredReservation(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	for {
		if condition() {
			return
		}
		select {
		case <-ticker.C:
		case <-timeoutTimer.C:
			t.Fatal("timed out waiting for real Redis expiration")
		}
	}
}
