// Package oauth2 provides provider-neutral OAuth2 bearer-token helpers.
//
// The package is stable core API. It models validated token claims, scope
// checks, bearer-token extraction, OpenAPI security-scheme registration, and
// mapping into authorization.Actor and authorization.Scope. It deliberately does
// not fetch JWKS documents, cache provider keys, implement issuer-specific
// validation, or adapt to any single identity provider.
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
package oauth2
