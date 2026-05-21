// Package oidc provides supported-adapter OIDC/JWKS bearer-token middleware.
//
// Use New with Config to validate bearer tokens against an issuer, audiences,
// allowed algorithms, and a JWKS source. The middleware stores the authenticated
// subject and optional tenant/scope claims in request context and exposes
// HealthChecker for readiness.
//
// Configuration fails closed when required identity-provider settings are
// missing or unsafe. Keep skip-header or development bypass behavior out of
// production identity paths.
package oidc
