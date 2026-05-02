package idempotencyredis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/idempotencytest"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestStoreTryBeginGetSaveAndRelease(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store := New(client, Options{KeyPrefix: "idem:"})
	ctx := context.Background()
	key := "order:123"
	inFlightTTL := 2 * time.Minute
	inFlight := ports.IdempotencyRecord{
		State:       ports.IdempotencyStateInFlight,
		RequestHash: "hash-a",
		CreatedAt:   time.Unix(1_700_000_000, 123_000_000).UTC(),
	}

	ok, err := store.TryBegin(ctx, key, inFlight, inFlightTTL)
	if err != nil {
		t.Fatalf("TryBegin() error = %v", err)
	}
	if !ok {
		t.Fatal("TryBegin() = false, want true")
	}
	if got := mini.TTL("idem:" + key); got != inFlightTTL {
		t.Fatalf("TryBegin() TTL = %v, want %v", got, inFlightTTL)
	}

	got, found, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found {
		t.Fatal("Get() found = false, want true")
	}
	if got.State != inFlight.State {
		t.Fatalf("Get() state = %v, want %v", got.State, inFlight.State)
	}
	if got.RequestHash != inFlight.RequestHash {
		t.Fatalf("Get() request hash = %q, want %q", got.RequestHash, inFlight.RequestHash)
	}
	if !got.CreatedAt.Equal(inFlight.CreatedAt) {
		t.Fatalf("Get() created at = %v, want %v", got.CreatedAt, inFlight.CreatedAt)
	}

	ok, err = store.TryBegin(ctx, key, ports.IdempotencyRecord{
		State:       ports.IdempotencyStateInFlight,
		RequestHash: "hash-b",
	}, inFlightTTL)
	if err != nil {
		t.Fatalf("TryBegin() second call error = %v", err)
	}
	if ok {
		t.Fatal("TryBegin() second call = true, want false")
	}

	completedTTL := 5 * time.Minute
	completed := ports.IdempotencyRecord{
		State:       ports.IdempotencyStateCompleted,
		RequestHash: inFlight.RequestHash,
		Status:      http.StatusCreated,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Trace":      []string{"abc-123"},
		},
		Body:      []byte(`{"ok":true}`),
		CreatedAt: inFlight.CreatedAt,
	}
	if err := store.Save(ctx, key, completed, completedTTL); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got := mini.TTL("idem:" + key); got != completedTTL {
		t.Fatalf("Save() TTL = %v, want %v", got, completedTTL)
	}

	got, found, err = store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() after Save error = %v", err)
	}
	if !found {
		t.Fatal("Get() after Save found = false, want true")
	}
	if got.State != completed.State {
		t.Fatalf("Get() after Save state = %v, want %v", got.State, completed.State)
	}
	if got.Status != completed.Status {
		t.Fatalf("Get() after Save status = %d, want %d", got.Status, completed.Status)
	}
	if !reflect.DeepEqual(got.Header, completed.Header) {
		t.Fatalf("Get() after Save header = %#v, want %#v", got.Header, completed.Header)
	}
	if string(got.Body) != string(completed.Body) {
		t.Fatalf("Get() after Save body = %q, want %q", string(got.Body), string(completed.Body))
	}

	if err := store.Release(ctx, key); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if !mini.Exists("idem:" + key) {
		t.Fatal("Release() removed completed Redis key")
	}
	got, found, err = store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() after completed Release error = %v", err)
	}
	if !found || got.State != ports.IdempotencyStateCompleted {
		t.Fatalf("expected completed record to remain after Release, got found=%v record=%#v", found, got)
	}

	tokened := ports.IdempotencyRecord{
		State:            ports.IdempotencyStateInFlight,
		RequestHash:      "hash-tokened",
		ReservationToken: "reservation-legacy-release",
		CreatedAt:        inFlight.CreatedAt,
	}
	if err := store.Save(ctx, "legacy-release", tokened, inFlightTTL); err != nil {
		t.Fatalf("Save() tokened error = %v", err)
	}
	if err := store.Release(ctx, "legacy-release"); err != nil {
		t.Fatalf("Release() tokened error = %v", err)
	}
	if _, found, err := store.Get(ctx, "legacy-release"); err != nil {
		t.Fatalf("Get() after legacy Release error = %v", err)
	} else if found {
		t.Fatal("expected legacy Release to remove tokened in-flight Redis key")
	}
}

