// Package querylimits provides stable core query-parameter guardrail middleware.
//
// Use New with Options when handlers need bounded query parameter counts, key
// lengths, value lengths, and pagination limit values before application
// parsing. Middleware and Handler expose the same guardrail for ports.Middleware
// wiring or direct net/http usage.
//
// Invalid query shapes fail with Problem Details 400 responses and do not reach
// the wrapped handler. The package does not parse business filters or database
// queries; use queryparams for typed collection-query parsing after these size
// and limit checks pass.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/querylimits`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package querylimits
