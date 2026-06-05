// Package apikey provides stable API key authentication middleware.
//
// The middleware extracts credentials from Authorization: ApiKey <secret> or
// X-API-Key, delegates verification to an application-owned Verifier, stores the
// authenticated principal in request context, and can enforce required scopes.
// Storage, hashing, rotation, and last-used tracking intentionally belong to the
// Verifier implementation instead of the core middleware.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/auth/apikey`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package apikey
