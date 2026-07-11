package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/idempotencytest"
	"github.com/aatuh/api-toolkit/v4/middleware/idempotency"
)

func TestMemoryStoreReleaseContract(t *testing.T) {
	t.Parallel()

	idempotencytest.AssertReservationReleaseContract(t, func(t testing.TB) idempotency.ReleasableStore {
		t.Helper()
		return NewMemoryStore()
	})
}

func TestMemoryStoreReleaseRespectsReservationToken(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	key := "order:123"
	record := idempotency.Record{
		State:            idempotency.StateInFlight,
		ReservationToken: "token-1",
		RequestHash:      "hash-a",
		CreatedAt:        time.Unix(1_700_000_000, 123_000_000).UTC(),
	}

	if err := store.Save(context.Background(), key, record, time.Minute); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.ReleaseReservation(context.Background(), key, "token-mismatch"); err == nil {
		t.Fatal("Release() expected token mismatch, got nil")
	}
	if got, found, err := store.Get(context.Background(), key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if !found || got.ReservationToken != "token-1" {
		t.Fatalf("expected inflight record to remain, got %#v", got)
	}

	if err := store.ReleaseReservation(context.Background(), key, "token-1"); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, found, err := store.Get(context.Background(), key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if found {
		t.Fatal("expected record to be removed after matching token")
	}
}

func TestMemoryStoreReleaseReservationDoesNotDeleteSuccessorReservation(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	now := time.Unix(1_700_000_000, 123_000_000).UTC()
	store.now = func() time.Time { return now }
	ctx := context.Background()
	key := "order:successor"
	if ok, err := store.TryBegin(ctx, key, idempotency.Record{
		State:            idempotency.StateInFlight,
		RequestHash:      "hash-old",
		ReservationToken: "token-old",
		CreatedAt:        now,
	}, time.Millisecond); err != nil {
		t.Fatalf("TryBegin() old reservation error = %v", err)
	} else if !ok {
		t.Fatal("TryBegin() old reservation = false, want true")
	}

	now = now.Add(2 * time.Millisecond)
	if ok, err := store.TryBegin(ctx, key, idempotency.Record{
		State:            idempotency.StateInFlight,
		RequestHash:      "hash-new",
		ReservationToken: "token-new",
		CreatedAt:        now,
	}, time.Minute); err != nil {
		t.Fatalf("TryBegin() successor error = %v", err)
	} else if !ok {
		t.Fatal("TryBegin() successor = false, want true after expiry")
	}

	if err := store.ReleaseReservation(ctx, key, "token-old"); !errors.Is(err, idempotency.ErrLegacyInFlightTokenMismatch) {
		t.Fatalf("ReleaseReservation() stale token error = %v, want %v", err, idempotency.ErrLegacyInFlightTokenMismatch)
	}
	got, found, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() successor error = %v", err)
	}
	if !found || got.ReservationToken != "token-new" || got.RequestHash != "hash-new" {
		t.Fatalf("successor reservation was not preserved: found=%v record=%#v", found, got)
	}

	if err := store.ReleaseReservation(ctx, key, "token-new"); err != nil {
		t.Fatalf("ReleaseReservation() successor error = %v", err)
	}
	if _, found, err := store.Get(ctx, key); err != nil {
		t.Fatalf("Get() after successor release error = %v", err)
	} else if found {
		t.Fatal("successor reservation should be deleted after matching release")
	}
}

func TestMemoryStoreReleaseRecoversLegacyTokenlessInflightRecord(t *testing.T) {
	t.Parallel()

	var seen []LegacyInFlightRecoveryEvent
	store := NewMemoryStoreWithOptions(MemoryStoreOptions{
		OnLegacyInFlightRecovery: func(_ context.Context, event LegacyInFlightRecoveryEvent) {
			seen = append(seen, event)
		},
	})
	key := "order:legacy"
	record := idempotency.Record{
		State:       idempotency.StateInFlight,
		RequestHash: "hash-legacy",
		CreatedAt:   time.Unix(1_700_000_000, 123_000_000).UTC(),
	}

	if err := store.Save(context.Background(), key, record, time.Minute); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.ReleaseReservation(context.Background(), key, ""); !errors.Is(err, idempotency.ErrLegacyInFlightReservationMissingToken) {
		t.Fatalf("Release() error = %v, want %v", err, idempotency.ErrLegacyInFlightReservationMissingToken)
	}
	if len(seen) != 1 {
		t.Fatalf("expected one legacy recovery event, got %d", len(seen))
	}
	if got := seen[0].Outcome; got != LegacyInFlightRecoveryRecovered {
		t.Fatalf("expected outcome %q, got %q", LegacyInFlightRecoveryRecovered, got)
	}
	if seen[0].Key != hashTestValue(key) || seen[0].KeyHash != hashTestValue(key) {
		t.Fatalf("expected hashed recovery key, got key=%q hash=%q", seen[0].Key, seen[0].KeyHash)
	}
	if seen[0].RawKey != "" {
		t.Fatalf("raw key leaked by default: %q", seen[0].RawKey)
	}
	if _, found, err := store.Get(context.Background(), key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if found {
		t.Fatal("expected legacy tokenless in-flight record to be removed")
	}
}

func TestMemoryStoreReleaseRejectsTokenlessLegacyRecordWhenTokenSupplied(t *testing.T) {
	t.Parallel()

	var events []LegacyInFlightRecoveryEvent
	store := NewMemoryStoreWithOptions(MemoryStoreOptions{
		OnLegacyInFlightRecovery: func(_ context.Context, event LegacyInFlightRecoveryEvent) {
			events = append(events, event)
		},
	})
	key := "order:legacy-mismatch"
	record := idempotency.Record{
		State:       idempotency.StateInFlight,
		RequestHash: "hash-legacy",
		CreatedAt:   time.Unix(1_700_000_000, 123_000_000).UTC(),
	}

	if err := store.Save(context.Background(), key, record, time.Minute); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.ReleaseReservation(context.Background(), key, "unexpected-token"); err == nil {
		t.Fatal("Release() expected token mismatch error, got nil")
	}
	if len(events) != 1 {
		t.Fatalf("expected one legacy recovery event, got %d", len(events))
	}
	if got := events[0].Outcome; got != LegacyInFlightRecoveryTokenMismatch {
		t.Fatalf("expected outcome %q, got %q", LegacyInFlightRecoveryTokenMismatch, got)
	}
	if events[0].Key == key || events[0].RawKey != "" {
		t.Fatalf("expected default recovery event to avoid raw key, got key=%q raw=%q", events[0].Key, events[0].RawKey)
	}
	if _, found, err := store.Get(context.Background(), key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if !found {
		t.Fatal("expected legacy tokenless record to remain when token mismatches")
	}
}

func TestMemoryStoreLegacyReleaseRemovesInflightRecordByKey(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	key := "order:legacy-release"
	record := idempotency.Record{
		State:            idempotency.StateInFlight,
		ReservationToken: "token-1",
		RequestHash:      "hash-a",
		CreatedAt:        time.Unix(1_700_000_000, 123_000_000).UTC(),
	}

	if err := store.Save(context.Background(), key, record, time.Minute); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Release(context.Background(), key); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, found, err := store.Get(context.Background(), key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if found {
		t.Fatal("expected legacy Release to remove in-flight record")
	}
}

func TestMemoryStoreLegacyRecoveryRawKeyRequiresOptIn(t *testing.T) {
	t.Parallel()

	var events []LegacyInFlightRecoveryEvent
	store := NewMemoryStoreWithOptions(MemoryStoreOptions{
		LegacyInFlightRecoveryRawKey: true,
		OnLegacyInFlightRecovery: func(_ context.Context, event LegacyInFlightRecoveryEvent) {
			events = append(events, event)
		},
	})
	key := "order:raw-key"
	if err := store.Save(context.Background(), key, idempotency.Record{
		State:       idempotency.StateInFlight,
		RequestHash: "hash-legacy",
		CreatedAt:   time.Unix(1_700_000_000, 123_000_000).UTC(),
	}, time.Minute); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.ReleaseReservation(context.Background(), key, ""); !errors.Is(err, idempotency.ErrLegacyInFlightReservationMissingToken) {
		t.Fatalf("Release() error = %v, want %v", err, idempotency.ErrLegacyInFlightReservationMissingToken)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	if events[0].Key != key || events[0].RawKey != key {
		t.Fatalf("expected explicit raw key opt-in, got key=%q raw=%q", events[0].Key, events[0].RawKey)
	}
	if events[0].KeyHash != hashTestValue(key) {
		t.Fatalf("expected hashed key alongside raw opt-in, got %q", events[0].KeyHash)
	}
}

func hashTestValue(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}
