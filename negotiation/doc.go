// Package negotiation provides stable core HTTP content negotiation helpers.
//
// Use New when a service wants one middleware to enforce both Accept and
// Content-Type policy, or RequireAccept and RequireContentType for focused
// checks. ParseAccept, Negotiate, and ContentTypeAllowed expose the same matching
// behavior for handlers and tests.
//
// The package writes Problem Details 406 and 415 responses and does not inspect
// request bodies, route metadata, or OpenAPI documents. Keep media-type policy
// explicit in application routing or route contracts.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/negotiation`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package negotiation
