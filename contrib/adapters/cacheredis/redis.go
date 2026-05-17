package cacheredis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/contrib/v3/cache"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	"github.com/aatuh/api-toolkit/v3/ports"
)

// Options configures a Redis-backed cache store.
type Options struct {
	KeyPrefix  string
	DefaultTTL time.Duration
}

// Store implements cache.Store using Redis.
type Store struct {
	client     redis.UniversalClient
	keyPrefix  string
	defaultTTL time.Duration
}

var _ cache.Store = (*Store)(nil)

// New constructs a Redis-backed cache store.
func New(client redis.UniversalClient, opts Options) *Store {
	if opts.KeyPrefix == "" {
		opts.KeyPrefix = "cache:"
	}
	return &Store{
		client:     client,
		keyPrefix:  opts.KeyPrefix,
		defaultTTL: opts.DefaultTTL,
	}
}

// Get returns a cached value if present.
func (s *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := cache.ValidateKey(key); err != nil {
		return nil, false, err
	}
	if s == nil || s.client == nil {
		return nil, false, fmt.Errorf("redis cache store not configured")
	}
	value, err := s.client.Get(ctx, s.key(key)).Bytes()
	if err == nil {
		return cache.CloneBytes(value), true, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	return nil, false, err
}

// Set writes a cached value. When ttl is zero or negative, DefaultTTL is used.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := cache.ValidateKey(key); err != nil {
		return err
	}
	if s == nil || s.client == nil {
		return fmt.Errorf("redis cache store not configured")
	}
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	return s.client.Set(ctx, s.key(key), cache.CloneBytes(value), ttl).Err()
}

// Delete removes a cached value.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := cache.ValidateKey(key); err != nil {
		return err
	}
	if s == nil || s.client == nil {
		return fmt.Errorf("redis cache store not configured")
	}
	return s.client.Del(ctx, s.key(key)).Err()
}

// HealthChecker returns a Redis cache dependency health checker.
func HealthChecker(client redis.UniversalClient) ports.HealthChecker {
	return health.NewCustomChecker(
		"redis-cache",
		func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
			if client == nil {
				return ports.HealthStatusUnhealthy, "redis cache client not configured", nil
			}
			if err := client.Ping(ctx).Err(); err != nil {
				return ports.HealthStatusUnhealthy, fmt.Sprintf("redis cache ping failed: %v", err), nil
			}
			return ports.HealthStatusHealthy, "redis cache healthy", nil
		},
	)
}

// HealthChecker returns a Redis cache dependency health checker for this store.
func (s *Store) HealthChecker() ports.HealthChecker {
	if s == nil {
		return HealthChecker(nil)
	}
	return HealthChecker(s.client)
}

func (s *Store) key(key string) string {
	return s.keyPrefix + key
}
