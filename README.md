# api-toolkit

Reusable building blocks for Go HTTP APIs that enforce Dependency Inversion. Your
application depends on stable `ports` interfaces, while this toolkit ships
adapters for popular libraries in a separate contrib module.

Audience: Go service developers evaluating the toolkit, maintainers checking the
stable surface, and release reviewers looking for the canonical docs path.

## Modules

| Module | Import path | Use for |
| --- | --- | --- |
| Core | `github.com/aatuh/api-toolkit/v2` | Stable ports, middleware, `httpx`, endpoint helpers, scheduler, and compatibility surfaces covered by the v2 API gate. |
| Contrib | `github.com/aatuh/api-toolkit/contrib/v2` | Third-party adapters, integrations, examples, and tooling outside the stable v2 API promise. |

Install both when following the getting-started guide:

```sh
go get github.com/aatuh/api-toolkit/v2
go get github.com/aatuh/api-toolkit/contrib/v2
```

Supported development and CI toolchain policy: root and contrib target Go 1.25.x.
Local and release gates should run with `GOTOOLCHAIN=local` so drift
between module `go` directives, GitHub Actions setup, and release evidence is
visible before publication.

## Start here

- Tutorial: [docs/getting-started.md](docs/getting-started.md)
- Task recipes: [docs/cookbook.md](docs/cookbook.md)
- Full documentation map: [docs/README.md](docs/README.md)
- Runnable examples: [contrib/examples/README.md](contrib/examples/README.md)

## Package map

Core packages:

- `ports` defines toolkit-wide boundary contracts.
- `compat/billing` is the explicit v2 compatibility package for the current hosted-checkout and invoicing model.
- `middleware/*` covers JSON enforcement, timeouts, max body limits, query limits, rate limiting, idempotency, secure headers, tracing, tenant context, deprecation headers, API key auth, JWT auth, and role authz.
- `binding` decodes validated JSON, query, and path input into typed request structs with Problem Details-compatible field errors.
- `queryparams` parses collection sort, filter, sparse fieldset, and include parameters without storage coupling.
- `httpcache` provides conditional request helpers for ETags, Last-Modified, `304`, and `412` flows.
- `httpx`, `httpx/identity`, and `httpx/recover` provide JSON responses, Problem Details, request identity, and panic recovery.
- `negotiation` provides Accept and Content-Type negotiation for JSON and vendor media types.
- `endpoints/*` provides docs, health, pprof, version, and list helpers, including offset and signed cursor pagination.
- `operations` standardizes `202 Accepted` and pollable asynchronous operation resources.
- `authorization`, `contracttest`, `routecontracts`, `securityprofile`, `specs`, `swagstub`, `scheduler`, `email`, `webhooks`, and `fielderrors` cover common API support contracts.
- The legacy response helper package is retained for v2 source compatibility; new response code should prefer `httpx`.

Contrib packages:

- `adapters/*` provides concrete implementations for routers, logging, validation, Postgres, Redis, Stripe, Resend, outbound HTTP, IDs, clocks, and policy engines.
- `middleware/*` provides CORS, request logging, metrics, OpenAPI validation, OpenTelemetry tracing, Clerk auth, and development-only auth headers.
- `integrations/*` provides convenience wrappers around selected adapters.
- `bootstrap`, `config`, `telemetry`, `migrator`, `countrycodes`, and contrib email helpers support application wiring.

For API and test-status ownership, use `docs/package-classification.tsv` and the
human guide in [docs/package-doc-standard.md](docs/package-doc-standard.md).

## Health endpoint contract

The health package exposes separate liveness, readiness, and detailed health
behaviors:

