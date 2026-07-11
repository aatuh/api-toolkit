// Package authorization provides stable helpers for API authorization boundaries.
//
// Use Require with an Authorizer when a route needs an explicit policy decision.
// The package-local authorization and policy aliases remain exactly
// source-compatible with their root ports counterparts during v3.
// AllowlistAuthorizer is a small default-deny implementation for simple
// route/action maps, while owner, tenant, scope, and actor helpers cover common
// BOLA and multi-tenant checks.
//
// This package is part of the stable core API surface. For runnable authz and
// policy examples, see docs/cookbook.md and contrib/examples/README.md.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/authorization`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package authorization
