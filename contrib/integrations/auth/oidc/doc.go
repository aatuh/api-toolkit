// Package oidc provides a supported-adapter integration wrapper for OIDC auth.
//
// Use NewFromEnv when application bootstrap wants environment-loaded OIDC/JWKS
// bearer-token middleware and a readiness health checker from one construction
// path. The wrapper delegates validation behavior to middleware/auth/oidc and
// keeps configuration loading in the contrib module.
//
// Treat issuer, audience, JWKS URL, and allowed algorithms as production
// security configuration. Do not use placeholder identity-provider values in
// deployed services.
package oidc
