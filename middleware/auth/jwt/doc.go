// Package jwt provides stable JWT authentication middleware.
//
// NewMiddleware validates bearer tokens with configured issuer, audience, JWKS,
// algorithm allowlist, clock skew, and optional claim requirements. Subject
// helpers store and retrieve authenticated identity from request context.
//
// Dangerous bypass and skip-header behavior must be configured explicitly and
// should be restricted to trusted proxies. The contrib JWT integration adds
// environment loading around this stable middleware.
package jwt
