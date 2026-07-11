// Package health registers stable liveness, readiness, and detailed health endpoints.
//
// HTTP contract:
//   - Liveness and readiness should reflect configured checker state rather
//     than silently succeeding on empty probe sets.
//   - Detailed health output is intended for explicitly enabled operational
//     use and may expose dependency-level status details. Prefer
//     RegisterPublicRoutesTo for public probes and RegisterAdminDetailedHealthRoute
//     for admin mounts so an explicit wrapper is required at construction time.
//     Treat policy-free RegisterRoutes, RegisterRoutesTo, RegisterCustomRoutes,
//     and RegisterCustomRoutesTo detailed-health mounts as compatibility
//     convenience behavior that should be wrapped by callers before public use.
//   - Custom managers can opt into detailed route exposure and cached snapshot
//     middleware behavior by implementing DetailedManager and CachedManager in
//     addition to ManagerContract. These package-local aliases remain exactly
//     source-compatible with their root ports counterparts during v3.
//   - When caching is enabled, cached checker results may be reused across
//     liveness, readiness, and detailed responses until CacheDuration expires.
//   - LoadConfig falls back to a 30-second refresh interval and a cache
//     duration of twice that refresh interval when env values are missing,
//     invalid, zero, or negative.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/endpoints/health`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package health
