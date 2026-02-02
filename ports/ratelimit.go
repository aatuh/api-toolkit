package ports

import (
	"context"
	"time"
)

// RateLimiter defines a distributed rate limiter contract.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}
