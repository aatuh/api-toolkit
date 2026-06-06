# Documentation

Audience: readers who need the fastest path to the right api-toolkit document
without scanning the root README.

## New users and application developers

| Document | Audience | Purpose |
| --- | --- | --- |
| [Library-first path](library-first.md) | Existing-service users | Add the smallest useful root package set to a `net/http`, chi, or app-owned router service. |
| [Minimal core path](minimal-core.md) | Existing-service users | Use only `httpx`, `binding`, `middleware/maxbody`, and `middleware/timeout` without contrib or generators. |
| [Core package decision guide](core-package-guide.md) | Adopters and reviewers | Choose packages by use case, when-not-to-use guidance, stability tier, dependency note, and example link. |
| [Scaffold-first path](scaffold-first.md) | New service teams | Generate app-owned service code and understand what the toolkit owns versus what the generated app owns. |
| [Contrib adapter path](contrib-adapters.md) | Adapter adopters | Decide when to add supported contrib adapters, integrations, examples, or generator tooling. |
| [Stable core charter](stable-core.md) | New users and maintainers | Decide which root packages are the recommended small-core dependency surface and what evidence stable packages need. |
| [Core readiness matrix](core-readiness.md) | API consumers and release reviewers | Review docs, examples, tests, fuzz, benchmark, compatibility, security review, and production caveats for each stable package. |
| [Alternatives](alternatives.md) | Evaluators | Decide when to use `api-toolkit` instead of `net/http`, chi, oapi-codegen, Goa, Connect, or app-owned helpers. |
| [Getting started](getting-started.md) | Scaffold users | Generate, run, and verify the production-oriented app-owned service scaffold. |
| [Full service scaffold](full-service-scaffold.md) | Application teams | Understand the `saas-api-full` production foundation, support tier, and integration-test policy. |
| [Reference service](reference-service.md) | Maintainers and release reviewers | Verify the checked-in `saas-api-full` adoption proof and know which evidence is local, Docker-backed, or deployment-owned. |
| [Production readiness](production-readiness.md) | Technical leads and platform owners | Decide which surfaces are production-ready, supported-adapter, experimental, caveated, or part of the adapter maturity review. |
| [V3 migration guide](migration/v3.md) | Application teams upgrading dependencies | Upgrade root, contrib, and generated-service adoption paths within the v3 line. |
| [Troubleshooting](troubleshooting.md) | Application developers and maintainers | Diagnose Go version, contrib tier, timeout buffering, health, idempotency, auth, and generated-service issues. |
| [Test coverage evidence](test-coverage.md) | Maintainers and release reviewers | Read the coverage gate outputs, package-level floor summary, and release-evidence relationship. |
| [Benchmark baselines](performance.md) | Maintainers and release reviewers | Run and interpret package-level benchmark baselines before performance-sensitive changes or releases. |
| [Coverage hardening backlog](coverage-hardening-backlog.md) | Maintainers | Track behavior-test prerequisites before raising high-risk package coverage floors. |
| [Cookbook](cookbook.md) | Application developers | Complete common API tasks with commands, requests, expected responses, and caveats. |
| [Examples catalog](../contrib/examples/README.md) | Developers copying runnable patterns | Find each contrib example, its command, endpoint, expected result, required env, and safety note. |
| [Architecture](architecture.md) | Developers and maintainers | Understand the hexagonal boundary between stable core ports and contrib adapters. |

The contrib CLI can scaffold the fuller reusable service baseline:

```sh
go run github.com/aatuh/api-toolkit/contrib/v3/cmd/api-toolkit@latest new service \
  --module example.com/my-api \
  --profile saas-api \
  --auth api-key \
  --dir ./my-api
```

Use `--auth jwt` or `--auth clerk` when the generated service should validate
bearer tokens via JWKS instead of local API keys. Bearer scaffolds require the
matching issuer, audience, and JWKS URL environment variables, extract tenant
scope from validated token claims, and keep the same tenant mismatch,
idempotency, OpenAPI, and admin-route defaults.

Generated services wire the default router to the contrib Prometheus recorder,
so protected `/metrics` includes bounded HTTP request counters and histograms
using method, route pattern, and status labels.

Generated services also expose `/version` with build metadata. The generated
Makefile `build` target stamps the binary with `VERSION`, `BUILD_COMMIT`, and
`BUILD_DATE`; the Dockerfile accepts matching build args and uses
`dev`/`unknown` defaults for local builds.

