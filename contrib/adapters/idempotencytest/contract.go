// Package idempotencytest provides reusable idempotency adapter contract tests.
package idempotencytest

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

// StoreFactory builds a fresh store for one contract test run.
type StoreFactory func(testing.TB) ports.ReservationReleasableIdempotencyStore

// AssertReservationReleaseContract verifies token-aware reservation release
// behavior shared by idempotency stores.
func AssertReservationReleaseContract(t testing.TB, newStore StoreFactory) {
	t.Helper()
	store := newStore(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()

	if err := store.ReleaseReservation(ctx, "missing", "token-a"); err != nil {
		t.Fatalf("release missing: %v", err)
	}

	completedKey := "completed"
	if err := store.Save(ctx, completedKey, ports.IdempotencyRecord{
		State:       ports.IdempotencyStateCompleted,
		RequestHash: "hash-completed",
		Status:      http.StatusCreated,
		CreatedAt:   now,
	}, time.Minute); err != nil {
		t.Fatalf("save completed: %v", err)
	}
	if err := store.ReleaseReservation(ctx, completedKey, ""); err != nil {
		t.Fatalf("release completed: %v", err)
	}
	if record, found, err := store.Get(ctx, completedKey); err != nil {
		t.Fatalf("get completed: %v", err)
	} else if !found || record.State != ports.IdempotencyStateCompleted {
		t.Fatalf("completed record should remain after release: found=%v record=%#v", found, record)
	}

	ambiguousKey := "ambiguous"
	if err := store.Save(ctx, ambiguousKey, ports.IdempotencyRecord{
		State:       ports.IdempotencyStateAmbiguous,
		RequestHash: "hash-ambiguous",
		CreatedAt:   now,
	}, time.Minute); err != nil {
		t.Fatalf("save ambiguous: %v", err)
	}
	if err := store.ReleaseReservation(ctx, ambiguousKey, ""); err != nil {
		t.Fatalf("release ambiguous: %v", err)
	}
	if record, found, err := store.Get(ctx, ambiguousKey); err != nil {
		t.Fatalf("get ambiguous: %v", err)
	} else if !found || record.State != ports.IdempotencyStateAmbiguous {
		t.Fatalf("ambiguous record should remain after release: found=%v record=%#v", found, record)
	}

	tokenedKey := "tokened"
	if ok, err := store.TryBegin(ctx, tokenedKey, ports.IdempotencyRecord{
		State:            ports.IdempotencyStateInFlight,
		RequestHash:      "hash-tokened",
		ReservationToken: "token-a",
		CreatedAt:        now,
	}, time.Minute); err != nil {
		t.Fatalf("try begin tokened: %v", err)
	} else if !ok {
		t.Fatal("try begin tokened returned false")
	}
	if err := store.ReleaseReservation(ctx, tokenedKey, "TOKEN-A"); err == nil {
		t.Fatal("release tokened with case-mismatched token should fail")
	}
	if _, found, err := store.Get(ctx, tokenedKey); err != nil {
		t.Fatalf("get tokened after mismatch: %v", err)
	} else if !found {
		t.Fatal("tokened record should remain after mismatch")
	}
	if err := store.ReleaseReservation(ctx, tokenedKey, "token-a"); err != nil {
		t.Fatalf("release tokened with matching token: %v", err)
	}
	if _, found, err := store.Get(ctx, tokenedKey); err != nil {
		t.Fatalf("get tokened after release: %v", err)
	} else if found {
		t.Fatal("tokened record should be deleted after matching release")
	}

	legacyKey := "legacy"
	if err := store.Save(ctx, legacyKey, ports.IdempotencyRecord{
		State:       ports.IdempotencyStateInFlight,
		RequestHash: "hash-legacy",
		CreatedAt:   now,
	}, time.Minute); err != nil {
		t.Fatalf("save legacy: %v", err)
	}
	if err := store.ReleaseReservation(ctx, legacyKey, "unexpected-token"); !errors.Is(err, ports.ErrLegacyInFlightTokenMismatch) {
		t.Fatalf("legacy release with token error = %v", err)
	}
	if _, found, err := store.Get(ctx, legacyKey); err != nil {
		t.Fatalf("get legacy after mismatch: %v", err)
	} else if !found {
		t.Fatal("legacy tokenless record should remain after non-empty token")
	}
	if err := store.ReleaseReservation(ctx, legacyKey, ""); !errors.Is(err, ports.ErrLegacyInFlightReservationMissingToken) {
		t.Fatalf("legacy release without token error = %v", err)
	}
	if _, found, err := store.Get(ctx, legacyKey); err != nil {
		t.Fatalf("get legacy after recovery: %v", err)
	} else if found {
		t.Fatal("legacy tokenless record should be deleted after empty-token release")
	}

	expiredKey := "expired"
	if err := store.Save(ctx, expiredKey, ports.IdempotencyRecord{
		State:            ports.IdempotencyStateInFlight,
		RequestHash:      "hash-expired",
		ReservationToken: "token-expired",
		CreatedAt:        now,
	}, time.Nanosecond); err != nil {
		t.Fatalf("save expired: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := store.ReleaseReservation(ctx, expiredKey, "token-expired"); err != nil {
		t.Fatalf("release expired: %v", err)
	}
	if _, found, err := store.Get(ctx, expiredKey); err != nil {
		t.Fatalf("get expired after release: %v", err)
	} else if found {
		t.Fatal("expired record should not remain visible after release")
	}

	legacyReleaseKey := "legacy-release"
	if ok, err := store.TryBegin(ctx, legacyReleaseKey, ports.IdempotencyRecord{
		State:            ports.IdempotencyStateInFlight,
		RequestHash:      "hash-legacy-release",
		ReservationToken: "reservation-legacy-release",
		CreatedAt:        now,
	}, time.Minute); err != nil {
		t.Fatalf("try begin legacy release: %v", err)
	} else if !ok {
		t.Fatal("try begin legacy release returned false")
	}
	if err := store.Release(ctx, legacyReleaseKey); err != nil {
		t.Fatalf("legacy release: %v", err)
	}
	if _, found, err := store.Get(ctx, legacyReleaseKey); err != nil {
		t.Fatalf("get legacy release after release: %v", err)
	} else if found {
		t.Fatal("legacy Release should delete in-flight record by key")
	}
}
