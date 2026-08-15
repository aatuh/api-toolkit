# Stable Core Charter

Audience: API consumers and maintainers deciding which parts of `api-toolkit`
are the recommended small-core dependency surface.

The primary identity is **small Go HTTP API building blocks**. The project can
generate service scaffolds and maintain contrib adapters, but those are optional
paths. The core module should stay useful to a normal `net/http` or chi service
without requiring the user to adopt the scaffold, provider adapters, or broad
platform contracts.

## Stable Core Packages

`VERSIONING.md` is the source of truth for the v3 stable API compatibility
promise. Stable means exported identifiers are protected by SemVer and the
release API gate. It does not mean every package is equally recommended as a
first dependency for every user.

Recommended stable-core packages should meet all of these conditions:

| Requirement | Meaning |
| --- | --- |
| Broad HTTP/API usefulness | A conventional Go HTTP API can use the package without adopting the scaffold. |
| Toolkit-owned behavior | The behavior belongs to API guardrails, not to a provider, database, or product domain. |
| Low dependency risk | The package does not drag provider SDKs, database drivers, or framework dependencies into root. |
| Package-local clarity | A reader can understand the package without understanding the full repository. |
| Evidence-backed stability | The package has docs, direct tests, examples or an explicit exception, a benchmark decision, and compatibility notes. |

The starting path for new adopters is:

- `httpx` and `fielderrors` for JSON responses and Problem Details,
- `binding`, `queryparams`, `upload`, and request-bound middleware,
- `middleware/json`, `middleware/maxbody`, `middleware/querylimits`,
  `middleware/secure`, `middleware/timeout`, `middleware/trace`, and
  `middleware/deprecation`,
- `endpoints/health`, `endpoints/version`, and `endpoints/docs`,
- `routecontracts`, `routepolicy`, and `specs`,
- `idempotent` and `webhooks` when the app needs those HTTP contracts.

## Compatibility-Only Packages

Compatibility-only packages remain part of the v3 compatibility commitment, but
they are not examples for new generic API design.

Use this category for migration-shaped or historically broad surfaces such as
hosted-checkout billing compatibility, root `ports` shapes that should become
package-local in v4, and scaffold-facing support APIs that are preserved for v3
consumers.

Rules for compatibility-only surfaces:

- Keep exported API compatible until a major version or documented security
  exception.
- Prefer app-owned or package-local interfaces for new code.
- Do not present the surface as the default recommended stable abstraction.
- Document the replacement path in `docs/ports-surface.md`,
  `docs/v3-compatibility-roadmap.md`, or package docs before future removal.

## Ports Policy

Root `ports` is a v3 compatibility surface, not a place to collect every future
application boundary.

No new `ports` export should be added unless a design note proves:

- adapter neutrality,
- at least two real implementations,
- at least two stable root consumers or one unavoidable cross-package contract,
- why the application should not own the interface,
- why a package-local interface is not sufficient.

When only one package consumes an abstraction, define the interface in that
package. When the interface is business-specific, provider-specific, or
deployment-specific, the application or contrib adapter should own it.

## Evidence Required Per Stable Package

Each stable package should have:

- package docs that explain purpose, install/import, errors, concurrency,
  stability, and when not to use it,
- direct tests for owned behavior,
- examples that compile under `go test`, or a documented exception for packages
  where examples would be misleading,
- a benchmark decision: benchmark exists, benchmark is not relevant, or
  benchmark is deferred with a reason,
- compatibility notes in `VERSIONING.md`, package docs, or release notes when
  behavior is sharp or migration-sensitive.

The machine-readable status lives in `docs/package-classification.tsv`; the
rendered companion is `docs/package-classification.md`.

The per-package readiness companion is `docs/core-readiness.md`. It records the
docs, examples, tests, fuzz, benchmark, compatibility, security review, and
production caveat status for each stable or compatibility-only root package.

## Proposed V5 Focus

The current v4 package list above is a compatibility and maintenance statement,
not a recommendation that every stable package belongs in a future minimal
core. The proposed twelve-package v5 adoption surface, its reuse evidence,
non-core destinations, and release preconditions are defined once in
[v5-core-surface.md](v5-core-surface.md). It is a planning record until a v5
release is published; do not present it as current v4 support policy.