Use `--profile dev-api --auth dev-headers` only for local development services
that need debug-header authentication. The generated service requires explicit
dev-bypass environment variables, trusts only configured loopback proxies by
default, uses separate debug tenant and scope headers, and refuses to start with
dev-header auth when `ENV=production`.

The `saas-api-full` profile keeps the lean `saas-api` default intact and starts
the heavier Postgres + Redis production foundation described in
[full-service-scaffold.md](full-service-scaffold.md). The full profile is wired
through `bootstrap.NewAPIService`, exposes `/livez` separately from `/readyz`,
keeps detailed health/metrics/pprof on admin routes, and enables runtime
OpenAPI request validation by default.

Reference-service evidence starts at [reference-service.md](reference-service.md)
and then follows the app-owned docs under `examples/reference-saas-api`,
including its README, deployment starter docs, observability runbook, and
provider workflow runbook.

The same CLI can review OpenAPI artifacts before release. `contracts lint`
checks operation IDs, non-public security requirements, unsafe-write tenant,
idempotency, rate-limit metadata, request body metadata, documented 2xx success
responses, Problem Details responses, and protected operator paths. `contracts
diff` allows additive operations and fails closed on removed operations, changed
operation IDs, removed documented parameters, added required parameters,
removed documented responses, request-body tightening or content removal,
response content removal, changed operation or inherited global security
requirements, component and inline schema removals, obvious schema
type/required/property/enum narrowing, or drift in tenant, idempotency,
rate-limit, admin, and deprecation route policy metadata:

```sh
api-toolkit contracts lint --openapi ./openapi.json
api-toolkit contracts lint --openapi ./openapi.json --public-path /status --admin-path /internal/debug
api-toolkit contracts diff --base ./openapi.previous.json --head ./openapi.json
api-toolkit clients go --openapi ./openapi.json --out ./client --package apiclient
api-toolkit clients go --openapi ./openapi.json --out ./client --package apiclient --style typed
api-toolkit version
api-toolkit version --json
```

`clients go` emits a stdlib-only Go client package for the supported OpenAPI
subset: JSON request bodies, path/query/header options, API-key auth, bearer
auth, and Problem Details error decoding. The default `raw` style preserves the
original operation helpers; `--style typed` also generates component schema
structs, typed request/response methods, and raw method escape hatches.
The contract and client commands accept OpenAPI 3.1 schema `type` arrays that
include `null` and schema-level `examples`, normalizing them to the toolkit's
compatibility model before validation.

`api-toolkit version` prints the tool version, Go runtime, main module, core
module version, contrib module version, and optional build commit/date fields.
Use `api-toolkit version --json` for machine-readable release evidence that
identifies the installed generator and contract tool.

## Security, operations, and runtime behavior

