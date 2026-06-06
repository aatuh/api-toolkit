# V4 Scope Cleanup Plan

Audience: maintainers and advanced adopters reviewing what a future v4 major
release should keep stable, narrow, split, or remove.

This plan is not a v3 change. No breaking v3 change is allowed without a major
version, release notes, API-diff evidence, migration guidance, and accepted
design-review feedback. The current v3 stable surface remains governed by
`VERSIONING.md`, `docs/package-classification.tsv`, and the release API gate.

## Goals

- Keep the root module focused on conventional Go JSON/HTTP API guardrails.
- Reduce compatibility-shaped root surfaces that encourage broad app
  architecture coupling.
- Keep provider SDKs, database drivers, router adapters, generated apps, and
  CLI/scaffold tooling outside the stable core module.
- Give v3 users migration notes before removing or moving stable APIs in v4.

## Keep Stable

These packages match the library-first identity and should remain stable in v4
unless design review finds a narrower replacement:

| Package group | V4 target | Reason |
| --- | --- | --- |
| `httpx`, `fielderrors`, `binding`, `queryparams`, `upload` | Keep stable | Core request/response and validation guardrails for ordinary JSON/HTTP APIs. |
| `middleware/json`, `middleware/maxbody`, `middleware/querylimits`, `middleware/secure`, `middleware/timeout`, `middleware/trace`, `middleware/deprecation` | Keep stable | Small, router-neutral HTTP middleware with direct tests and production caveats. |
| `middleware/idempotency`, `idempotent`, `webhooks` | Keep stable after review | API contract packages with security-sensitive behavior; keep if v4 docs and negative-path tests remain strong. |
| `endpoints/health`, `endpoints/version`, `endpoints/docs`, `endpoints/pprof`, `endpoints/list` | Keep stable after endpoint-scope review | Useful HTTP endpoints and helpers, with admin/public caveats kept explicit. |
| `routecontracts`, `routepolicy`, `specs`, `contracttest`, `apitest` | Keep stable after API-contract review | Contract-test and route metadata surfaces are central to the guardrail value proposition. |
| `httpcache`, `negotiation`, `operations`, `securityprofile` | Keep stable after package-level review | HTTP support packages that can stay root-owned if examples and caveats remain narrow. |

## Demote Or Narrow In V4

These surfaces are stable in v3, but v4 should either demote them to
compatibility modules, narrow their public shape, or document them as advanced
instead of default adoption paths:

| Package or surface | V4 target | Required migration note |
| --- | --- | --- |
| `github.com/aatuh/api-toolkit/v3/ports` | Narrow root `ports`; move broad database/router/config abstractions to package-local, contrib-owned, or app-owned interfaces where practical. | Explain replacements in `docs/ports-surface.md` and keep only narrow adapter-neutral contracts. |
| `github.com/aatuh/api-toolkit/v3/compat/billing` | Demote from root default path; keep only as a compatibility module or move to a separate billing compatibility module. | Tell users to keep billing workflows app-owned unless the hosted-checkout model is exact. |
| `github.com/aatuh/api-toolkit/v3/scheduler/migrations` | Demote or move to contrib/app-owned migration orchestration. | Point users to app-owned migration commands or contrib migration adapters. |
| `github.com/aatuh/api-toolkit/v3/swagstub` | Demote or remove from root stable surface if v4 tooling no longer needs it. | Point generated tooling to its replacement or keep it internal to tooling. |
| `authorization` broad domain helpers | Narrow examples and docs before expanding. | Prefer app-owned authorization policy shapes unless helpers remain plainly reusable. |
| `email.Sender` | Review for app-owned or contrib-owned ownership. | Keep provider-specific email behavior out of root. |

## Split From Root

These areas should not force simple root users to inherit broad dependencies or
scaffold identity:

| Area | V4 target | Reason |
| --- | --- | --- |
| `middleware/auth/jwt`, JWK handling, and OAuth2 helpers | Split to an auth module or keep behind an explicitly imported package boundary that does not affect simple middleware adopters; use `docs/auth-dependency-split.md` as the decision record. | Users of `httpx`, `binding`, or `middleware/maxbody` should not inherit JWT/JWK dependency weight. |
| CLI and generated scaffolds | Keep in contrib or split to `api-toolkit-cli`; do not present the root module as scaffold-first. | The project remains library-first even when generator tooling is maintained. |
| Provider adapters and integrations | Keep in contrib or split per provider family; use `docs/provider-adapter-split.md` as the decision record. | Postgres, Redis, Stripe, Resend, Clerk, OpenTelemetry, CORS, and router adapters stay outside stable core. |
| Reference service and generated application code | Keep outside root API classification. | Generated apps are adoption evidence and app-owned templates, not root package contracts. |
| Router-specific helpers | Keep in contrib or examples unless the root interface is tiny and adapter-neutral. | chi, Fiber, Echo, Gin, and other routers should remain optional choices. |

## Remove In V4 Only

Removal candidates must have a replacement, release-note entry, and migration
example before the v4 branch cuts:

| Candidate | Replacement path | Removal condition |
| --- | --- | --- |
| Deprecated APIs with `// Deprecated:` comments | Documented replacement in package docs or release notes. | At least one minor release carried the deprecation notice unless a security fix requires faster action. |
| Compatibility-only shims that only supported v2-to-v3 migration | Use the v3 replacement package or app-owned code. | The v4 plan lists the exact symbol or package and docscheck guards examples against reintroducing it. |
| Broad root `ports` exports without two real implementations | Package-local, app-owned, or contrib-owned interfaces. | Design review confirms the app should own the interface or that the package has no root consumers. |
| Tooling-only runtime stubs | Internal tooling packages or contrib CLI code. | Generated outputs no longer need the root stub as a public import. |

## Required V4 Evidence

Before any v4 cleanup lands:

- Open or update design-review issues for stable core size, root `ports`, CLI
  and scaffold identity, auth dependency split, and contrib policy.
- Update `VERSIONING.md`, `docs/package-classification.tsv`,
  `docs/package-owners.tsv`, `docs/api-inventory.md`, and `docs/release-notes.md`.
- Add migration snippets for every moved or removed public import path.
- Keep `make release-api-check` covering the v3 baseline until the v4 branch
  intentionally changes the stable surface.
- Add docscheck guardrails so public examples use v4 replacement APIs.

## Not In Scope For V3

Do not use this plan to break v3 users. In v3:

- compatibility-only packages remain importable,
- stable root packages stay under the current SemVer promise,
- contrib stays outside the stable core API promise,
- package classification remains the source of truth for public status,
- migration work should be additive: docs, shims, examples, and warnings.

Related planning issues:

- Issue #29: stable core size and package boundaries.
- Issue #30: root ports shape and interface ownership.
- Issue #31: generated scaffold scope and ownership.
- Issue #32: contrib adapter policy and support tiers.
