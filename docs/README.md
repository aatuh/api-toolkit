# Documentation

Audience: readers who need the fastest path to the right api-toolkit document
without scanning the root README.

## New users and application developers

| Document | Audience | Purpose |
| --- | --- | --- |
| [Getting started](getting-started.md) | New users | Generate, run, and verify the production-oriented service scaffold. |
| [Full service scaffold](full-service-scaffold.md) | Application teams | Understand the `saas-api-full` production foundation, support tier, and integration-test policy. |
| [Reference service](reference-service.md) | Maintainers and release reviewers | Verify the checked-in `saas-api-full` adoption proof and know which evidence is local, Docker-backed, or deployment-owned. |
| [Production readiness](production-readiness.md) | Technical leads and platform owners | Decide which surfaces are production-ready, supported-adapter, experimental, caveated, or part of the adapter maturity review. |
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
| [Security policy](../SECURITY.md) | Security reporters and release consumers | Report vulnerabilities and understand supported release security handling. |
| [Panic policy](../PANIC_POLICY.md) | Maintainers and API designers | Decide when panics are allowed and how HTTP recovery behaves. |
| [Metrics](metrics.md) | Operators and developers | Use low-cardinality HTTP metric names and labels. |
| [Dependency boundary](dependency-boundary.md) | Maintainers | Keep root stable code free of contrib adapter dependencies. |
| [Dependency risk](dependency-risk.md) | Release reviewers and security maintainers | Review imported-but-not-called vulnerability disposition and ownership. |

## Stability, compatibility, and package docs

| Document | Audience | Purpose |
| --- | --- | --- |
| [Versioning](../VERSIONING.md) | API consumers and maintainers | Define the stable core API surface and contrib compatibility policy. |
| [Ports surface](ports-surface.md) | Maintainers and advanced API consumers | Identify compatibility-sensitive port history and preferred replacements. |
| [V3 compatibility record](v3-compatibility-roadmap.md) | Maintainers | Track completed v3 cleanup decisions and remaining compatibility-sensitive guardrails. |
| [Package doc standard](package-doc-standard.md) | Maintainers | Apply the minimum package-doc template and see the placeholder inventory remediated in this pass. |
| `docs/package-classification.tsv` | Maintainers and automation | Machine-readable API and test-status classification for every package. |
| `docs/supported-adapter-contracts.tsv` | Maintainers and automation | Machine-readable behavior contracts and evidence paths for supported contrib adapters. |

## Release and evidence

| Document | Audience | Purpose |
| --- | --- | --- |
| [Release runbook](release-runbook.md) | Release operators | Command source of truth for local checks, release evidence, artifact verification, and baseline policy. |
| [Release review checklist](release-review.md) | Release reviewers | Short path through summary fields, manifests, dirty-tree decisions, artifacts, and release notes. |
| [Governance](governance.md) | Maintainers | Branch protection, CODEOWNERS, tag protection, required checks, and release approval expectations. |
| [Release notes](release-notes.md) | Release consumers and maintainers | Dated behavior changes, upgrade notes, and package-tied contrib drift acknowledgements. |
| [Release manifests](release-manifests.md) | Release reviewers and maintainers | Human guide for package classification, contrib drift, contrib dispositions, and vulnerability dispositions. |
| `docs/contrib-api-drift-packages.txt` | Maintainers and automation | Selected contrib packages reviewed by drift checks; supported-adapter incompatible drift is gate-enforced. |
| `docs/supported-adapter-contracts.tsv` | Maintainers and automation | Required supported-adapter behavior contracts with direct-test and release-drift evidence. |
| `docs/contrib-api-drift-dispositions.tsv` | Release reviewers and automation | Owner, status, review date, expiry, and acknowledgement for current contrib drift. |
| `docs/vulnerability-dispositions.tsv` | Release reviewers and automation | Owner, review, expiry, and upgrade trigger rows for imported-only vulnerability IDs when present. |
| `release-check-summary.json` | Release reviewers | Generated local release evidence summary; only clean publication evidence is publishable. |

## Documentation quality workflow

Use the narrowest check that matches the change:

| Change type | Preferred command | Notes |
| --- | --- | --- |
| Documentation-only edits | `GOTOOLCHAIN=local make docs-check` | Runs documentation contracts, getting-started build extraction, API/docs policy checks, and release evidence parser contracts. |
| V3 cleanup readiness | `GOTOOLCHAIN=local make v3-readiness-check` | Runs focused compatibility-sensitive surface guardrails for major-version cleanup planning and release-note requirements. |
| Docs plus ordinary code changes | `GOTOOLCHAIN=local make fast-check` | Runs `docs-check` and unit tests without rewriting files. |
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

`README.md`, `docs/getting-started.md`, `docs/cookbook.md`,
`docs/architecture.md`, `docs/security.md`, `SECURITY.md`, `docs/metrics.md`,
`VERSIONING.md`, `docs/release-runbook.md`, `docs/release-review.md`,
`docs/release-notes.md`, `docs/release-manifests.md`, `docs/ports-surface.md`,
`docs/v3-compatibility-roadmap.md`, `docs/production-readiness.md`,
`docs/governance.md`,
`docs/dependency-boundary.md`, `docs/dependency-risk.md`,
`docs/package-doc-standard.md`, `docs/full-service-scaffold.md`,
`docs/package-classification.tsv`, `docs/supported-adapter-contracts.tsv`,
`docs/contrib-api-drift-packages.txt`,
`docs/contrib-api-drift-dispositions.tsv`,
`docs/vulnerability-dispositions.tsv`, `contrib/examples/README.md`,
`PANIC_POLICY.md`, and `release-check-summary.json`.
