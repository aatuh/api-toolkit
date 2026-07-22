// Package ratelimit provides stable rate-limit middleware over Limiter and
// DecisionLimiter.
//
// The middleware owns HTTP behavior while storage and quota decisions stay
// behind limiter contracts. Limiter is the v3-compatible migration contract;
// prefer the package-local name in new application code before v4 shrinks broad
// root ports. Use DecisionLimiter when a distributed adapter is the source of
// truth for limit, remaining, reset, and retry header metadata. Configure only
// one limiter contract at a time.
//
// The in-memory implementation is per-process and is not an exact shared quota
// across replicas. It performs bounded incremental LRU expiry on request paths
// (64 expired buckets per cleanup pass by default), never starts a background
// goroutine, and does not scan the entire bucket map. Use a supported adapter
// such as ratelimitredis for distributed enforcement.
//
// Keys must be bounded and privacy-safe. Empty or whitespace-only keys become
// the shared anonymous bucket. The default key uses the peer address unless a
// trusted ClientIPResolver is configured; configure trusted proxies explicitly
// before accepting forwarded client identity. Dangerous bypass headers are
// disabled by default and require explicit opt-in plus a trusted proxy.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/middleware/ratelimit`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package ratelimit
