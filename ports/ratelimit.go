package ports

import (
	"context"
	"time"
)

// RateLimiter defines a distributed rate limiter contract.
//
// Deprecated: Use middleware/ratelimit.Limiter. This interface remains
// available for v3 compatibility and may be removed in v4.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}
