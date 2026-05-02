// Package authorization provides stable helpers for API authorization boundaries.
//
// Use Require with a ports.Authorizer when a route needs an explicit policy
// decision. AllowlistAuthorizer is a small default-deny implementation for
// simple route/action maps, while owner, tenant, scope, and actor helpers cover
// common BOLA and multi-tenant checks.
//
// This package is part of the stable core API surface. For runnable authz and
// policy examples, see docs/cookbook.md and contrib/examples/README.md.
package authorization
