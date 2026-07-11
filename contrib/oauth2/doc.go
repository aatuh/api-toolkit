// Package oauth2 provides provider-neutral OAuth2 bearer-token helpers.
//
// It models validated token claims, scope checks, bearer-token extraction,
// OpenAPI security-scheme registration, and mapping into authorization.Actor
// and authorization.Scope. It deliberately does not fetch JWKS documents,
// cache provider keys, implement issuer-specific validation, or adapt to any
// single identity provider.
//
// Application code supplies a Validator or ValidatorFunc that verifies a bearer
// token and returns TokenClaims. Use RequireScopes for route-level scope checks,
// ScopeSet for normalized scope lookup, SecurityScheme for OpenAPI metadata, and
// RegisterSecurityScheme to attach that metadata to a specs.Registry.
//
// Treat JWKSConfig as configuration data for app-owned validators. Validate
// issuers, audiences, expiry, not-before, clock skew, and tenant mapping in the
// validator before constructing authorization context. For examples, see
// docs/cookbook.md.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/contrib/v4/oauth2`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Contrib API; it is not part of the stable root API promise.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package oauth2
