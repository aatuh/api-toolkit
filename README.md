# api-toolkit

`api-toolkit` provides small, composable Go HTTP API building blocks for teams
that already own their service architecture and want production guardrails
without adopting a full framework.

Target user: Go developers building conventional JSON/HTTP APIs with `net/http`,
chi, or app-owned routers.

Differentiator: production guardrails for conventional Go JSON APIs, not a
router or full framework.

Non-goals: not a router, persistence framework, billing framework, universal auth
platform, RPC framework, streaming middleware suite, or replacement for
app-owned business ports.

Core-only install:

```sh
go get github.com/aatuh/api-toolkit/v3
```

Minimal existing-service example:

```go
package main

import (
	"net/http"

	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/middleware/maxbody"
)

func main() {
	bodyLimit, err := maxbody.New(maxbody.Options{MaxBytes: 1 << 20})
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	if err := http.ListenAndServe(":8080", bodyLimit.Handler(mux)); err != nil {
		panic(err)
	}
}
```

Stable core package list: `VERSIONING.md` is the source of truth, and
`scripts/apicheck.sh` must cover the same package list. Root and contrib target
Go 1.25.x.

## Start Here

- Add the core library to an existing service: [docs/library-first.md](docs/library-first.md)
- Use the smallest root-only path: [docs/minimal-core.md](docs/minimal-core.md)
- Choose a core package: [docs/core-package-guide.md](docs/core-package-guide.md)
- Browse stable API references: [docs/api-reference.md](docs/api-reference.md)
- Generate a new app-owned service scaffold: [docs/scaffold-first.md](docs/scaffold-first.md)
- Wire supported adapters: [docs/contrib-adapters.md](docs/contrib-adapters.md)
- Review the stable-core charter: [docs/stable-core.md](docs/stable-core.md)
- Compare alternatives and non-goals: [docs/alternatives.md](docs/alternatives.md)
- Find task recipes: [docs/cookbook.md](docs/cookbook.md)
- Review benchmark baselines: [docs/performance.md](docs/performance.md)
- Review contributor rules: [CONTRIBUTING.md](CONTRIBUTING.md)
- Use the PR checklist: [.github/pull_request_template.md](.github/pull_request_template.md)
- Use the documentation map: [docs/README.md](docs/README.md)
- Review safe defaults: [docs/safe-defaults.md](docs/safe-defaults.md)
- Review middleware placement: [docs/middleware-safety.md](docs/middleware-safety.md)
- Review core readiness: [docs/core-readiness.md](docs/core-readiness.md)

## Modules

| Module | Import path | Use for |
| --- | --- | --- |
| Core | `github.com/aatuh/api-toolkit/v3` | Stable HTTP/API primitives, middleware, helpers, route contracts, and compatibility-gated root packages. |
| Contrib | `github.com/aatuh/api-toolkit/contrib/v3` | Third-party adapters, integrations, runnable examples, and generator tooling outside the stable core API promise. |

Install contrib only when you need maintained adapters, generated scaffolds, or
contrib examples:

```sh
go get github.com/aatuh/api-toolkit/contrib/v3
```

Supported development and CI toolchain policy: root and contrib target Go 1.25.x.
Local and release gates should run with `GOTOOLCHAIN=local` so drift between
module `go` directives, GitHub Actions setup, and release evidence is visible
before publication.

## Package Map

Recommended small-core starting points are mapped in
[docs/core-package-guide.md](docs/core-package-guide.md). The minimum
root-only adoption path is documented in [docs/minimal-core.md](docs/minimal-core.md).

Common first packages:

- HTTP responses and errors: `httpx`, `fielderrors`.
- Request parsing and bounds: `binding`, `queryparams`, `upload`,
  `middleware/maxbody`, `middleware/querylimits`, and `middleware/json`.
- Runtime guardrails: `middleware/timeout`, `middleware/secure`,
  `middleware/deprecation`, `middleware/trace`, and selected auth middleware.
- API contracts: `routecontracts`, `routepolicy`, `specs`, and
  `endpoints/health`, `endpoints/version`, `endpoints/docs`.
- App-owned integration boundaries: `idempotent`, `webhooks`, and small
  package-local interfaces where the consuming package owns the shape.

Stable but not the default adoption story:

- `ports` remains a v3 compatibility commitment, but new app design should
  prefer package-local or app-owned interfaces unless a shared root abstraction
  is proven.
- `compat/billing`, migration-shaped packages, scaffolding support, test
  helpers, and client helpers are stable where listed in `VERSIONING.md`, but
  they are not the recommended minimal path for a new library user.

Contrib packages provide routers, logging, validation, Postgres, Redis, Stripe,
Resend, OpenTelemetry, CORS, OpenAPI validation, request logging, config,
bootstrap composition, generated service tooling, and runnable examples. Contrib
supported-adapter incompatible drift is gate-enforced and does not make contrib
stable.

