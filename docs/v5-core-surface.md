# Focused V5 Core Surface

Audience: API consumers and maintainers planning a future v5 adoption path.

Status: proposed v5 architecture, not a released module or a change to the
current v4 compatibility promise. `VERSIONING.md` remains the source of truth
for released-version support. This document is the canonical decision record
for the focused v5 core; it deliberately does not promote all v4-stable
packages as the default first adoption.

## Adoption Story

V5 is intended for teams building conventional JSON/HTTP APIs with `net/http`,
chi, or an app-owned router. Start with bounded input, predictable validation,
safe response writing, and a small set of HTTP middleware. Keep providers,
persistence, routers, generated applications, and product-specific contracts
outside the root module.

The core contains twelve primary packages. A team need not import all twelve:
the initial choice is normally `httpx`, `fielderrors`, `binding`, and only the
middleware that owns a concrete route concern.

| Primary package | Strong standardization reason | Credible reuse cases |
| --- | --- | --- |
| `httpx` | Safe JSON and Problem Details response behavior is shared by ordinary HTTP services. | JSON API handlers; package-level error writers. |
| `fielderrors` | Stable field/code/message errors avoid application-specific validation leakage. | JSON request validation; query and multipart validation. |
| `binding` | Bounded decoding and safe validation mapping are a common API boundary. | JSON request bodies; query/path inputs. |
| `negotiation` | Content negotiation is transport-neutral and reusable without a router framework. | Versioned media types; multi-representation endpoints. |
| `queryparams` | Sort, filter, and list parameter parsing prevents repeated inconsistent implementations. | Collection endpoints; export/search endpoints. |
| `upload` | Multipart limits and validation own a common high-risk HTTP boundary. | File upload endpoints; form-based import endpoints. |
| `middleware/json` | JSON content-type and decoder guardrails are independently composable. | JSON-only APIs; mixed routes that explicitly opt in. |
| `middleware/maxbody` | Finite-body limits are a baseline DoS guardrail. | JSON writes; multipart endpoints with a bounded route limit. |
| `middleware/querylimits` | Query cardinality and size limits are an independent request-boundary control. | Search/list endpoints; public filter APIs. |
| `middleware/secure` | Conservative HTTP security headers are reusable without provider coupling. | Browser-adjacent JSON APIs; administrative API routes. |
| `routecontracts` | Route and OpenAPI registration rules are toolkit-owned and router-neutral. | Contract-tested services; generated OpenAPI documentation. |
| `endpoints/health` | Public liveness/readiness and explicit operator detail are common operational contracts. | Container orchestration probes; application dependency readiness. |

`docs/core-package-guide.md` remains the detailed v4 package selector until a
v5 module is released. The table above is the v5 first-adoption recommendation.

## Boundary Decisions

The following v4 surfaces are not default v5 core packages. This is a
classification decision, not an implementation deletion; `ARC-006` owns actual
v5 removal and `ARC-007` owns automated migration.

| V4 surface family | V5 destination | Reason |
| --- | --- | --- |
| `compat/billing`, `scheduler/migrations`, `swagstub`, and broad `ports` shapes | Compatibility package or application-owned contract | They preserve historical compatibility, not a new generic API design. |
| Auth, authorization, tenant, webhook, idempotency, rate-limit, trace, timeout, deprecation, recovery, and security-profile middleware | Optional module or application composition | They need product policy, durable stores, trusted-proxy posture, identity policy, or route capability evidence. |
| `endpoints/docs`, `endpoints/list`, `endpoints/pprof`, `endpoints/version`, `specs`, `routepolicy`, `operations`, `httpcache` | Optional module or app-owned integration | They are useful but not necessary for the smallest universal HTTP guardrail story. |
| `apiclient`, `apitest`, `contracttest`, `compatkit` | Separate test/client module | They are consumer or test tooling rather than runtime core. |
| `email`, `scheduler`, provider adapters, router adapters, telemetry, CLI, and generated service code | Contrib, independent module, or application code | They have provider, operational, release-cadence, or product ownership. |

Each destination must be made concrete before a v5 release. Existing v4 users
retain their documented compatibility promise until a released migration path
supersedes it.

## Core Contract Rules

The v5 core has one policy source for each cross-cutting concern:

| Concern | Canonical policy |
| --- | --- |
| Root dependency boundary | [Dependency policy](dependency-policy.md) |
| New public API and non-goals | [API review checklist](api-review-checklist.md) and [stable core charter](stable-core.md) |
| Interface ownership | [Interface ownership](interface-ownership.md) |
| Context and cancellation | [Context and cancellation](context-cancellation.md) |
| Error behavior and safe HTTP mapping | [Error taxonomy](errors.md) |
| Zero values and options | [Options struct audit](options-structs.md) |
| Compatibility and deprecation | [Versioning](../VERSIONING.md) and [deprecations](deprecations.md) |

No new root package, export, or promotion can enter this proposed set while the
temporary v5 stable-core freeze is active. A future exception needs the public
design evidence required by the API review checklist and an update to this
decision record.

## Non-goals

V5 core is not a router, persistence framework, provider SDK collection,
authentication platform, workflow engine, streaming middleware suite, CLI, or
generated application framework. Those concerns can be supported outside the
root core when their ownership, compatibility, dependency, and release evidence
justify it.

## Release Preconditions

Before this proposal becomes a released v5 surface:

1. Every package that leaves root has an implemented module or application-owned
   migration destination.
2. `ARC-003` through `ARC-008` record module boundaries, removals, migration,
   and module-specific compatibility gates.
3. External review and adopter evidence required by `EXT-006` are attached.
4. `VERSIONING.md`, the API reference, and the README identify the released
   version rather than presenting this proposal as current.
