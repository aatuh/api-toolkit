// Package tenant provides stable tenant scoping middleware.
//
// Use the middleware to place a tenant identifier in request context before
// authorization, idempotency, repositories, or audit logging need tenant-aware
// behavior. Invalid or missing tenant data should fail at the application edge
// according to the service's routing policy. Set Options.RequireAllSources when
// request-carried tenant identifiers must match an authenticated tenant scope
// or route parameter before the handler runs.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/auth/tenant`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package tenant
