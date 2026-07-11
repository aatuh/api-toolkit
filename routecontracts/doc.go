// Package routecontracts registers HTTP routes and OpenAPI operations together.
//
// It is intentionally router-neutral and uses only the common method
// registration shape already provided by ports.HTTPRouter. PATCH support is
// detected with a local optional interface so stable router interfaces are not
// widened.
//
// Registered routes automatically attach bounded routepolicy observability
// labels to the request context so outer metrics and request logging middleware
// can record policy shape without seeing raw scopes, tenants, or policy names.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/routecontracts`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package routecontracts
