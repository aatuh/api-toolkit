# ADR 0001: Module Boundaries

Status: Accepted

Date: 2026-06-05

## Context

`api-toolkit` has two modules:

- `github.com/aatuh/api-toolkit/v3` for stable root packages,
- `github.com/aatuh/api-toolkit/contrib/v3` for adapters, integrations,
  examples, and generator tooling.

The root module already carries JWT/JWK dependencies for stable auth middleware.
Contrib carries the heavier provider, database, Redis, OpenTelemetry, router,
validation, email, Stripe, and generator dependencies.

The audit backlog asked whether to split modules further so users of simple
HTTP helpers do not inherit unrelated dependency risk.

## Decision

Keep the current two-module layout for v3:

1. Root remains the stable API module.
2. Contrib remains the adapter, integration, example, and tooling module.
3. No new provider, database, cache, router, telemetry exporter, or generated
   application dependency may move into root.
4. New root dependencies require the review gates in
   `docs/dependency-policy.md`.
5. A v4 plan may split auth-heavy packages or scaffold/CLI tooling only after
   compatibility shims and migration evidence exist.

## Rationale

Splitting root during v3 would change import paths and create a breaking
adoption event. The current v3 promise is more valuable if the project holds the
line on future root growth, keeps contrib dependency-heavy code outside stable
core, and publishes dependency footprint evidence for reviewers.

The root JWT/JWK dependency inheritance is accepted for v3 because
`middleware/auth/jwt` is already stable. The mitigation is documentation,
footprint reporting, and a v4 cleanup plan if simple-core users need a smaller
module graph.

## Consequences

- Existing v3 users keep stable import paths.
- Simple-core adopters get an explicit minimum dependency path rather than a
  disruptive module split.
- Contrib remains intentionally outside the stable core API promise.
- Future auth-heavy, scaffold, or provider-module splits are v4 work, not v3
  patch or minor work.
