package app

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/cache"
)

const webhookEventTypesCacheKey = "catalog:webhook-events"

type CacheService struct {
	store cache.Store
}

func NewCacheService(store cache.Store) *CacheService {
	if store == nil {
		store = NewMemoryCacheStore()
	}
	return &CacheService{store: store}
}

func (s *CacheService) WebhookEventTypes(ctx context.Context, loader func() []string) ([]string, bool, error) {
	var cached []string
	ok, err := s.GetJSON(ctx, webhookEventTypesCacheKey, &cached)
	if err != nil {
		return nil, false, err
	}
	if ok {
		return append([]string(nil), cached...), true, nil
	}
	if loader == nil {
		return nil, false, ErrValidation
	}
	loaded := append([]string(nil), loader()...)
	if err := s.SetJSON(ctx, webhookEventTypesCacheKey, loaded, time.Minute); err != nil {
		return nil, false, err
	}
	return loaded, false, nil
}

func (s *CacheService) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.store == nil || dst == nil {
		return false, ErrValidation
	}
	value, ok, err := s.store.Get(ctx, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(value, dst); err != nil {
		_ = s.store.Delete(ctx, key)
		return false, nil
	}
	return true, nil
}

func (s *CacheService) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.store == nil {
		return ErrValidation
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.store.Set(ctx, key, encoded, ttl)
}

func (s *CacheService) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.store == nil {
		return ErrValidation
	}
	return s.store.Delete(ctx, key)
}

func (s *CacheService) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.store == nil {
		return ErrValidation
	}
	const key = "health:cache"
	if err := s.store.Set(ctx, key, []byte("ok"), time.Second); err != nil {
		return err
	}
	value, ok, err := s.store.Get(ctx, key)
	if err != nil {
		return err
	}
	if !ok || string(value) != "ok" {
		return ErrValidation
	}
	return s.store.Delete(ctx, key)
}

type MemoryCacheStore struct {
	mu     sync.Mutex
	now    func() time.Time
	values map[string]memoryCacheEntry
}

type memoryCacheEntry struct {
	value     []byte
	expiresAt time.Time
}

func NewMemoryCacheStore() *MemoryCacheStore {
	return &MemoryCacheStore{now: time.Now, values: map[string]memoryCacheEntry{}}
}

func (s *MemoryCacheStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if err := cache.ValidateKey(key); err != nil {
		return nil, false, err
	}
	if s == nil {
		return nil, false, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.values[key]
	if !ok {
		return nil, false, nil
	}
	if !entry.expiresAt.IsZero() && !s.now().Before(entry.expiresAt) {
		delete(s.values, key)
		return nil, false, nil
	}
	return cache.CloneBytes(entry.value), true, nil
}

func (s *MemoryCacheStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cache.ValidateKey(key); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = s.now().Add(ttl)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = memoryCacheEntry{value: cache.CloneBytes(value), expiresAt: expiresAt}
	return nil
}

func (s *MemoryCacheStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cache.ValidateKey(key); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
	return nil
}

var _ cache.Store = (*MemoryCacheStore)(nil)