- Liveness and readiness are expected to reflect configured checker state and should not silently report healthy when no probe checks are configured.
- Detailed health output is an operator-focused surface because it can include dependency-level status and check details.
- `ports.HealthCheckConfig.EnableDetailed` controls whether HTTP packages should expose detailed health responses.
- Mount detailed health and pprof routes behind admin/internal access control or upstream network policy; the endpoint helpers do not add authorization.
- HTTP dependency check URLs are application configuration. Do not derive them from request parameters or tenant-controlled input.
- Missing checker registrations or invalid probe wiring should fail closed and surface as unhealthy state rather than synthetic success.
- When `EnableCaching` is true, checker results may be reused across health endpoints until `CacheDuration` expires.

Safe detailed-health mounting should keep public probes separate from
operator-only dependency detail:

```go
publicMux.Handle("/live", health.NewLivenessHandler(checker))
publicMux.Handle("/ready", health.NewReadinessHandler(checker))

adminMux.Handle("/health/details", requireAdmin(
	health.NewDetailedHealthHandler(checker),
))
```

## Security and operations

- Security posture and dangerous-bypass configuration: [docs/security.md](docs/security.md)
- Vulnerability reporting policy: [SECURITY.md](SECURITY.md)
- Panic policy: [PANIC_POLICY.md](PANIC_POLICY.md)
- Metrics naming and labels: [docs/metrics.md](docs/metrics.md)
- Dependency boundary: [docs/dependency-boundary.md](docs/dependency-boundary.md)
- Dependency risk disposition: [docs/dependency-risk.md](docs/dependency-risk.md)
- Vulnerability disposition manifest: `docs/vulnerability-dispositions.tsv`

## Stability

Stable core package list: `VERSIONING.md` is the source of truth, and
`scripts/apicheck.sh` must cover the same package list.

- Versioning and stable API policy: [VERSIONING.md](VERSIONING.md)
- Compatibility-sensitive ports: [docs/ports-surface.md](docs/ports-surface.md)
- V3 compatibility roadmap: [docs/v3-compatibility-roadmap.md](docs/v3-compatibility-roadmap.md)
- Response writer compatibility inventory: [docs/response-writer-inventory.md](docs/response-writer-inventory.md)
- Public package classification: `docs/package-classification.tsv`

Release command details live in [docs/release-runbook.md](docs/release-runbook.md).
Keep this landing page as a pointer, not a second release runbook.

- Release review checklist: [docs/release-review.md](docs/release-review.md)
- Release notes: [docs/release-notes.md](docs/release-notes.md)
- Release manifests guide: [docs/release-manifests.md](docs/release-manifests.md)
- Contrib drift package manifest: `docs/contrib-api-drift-packages.txt`
- Contrib drift disposition manifest: `docs/contrib-api-drift-dispositions.tsv`
- Current supported v2 API baseline: see `docs/release-runbook.md`.
- Release readiness requires `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-check`.
- Publication evidence requires `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` from a clean worktree.
- `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` is only for local dirty-tree audit evidence and is not acceptable before publishing.
- `make finalize` is not release evidence.
- `make release-api-check`, `make contrib-api-drift-report`, and `make contrib-release-notes-check` are explained in the runbook. Supported-adapter contrib packages are still outside the stable core API promise, but incompatible public API drift in that tier fails the contrib drift gate.

### Adapter coverage policy

Use `docs/package-classification.tsv` as the source of truth for API and test
coverage status.

- `wrapper-only` packages may use `wrapper-smoke-tested` only when smoke coverage is sufficient because the wrapper delegates behavior to another maintained package.
- Wrapper smoke tests must prove interface satisfaction, constructor/defaults, disabled or nil behavior, and option propagation.
- `example-only` packages are build-smoke checked and are not behavior-complete coverage.
- Public packages need direct tests unless explicitly classified as wrapper, example, generated, tooling, test-support, or excluded.
- `needs-tests` is a release blocker until replaced with direct tests or a documented exception.

## Local documentation checks

For documentation-only changes, prefer:

```sh
GOTOOLCHAIN=local make docs-check
```

For implementation changes, examples, package docs, generated files, scripts, or
repo-wide contracts, use the workflow in [docs/README.md](docs/README.md) to
choose between `docs-check`, `fast-check`, `audit-check`, and `finalize`.
