# Remediation Backlog

## Delivery order
1. Fix silent or misleading public-contract behavior first.
2. Remove hidden extension contracts from handler codepaths.
3. Reconcile the stable `ports` surface with the architecture promise.
4. Harden operational config edge cases and add regression coverage.

## Ticket summary
| ID | Priority | Area | Outcome |
|----|---------:|------|---------|
| [x] AUDIT-16-01 | P1 | Validation contract | `ports.Validator` becomes explicit, non-silent, and toolkit-shaped on all public codepaths. |
| [x] AUDIT-16-02 | P1 | Health/docs capabilities | Handler behavior depends only on exported, documented contracts. |
| AUDIT-16-03 | P2 | Stable ports surface | Vendor-shaped billing/database contracts are either narrowed, reclassified, or staged for versioned extraction. |
| AUDIT-16-04 | P2 | Health config | Invalid health cache/refresh settings fail closed or clamp to documented defaults. |
| AUDIT-16-05 | P2 | Contract regression suite | New tests lock the public semantics that this audit found weak. |

## [x] AUDIT-16-01: Harden the validator contract
- Problem: `contrib/adapters/validation.playgroundValidator.Validate` currently returns success for unsupported non-struct inputs, and `ValidateField` exposes a field-selection contract that is inconsistent with the JSON-tagged errors it returns.
- Scope: [contrib/adapters/validation/validation.go](/home/aatu/projects/saas/api-toolkit/contrib/adapters/validation/validation.go:115) [contrib/adapters/validation/validation_test.go](/home/aatu/projects/saas/api-toolkit/contrib/adapters/validation/validation_test.go:11) [ports/core.go](/home/aatu/projects/saas/api-toolkit/ports/core.go:26)
- Required changes: change `Validate` so unsupported targets return a toolkit-owned validation error instead of `nil`; normalize upstream invalid-invocation errors into toolkit error types; make `ValidateField` accept both Go field names and JSON field names or return a clear toolkit error for unknown fields; document the exact semantics in package docs.
- Acceptance criteria: `Validate(nil)` still returns a validation error; `Validate` on scalar, slice, and map inputs no longer returns success; `ValidateField` works for both `"Email"` and `"email"` on the current fixture shape or fails with a deterministic toolkit error; no public path leaks raw `validator.InvalidValidationError`.
- Verification: add focused unit tests for scalar input, slice input, map input, pointer-to-struct input, unknown field input, JSON-name field input, and nil context behavior if nil contexts are intended to be supported.

## [x] AUDIT-16-02: Export health/docs capability contracts
- Problem: health and docs handlers currently discover extra behavior through hidden local interfaces, so external implementations of `ports.HealthManager` and `ports.DocsManager` can satisfy the public interface while silently losing features.
- Scope: [ports/health.go](/home/aatu/projects/saas/api-toolkit/ports/health.go:37) [endpoints/health/handlers.go](/home/aatu/projects/saas/api-toolkit/endpoints/health/handlers.go:18) [ports/docs.go](/home/aatu/projects/saas/api-toolkit/ports/docs.go:32) [endpoints/docs/handlers.go](/home/aatu/projects/saas/api-toolkit/endpoints/docs/handlers.go:136)
- Required changes: introduce exported capability interfaces in `ports` for detailed-health enablement, cached-health snapshots, and docs HTML mode, or fold the methods into the existing manager interfaces if that is acceptable under the compatibility policy; update handlers to use only exported interfaces; document the extension points in package docs.
- Acceptance criteria: custom manager implementations can discover all required methods from the exported API surface alone; detailed health route registration, cached health middleware behavior, and docs CSP behavior no longer depend on package-private knowledge.
- Verification: add tests with stub managers defined outside the concrete manager types that implement only the exported contracts; ensure route registration and middleware behavior match the built-in managers.