func TestStoreReleaseContract(t *testing.T) {
	t.Parallel()

	idempotencytest.AssertReservationReleaseContract(t, func(t testing.TB) ports.ReservationReleasableIdempotencyStore {
		t.Helper()
		mini := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
		t.Cleanup(func() {
			_ = client.Close()
		})
		return New(client, Options{KeyPrefix: "idem:"})
	})
}

func TestStoreReleaseRequiresMatchingToken(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store := New(client, Options{KeyPrefix: "idem:"})
	ctx := context.Background()
	key := "order:tokenized"
	inFlightTTL := 2 * time.Minute
	inFlight := ports.IdempotencyRecord{
		State:            ports.IdempotencyStateInFlight,
		RequestHash:      "hash-a",
		ReservationToken: "token-abc",
		CreatedAt:        time.Unix(1_700_000_000, 123_000_000).UTC(),
	}

	ok, err := store.TryBegin(ctx, key, inFlight, inFlightTTL)
	if err != nil {
		t.Fatalf("TryBegin() error = %v", err)
	}
	if !ok {
		t.Fatal("TryBegin() = false, want true")
	}

	if err := store.ReleaseReservation(ctx, key, "token-wrong"); err == nil {
		t.Fatal("Release() expected token mismatch error, got nil")
	}
	if !mini.Exists("idem:" + key) {
		t.Fatal("expected key to remain after mismatched token")
	}

	if err := store.ReleaseReservation(ctx, key, inFlight.ReservationToken); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if mini.Exists("idem:" + key) {
		t.Fatal("Release() left Redis key behind")
	}
}

func TestStoreReleaseRecoversLegacyTokenlessInflightRecord(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	events := make([]LegacyInFlightRecoveryEvent, 0, 1)
	store := New(client, Options{
		KeyPrefix: "idem:",
		OnLegacyInFlightRecovery: func(_ context.Context, event LegacyInFlightRecoveryEvent) {
			events = append(events, event)
		},
	})
	ctx := context.Background()
	key := "order:legacy-recovery"
	inFlightTTL := 2 * time.Minute
	inFlight := ports.IdempotencyRecord{
		State:       ports.IdempotencyStateInFlight,
		RequestHash: "hash-a",
		CreatedAt:   time.Unix(1_700_000_000, 123_000_000).UTC(),
	}

	err := store.Save(ctx, key, inFlight, inFlightTTL)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !mini.Exists("idem:" + key) {
		t.Fatal("expected legacy tokenless record to be saved")
	}

	if err := store.ReleaseReservation(ctx, key, ""); !errors.Is(err, ports.ErrLegacyInFlightReservationMissingToken) {
		t.Fatalf("Release() error = %v, want %v", err, ports.ErrLegacyInFlightReservationMissingToken)
	}
	if len(events) != 1 {
		t.Fatalf("expected one legacy recovery event, got %d", len(events))
	}
	if events[0].Outcome != LegacyInFlightRecoveryRecovered {
		t.Fatalf("expected outcome %q, got %q", LegacyInFlightRecoveryRecovered, events[0].Outcome)
	}
	if events[0].Key != hashTestValue(key) || events[0].KeyHash != hashTestValue(key) {
		t.Fatalf("expected hashed recovery key, got key=%q hash=%q", events[0].Key, events[0].KeyHash)
	}
	if events[0].RawKey != "" {
		t.Fatalf("raw key leaked by default: %q", events[0].RawKey)
	}
	if mini.Exists("idem:" + key) {
		t.Fatal("Release() left Redis key behind")
	}
}

