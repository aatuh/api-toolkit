# Stable Ports Boundary

Audience: maintainers and advanced API consumers reviewing the v3 stable ports
surface and the compatibility-shaped history that should not be copied into new
generic contracts.

This note explains which parts of `github.com/aatuh/api-toolkit/v3/ports` are
broadly reusable and which historical shapes were moved out of generic core
contracts during the v3 cleanup.

## What Stays In Core

- Generic application boundaries such as logging, clocks, IDs, HTTP, rate
  limiting, validation, docs, health management, and transaction/query
  lifecycles stay in core.
- These contracts are the template for new `ports` additions: narrow,
  adapter-neutral, and safe to implement across multiple backends.
- Database observability should use `DatabasePoolSnapshotProvider`,
  `SnapshotDatabasePoolStats`, or adapter `StatSnapshot()` methods.

## Compatibility-Sensitive History

These compatibility-sensitive examples are historical guardrails for new
design, not current invitations to widen generic core ports.

- Historical `ports/billing.go` hosted-checkout and invoicing contracts now live in
  `github.com/aatuh/api-toolkit/v3/compat/billing`. Keep app-owned billing
  ports when the hosted-checkout model is not an exact fit.
- Driver-shaped database counters, including `DatabasePool.Stat` and
  `DatabaseStats`, were removed from the generic pool contract. Keep
  backend-specific counters in adapters and expose plain-value snapshots to
  generic observability code.
- `ports.RateLimiter` remains for v3 source compatibility. New rate-limit
  adapter contracts should import `middleware/ratelimit.Limiter`, which is a
  package-local migration shim over the same interface.
- Legacy response helpers were removed from the stable core surface. Use
  `httpx` for JSON and Problem Details responses and package-local response
  recorders for middleware internals.

## Guidance For New Code

- Do not add provider-specific billing fields or driver-specific database
  counters to generic `ports`.
- No new `ports` export is accepted without an accepted design note proving
  adapter neutrality, at least two real implementations, and why the application
  should not own the interface. The design note must explicitly answer why the
  application should not own the interface.
- If only one package consumes an interface, define it in that package. If the
  behavior is business-specific or provider-specific, the application or contrib
  adapter should own it.
- New examples must use `compat/billing` for the existing hosted-checkout model
  or define an app-owned port for provider-neutral billing.
- New database examples must prefer `DatabasePoolSnapshotProvider`,
  `SnapshotDatabasePoolStats`, `SnapshotDatabaseStats`, or adapter
  `StatSnapshot()` methods.
- New rate-limit examples should use `middleware/ratelimit.Limiter` rather than
  importing `ports.RateLimiter` directly.
- New idempotency stores should use `middleware/idempotency.Store`,
  `ReservationReleaser`, and `ReleasableStore`.
- New authorization integrations should use `authorization.Authorizer`,
  `AuthorizerFunc`, `PolicyEngine`, `PolicyRequest`, and `PolicyDecision`.
- New health integrations should use `endpoints/health.Checker`,
  `ManagerContract`, `DetailedManager`, `CachedManager`, and `RouteRegistrar`.
- New documentation integrations should use `endpoints/docs.Provider`,
  `ManagerContract`, `HTMLModeProvider`, and `RouteRegistrar`.
- New response examples must use `httpx`; middleware that needs capture should
  keep package-local recorders.

## Governance

If the stable ports surface changes, update `VERSIONING.md`,
`docs/package-classification.tsv`, `docs/v3-compatibility-roadmap.md`,
docscheck coverage, and `docs/release-notes.md` in the same change.

`make api-additions-check` rejects every new root `ports` export unless
`docs/ports-export-exceptions.tsv` has an exact record that points to an
accepted ADR. The ADR must prove adapter neutrality, at least two real
implementations, and why the application should not own the interface.
