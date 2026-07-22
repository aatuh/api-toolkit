# Deprecation Policy

Audience: maintainers planning stable API replacements and release reviewers
checking migration evidence.

Deprecation is a compatibility promise, not a cleanup shortcut. A deprecated v3
symbol remains available until the earliest documented major version unless a
security exception requires otherwise.

## Required Format

Every deprecation entry must include:

| Field | Meaning |
| --- | --- |
| Symbol | Fully qualified package symbol, such as `example.OldSymbol`. |
| Since | Release or date when the deprecation became public. |
| Replacement | Preferred symbol, package, or app-owned pattern. |
| Removal earliest major | Earliest major version where removal may happen. |
| Migration snippet | Short before/after snippet or link to a focused guide. |
| Release note | Link or pointer to the release note that announced it. |

Source comments should use Go's `Deprecated:` convention so pkg.go.dev and
`go doc` expose the status.

## Active Deprecation Register

| Symbol | Since | Replacement | Removal earliest major | Migration snippet | Release note |
| --- | --- | --- | --- | --- | --- |
| `middleware/timeout.New` | 2026-06-07 | `middleware/timeout.NewPropagator` | v4 | `timeout.New(opts)` -> `timeout.NewPropagator(opts)` when the route only needs cooperative context deadlines. | `docs/release-notes.md` 2026-06-07 Migration |
| `middleware/timeout.HardTimeout.Middleware` | 2026-07-22 | `middleware/timeout.HardTimeout.WrapRoute` | v5 | Replace global `hard.Middleware()` wiring with `timeout.NewPropagator` globally and `hard.WrapRoute(finiteHandler, timeout.RouteCapabilityFiniteJSON)` on the one finite route that needs a hard cutoff. | `docs/release-notes.md` Unreleased API-009 |
| `middleware/timeout.HardTimeout.Handler` | 2026-07-22 | `middleware/timeout.HardTimeout.WrapRoute` | v5 | Replace generic `hard.Handler(next)` with `hard.WrapRoute(next, timeout.RouteCapabilityFiniteJSON)` after confirming the route is finite and does not need optional writer interfaces. | `docs/release-notes.md` Unreleased API-009 |
| `securityprofile.WithHardTimeout` | 2026-07-22 | `securityprofile.WithTimeout` plus a finite `RouteOverride` | v5 | Use `WithTimeout` globally; a route-specific hard cutoff requires `HardTimeout`, `HardTimeoutCapabilities`, and `timeout.RouteCapabilityFiniteJSON`. | `docs/release-notes.md` Unreleased API-009 |
| `middleware/trace.Use` | 2026-06-07 | `middleware/trace.New(opts).Middleware()` | v4 | `trace.Use(opts)` -> `mw, _ := trace.New(opts); mw.Middleware()` for explicit middleware construction. | `docs/release-notes.md` 2026-06-07 Migration |
| `ports.RateLimiter` | 2026-07-11 | `middleware/ratelimit.Limiter` | v4 | `ports.RateLimiter` -> `ratelimit.Limiter` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.IdempotencyStore` | 2026-07-11 | `middleware/idempotency.Store` | v4 | `ports.IdempotencyStore` -> `idempotency.Store` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.IdempotencyReservationReleaser` | 2026-07-11 | `middleware/idempotency.ReservationReleaser` | v4 | `ports.IdempotencyReservationReleaser` -> `idempotency.ReservationReleaser` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.ReservationReleasableIdempotencyStore` | 2026-07-11 | `middleware/idempotency.ReleasableStore` | v4 | `ports.ReservationReleasableIdempotencyStore` -> `idempotency.ReleasableStore` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.Authorizer` | 2026-07-11 | `authorization.Authorizer` | v4 | `ports.Authorizer` -> `authorization.Authorizer` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.AuthorizerFunc` | 2026-07-11 | `authorization.AuthorizerFunc` | v4 | `ports.AuthorizerFunc` -> `authorization.AuthorizerFunc` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.PolicyEngine` | 2026-07-11 | `authorization.PolicyEngine` | v4 | `ports.PolicyEngine` -> `authorization.PolicyEngine` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.PolicyRequest` | 2026-07-11 | `authorization.PolicyRequest` | v4 | `ports.PolicyRequest` -> `authorization.PolicyRequest` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.PolicyDecision` | 2026-07-11 | `authorization.PolicyDecision` | v4 | `ports.PolicyDecision` -> `authorization.PolicyDecision` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.HealthChecker` | 2026-07-11 | `endpoints/health.Checker` | v4 | `ports.HealthChecker` -> `health.Checker` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.HealthManager` | 2026-07-11 | `endpoints/health.ManagerContract` | v4 | `ports.HealthManager` -> `health.ManagerContract` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.DetailedHealthManager` | 2026-07-11 | `endpoints/health.DetailedManager` | v4 | `ports.DetailedHealthManager` -> `health.DetailedManager` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.CachedHealthManager` | 2026-07-11 | `endpoints/health.CachedManager` | v4 | `ports.CachedHealthManager` -> `health.CachedManager` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.DocsProvider` | 2026-07-11 | `endpoints/docs.Provider` | v4 | `ports.DocsProvider` -> `docs.Provider` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.DocsManager` | 2026-07-11 | `endpoints/docs.ManagerContract` | v4 | `ports.DocsManager` -> `docs.ManagerContract` | `docs/release-notes.md` 2026-07-11 Migration |
| `ports.DocsHTMLModeProvider` | 2026-07-11 | `endpoints/docs.HTMLModeProvider` | v4 | `ports.DocsHTMLModeProvider` -> `docs.HTMLModeProvider` | `docs/release-notes.md` 2026-07-11 Migration |

