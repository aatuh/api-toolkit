package ratelimitredis

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/v4/middleware/ratelimit"
)

// Options configures a Redis-backed token bucket rate limiter.
type Options struct {
	Capacity   float64
	RefillRate float64
	StateTTL   time.Duration
	KeyPrefix  string
	Clock      func() time.Time
}

// Limiter implements ratelimit.Limiter using Redis.
type Limiter struct {
	client redis.UniversalClient
	opts   Options
	now    func() time.Time
}

var _ ratelimit.Limiter = (*Limiter)(nil)

// DecisionLimiter implements ratelimit.DecisionLimiter using the same Redis
// token bucket as Limiter. It is separate from Limiter so existing callers can
// retain the v4 Allow signature while middleware can opt in to authoritative
// quota headers.
type DecisionLimiter struct {
	limiter *Limiter
}

var _ ratelimit.DecisionLimiter = (*DecisionLimiter)(nil)

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

local remaining = math.floor(tokens)
local reset = now
if refill > 0 and tokens < capacity then
  reset = now + ((capacity - tokens) / refill)
end

redis.call("HMSET", KEYS[1], "tokens", tokens, "ts", ts)
if ttl > 0 then
  redis.call("EXPIRE", KEYS[1], ttl)
end

return {allowed, tostring(retry_after), tostring(remaining), tostring(reset)}
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

// NewDecisionLimiter constructs a Redis-backed limiter that provides complete
// quota metadata to ratelimit.Middleware through ratelimit.DecisionLimiter.
func NewDecisionLimiter(client redis.UniversalClient, opts Options) *DecisionLimiter {
	return &DecisionLimiter{limiter: New(client, opts)}
}

// Allow consumes a token for the key when available.
func (l *Limiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	decision, err := l.allowDecision(ctx, key)
	return decision.Allowed, decision.RetryAfter, err
}

func (l *Limiter) allowDecision(ctx context.Context, key string) (ratelimit.Decision, error) {
	if l == nil || l.client == nil {
		return ratelimit.Decision{}, fmt.Errorf("redis rate limiter not configured")
	}
	if key == "" {
		return ratelimit.Decision{Allowed: true}, nil
	}
	now := float64(l.now().UnixNano()) / float64(time.Second)
	ttlSeconds := int64(math.Ceil(l.opts.StateTTL.Seconds()))
	res, err := tokenBucketScript.Run(ctx, l.client, []string{l.key(key)}, l.opts.Capacity, l.opts.RefillRate, now, ttlSeconds).Result()
	if err != nil {
		return ratelimit.Decision{}, err
	}
	decision, err := parseDecisionResult(res)
	if err != nil {
		return ratelimit.Decision{}, err
	}
	decision.Limit = int(l.opts.Capacity)
	return decision, nil
}

// Allow returns a complete decision for use with ratelimit.Middleware.
func (l *DecisionLimiter) Allow(ctx context.Context, key string) (ratelimit.Decision, error) {
	if l == nil || l.limiter == nil {
		return ratelimit.Decision{}, fmt.Errorf("redis decision limiter not configured")
	}
	return l.limiter.allowDecision(ctx, key)
}

func (l *Limiter) key(key string) string {
	return l.opts.KeyPrefix + key
}

func parseLimiterResult(res any) (bool, float64, error) {
	values, err := limiterResultValues(res, 2)
	if err != nil {
		return false, 0, err
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
}

func parseDecisionResult(res any) (ratelimit.Decision, error) {
	values, err := limiterResultValues(res, 4)
	if err != nil {
		return ratelimit.Decision{}, err
	}
	allowed, err := toFloat(values[0])
	if err != nil {
		return ratelimit.Decision{}, err
	}
	retryAfter, err := toFloat(values[1])
	if err != nil {
		return ratelimit.Decision{}, err
	}
	remaining, err := toFloat(values[2])
	if err != nil {
		return ratelimit.Decision{}, err
	}
	reset, err := toFloat(values[3])
	if err != nil {
		return ratelimit.Decision{}, err
	}
	if remaining < 0 {
		remaining = 0
	}
	return ratelimit.Decision{
		Allowed:    allowed >= 1,
		Remaining:  int(math.Floor(remaining)),
		Reset:      time.Unix(0, int64(reset*float64(time.Second))).UTC(),
		RetryAfter: time.Duration(retryAfter * float64(time.Second)),
	}, nil
}

func limiterResultValues(res any, min int) ([]any, error) {
	switch values := res.(type) {
	case []any:
		if len(values) < min {
			return nil, fmt.Errorf("unexpected redis limiter response")
		}
		return values, nil
	case []int64:
		if len(values) < min {
			return nil, fmt.Errorf("unexpected redis limiter response")
		}
		result := make([]any, len(values))
		for i, value := range values {
			result[i] = value
		}
		return result, nil
	case []float64:
		if len(values) < min {
			return nil, fmt.Errorf("unexpected redis limiter response")
		}
		result := make([]any, len(values))
		for i, value := range values {
			result[i] = value
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unexpected redis limiter response type %T", res)
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
