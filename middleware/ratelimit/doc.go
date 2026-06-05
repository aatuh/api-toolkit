// Package ratelimit provides stable rate-limit middleware over ports.RateLimiter.
//
// The middleware owns HTTP behavior while storage and quota decisions stay
// behind the RateLimiter port. Use contrib adapters for concrete stores, and
// keep dangerous local bypass configuration restricted to trusted proxies.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/ratelimit`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package ratelimit
