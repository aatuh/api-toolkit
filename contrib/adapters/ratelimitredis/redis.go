// Package ratelimitredis provides Redis-backed rate limiting.
package ratelimitredis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/v2/ports"
)

// Options configures a Redis-backed token bucket rate limiter.
type Options struct {
	Capacity   float64
	RefillRate float64
	StateTTL   time.Duration
	KeyPrefix  string
	Clock      func() time.Time
}

// Limiter implements ports.RateLimiter using Redis.
type Limiter struct {
	client redis.UniversalClient
	opts   Options
	now    func() time.Time
}

var _ ports.RateLimiter = (*Limiter)(nil)

var tokenBucketScript = redis.NewScript(`
local capacity = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])

local data = redis.call("HMGET", KEYS[1], "tokens", "ts")
local tokens = tonumber(data[1])
local ts = tonumber(data[2])

if tokens == nil or ts == nil then
  tokens = capacity
  ts = now
else
  local delta = now - ts
  if delta < 0 then delta = 0 end
  tokens = math.min(capacity, tokens + (delta * refill))
  ts = now
end

local allowed = 0
local retry_after = 0
if tokens >= 1 then
  allowed = 1
  tokens = tokens - 1
else
  if refill > 0 then
    retry_after = (1 - tokens) / refill
  end
end

redis.call("HMSET", KEYS[1], "tokens", tokens, "ts", ts)
if ttl > 0 then
  redis.call("EXPIRE", KEYS[1], ttl)
end

return {allowed, retry_after}
`)

// New constructs a Redis-backed limiter.
func New(client redis.UniversalClient, opts Options) *Limiter {
	if opts.Capacity <= 0 {
		opts.Capacity = 20
	}
	if opts.RefillRate <= 0 {
		opts.RefillRate = 10
	}
	if opts.StateTTL <= 0 {
		opts.StateTTL = 10 * time.Minute
	}
	if opts.KeyPrefix == "" {
		opts.KeyPrefix = "ratelimit:"
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	return &Limiter{
		client: client,
		opts:   opts,
		now:    opts.Clock,
	}
}

// Allow consumes a token for the key when available.
func (l *Limiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	if l == nil || l.client == nil {
		return false, 0, fmt.Errorf("redis rate limiter not configured")
	}
	if key == "" {
		return true, 0, nil
	}
	now := float64(l.now().UnixNano()) / float64(time.Second)
	ttlSeconds := int64(l.opts.StateTTL.Seconds())
	res, err := tokenBucketScript.Run(ctx, l.client, []string{l.key(key)}, l.opts.Capacity, l.opts.RefillRate, now, ttlSeconds).Result()
	if err != nil {
		return false, 0, err
	}
	allowed, retryAfter, err := parseLimiterResult(res)
	if err != nil {
		return false, 0, err
	}
	if retryAfter <= 0 {
		return allowed, 0, nil
	}
	return allowed, time.Duration(retryAfter * float64(time.Second)), nil
}

func (l *Limiter) key(key string) string {
	return l.opts.KeyPrefix + key
}

func parseLimiterResult(res any) (bool, float64, error) {
	switch values := res.(type) {
	case []any:
		if len(values) < 2 {
			return false, 0, fmt.Errorf("unexpected redis limiter response")
		}
		allowed, err := toFloat(values[0])
		if err != nil {
			return false, 0, err
		}
		retryAfter, err := toFloat(values[1])
		if err != nil {
			return false, 0, err
		}
		return allowed >= 1, retryAfter, nil
	case []int64:
		if len(values) < 2 {
			return false, 0, fmt.Errorf("unexpected redis limiter response")
		}
		return values[0] >= 1, float64(values[1]), nil
	case []float64:
		if len(values) < 2 {
			return false, 0, fmt.Errorf("unexpected redis limiter response")
		}
		return values[0] >= 1, values[1], nil
	default:
		return false, 0, fmt.Errorf("unexpected redis limiter response type %T", res)
	}
}

func toFloat(v any) (float64, error) {
	switch t := v.(type) {
	case int64:
		return float64(t), nil
	case float64:
		return t, nil
	case string:
		parsed, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected redis response type %T", v)
	}
}
