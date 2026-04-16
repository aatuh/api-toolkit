package idempotencyredis

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

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
	if mini.Exists("idem:" + key) {
		t.Fatal("Release() left Redis key behind")
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