| Document | Audience | Purpose |
| --- | --- | --- |
| [Security posture](security.md) | Developers and operators | Configure secure defaults, dangerous bypasses, trusted proxies, health detail, and docs surfaces. |
| [Security threat model](threat-model.md) | Maintainers, application teams, and security reviewers | Review protected assets, assumptions, threats, mitigations, and verification evidence for security-sensitive surfaces. |
| [Auth production guide](auth.md) | Developers and operators | Configure API-key, JWT, tenant, role, JWK rotation, clock skew, failure modes, and auth tests. |
| [Idempotency production guide](idempotency.md) | Developers and operators | Configure storage, locking, TTL, replay semantics, request hashes, tenant scoping, conflicts, and Redis/Postgres ownership. |
| [Health and admin operations](operations.md) | Operators and developers | Split public probes from detailed health, metrics, pprof, admin auth, network policy, and fail-closed checks. |
| [OpenAPI contract workflow](openapi-workflow.md) | Maintainers and application teams | Run route metadata, golden diff, contract tests, generated docs, validation, and drift handling. |
| [Runtime configuration](configuration.md) | Operators and developers | Review required production env vars, defaults, unsafe dev defaults, and startup validation. |
| [Observability](observability.md) | Operators and developers | Keep metrics, logs, traces, correlation IDs, and dashboards useful and redaction-safe. |
| [Scaffold support matrix](scaffold-support.md) | Generated service teams | Understand what generated code is supported, app-owned, fragile on regeneration, and migration-owned. |
| [Adapter maturity matrix](adapter-maturity.md) | Contrib adopters | Review supported/tested/experimental posture for Postgres, Redis, Stripe, Resend, Clerk, OpenTelemetry, CORS, validation, and related adapters. |
| [Safe defaults audit](safe-defaults.md) | Developers and reviewers | Check fail-open and fail-closed behavior for root and contrib middleware before broad rollout. |
| [Middleware safety matrix](middleware-safety.md) | Developers and reviewers | Decide which middleware is safe globally, route-specific, forbidden for streaming, or requires opt-outs. |
| [Input-size threat review](input-size-threat-review.md) | Developers, reviewers, and operators | Review header, body, JSON, query, multipart, replay-capture, and timeout-capture size limits before changing route contracts. |
| [Negative-path test matrix](negative-path-test-matrix.tsv) | Maintainers and release reviewers | Verify stable-package tests for malformed input, missing headers, bad content types, invalid auth, invalid tenant, oversized bodies, and invalid query limits. |
| [Testing policy](testing.md) | Maintainers and release reviewers | Keep tests deterministic with fake clocks, injected sleep, bounded retries, and documented deadlock guards. |
| [Security policy](../SECURITY.md) | Security reporters and release consumers | Report vulnerabilities and understand supported release security handling. |
| [Code of conduct](../CODE_OF_CONDUCT.md) | Contributors and maintainers | Set expectations for respectful project participation and conduct reporting. |
| [Panic policy](../PANIC_POLICY.md) | Maintainers and API designers | Decide when panics are allowed and how HTTP recovery behaves. |
| [Metrics](metrics.md) | Operators and developers | Use low-cardinality HTTP metric names and labels. |
| [Support policy](support-policy.md) | Adopters and maintainers | Understand the supported Go line, platform gate, and generated-service ownership limits. |
| [Dependency boundary](dependency-boundary.md) | Maintainers | Keep root stable code free of contrib adapter dependencies. |
| [Dependency policy](dependency-policy.md) | Maintainers | Review allowed dependency classes, banned patterns, update SLA, and security-sensitive review gates. |
| [License policy](license-policy.md) | Maintainers | Review allowed dependency licenses, dependency-review enforcement, and exception handling. |
| [Dependency risk](dependency-risk.md) | Release reviewers and security maintainers | Review imported-but-not-called vulnerability disposition and ownership. |
| [Dependency footprint](dependency-footprint.md) | Adopters and release reviewers | Run and interpret root/contrib dependency footprint and base-ref diff reports. |

## Stability, compatibility, and package docs

| Document | Audience | Purpose |
| --- | --- | --- |
| [Versioning](../VERSIONING.md) | API consumers and maintainers | Define the stable core API surface and contrib compatibility policy. |
| [Public API inventory](api-inventory.md) | API consumers and maintainers | Review generated exported symbols grouped by package, stability tier, added version, and deprecation status. |
| [API reference index](api-reference.md) | API consumers | Jump from each stable or compatibility-only root package to pkg.go.dev and its compile-checked example. |
| [API review checklist](api-review-checklist.md) | Maintainers | Review naming, zero values, context, cancellation, errors, concurrency, options, return types, and interface necessity. |
| [API addition example exceptions](api-addition-exceptions.tsv) | Maintainers | Record exact symbol exceptions when a new stable exported identifier has a doc comment and release note but a compile-checked example would mislead. |
| [Deprecation policy](deprecations.md) | Maintainers and release reviewers | Track deprecation format, replacements, removal horizon, migration snippets, and release-note requirements. |
| [Interface ownership](interface-ownership.md) | Maintainers | Document whether exported interfaces are user-implemented, adapter-owned, test-only, or compatibility-sensitive. |
| [Options struct audit](options-structs.md) | Maintainers | Review defaults, validation behavior, zero-value behavior, and example evidence for stable exported options structs. |
| [Global state audit](global-state-audit.md) | Maintainers | Review package-level globals in stable packages and the limited mutable-state exceptions. |
| [Context and cancellation](context-cancellation.md) | Maintainers and adopters | Apply context propagation and bounded cleanup rules across HTTP, auth, idempotency, scheduler, and client APIs. |
| [Error taxonomy](errors.md) | Maintainers and API consumers | Match sentinel, typed, field, wrapped, configuration, and Problem Details errors safely. |
| [Concurrency safety](concurrency.md) | API consumers and maintainers | Decide which values are immutable, request-scoped, synchronized, or implementation-owned. |
| [Resource lifecycle](resource-lifecycle.md) | Maintainers and adopters | Track ownership for close, shutdown, timers, goroutines, stores, adapters, and generated service resources. |
| [Ports surface](ports-surface.md) | Maintainers and advanced API consumers | Identify compatibility-sensitive port history and preferred replacements. |
| [V3 compatibility record](v3-compatibility-roadmap.md) | Maintainers | Track completed v3 cleanup decisions and remaining compatibility-sensitive guardrails. |
| [Package doc standard](package-doc-standard.md) | Maintainers | Apply the minimum package-doc template and see the placeholder inventory remediated in this pass. |
| [Package classification guide](package-classification.md) | Maintainers and adopters | Read the rendered status glossary before using the TSV source of truth. |
| [Core readiness matrix](core-readiness.md) | API consumers and release reviewers | Review stable package readiness by docs, examples, tests, fuzz, benchmark, compatibility, security review, and production caveat. |
| [Module-boundary ADR](adr/0001-module-boundaries.md) | Maintainers | Record the v3 decision to keep root and contrib modules while deferring deeper splits to v4 planning. |
| `docs/package-classification.tsv` | Maintainers and automation | Machine-readable API and test-status classification for every package. |
| `docs/package-owners.tsv` | Maintainers and automation | Machine-readable maintainer owner, test owner, stability tier, and release-blocker status for every package. |
| `docs/supported-adapter-contracts.tsv` | Maintainers and automation | Machine-readable behavior contracts and evidence paths for supported contrib adapters. |
| `docs/supported-adapter-test-realism.tsv` | Maintainers and automation | Machine-readable default and scheduled/manual test-realism evidence for each supported contrib adapter. |

