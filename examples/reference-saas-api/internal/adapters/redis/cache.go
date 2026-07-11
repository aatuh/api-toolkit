package redis

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/cacheredis"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/idempotencyredis"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/ratelimitredis"
	"github.com/aatuh/api-toolkit/contrib/v4/cache"
	"github.com/aatuh/api-toolkit/v4/middleware/idempotency"
	"github.com/aatuh/api-toolkit/v4/middleware/ratelimit"
)

var (
	ErrRedisAddrRequired  = errors.New("REDIS_ADDR is required")
	ErrRedisClientMissing = errors.New("redis cache client is required")
)

type Cache struct {
	Store  cache.Store
	client redis.UniversalClient
}

type Idempotency struct {
	Store  idempotency.Store
	client redis.UniversalClient
}

type RateLimiter struct {
	Limiter ratelimit.Limiter
	client  redis.UniversalClient
}

func OpenCache(ctx context.Context, addr string) (*Cache, error) {
	addrs := parseRedisAddrs(addr)
	if len(addrs) == 0 {
		return nil, ErrRedisAddrRequired
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Cache{
		Store:  cacheredis.New(client, cacheredis.Options{KeyPrefix: "api:", DefaultTTL: 5 * time.Minute}),
		client: client,
	}, nil
}

func OpenRateLimiter(ctx context.Context, addr, keyPrefix string, capacity, refillRate float64) (*RateLimiter, error) {
	addrs := parseRedisAddrs(addr)
	if len(addrs) == 0 {
		return nil, ErrRedisAddrRequired
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RateLimiter{
		Limiter: ratelimitredis.New(client, ratelimitredis.Options{Capacity: capacity, RefillRate: refillRate, KeyPrefix: strings.TrimSpace(keyPrefix)}),
		client:  client,
	}, nil
}

func OpenIdempotencyStore(ctx context.Context, addr, keyPrefix string) (*Idempotency, error) {
	addrs := parseRedisAddrs(addr)
	if len(addrs) == 0 {
		return nil, ErrRedisAddrRequired
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Idempotency{
		Store:  idempotencyredis.New(client, idempotencyredis.Options{KeyPrefix: strings.TrimSpace(keyPrefix)}),
		client: client,
	}, nil
}

func (c *Cache) Check(ctx context.Context) error {
	if c == nil || c.client == nil {
		return ErrRedisClientMissing
	}
	return c.client.Ping(ctx).Err()
}

func (c *Cache) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (r *RateLimiter) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

func (i *Idempotency) Close() error {
	if i == nil || i.client == nil {
		return nil
	}
	return i.client.Close()
}

func parseRedisAddrs(addr string) []string {
	parts := strings.FieldsFunc(addr, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
