# api-toolkit

[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/aatuh/api-toolkit/badge)](https://scorecard.dev/viewer/?uri=github.com%2Faatuh%2Fapi-toolkit)

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
go get github.com/aatuh/api-toolkit/v4
```

Stable core package list: `VERSIONING.md` is the source of truth, and
`scripts/apicheck.sh` must cover the same package list. The minimum supported
Go line is Go 1.25.x and the current tested Go line is Go 1.26.x for both root
and contrib.

Minimal existing-service example
([source](examples/snippets/minimal-existing-service/main.go)):

```go
package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/middleware/maxbody"
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

	server := &http.Server{
		Addr:              ":8080",
		Handler:           bodyLimit.Handler(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}
```

## Start Here

- Add the core library to an existing service: [docs/library-first.md](docs/library-first.md)
- Use the smallest root-only path: [docs/minimal-core.md](docs/minimal-core.md)
- Choose a core package: [docs/core-package-guide.md](docs/core-package-guide.md)
- Browse stable API references: [docs/api-reference.md](docs/api-reference.md)
- Generate a new app-owned service scaffold: [docs/scaffold-first.md](docs/scaffold-first.md)
- Wire supported adapters: [docs/contrib-adapters.md](docs/contrib-adapters.md)
- Review CLI/scaffold identity: [docs/cli-scaffold-identity.md](docs/cli-scaffold-identity.md)
- Review the stable-core charter: [docs/stable-core.md](docs/stable-core.md)
- Review the public roadmap and non-goals: [ROADMAP.md](ROADMAP.md)
- Browse concise release history: [CHANGELOG.md](CHANGELOG.md)
- Compare alternatives and non-goals: [docs/alternatives.md](docs/alternatives.md)
- Find task recipes: [docs/cookbook.md](docs/cookbook.md)
- Review benchmark baselines: [docs/performance.md](docs/performance.md)
- Review contributor rules: [CONTRIBUTING.md](CONTRIBUTING.md)
- Ask a focused usage question: [.github/ISSUE_TEMPLATE/question.md](.github/ISSUE_TEMPLATE/question.md)
- Share adopter feedback: [.github/ISSUE_TEMPLATE/adopter_review.md](.github/ISSUE_TEMPLATE/adopter_review.md)
- Use the PR checklist: [.github/pull_request_template.md](.github/pull_request_template.md)
- Use the documentation map: [docs/README.md](docs/README.md)
- Review safe defaults: [docs/safe-defaults.md](docs/safe-defaults.md)
- Review middleware placement: [docs/middleware-safety.md](docs/middleware-safety.md)
- Review core readiness: [docs/core-readiness.md](docs/core-readiness.md)
- Upgrade within v3: [docs/migration/v3.md](docs/migration/v3.md)
- Troubleshoot adoption issues: [docs/troubleshooting.md](docs/troubleshooting.md)

## Funding And Sponsorship

api-toolkit does not currently accept or solicit sponsorships, donations,
grants, or paid support through this repository. Users and contributors have no
financial obligation to use, evaluate, report issues for, or contribute to the
project.

Contributions, issue triage, API review, and security handling remain governed
by the published technical policies. A payment, offer of funding, or request
for priority would not create paid support, release priority, maintainer access,
or a security-response commitment. Do not send money or payment details to
maintainers based on repository activity or public comments.

If the project later accepts funding, maintainers must add a reviewed public
funding link and state the recipient, scope, conflicts-of-interest handling,
and whether funding changes any support expectations before soliciting or
accepting funds.

## Modules

| Module | Import path | Use for |
| --- | --- | --- |
| Core | `github.com/aatuh/api-toolkit/v4` | Stable HTTP/API primitives, middleware, helpers, route contracts, and compatibility-gated root packages. |
| Contrib | `github.com/aatuh/api-toolkit/contrib/v4` | Third-party adapters, integrations, runnable examples, and generator tooling outside the stable core API promise. |

Install contrib only when you need maintained adapters, generated scaffolds, or
contrib examples:

```sh
go get github.com/aatuh/api-toolkit/contrib/v4
```

Supported development and CI toolchain policy: root and contrib support Go
1.25.x as the minimum line and test Go 1.26.x as the current line. Local and
release gates should run with `GOTOOLCHAIN=local` so drift between module `go`
directives, GitHub Actions setup, and release evidence is visible before
publication.

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

For API, owner, and test-status metadata, use
`docs/package-classification.tsv`, `docs/package-owners.tsv`, the rendered
guide in [docs/package-classification.md](docs/package-classification.md), and
the human guide in [docs/package-doc-standard.md](docs/package-doc-standard.md).
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
  `docs/provenance.md`, and `release-check-summary.json`.
- OpenSSF Scorecard results are published by `.github/workflows/scorecard.yml`
  as a public badge/API report, uploaded SARIF artifact `scorecard-sarif`, and
  code-scanning SARIF. Target `>= 8`; a lower public score requires remediation
  or an explicit release-review disposition before publishing.
- OpenSSF Best Practices Badge status is documented in
  [docs/openssf-best-practices.md](docs/openssf-best-practices.md): not claimed
  until a registered bestpractices.dev project ID exists and the remaining gaps
  are resolved or explicitly accepted.
- Code scanning merge protection is documented in
  [docs/governance.md](docs/governance.md): the protected branch ruleset must
  require CodeQL code scanning results for pull requests with explicit alert
  thresholds.
- Governance docs publish required checks, CODEOWNERS expectations, tag policy,
  and branch-protection evidence boundaries.
- Maintenance status: this is currently a single-maintainer project. Security
  reports use the acknowledgement and remediation targets in
  [SECURITY.md](SECURITY.md); routine issues, feature requests, adopter
  reviews, and non-security pull requests are handled best effort. Use
  [.github/ISSUE_TEMPLATE/adopter_review.md](.github/ISSUE_TEMPLATE/adopter_review.md)
  to report API friction, missing docs, and migration pain without posting
  secrets or vulnerability details publicly. Supported versions are the
  latest release on the default branch, with Go/platform support documented in
  [docs/support-policy.md](docs/support-policy.md). There is no automatic
  successor; [governance documents the unavailability and verified handover
  policy](docs/governance.md#maintainer-succession-and-unavailability).

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
operator detail. The bootstrap snippet below is compile checked in
[contrib/bootstrap/example_test.go](contrib/bootstrap/example_test.go).

```go
err := bootstrap.MountSystemEndpointsToWithAdmin(router, bootstrap.SystemEndpoints{
	Health:  healthHandler,
	Metrics: bootstrap.PrometheusMetricsHandler(),
	Pprof:   http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
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
- Security threat model: [docs/threat-model.md](docs/threat-model.md)
- Package security review: [docs/security-review.md](docs/security-review.md)
- Auth production guide: [docs/auth.md](docs/auth.md)
- Idempotency production guide: [docs/idempotency.md](docs/idempotency.md)
- Health and admin operations: [docs/operations.md](docs/operations.md)
- OpenAPI contract workflow: [docs/openapi-workflow.md](docs/openapi-workflow.md)
- Runtime configuration: [docs/configuration.md](docs/configuration.md)
- Observability: [docs/observability.md](docs/observability.md)
- Scaffold support: [docs/scaffold-support.md](docs/scaffold-support.md)
- Adapter maturity: [docs/adapter-maturity.md](docs/adapter-maturity.md)
- Safe defaults audit: [docs/safe-defaults.md](docs/safe-defaults.md)
- Middleware safety matrix: [docs/middleware-safety.md](docs/middleware-safety.md)
- Vulnerability reporting policy: [SECURITY.md](SECURITY.md)
- Panic policy: [PANIC_POLICY.md](PANIC_POLICY.md)
- Metrics naming and labels: [docs/metrics.md](docs/metrics.md)
- Go and platform support policy: [docs/support-policy.md](docs/support-policy.md)
- Dependency boundary: [docs/dependency-boundary.md](docs/dependency-boundary.md)
- Dependency policy: [docs/dependency-policy.md](docs/dependency-policy.md)
- Dependency license policy: [docs/license-policy.md](docs/license-policy.md)
- Dependency risk disposition: [docs/dependency-risk.md](docs/dependency-risk.md)
- Dependency footprint report: [docs/dependency-footprint.md](docs/dependency-footprint.md)
- Test coverage evidence: [docs/test-coverage.md](docs/test-coverage.md)
- Vulnerability disposition manifest: `docs/vulnerability-dispositions.tsv`
- Community conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

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
- V3 migration guide: [docs/migration/v3.md](docs/migration/v3.md)
- Troubleshooting guide: [docs/troubleshooting.md](docs/troubleshooting.md)
- Published v4 tags: `v4.0.0`, `contrib/v4.0.0`, `v4.0.1`, and
  `contrib/v4.0.1`. Before a new deployment or release, follow the
  [v4 release-identity incident](docs/release-incident-v4-release-identity.md);
  `v4.0.1` and `contrib/v4.0.1` are not approved release baselines.
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
- Current supported v4 API baseline: see `docs/release-runbook.md`.
- The v4 major-release evidence compared against `API_BASE_REF=v3.1.2`; v4 patch and minor releases compare against the latest published v4 tag from the runbook.
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
