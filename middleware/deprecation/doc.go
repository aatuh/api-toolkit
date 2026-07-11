// Package deprecation emits runtime Deprecation, Sunset, and Link headers.
//
// The middleware follows the current HTTP header contracts for deprecation and
// sunset signaling while leaving response status and body behavior unchanged.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/middleware/deprecation`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package deprecation
