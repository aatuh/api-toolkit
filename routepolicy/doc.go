// Package routepolicy derives opt-in runtime middleware from route contracts.
//
// The package is stable core API. It connects specs.Operation metadata to
// middleware decisions without owning storage, identity providers, quota stores,
// or route registration. Use New to construct a Policy and pass it to
// routecontracts.NewRegistryWithOptions when a service wants route metadata to
// drive deprecation headers, content negotiation, auth, idempotency, or
// rate-limit middleware.
//
// The primary abstractions are Config, Policy, AuthPolicyFunc,
// IdempotencyPolicyFunc, RateLimitPolicyFunc, and ProblemCatalogPolicy.
// Application code supplies the factories that enforce auth, idempotency, and
// quota decisions. routepolicy only decides when those factories should be
// applied based on the operation contract.
//
// LintOperations provides release and CI checks for operation IDs, unique
// operation identity, security metadata, Problem Details responses,
// unsafe-write tenant/idempotency/rate-limit policies, and admin-only system
// routes.
//
// Policies fail closed by returning errors from Apply. EmitPolicyExtension
// output only when the generated OpenAPI document should expose
// x-api-toolkit-policy metadata. For examples, see docs/cookbook.md.
package routepolicy