func TestStoreReleaseReportsLegacyRecoveryTokenMismatch(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	events := make([]LegacyInFlightRecoveryEvent, 0, 1)
	store := New(client, Options{
		KeyPrefix: "idem:",
		OnLegacyInFlightRecovery: func(_ context.Context, event LegacyInFlightRecoveryEvent) {
			events = append(events, event)
		},
	})
	ctx := context.Background()
	key := "order:legacy-recovery-mismatch"
	inFlightTTL := 2 * time.Minute
	inFlight := ports.IdempotencyRecord{
		State:       ports.IdempotencyStateInFlight,
		RequestHash: "hash-a",
		CreatedAt:   time.Unix(1_700_000_000, 123_000_000).UTC(),
	}

	if err := store.Save(ctx, key, inFlight, inFlightTTL); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !mini.Exists("idem:" + key) {
		t.Fatal("expected legacy tokenless record to be saved")
	}

	if err := store.ReleaseReservation(ctx, key, "wrong-token"); err == nil {
		t.Fatal("Release() expected token mismatch error, got nil")
	}
	if len(events) != 1 {
		t.Fatalf("expected one legacy recovery event, got %d", len(events))
	}
	if events[0].Outcome != LegacyInFlightRecoveryTokenMismatch {
		t.Fatalf("expected outcome %q, got %q", LegacyInFlightRecoveryTokenMismatch, events[0].Outcome)
	}
	if events[0].Key == key || events[0].RawKey != "" {
		t.Fatalf("expected default recovery event to avoid raw key, got key=%q raw=%q", events[0].Key, events[0].RawKey)
	}
	if !mini.Exists("idem:" + key) {
		t.Fatal("expected legacy tokenless record to remain after mismatch")
	}
}

func TestStoreReleaseReservationDoesNotDeleteSuccessorReservation(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store := New(client, Options{KeyPrefix: "idem:"})
	ctx := context.Background()
	key := "order:successor"
	now := time.Unix(1_700_000_000, 123_000_000).UTC()
	if err := store.Save(ctx, key, ports.IdempotencyRecord{
		State:            ports.IdempotencyStateInFlight,
		RequestHash:      "hash-old",
		ReservationToken: "token-old",
		CreatedAt:        now,
	}, time.Millisecond); err != nil {
		t.Fatalf("Save() old reservation error = %v", err)
	}
	mini.FastForward(2 * time.Millisecond)
	ok, err := store.TryBegin(ctx, key, ports.IdempotencyRecord{
		State:            ports.IdempotencyStateInFlight,
		RequestHash:      "hash-new",
		ReservationToken: "token-new",
		CreatedAt:        now.Add(time.Second),
	}, time.Minute)
	if err != nil {
		t.Fatalf("TryBegin() successor error = %v", err)
	}
	if !ok {
		t.Fatal("TryBegin() successor = false, want true after expiry")
	}

	if err := store.ReleaseReservation(ctx, key, "token-old"); !errors.Is(err, ports.ErrLegacyInFlightTokenMismatch) {
		t.Fatalf("ReleaseReservation() stale token error = %v, want %v", err, ports.ErrLegacyInFlightTokenMismatch)
	}
	got, found, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() successor error = %v", err)
	}
	if !found || got.ReservationToken != "token-new" {
		t.Fatalf("successor reservation was not preserved: found=%v record=%#v", found, got)
	}
}

func TestStoreReleaseReservationRawKeyRequiresOptIn(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	events := make([]LegacyInFlightRecoveryEvent, 0, 1)
	store := New(client, Options{
		KeyPrefix:                    "idem:",
		LegacyInFlightRecoveryRawKey: true,
		OnLegacyInFlightRecovery: func(_ context.Context, event LegacyInFlightRecoveryEvent) {
			events = append(events, event)
		},
	})
	ctx := context.Background()
	key := "order:raw-key"
	if err := store.Save(ctx, key, ports.IdempotencyRecord{
		State:       ports.IdempotencyStateInFlight,
		RequestHash: "hash-a",
		CreatedAt:   time.Unix(1_700_000_000, 123_000_000).UTC(),
	}, time.Minute); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.ReleaseReservation(ctx, key, ""); !errors.Is(err, ports.ErrLegacyInFlightReservationMissingToken) {
		t.Fatalf("ReleaseReservation() error = %v, want %v", err, ports.ErrLegacyInFlightReservationMissingToken)
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

func TestStoreGetReturnsDecodeErrorForInvalidPayload(t *testing.T) {
	t.Parallel()

	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store := New(client, Options{KeyPrefix: "idem:"})
	ctx := context.Background()
	if err := mini.Set("idem:bad", "{not-json"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	_, _, err := store.Get(ctx, "bad")
	if err == nil {
		t.Fatal("Get() error = nil, want decode error")
	}
	if got := err.Error(); got != "decode idempotency record: invalid character 'n' looking for beginning of object key string" {
		t.Fatalf("Get() error = %q", got)
	}
}

func hashTestValue(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}
