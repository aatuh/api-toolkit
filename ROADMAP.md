# Roadmap

Audience: adopters and contributors deciding whether api-toolkit's future
direction matches their service needs.

api-toolkit is a small Go HTTP API guardrail library first. The project also
maintains optional contrib adapters and generated service scaffolds, but those
paths should not widen the stable core module or turn the toolkit into a full
application framework.

This roadmap is an intent document, not a release promise. Release scope is
still governed by `VERSIONING.md`, `docs/release-runbook.md`, package
classification, and the release notes.

## Current Focus

| Area | Direction | Done when |
| --- | --- | --- |
| Stable core quality | Keep root packages small, documented, directly tested, and compatibility-gated. | `docs/core-readiness.md` and package docs remain current for stable and compatibility-only packages. |
| Existing-service adoption | Improve examples for `net/http`, chi, and app-owned routers that keep application architecture intact. | Examples compile, have behavior tests where useful, and avoid contrib unless the adapter is the point. |
| API guardrails | Maintain request bounds, Problem Details, health/version/docs endpoints, idempotency/webhook contracts, route metadata, and security defaults. | Contracts and docs explain failure modes, opt-outs, and production caveats. |
| Contrib adapter evidence | Keep supported adapters documented, tested, and drift-reviewed without making contrib part of stable core. | Supported-adapter rows have direct tests, behavior evidence, and release-drift review. |
| Generated service evidence | Keep scaffolds app-owned and production-conscious without making generated apps part of the root API. | Generated profiles have golden OpenAPI, contract checks, and reference-service evidence. |

## Next Candidates

These are candidates for future work when issue feedback, adopter use, or
release-review evidence justifies the maintenance cost:

| Candidate | Requirement before accepting |
| --- | --- |
| More existing-router examples | The example must show a common router without introducing a root dependency. |
| More route-contract recipes | The recipe must map to existing stable packages and avoid generated-only assumptions. |
| V4 scope cleanup | The plan must preserve v3 compatibility while listing keep, demote, split, and remove candidates in `docs/v4-plan.md`. |
| CLI/scaffold split | The current decision stays in `docs/cli-scaffold-identity.md`; any future `api-toolkit-cli` split must preserve library-first root identity. |
| Adapter promotion from experimental to supported-adapter | Direct tests, behavior contracts, docs, drift coverage, owner metadata, and release-note policy must already exist. |
| Generated scaffold refinements | The generated change must remain app-owned and should not add product-domain assumptions. |
| Performance-focused hardening | Benchmarks must identify a real hot path and preserve safety behavior. |

## Open Design Review Issues

Use these issues to give focused external feedback before roadmap candidates
turn into implementation work:

- [Stable core size and package boundaries](https://github.com/aatuh/api-toolkit/issues/29)
- [Root ports shape and interface ownership](https://github.com/aatuh/api-toolkit/issues/30)
- [Generated scaffold scope and ownership](https://github.com/aatuh/api-toolkit/issues/31)
- [Contrib adapter policy and support tiers](https://github.com/aatuh/api-toolkit/issues/32)

## Non-Goals

api-toolkit will not try to become:

- a router or replacement for chi, `net/http`, Fiber, Echo, Gin, or app-owned
  routing,
- an ORM, persistence framework, migration platform, or database abstraction
  layer,
- a billing, entitlement, subscription, or payment platform,
- a universal auth, identity, session, or user-management platform,
- a Protobuf/RPC framework or replacement for Connect, gRPC, or Buf tooling,
- a streaming, SSE, WebSocket, or large-download middleware suite,
- a managed hosting, deployment, observability, or infrastructure platform,
- a cross-language SDK platform,
- a place to collect every application boundary in root `ports`,
- a stable-core wrapper around provider SDKs, database drivers, router
  adapters, generated app internals, or contrib-only dependencies.

Small app-owned helpers should stay app-owned when they are clearer than adding
a toolkit dependency. Provider-specific, database-specific, router-specific, and
business-specific code belongs in the application, contrib, generated service
output, or a future explicitly scoped module split.

## Decision Rules

New roadmap items should satisfy all relevant rules:

- Core changes must fit the stable-core charter in `docs/stable-core.md`.
- New stable APIs must pass the API review checklist and release API gate.
- New root dependencies must satisfy `docs/dependency-boundary.md` and should
  avoid provider, database, router, and generated-application dependencies.
- Contrib work must stay outside the stable core API promise unless a future
  release line adds a dedicated compatibility gate for contrib.
- Scaffold work must preserve generated-service ownership boundaries in
  `docs/scaffold-support.md`.
- Security-sensitive behavior must document failure modes and have negative-path
  tests or a clear evidence exception.

## How To Propose Changes

Open a focused issue or design note with:

- the user problem and the smallest API shape that solves it,
- why app-owned code is not enough,
- which package or module owns the behavior,
- dependency and compatibility impact,
- tests, docs, examples, and release evidence needed before merge.

Use `docs/alternatives.md` before proposing broad platform features. If an
existing tool is the better fit, the roadmap should say so rather than absorb
that tool's category.
