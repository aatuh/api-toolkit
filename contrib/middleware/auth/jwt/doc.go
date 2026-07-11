// Package jwt provides JWT authentication middleware for the optional contrib module.
//
// NewMiddleware validates bearer tokens with configured issuer, audience, JWKS,
// algorithm allowlist, clock skew, and optional claim requirements. Subject
// helpers store and retrieve authenticated identity from request context.
//
// Dangerous bypass and skip-header behavior must be configured explicitly and
// should be restricted to trusted proxies. The JWT integration adds environment
// loading around this middleware.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/jwt`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Contrib API; it is not part of the stable root API promise.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package jwt
