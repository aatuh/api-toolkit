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
// Metadata helpers such as WithAuth, WithDeprecated, WithSunset,
// WithTenantRequired, WithIdempotencyRequired, WithRateLimit, WithAdminPolicy,
// and WithProblemResponses keep route contracts code-first without relying on
// raw OpenAPI extension maps at call sites.
// Typed readers such as AuthPolicyFromOperation, TenantPolicyFromOperation,
// IdempotencyPolicyFromOperation, RateLimitPolicyFromOperation,
// DeprecationPolicyFromOperation, AdminPolicyFromOperation, and
// ProblemResponseStatuses let contract tooling inspect the same metadata
// without ad hoc map parsing.
//
// LintOperations provides release and CI checks for operation IDs, unique
// operation identity, security metadata, Problem Details responses,
// unsafe-write tenant/idempotency/rate-limit/request/response policies, and
// admin-only system routes.
//
// ObservabilityLabelsFromOperation and ObservabilityMiddleware expose only
// bounded route-policy labels for request logs and metrics. They intentionally
// collapse raw scopes, tenant sources, rate-limit names, and admin policy names
// into small enums.
//
// Policies fail closed by returning errors from Apply. EmitPolicyExtension
// output only when the generated OpenAPI document should expose
// x-api-toolkit-policy metadata. For examples, see docs/cookbook.md.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/routepolicy`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package routepolicy
