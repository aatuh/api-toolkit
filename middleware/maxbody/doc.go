// Package maxbody provides stable request-body size limiting middleware.
//
// Use New with an explicit byte limit before handlers read request bodies,
// especially upload, webhook, and JSON write endpoints. Oversized requests fail
// at the edge instead of allowing unbounded memory or disk pressure.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/maxbody`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package maxbody
