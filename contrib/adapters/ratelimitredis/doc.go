// Package ratelimitredis provides the supported-adapter Redis rate limiter.
//
// Use New with a redis.UniversalClient and Options to implement
// ratelimit.Limiter with token-bucket state shared across API replicas. Use
// NewDecisionLimiter when middleware clients need the authoritative limit,
// remaining, reset, and retry metadata for standard response headers. Options
// configure capacity, refill rate, state TTL, key prefix, and clock source for
// deterministic tests.
//
// Use service-specific key prefixes on shared Redis deployments. The adapter
// returns allow/deny decisions and reset timing; route policy and client-facing
// Problem Details remain owned by middleware or application code.
package ratelimitredis