## AUDIT-16-03: Rationalize the stable `ports` surface
- Problem: the stable core `ports` package now includes Stripe-shaped billing concepts and pgx-shaped database statistics, which weakens the “narrow, stable, dependency-inverted” story and makes future adapter diversity expensive.
- Scope: [ports/billing.go](/home/aatu/projects/saas/api-toolkit/ports/billing.go:9) [ports/database.go](/home/aatu/projects/saas/api-toolkit/ports/database.go:8) [contrib/adapters/stripe/billing.go](/home/aatu/projects/saas/api-toolkit/contrib/adapters/stripe/billing.go:336) [VERSIONING.md](/home/aatu/projects/saas/api-toolkit/VERSIONING.md:1) [docs/architecture.md](/home/aatu/projects/saas/api-toolkit/docs/architecture.md:1)
- Required changes: decide which contracts truly belong in stable core; for `v2`, either reclassify the most provider-shaped surfaces as compatibility-sensitive/experimental or introduce narrower sub-interfaces without breaking existing callers; for the longer-term roadmap, design a `v3` extraction path that moves provider-shaped billing flows and driver-shaped database stats out of the core `ports` package.
- Acceptance criteria: README, architecture docs, and versioning policy all describe the same stability boundary; the stable core surface no longer claims to be generic where it is actually provider-specific; any deferred breaking cleanup is captured as an explicit versioned migration plan.
- Verification: update API-diff/stability checks if package classifications change; add a short design note describing what stays in core, what moves, and what deprecation path applies.

## AUDIT-16-04: Validate health refresh/cache configuration
- Problem: `endpoints/health.LoadConfig` accepts zero or negative durations and can produce ineffective cache settings without surfacing a configuration error.
- Scope: [endpoints/health/config.go](/home/aatu/projects/saas/api-toolkit/endpoints/health/config.go:20) [endpoints/health/health.go](/home/aatu/projects/saas/api-toolkit/endpoints/health/health.go:316)
- Required changes: define explicit semantics for `HEALTH_REFRESH_INTERVAL` and `HEALTH_CACHE_DURATION`; reject or clamp non-positive values; ensure defaults remain sane when one variable is missing or invalid.
- Acceptance criteria: `0s` and negative values do not silently create inert cache behavior; the returned config is always internally consistent; package docs state the exact fallback behavior.
- Verification: add unit tests for default values, explicit positive overrides, zero refresh, zero cache, negative refresh, negative cache, and invalid duration strings.

## AUDIT-16-05: Add contract-focused regression tests
- Problem: the codebase has strong implementation tests, but the findings in this audit all sit at public-contract edges that are not currently locked down.
- Scope: [contrib/adapters/validation/validation_test.go](/home/aatu/projects/saas/api-toolkit/contrib/adapters/validation/validation_test.go:1) [endpoints/health/handlers_test.go](/home/aatu/projects/saas/api-toolkit/endpoints/health/handlers_test.go:1) [endpoints/docs/handlers_test.go](/home/aatu/projects/saas/api-toolkit/endpoints/docs/handlers_test.go:1)
- Required changes: add regression tests for unsupported validator targets, field-name resolution semantics, custom health manager capability discovery, custom docs manager HTML-mode behavior, and health config edge cases; keep these tests in the packages that own the public contract rather than only on concrete manager types.
- Acceptance criteria: each finding in `audit.md` is backed by at least one failing-then-passing automated test; future refactors of handlers and adapters will fail fast if they reintroduce hidden capability coupling or silent validation no-ops.
- Verification: run `make test`, both race suites, `make lint`, and `go vet ./...` after the new tests land.

## Definition of done
- All P1 tickets are completed before more surface area is added to `ports` or the validation adapter.
- P2 ticket `AUDIT-16-03` has either an implemented `v2` reduction or an approved, documented `v3` migration plan; leaving it implicit is not acceptable.
- The repo-wide quality gates used in this audit remain green after remediation: `make test`, root and contrib `go test ./... -race -count=1`, `make lint`, `make gosec`, `make vuln`, and `go vet ./...` in both modules.
