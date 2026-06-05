// Package recover provides panic recovery utilities for HTTP handlers.
//
// Contract:
//   - if the response is still uncommitted, a recovered panic becomes a 500
//     Problem Details response
//   - if headers or body bytes have already been committed, the middleware logs
//     the panic and aborts the request instead of preserving a misleading
//     partial success response
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/httpx/recover`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package recover
