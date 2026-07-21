// Package fielderrors defines stable field-level validation error shapes.
//
// FieldErrors implements error and Provider. HTTP response helpers map provider
// messages into Problem Details extensions only when every FieldError is marked
// Public; callers must classify messages before they can reach clients.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/fielderrors`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package fielderrors
