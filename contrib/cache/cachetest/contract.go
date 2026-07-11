// Package cachetest provides reusable cache adapter contract tests.
package cachetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/cache"
)

// StoreFactory builds a fresh cache store for one contract test run.
type StoreFactory func(testing.TB) cache.Store

// AssertStoreContract verifies behavior shared by cache store implementations.
func AssertStoreContract(t testing.TB, newStore StoreFactory, advanceTime ...func(time.Duration)) {
	t.Helper()

	store := newStore(t)
	if store == nil {
		t.Fatal("newStore returned nil")
	}
	ctx := context.Background()

	if err := store.Set(ctx, "", []byte("value"), time.Minute); !errors.Is(err, cache.ErrInvalidKey) {
		t.Fatalf("Set(empty key) error = %v, want ErrInvalidKey", err)
	}
	if _, _, err := store.Get(ctx, " "); !errors.Is(err, cache.ErrInvalidKey) {
		t.Fatalf("Get(blank key) error = %v, want ErrInvalidKey", err)
	}
	if err := store.Delete(ctx, " "); !errors.Is(err, cache.ErrInvalidKey) {
		t.Fatalf("Delete(blank key) error = %v, want ErrInvalidKey", err)
	}

	key := "tenant-a:widget-1"
	value := []byte("cached-widget")
	if err := store.Set(ctx, key, value, time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value[0] = 'C'

	got, found, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || string(got) != "cached-widget" {
		t.Fatalf("Get() = (%q, %v), want cached-widget true", got, found)
	}
	got[0] = 'C'
	gotAgain, found, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() after caller mutation error = %v", err)
	}
	if !found || string(gotAgain) != "cached-widget" {
		t.Fatalf("Get() after caller mutation = (%q, %v), want cached-widget true", gotAgain, found)
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got, found, err := store.Get(ctx, key); err != nil {
		t.Fatalf("Get() after delete error = %v", err)
	} else if found || got != nil {
		t.Fatalf("Get() after delete = (%q, %v), want nil false", got, found)
	}

	expiringKey := "tenant-a:expiring"
	if err := store.Set(ctx, expiringKey, []byte("short"), 5*time.Millisecond); err != nil {
		t.Fatalf("Set(expiring) error = %v", err)
	}
	if len(advanceTime) > 0 && advanceTime[0] != nil {
		advanceTime[0](20 * time.Millisecond)
	} else {
		time.Sleep(20 * time.Millisecond)
	}
	if got, found, err := store.Get(ctx, expiringKey); err != nil {
		t.Fatalf("Get(expiring) error = %v", err)
	} else if found || got != nil {
		t.Fatalf("Get(expiring) = (%q, %v), want nil false", got, found)
	}
}
