// Package idempotencytest provides reusable idempotency adapter contract tests.
package idempotencytest

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/middleware/idempotency"
)

// StoreFactory builds a fresh store for one contract test run.
type StoreFactory func(testing.TB) idempotency.ReleasableStore

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
	if err := store.Save(ctx, completedKey, idempotency.Record{
		State:       idempotency.StateCompleted,
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
	} else if !found || record.State != idempotency.StateCompleted {
		t.Fatalf("completed record should remain after release: found=%v record=%#v", found, record)
	}

	ambiguousKey := "ambiguous"
	if err := store.Save(ctx, ambiguousKey, idempotency.Record{
		State:       idempotency.StateAmbiguous,
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
	} else if !found || record.State != idempotency.StateAmbiguous {
		t.Fatalf("ambiguous record should remain after release: found=%v record=%#v", found, record)
	}

	tokenedKey := "tokened"
	if ok, err := store.TryBegin(ctx, tokenedKey, idempotency.Record{
		State:            idempotency.StateInFlight,
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
	if err := store.Save(ctx, legacyKey, idempotency.Record{
		State:       idempotency.StateInFlight,
		RequestHash: "hash-legacy",
		CreatedAt:   now,
	}, time.Minute); err != nil {
		t.Fatalf("save legacy: %v", err)
	}
	if err := store.ReleaseReservation(ctx, legacyKey, "unexpected-token"); !errors.Is(err, idempotency.ErrLegacyInFlightTokenMismatch) {
		t.Fatalf("legacy release with token error = %v", err)
	}
	if _, found, err := store.Get(ctx, legacyKey); err != nil {
		t.Fatalf("get legacy after mismatch: %v", err)
	} else if !found {
		t.Fatal("legacy tokenless record should remain after non-empty token")
	}
	if err := store.ReleaseReservation(ctx, legacyKey, ""); !errors.Is(err, idempotency.ErrLegacyInFlightReservationMissingToken) {
		t.Fatalf("legacy release without token error = %v", err)
	}
	if _, found, err := store.Get(ctx, legacyKey); err != nil {
		t.Fatalf("get legacy after recovery: %v", err)
	} else if found {
		t.Fatal("legacy tokenless record should be deleted after empty-token release")
	}

	expiredKey := "expired"
	if err := store.Save(ctx, expiredKey, idempotency.Record{
		State:            idempotency.StateInFlight,
		RequestHash:      "hash-expired",
		ReservationToken: "token-expired",
		CreatedAt:        now,
	}, time.Nanosecond); err != nil {
		t.Fatalf("save expired: %v", err)
	}
	if err := store.ReleaseReservation(ctx, expiredKey, "token-expired"); err != nil {
		t.Fatalf("release expired: %v", err)
	}
	if _, found, err := store.Get(ctx, expiredKey); err != nil {
		t.Fatalf("get expired after release: %v", err)
	} else if found {
		t.Fatal("expired record should not remain visible after release")
	}

}
