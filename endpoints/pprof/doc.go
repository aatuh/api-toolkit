// Package pprof registers Go pprof handlers on an HTTP router.
//
// RegisterRoutes wires profiling handlers only; it does not add authentication,
// authorization, or network restrictions. Prefer RegisterAdminRoutes for new
// admin mounts so an explicit wrapper is required at construction time.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/endpoints/pprof`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package pprof