## Release and evidence

| Document | Audience | Purpose |
| --- | --- | --- |
| [Audit and scratch archive policy](audits.md) | Maintainers and release reviewers | Keep local `.audits` and `.trash` scratch material out of tracked release evidence. |
| [OpenSSF Best Practices gap review](openssf-best-practices.md) | Maintainers and release reviewers | Track Best Practices badge readiness, unclaimed status, and remaining gaps before publishing a badge. |
| [Release runbook](release-runbook.md) | Release operators | Command source of truth for local checks, release evidence, artifact verification, and baseline policy. |
| [Release review checklist](release-review.md) | Release reviewers | Short path through summary fields, manifests, dirty-tree decisions, artifacts, and release notes. |
| [Governance](governance.md) | Maintainers | Branch protection, CODEOWNERS, tag protection, required checks, and release approval expectations. |
| [Release notes](release-notes.md) | Release consumers and maintainers | Dated behavior changes, upgrade notes, and package-tied contrib drift acknowledgements. |
| [Release manifests](release-manifests.md) | Release reviewers and maintainers | Human guide for package classification, contrib drift, contrib dispositions, and vulnerability dispositions. |
| `docs/contrib-api-drift-packages.txt` | Maintainers and automation | Selected contrib packages reviewed by drift checks; supported-adapter incompatible drift is gate-enforced. |
| `docs/supported-adapter-contracts.tsv` | Maintainers and automation | Required supported-adapter behavior contracts with direct-test and release-drift evidence. |
| `docs/supported-adapter-test-realism.tsv` | Maintainers and automation | Required supported-adapter realism rows that distinguish direct-unit, fake DB, miniredis, hermetic fixture, and scheduled/manual real-service evidence. |
| `docs/contrib-api-drift-dispositions.tsv` | Release reviewers and automation | Owner, status, review date, expiry, and acknowledgement for current contrib drift. |
| `docs/vulnerability-dispositions.tsv` | Release reviewers and automation | Owner, review, expiry, and upgrade trigger rows for imported-only vulnerability IDs when present. |
| `release-check-summary.json` | Release reviewers | Generated local release evidence summary; only clean publication evidence is publishable. |

## Documentation quality workflow

Use the narrowest check that matches the change:

