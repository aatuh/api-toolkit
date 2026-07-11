# V4 Scope Cleanup Plan

Audience: maintainers and adopters migrating to the v4 major release.

The v4 branch is the completed major-version boundary. The core module is
`github.com/aatuh/api-toolkit/v4`, the extension module is
`github.com/aatuh/api-toolkit/contrib/v4`, and `go.work` joins local modules
for repository development. Published v3 artifacts remain historical release
evidence; they are not modified by this branch.

Every moved root port has an accountable owner and an executable import
migration. The source of truth is `docs/ports-v4-migration-ledger.tsv`, which
is verified against the current root package by `internal/tools/portsledger`.

## Goals

- Keep the root module focused on conventional Go JSON/HTTP API guardrails.
- Reduce compatibility-shaped root surfaces that encourage broad app
  architecture coupling.
- Keep provider SDKs, database drivers, router adapters, generated apps, and
  CLI/scaffold tooling outside the stable core module.
- Give v3 users migration notes before removing or moving stable APIs in v4.

## Keep Stable

These packages match the library-first identity and remain stable in v4:

| Package group | Accountable owner | V4 target | Replacement direction | Required migration evidence |
| --- | --- | --- | --- | --- |
| `httpx`, `fielderrors`, `binding`, `queryparams`, `upload` | `core-maintainers` | Keep stable | Retain import paths; review any narrower proposal separately. | V4 API diff confirms retention; package docs, examples, and direct tests remain current. |
| `middleware/json`, `middleware/maxbody`, `middleware/querylimits`, `middleware/secure`, `middleware/timeout`, `middleware/trace`, `middleware/deprecation` | `core-maintainers` | Keep stable | Retain import paths; keep router-neutral composition. | V4 API diff confirms retention; safety caveats and middleware tests remain current. |
| `middleware/idempotency`, `idempotent`, `webhooks` | `core-maintainers` | Keep stable after review | Retain imports only when the security-sensitive contract remains narrow. | V4 API diff, negative-path tests, fuzz evidence, and migration notes for any narrowed option. |
| `endpoints/health`, `endpoints/version`, `endpoints/docs`, `endpoints/pprof`, `endpoints/list` | `core-maintainers` | Keep stable after endpoint-scope review | Retain imports only for HTTP endpoint behavior; move app-specific integration contracts local. | V4 API diff, endpoint tests, admin/public exposure review, and import migration snippets for any moved contract. |
| `routecontracts`, `routepolicy`, `specs`, `contracttest`, `apitest` | `core-maintainers` | Keep stable after API-contract review | Retain imports while route metadata remains library-neutral. | V4 API diff, contract examples, and release review of generated-service implications. |
| `httpcache`, `negotiation`, `operations`, `securityprofile` | `core-maintainers` | Keep stable after package-level review | Retain imports only when examples and caveats remain narrow. | V4 API diff, package-level review, and migration snippets for any package that is narrowed or moved. |

## Demote Or Narrow In V4

These surfaces are stable in v3, but v4 should either demote them to
compatibility modules, narrow their public shape, or document them as advanced
instead of default adoption paths:

| Package or surface | Accountable owner | V4 target | Replacement direction | Required migration evidence |
| --- | --- | --- | --- | --- |
| `github.com/aatuh/api-toolkit/v4/ports` | `core-maintainers` | Narrowed and complete. | The root package contains only `Logger`, `Clock`, `IDGen`, `NopLogger`, and `SystemClock`. Endpoint, middleware, authorization, HTTP, and version contracts are package-local; adapter and composition contracts are in `contrib/contracts`. | [`docs/ports-v4-migration-ledger.tsv`](ports-v4-migration-ledger.tsv) is AST-verified; root, contrib, and reference-service test suites compile against the new imports. |
| `github.com/aatuh/api-toolkit/v4/compat/billing` | `core-maintainers` | Demote from root default path; keep only as a compatibility module or move to a separate billing compatibility module. | Keep billing workflows app-owned unless the hosted-checkout model is exact. | Exact package or symbol decision, compat import migration example, consumer evidence, API diff, and release note. |
| `github.com/aatuh/api-toolkit/v4/scheduler/migrations` | `core-maintainers` | Demote or move to contrib/app-owned migration orchestration. | Use app-owned migration commands or contrib migration adapters. | Package consumer inventory, import migration example, generated-service review, API diff, and release note. |
| `github.com/aatuh/api-toolkit/v4/swagstub` | `core-maintainers` | Demote or remove from root stable surface if v4 tooling no longer needs it. | Move the stub behind internal tooling or retain it only in contrib tooling. | Generated-output import audit, tooling migration example, API diff, and release note. |
| `authorization` broad domain helpers | `core-maintainers` | Narrow examples and docs before expanding. | Prefer app-owned authorization policy shapes unless helpers remain plainly reusable. | Security review, default-deny and tenant/owner regression tests, consumer inventory, and import migration snippets for any moved helper. |
| `email.Sender` | `core-maintainers` | Review for app-owned or contrib-owned ownership. | Keep provider-specific email behavior in the application or a contrib adapter. | Consumer and implementation inventory, provider-adapter boundary review, import migration example, API diff, and release note. |

