// Package ratelimit provides stable rate-limit middleware over ports.RateLimiter.
//
// The middleware owns HTTP behavior while storage and quota decisions stay
// behind the RateLimiter port. Use contrib adapters for concrete stores, and
// keep dangerous local bypass configuration restricted to trusted proxies.
package ratelimit
