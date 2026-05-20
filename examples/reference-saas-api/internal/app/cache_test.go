package app

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCacheStoreHonorsTTLAndClonesValues(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	store := NewMemoryCacheStore()
	store.now = func() time.Time { return now }
	value := []byte("alpha")
	if err := store.Set(ctx, "widgets:1", value, time.Second); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value[0] = 'x'
	got, ok, err := store.Get(ctx, "widgets:1")
	if err != nil || !ok || string(got) != "alpha" {
		t.Fatalf("Get() = %q ok=%v err=%v", got, ok, err)
	}
	got[0] = 'z'
	again, ok, err := store.Get(ctx, "widgets:1")
	if err != nil || !ok || string(again) != "alpha" {
		t.Fatalf("Get() after mutation = %q ok=%v err=%v", again, ok, err)
	}
	now = now.Add(time.Second)
	if _, ok, err := store.Get(ctx, "widgets:1"); err != nil || ok {
		t.Fatalf("expired Get() ok=%v err=%v", ok, err)
	}
}

func TestCacheServiceCachesWebhookEventCatalog(t *testing.T) {
	ctx := context.Background()
	service := NewCacheService(NewMemoryCacheStore())
	calls := 0
	events, hit, err := service.WebhookEventTypes(ctx, func() []string {
		calls++
		return []string{"widget.created"}
	})
	if err != nil || hit || calls != 1 || len(events) != 1 {
		t.Fatalf("first WebhookEventTypes() events=%v hit=%v calls=%d err=%v", events, hit, calls, err)
	}
	events, hit, err = service.WebhookEventTypes(ctx, func() []string {
		calls++
		return []string{"widget.deleted"}
	})
	if err != nil || !hit || calls != 1 || events[0] != "widget.created" {
		t.Fatalf("cached WebhookEventTypes() events=%v hit=%v calls=%d err=%v", events, hit, calls, err)
	}
	if err := service.Check(ctx); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
}
