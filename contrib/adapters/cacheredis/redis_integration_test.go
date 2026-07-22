//go:build redis

package cacheredis

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testredis"
)

func TestStoreUsesRealRedisTTLIsolationCancellationAndReconnect(t *testing.T) {
	h := testredis.New(t)
	ctx := context.Background()
	store := New(h.Client, Options{KeyPrefix: h.Key("cache:"), DefaultTTL: 75 * time.Millisecond})
	if err := store.Set(ctx, "empty", []byte{}, 0); err != nil {
		t.Fatalf("Set(empty) error = %v", err)
	}
	if value, found, err := store.Get(ctx, "empty"); err != nil || !found || len(value) != 0 {
		t.Fatalf("Get(empty) = (%q, %t, %v)", value, found, err)
	}
	large := bytes.Repeat([]byte("x"), 1<<20)
	if err := store.Set(ctx, "large", large, time.Second); err != nil {
		t.Fatalf("Set(large) error = %v", err)
	}
	if value, found, err := store.Get(ctx, "large"); err != nil || !found || !bytes.Equal(value, large) {
		t.Fatalf("Get(large) = found %t, length %d, error %v", found, len(value), err)
	}
	if err := store.Set(ctx, "expires", []byte("value"), 0); err != nil {
		t.Fatalf("Set(expires) error = %v", err)
	}
	eventuallyRedis(t, 3*time.Second, func() bool {
		_, found, err := store.Get(ctx, "expires")
		return err == nil && !found
	})
	other := New(h.Client, Options{KeyPrefix: h.Key("other:"), DefaultTTL: time.Second})
	if _, found, err := other.Get(ctx, "large"); err != nil || found {
		t.Fatalf("other-prefix Get() = found %t, error %v", found, err)
	}
	if err := store.Set(h.CanceledContext(t), "canceled", []byte("value"), time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("Set() canceled context error = %v", err)
	}

	reconnecting, err := h.NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = reconnecting.Close() })
	reconnectingStore := New(reconnecting, Options{KeyPrefix: h.Key("reconnect:"), DefaultTTL: time.Second})
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

func eventuallyRedis(t *testing.T, timeout time.Duration, condition func() bool) {
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
			t.Fatal("timed out waiting for real Redis condition")
		}
	}
}
