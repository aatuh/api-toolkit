// Package jwt provides stable JWT authentication middleware.
//
// NewMiddleware validates bearer tokens with configured issuer, audience, JWKS,
// algorithm allowlist, clock skew, and optional claim requirements. Subject
// helpers store and retrieve authenticated identity from request context.
//
// Dangerous bypass and skip-header behavior must be configured explicitly and
// should be restricted to trusted proxies. The contrib JWT integration adds
// environment loading around this stable middleware.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/auth/jwt`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package jwt