| Change type | Preferred command | Notes |
| --- | --- | --- |
| Documentation-only edits | `GOTOOLCHAIN=local make docs-check` | Runs documentation contracts, getting-started build extraction, API/docs policy checks, and release evidence parser contracts. |
| Architecture or dependency-boundary edits | `GOTOOLCHAIN=local make dependency-boundary-check` | Runs the stable-core import boundary check before the broader docs gate. |
| V3 cleanup readiness | `GOTOOLCHAIN=local make v3-readiness-check` | Runs focused compatibility-sensitive surface guardrails for major-version cleanup planning and release-note requirements. |
| Docs plus ordinary code changes | `GOTOOLCHAIN=local make fast-check` | Runs `docs-check` and unit tests without rewriting files. |
| Reference service coverage | `GOTOOLCHAIN=local make reference-service-coverage` | Records non-Docker generated-service coverage under `.ci-result/coverage/` without folding app-owned code into toolkit coverage thresholds. |
| Timeout determinism | `GOTOOLCHAIN=local make timeout-determinism-check` | Repeats the hard-timeout late-write test under normal and race runs, then runs the root timeout/idempotency/rate-limit/scheduler race subset. |
| Reviewer or audit pass | `GOTOOLCHAIN=local make audit-check` | Non-mutating reviewer gate with lint, vuln, gosec, build smoke, GitHub Actions pin audit, docs contracts, tests, race, and fuzz smoke. |
| Generated files, examples, scripts, package docs, or repo-wide contracts | `GOTOOLCHAIN=local make finalize` when practical | Installs tools and may rewrite Go formatting and module files through `fmt` and `tidy`; avoid it in shared dirty worktrees unless that mutation is intended. |

Do not treat `make finalize` as release evidence. Release publication evidence is
owned by [release-runbook.md](release-runbook.md).

If the local Go version is not Go 1.25.x, `GOTOOLCHAIN=local` failures are
expected. Install the supported toolchain or use the repository CI image before
running root and contrib gates.

## Canonical high-centrality paths

These literal paths are kept here so docs index coverage checks can detect when
important public docs disappear from navigation:

`README.md`, `docs/library-first.md`, `docs/minimal-core.md`,
`docs/core-package-guide.md`, `docs/scaffold-first.md`,
`docs/contrib-adapters.md`, `docs/getting-started.md`, `docs/cookbook.md`,
`docs/architecture.md`, `docs/migration/v3.md`, `docs/troubleshooting.md`,
`docs/security.md`, `docs/threat-model.md`, `docs/auth.md`, `docs/idempotency.md`,
`docs/operations.md`, `docs/openapi-workflow.md`, `docs/configuration.md`,
`docs/observability.md`, `docs/scaffold-support.md`,
`docs/adapter-maturity.md`, `docs/safe-defaults.md`,
`docs/middleware-safety.md`, `docs/input-size-threat-review.md`,
`docs/testing.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `docs/metrics.md`,
`docs/support-policy.md`, `docs/dependency-policy.md`, `docs/license-policy.md`,
`docs/dependency-footprint.md`, `docs/adr/0001-module-boundaries.md`,
`VERSIONING.md`, `docs/api-inventory.md`, `docs/api-review-checklist.md`,
`docs/api-reference.md`, `docs/core-readiness.md`, `docs/deprecations.md`, `docs/interface-ownership.md`,
`docs/context-cancellation.md`, `docs/errors.md`, `docs/concurrency.md`,
`docs/resource-lifecycle.md`, `docs/release-runbook.md`, `docs/release-review.md`,
`docs/audits.md`, `docs/openssf-best-practices.md`, `docs/release-notes.md`, `docs/release-manifests.md`, `docs/ports-surface.md`,
`docs/v3-compatibility-roadmap.md`, `docs/production-readiness.md`,
`docs/governance.md`, `docs/performance.md`,
`docs/dependency-boundary.md`, `docs/dependency-risk.md`,
`docs/package-doc-standard.md`, `docs/full-service-scaffold.md`,
`docs/package-classification.tsv`, `docs/supported-adapter-contracts.tsv`,
`docs/supported-adapter-test-realism.tsv`,
`docs/contrib-api-drift-packages.txt`,
`docs/contrib-api-drift-dispositions.tsv`,
`docs/vulnerability-dispositions.tsv`, `contrib/examples/README.md`,
`examples/reference-saas-api/README.md`,
`examples/reference-saas-api/deploy/helm/README.md`,
`examples/reference-saas-api/deploy/kubernetes/README.md`,
`examples/reference-saas-api/deploy/terraform/aws/README.md`,
`examples/reference-saas-api/observability/runbooks/observability.md`,
`examples/reference-saas-api/docs/providers/provider-runbook.md`,
`PANIC_POLICY.md`, and `release-check-summary.json`.