## Split From Root

These areas should not force simple root users to inherit broad dependencies or
scaffold identity:

| Area | Accountable owner | V4 target | Replacement direction | Required migration evidence |
| --- | --- | --- | --- | --- |
| `middleware/auth/jwt`, JWK handling, and OAuth2 helpers | `contrib-maintainers` | Complete: moved to `contrib/middleware/auth/jwt`, `contrib/middleware/auth/shared`, and `contrib/oauth2`; see `docs/auth-dependency-split.md`. | Import the contrib packages; simple root imports remain auth-light. | old-to-new import examples, root `go.mod` without JWT/JWK requirements, package-owner rows, generated-project tests, API transition evidence, and release note. |
| CLI and generated scaffolds | `contrib-tooling-maintainers` | Keep in contrib or split to `api-toolkit-cli`; do not present the root module as scaffold-first. | Retain the contrib CLI path or migrate generated-service users to the dedicated CLI module. | CLI command migration examples, generated-project upgrade test, package-owner rows, release note, and library-first docs check. |
| Provider adapters and integrations | `contrib-maintainers` | Keep in contrib or split per provider family; use `docs/provider-adapter-split.md` as the decision record. | Retain contrib imports or introduce provider-family modules without root aliases. | Per-family consumer and dependency report, import migration examples, adapter contract and realism evidence, package-owner rows, and release note. |
| Reference service and generated application code | `contrib-examples-maintainers` | Keep outside root API classification. | Keep generated code app-owned and templates under contrib or a dedicated generator module. | Generated-project upgrade test, ownership notice, package classification review, and release note for generator changes. |
| Router-specific helpers | `contrib-maintainers` | Keep in contrib or examples unless the root interface is tiny and adapter-neutral. | Use router adapters in contrib or app-owned router glue. | Router implementation inventory, import migration example, dependency-boundary check, API diff, and release note. |

## Remove In V4 Only

Removal candidates must have a replacement, release-note entry, and migration
example before the v4 branch cuts. This table records removal gates for the
candidate rows above; it does not authorize a v3 removal.

| Candidate | Accountable owner | Replacement direction | Required migration evidence | Removal condition |
| --- | --- | --- | --- | --- |
| Deprecated APIs with `// Deprecated:` comments | `core-maintainers` | Use the exact replacement in `docs/deprecations.md`; the current root-port rows point to package-local aliases. | Source deprecation, deprecation register row, API inventory status, consumer inventory, and import migration example. | At least one minor release carried the deprecation notice unless a security fix requires faster action. |
| Compatibility-only shims that only supported v2-to-v3 migration | `core-maintainers` | Use the v3 replacement package or app-owned code. | Exact symbol or package decision, docscheck guardrail, consumer inventory, API diff, and release note. | The v4 plan lists the exact symbol or package and docscheck guards examples against reintroducing it. |
| Broad root `ports` exports without two real implementations | `core-maintainers` | Use package-local, app-owned, or contrib-owned interfaces. | Symbol-level consumer and implementation ledger, design review, import migration example, API diff, and release note. | Design review confirms the app should own the interface or that the package has no root consumers. |
| Tooling-only runtime stubs | `core-maintainers` | Use internal tooling packages or contrib CLI code. | Generated-output import audit, tooling migration example, API diff, and release note. | Generated outputs no longer need the root stub as a public import. |

## Completed V4 Evidence

- `VERSIONING.md`, package classifications, owners, API inventory, and package
  docs use the v4 module paths.
- The migration ledger lists the five retained root contracts and is checked by
  `go run ./internal/tools/portsledger -verify docs/ports-v4-migration-ledger.tsv`.
- Generated and checked-in reference services import package-local contracts or
  `github.com/aatuh/api-toolkit/contrib/v4/contracts`.
- `make release-api-check API_BASE_REF=v3.1.2` validates that the explicit
  baseline exists and records the major module-path transition instead of
  attempting an invalid same-module diff.
- Docscheck compiles the generated `saas-api` service with local root and
  contrib replacements.

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