For API and test-status ownership, use `docs/package-classification.tsv`, the
rendered guide in [docs/package-classification.md](docs/package-classification.md),
and the human guide in [docs/package-doc-standard.md](docs/package-doc-standard.md).
Package maturity tier badges use the TSV-backed labels `[stable]`,
`[compatibility-only]`, `[supported-adapter]`, `[experimental]`, `[generated]`,
and `[tooling]`; do not treat a package as a different tier unless the TSV row
changes in the same review.

## Trust Proof

Current repository trust signals:

- CI runs `make coverage-check`, `make test-race`, `make vuln`, `make docs-check`,
  `make v3-readiness-check`, `make release-api-check`, and `make fuzz`.
- Stable SemVer policy is enforced by `VERSIONING.md`, `scripts/apicheck.sh`,
  `docs/api-inventory.md`, and release API compatibility checks.
- Package status is machine-owned by `docs/package-classification.tsv` and
  rendered in `docs/package-classification.md`.
- Release review covers dependency footprint, contrib drift, vulnerability
  dispositions, SBOM assets, signatures, and provenance/attestation policy
  through `docs/release-runbook.md`, `docs/release-review.md`, and
  `release-check-summary.json`.
- Governance docs publish required checks, CODEOWNERS expectations, tag policy,
  and branch-protection evidence boundaries.

## Production readiness

api-toolkit v3 is production-credible for conventional Go JSON/HTTP APIs and
generated SaaS/API services. It is not a universal backend platform for every
transport, streaming workload, provider workflow, or organization-specific
operating model. Use [docs/production-readiness.md](docs/production-readiness.md)
as the readiness matrix and adapter maturity review before standardizing on a
package or generated profile. Generated code is app-owned; api-toolkit
standardizes infrastructure defaults without becoming your product or provider
framework. Use [docs/core-readiness.md](docs/core-readiness.md) as the
package-specific production checklist and [docs/middleware-safety.md](docs/middleware-safety.md)
before applying request or response middleware globally.

| Area | Readiness | Notes |
| --- | --- | --- |
| Stable core packages | Stable SemVer surface | Covered by `VERSIONING.md`, `scripts/apicheck.sh`, package classification, and release evidence. |
| Supported contrib adapters | Supported adapter tier | Direct tests, docs, drift coverage, and behavior contracts are required; supported-adapter incompatible drift is gate-enforced and does not make contrib stable. |
| Experimental contrib packages | Maintained but unstable | Use only with app-owned compatibility expectations until promoted with evidence. |
| `saas-api` scaffold | Lean service starter | Keeps production-safe HTTP defaults without forcing persistence or membership models. |
| `saas-api-full` scaffold | Production reference scaffold | Includes Postgres, Redis, tenancy, API keys, async/outbox, audit, webhooks, OpenAPI 3.1, clients, and deployment starters. Generated code is app-owned. |
| Streaming, SSE, WebSockets, and large downloads | Explicit caveat | Use route-level opt-outs; do not apply hard-timeout response buffering, response validation, or idempotency response capture globally. |

Streaming routes, server-sent events, websocket upgrades, and large downloads
should be marked with `x-api-toolkit-streaming` and wired with
`securityprofile.StreamingRouteOverride` or equivalent route-specific
middleware. This preserves optional `http.ResponseWriter` interfaces and avoids
buffering or validating responses that are not finite JSON documents.

## Health endpoint contract

The health package exposes separate liveness, readiness, and detailed health
behaviors:

- Liveness and readiness are expected to reflect configured checker state and should not silently report healthy when no probe checks are configured.
- Detailed health output is an operator-focused surface because it can include dependency-level status and check details.
- `ports.HealthCheckConfig.EnableDetailed` controls whether HTTP packages should expose detailed health responses.
- Mount detailed health, pprof, and metrics behind admin/internal access control or upstream network policy; prefer `Handler.RegisterPublicRoutesTo`, `Handler.RegisterAdminDetailedHealthRoute`, `pprof.RegisterAdminRoutes`, and `bootstrap.MountSystemEndpointsToWithAdmin` for new mounts because operator-only routes require an explicit wrapper.
- HTTP dependency check URLs are application configuration. Do not derive them from request parameters or tenant-controlled input.
- Missing checker registrations or invalid probe wiring should fail closed and surface as unhealthy state rather than synthetic success.
- When `EnableCaching` is true, checker results may be reused across health endpoints until `CacheDuration` expires.

Safe system endpoint mounting should keep public probes separate from
operator-only dependency detail, metrics, and pprof. Web, mobile, and desktop
clients should use public probes only; they should never call operator-only
endpoints directly. If you mount pprof outside the bootstrap helper, use
`pprof.RegisterAdminRoutes`. If you split routers manually, use
`healthHandler.RegisterPublicRoutesTo(publicRouter)` for public probes and
`healthHandler.RegisterAdminDetailedHealthRoute(adminRouter, requireAdmin)` for
operator detail.

