# ADR 0001: Module Boundaries

Status: Accepted

Date: 2026-06-05

## Context

`api-toolkit` has two modules:

- `github.com/aatuh/api-toolkit/v4` for stable root packages,
- `github.com/aatuh/api-toolkit/contrib/v4` for adapters, integrations,
  examples, and generator tooling.

V3 root carried JWT/JWK dependencies for stable auth middleware. V4 moves the
JWT/JWK and OAuth2 packages into contrib, which carries provider, database,
Redis, OpenTelemetry, router, validation, email, Stripe, and generator
dependencies.

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
5. V4 keeps auth-heavy packages and scaffold/CLI tooling in contrib with
   migration evidence and no root compatibility aliases.

## Rationale

Splitting root during v3 would change import paths and create a breaking
adoption event. The current v3 promise is more valuable if the project holds the
line on future root growth, keeps contrib dependency-heavy code outside stable
core, and publishes dependency footprint evidence for reviewers.

V3 accepted root JWT/JWK dependency inheritance because `middleware/auth/jwt`
was stable. V4 removes that inheritance by moving JWT/JWK and OAuth2 packages
to contrib.

## Consequences

- Existing v3 users keep stable import paths.
- Simple-core adopters get an explicit minimum dependency path rather than a
  disruptive module split.
- Contrib remains intentionally outside the stable core API promise.
- Future auth-heavy, scaffold, or provider-module splits are v4 work, not v3
  patch or minor work.