## Compatibility Shim Register

| Shim | Existing surface | Preferred import | Purpose | Evidence |
| --- | --- | --- | --- | --- |
| `middleware/ratelimit.Limiter` | `ports.RateLimiter` | `github.com/aatuh/api-toolkit/v4/middleware/ratelimit` | Lets v3 users move rate-limit adapter contracts to the package that consumes them before v4 shrinks broad root ports. | `middleware/ratelimit/example_test.go`, `docs/v3-compatibility-roadmap.md`, `docs/api-inventory.md` |
| `middleware/idempotency.Store` | `ports.IdempotencyStore` | `github.com/aatuh/api-toolkit/v4/middleware/idempotency` | Lets v3 users move idempotency storage contracts to the middleware package before v4. | `middleware/idempotency/interfaces_test.go`, `docs/v3-compatibility-roadmap.md` |
| `authorization.Authorizer` | `ports.Authorizer` | `github.com/aatuh/api-toolkit/v4/authorization` | Lets v3 users move authorization and policy contracts to the consuming package before v4. | `authorization/interfaces_test.go`, `docs/v3-compatibility-roadmap.md` |
| `endpoints/health.Checker` | `ports.HealthChecker` | `github.com/aatuh/api-toolkit/v4/endpoints/health` | Lets v3 users move health endpoint contracts to the consuming package before v4. | `endpoints/health/interfaces_test.go`, `docs/v3-compatibility-roadmap.md` |
| `endpoints/docs.Provider` | `ports.DocsProvider` | `github.com/aatuh/api-toolkit/v4/endpoints/docs` | Lets v3 users move documentation endpoint contracts to the consuming package before v4. | `endpoints/docs/interfaces_test.go`, `docs/v3-compatibility-roadmap.md` |

Compatibility-sensitive but not source-deprecated surfaces are tracked in
[ports-surface.md](ports-surface.md),
[v3-compatibility-roadmap.md](v3-compatibility-roadmap.md), and
[api-inventory.md](api-inventory.md). Use those documents before deciding
whether to add a `Deprecated:` source comment.

## Rules

- Do not deprecate without a replacement or explicit app-owned alternative.
- Do not remove deprecated v3 API before a major version unless a documented
  security exception requires it.
- New deprecations must update `docs/api-inventory.md`,
  `docs/release-notes.md`, and this register in the same change.
- Examples should teach the replacement, not the deprecated path.
- Compatibility-only packages may remain active without source deprecation when
  the goal is to preserve v3 users while steering new users elsewhere.