```go
err := bootstrap.MountSystemEndpointsToWithAdmin(router, bootstrap.SystemEndpoints{
	Health:  healthHandler,
	Metrics: bootstrap.PrometheusMetricsHandler(),
	Pprof:   pprof.Handler(),
}, bootstrap.SystemEndpointAdminOptions{
	RequireAdmin: requireAdmin,
	EnablePprof:  true,
})
if err != nil {
	return err
}
```

## Security and operations

- Security posture and dangerous-bypass configuration: [docs/security.md](docs/security.md)
- Safe defaults audit: [docs/safe-defaults.md](docs/safe-defaults.md)
- Middleware safety matrix: [docs/middleware-safety.md](docs/middleware-safety.md)
- Vulnerability reporting policy: [SECURITY.md](SECURITY.md)
- Panic policy: [PANIC_POLICY.md](PANIC_POLICY.md)
- Metrics naming and labels: [docs/metrics.md](docs/metrics.md)
- Go and platform support policy: [docs/support-policy.md](docs/support-policy.md)
- Dependency boundary: [docs/dependency-boundary.md](docs/dependency-boundary.md)
- Dependency policy: [docs/dependency-policy.md](docs/dependency-policy.md)
- Dependency risk disposition: [docs/dependency-risk.md](docs/dependency-risk.md)
- Dependency footprint report: [docs/dependency-footprint.md](docs/dependency-footprint.md)
- Test coverage evidence: [docs/test-coverage.md](docs/test-coverage.md)
- Vulnerability disposition manifest: `docs/vulnerability-dispositions.tsv`

## Stability

Stable core package list: `VERSIONING.md` is the source of truth, and
`scripts/apicheck.sh` must cover the same package list.

- Public API inventory: [docs/api-inventory.md](docs/api-inventory.md)
- API reference index: [docs/api-reference.md](docs/api-reference.md)
- API review checklist: [docs/api-review-checklist.md](docs/api-review-checklist.md)
- Deprecation policy: [docs/deprecations.md](docs/deprecations.md)
- Interface ownership: [docs/interface-ownership.md](docs/interface-ownership.md)
- Context and cancellation: [docs/context-cancellation.md](docs/context-cancellation.md)
- Error taxonomy: [docs/errors.md](docs/errors.md)
- Concurrency safety: [docs/concurrency.md](docs/concurrency.md)
- Resource lifecycle: [docs/resource-lifecycle.md](docs/resource-lifecycle.md)
- Stable core readiness matrix: [docs/core-readiness.md](docs/core-readiness.md)
- Latest published release line: `v3.1.2` and `contrib/v3.1.2`.
- `master` may contain unreleased changes after the latest tag; release
  consumers should use tags and release notes instead of assuming `master` is
  published evidence.
- Versioning and stable API policy: [VERSIONING.md](VERSIONING.md)
- Compatibility-sensitive ports: [docs/ports-surface.md](docs/ports-surface.md)
- V3 compatibility record: [docs/v3-compatibility-roadmap.md](docs/v3-compatibility-roadmap.md)
- Response writer removal record: [docs/response-writer-inventory.md](docs/response-writer-inventory.md)
- Production readiness matrix: [docs/production-readiness.md](docs/production-readiness.md)
- Governance and branch protection: [docs/governance.md](docs/governance.md)
- Public package classification: `docs/package-classification.tsv`

Release command details live in [docs/release-runbook.md](docs/release-runbook.md).
Keep this landing page as a pointer, not a second release runbook.

- Release review checklist: [docs/release-review.md](docs/release-review.md)
- Release notes: [docs/release-notes.md](docs/release-notes.md)
- Release manifests guide: [docs/release-manifests.md](docs/release-manifests.md)
- Contrib drift package manifest: `docs/contrib-api-drift-packages.txt`
- Contrib drift disposition manifest: `docs/contrib-api-drift-dispositions.tsv`
- Current supported v3 API baseline: see `docs/release-runbook.md`.
- First v3 major-release evidence may compare against `API_BASE_REF=v2.1.0` only as documented v2-to-v3 breakage evidence; v3 patch and minor releases compare against the latest published v3 tag from the runbook.
- Release readiness and publication evidence require an explicit `API_BASE_REF`; use the current command examples in `docs/release-runbook.md`.
- `ALLOW_DIRTY_RELEASE_EVIDENCE=1` is only for local dirty-tree audit evidence and is not acceptable before publishing.
- `make finalize` is not release evidence.
- `make release-api-check`, `make contrib-api-drift-report`, and `make contrib-release-notes-check` are explained in the runbook. Supported-adapter contrib packages are still outside the stable core API promise; supported-adapter incompatible drift is gate-enforced and does not make contrib stable.

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
