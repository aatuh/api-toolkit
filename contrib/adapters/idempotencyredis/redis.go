// Package idempotencyredis provides Redis-backed idempotency storage.
package idempotencyredis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/v2/ports"
)

// Options configures Redis-backed idempotency storage.
type Options struct {
	KeyPrefix string
}

// Store implements ports.IdempotencyStore using Redis.
type Store struct {
	client redis.UniversalClient
	prefix string
}

var _ ports.IdempotencyStore = (*Store)(nil)

// New constructs a Redis-backed idempotency store.
func New(client redis.UniversalClient, opts Options) *Store {
	prefix := opts.KeyPrefix
	if prefix == "" {
		prefix = "idempotency:"
	}
	return &Store{
		client: client,
		prefix: prefix,
	}
}

// Get returns an idempotency record if present.
func (s *Store) Get(ctx context.Context, key string) (ports.IdempotencyRecord, bool, error) {
	if s == nil || s.client == nil {
		return ports.IdempotencyRecord{}, false, nil
	}
	val, err := s.client.Get(ctx, s.key(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return ports.IdempotencyRecord{}, false, nil
	}
	if err != nil {
		return ports.IdempotencyRecord{}, false, err
	}
	var record ports.IdempotencyRecord
	if err := json.Unmarshal(val, &record); err != nil {
		return ports.IdempotencyRecord{}, false, fmt.Errorf("decode idempotency record: %w", err)
	}
	return record, true, nil
}

// TryBegin reserves a key for an in-flight request.
func (s *Store) TryBegin(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) (bool, error) {
	if s == nil || s.client == nil {
		return false, nil
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("encode idempotency record: %w", err)
	}
	ok, err := s.client.SetNX(ctx, s.key(key), payload, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// Save persists a completed idempotency record.
func (s *Store) Save(ctx context.Context, key string, record ports.IdempotencyRecord, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return nil
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode idempotency record: %w", err)
	}
	return s.client.Set(ctx, s.key(key), payload, ttl).Err()
}

func (s *Store) key(key string) string {
	return s.prefix + key
}
