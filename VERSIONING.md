# Versioning and Stability

Audience: API consumers and maintainers who need the stable core surface,
compatibility-sensitive exceptions, and release-command intent.

This project follows semantic versioning for the core module
`github.com/aatuh/api-toolkit/v3`. From v1 onward, we treat the packages listed
below as stable: any breaking change requires a major version bump. Stability
does not imply every exported identifier is perfectly adapter-neutral; some v2
surfaces are explicitly classified as compatibility-sensitive so the public docs
do not over-claim genericity. `docs/package-classification.tsv` classifies every
root and contrib package so new public packages cannot appear without an
explicit API and test-coverage classification.

## Stable API surface (core module)

All exported identifiers in these packages are considered stable:

- `github.com/aatuh/api-toolkit/v3/apiclient`
- `github.com/aatuh/api-toolkit/v3/apitest`
- `github.com/aatuh/api-toolkit/v3/authorization`
- `github.com/aatuh/api-toolkit/v3/binding`
- `github.com/aatuh/api-toolkit/v3/compat/billing`
- `github.com/aatuh/api-toolkit/v3/contracttest`
- `github.com/aatuh/api-toolkit/v3/email`
- `github.com/aatuh/api-toolkit/v3/endpoints/docs`
- `github.com/aatuh/api-toolkit/v3/endpoints/health`
- `github.com/aatuh/api-toolkit/v3/endpoints/list`
- `github.com/aatuh/api-toolkit/v3/endpoints/pprof`
- `github.com/aatuh/api-toolkit/v3/endpoints/version`
- `github.com/aatuh/api-toolkit/v3/fielderrors`
- `github.com/aatuh/api-toolkit/v3/httpcache`
- `github.com/aatuh/api-toolkit/v3/httpx`
- `github.com/aatuh/api-toolkit/v3/httpx/identity`
- `github.com/aatuh/api-toolkit/v3/httpx/recover`
- `github.com/aatuh/api-toolkit/v3/idempotent`
- `github.com/aatuh/api-toolkit/v3/middleware/auth/apikey`
- `github.com/aatuh/api-toolkit/v3/middleware/auth/authz`
- `github.com/aatuh/api-toolkit/v3/middleware/auth/jwt`
- `github.com/aatuh/api-toolkit/v3/middleware/auth/tenant`
- `github.com/aatuh/api-toolkit/v3/middleware/deprecation`
- `github.com/aatuh/api-toolkit/v3/middleware/idempotency`
- `github.com/aatuh/api-toolkit/v3/middleware/json`
- `github.com/aatuh/api-toolkit/v3/middleware/maxbody`
- `github.com/aatuh/api-toolkit/v3/middleware/querylimits`
- `github.com/aatuh/api-toolkit/v3/middleware/ratelimit`
- `github.com/aatuh/api-toolkit/v3/middleware/secure`
- `github.com/aatuh/api-toolkit/v3/middleware/timeout`
- `github.com/aatuh/api-toolkit/v3/middleware/trace`
- `github.com/aatuh/api-toolkit/v3/negotiation`
- `github.com/aatuh/api-toolkit/v3/oauth2`
- `github.com/aatuh/api-toolkit/v3/operations`
- `github.com/aatuh/api-toolkit/v3/ports`
- `github.com/aatuh/api-toolkit/v3/queryparams`
- `github.com/aatuh/api-toolkit/v3/routecontracts`
- `github.com/aatuh/api-toolkit/v3/routepolicy`
- `github.com/aatuh/api-toolkit/v3/scheduler`
- `github.com/aatuh/api-toolkit/v3/scheduler/migrations`
- `github.com/aatuh/api-toolkit/v3/securityprofile`
- `github.com/aatuh/api-toolkit/v3/specs`
- `github.com/aatuh/api-toolkit/v3/swagstub`
- `github.com/aatuh/api-toolkit/v3/upload`
- `github.com/aatuh/api-toolkit/v3/webhooks`

## Compatibility-sensitive stable sub-surfaces

These exports remain part of the v2 compatibility promise, but they are not the
recommended model for new generic boundary design:

- `github.com/aatuh/api-toolkit/v3/ports` billing contracts in
  `ports/billing.go` are stable in v2 but intentionally Stripe-shaped today.
  They are deprecated in favor of `github.com/aatuh/api-toolkit/v3/compat/billing`,
  which is the explicit v2 compatibility import path for that model.
- `github.com/aatuh/api-toolkit/v3/ports` database stats contracts in
  `ports/database.go`, including `DatabasePool.Stat` and `DatabaseStats`, are
  stable in v2 but intentionally mirror pgxpool-style counters today.

Compatibility-sensitive means:

- Minor and patch releases must preserve these APIs unless a security fix
  requires otherwise.
- New design work should prefer narrower, plain-value, or app-owned contracts
  instead of widening these surfaces further.
- For the existing hosted-checkout and invoicing model, new v2 code should
  import `github.com/aatuh/api-toolkit/v3/compat/billing` instead of importing
  billing contracts from `ports`.
- The repository should document the migration path before any future major
  cleanup. The current plan lives in `docs/ports-surface.md`.
- The current v3 cleanup checklist covers the deprecated `ports/billing.go`
  aliases, `DatabasePool.Stat`/`DatabaseStats`, and the legacy
  `response_writer` package.

## Experimental or unstable surfaces

- `github.com/aatuh/api-toolkit/v3/middleware/auth/shared` is an implementation-sharing package for auth middleware and is not part of the stable compatibility promise.
- Examples, docs, and tooling are not API commitments.
- Any package explicitly documented as experimental is unstable.

## Contrib module policy

The contrib module is outside the stable API compatibility promise for v2.
`make release-api-check` covers only the core module
`github.com/aatuh/api-toolkit/v3`; it does not cover
`github.com/aatuh/api-toolkit/contrib/v3`.

`docs/package-classification.tsv` classifies contrib packages as
supported-adapter, experimental, wrapper-only, test-only, example-only,
generated, tooling, or excluded. Supported-adapter packages are maintained
runtime implementations for common production wiring such as Postgres, Redis,
Stripe, OpenTelemetry, request logging, metrics, CORS, OpenAPI validation, and
bootstrap composition. They are still outside the stable core API promise, but
minor releases should avoid incompatible exported API drift unless a security,
provider, or dependency requirement makes that unavoidable. Integrations are
convenience wrappers and have no independent compatibility contract beyond the
adapter behavior they delegate to. Examples, generated example code, commands,
and test-support packages are not public API commitments.

If a contrib package is promoted to stable in a future release line, add a
`release-contrib-api-check` gate before changing its classification to stable.
Until then, `make contrib-api-drift-report` compares selected high-use contrib
adapters and integrations against an explicit baseline without turning contrib
into stable API. supported-adapter incompatible drift fails this gate and
requires a major-release policy decision, reclassification, or an intentional
compatibility plan. Experimental and wrapper-only drift remains report-only
review evidence. Local release evidence archives this report and summarizes
compatible, incompatible, and enforced drift counts for reviewer visibility.

Contrib supported-tier adapter, integration, middleware, bootstrap, telemetry,
and production generator CLI changes that alter public behavior should update
`docs/release-notes.md` with migration notes or an explicit no-user-impact
rationale. `make contrib-release-notes-check` is a lightweight review gate for
that policy and requires explicit acknowledgement when report-only drift is
incompatible; it is not a substitute for human compatibility judgment. `make
release-check` and `make release-evidence` include that gate so unacknowledged
incompatible report-only drift cannot be published by the normal release path.

Compatibility-sensitive v3 cleanup stays gated separately from the stable v2
API check. `make v3-readiness-check` runs focused docscheck guardrails for
provider-shaped billing ports, driver-shaped database stats, legacy response
helpers, tokenless idempotency release, unchecked authz construction, checked
list parser shims, and release-note requirements before those surfaces can be
removed in a major version.

## Deprecation policy

- Use `// Deprecated:` Go doc comments with a replacement when possible.
- Deprecated APIs remain for at least one minor release unless a security fix
  requires removal.

## API compatibility checks

CI runs `scripts/apicheck.sh` to detect incompatible changes in the stable
packages. Breaking changes must coincide with a major version bump.

Command intent is deliberately split, and `docs/release-runbook.md` is the
command source of truth. `make api-check` is a local compatibility helper.
`make release-api-check` fails closed unless `API_BASE_REF` names an available supported baseline.
`make v3-readiness-check` runs focused compatibility-sensitive surface guardrails.
`make release-check` is the release-readiness gate.
`make release-evidence` runs the release-readiness subchecks through the evidence writer and writes `release-check-summary.json` schema v2.
`make contrib-api-drift-report` enforces supported-adapter incompatible drift.
`make contrib-release-notes-check` is a lightweight review gate for contrib
adapter, integration, middleware, bootstrap, telemetry, and production generator
CLI behavior notes.

Publication evidence must come from a clean worktree with
`API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-evidence`. A local
dirty-tree audit must opt in with
`ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-evidence`
and is not acceptable before publishing. This is local dirty-tree audit evidence,
not publication evidence.

For exact commands, artifact verification, manifest review, and baseline
maintenance rules, use `docs/release-runbook.md`.

Reviewer helpers remain part of the consolidated path: use
`make release-review-summary` for summary fields,
`make release-artifact-verify-fixture` for local verifier fixtures, and
publication artifact verification with `RELEASE_ARTIFACT_VERIFY_MODE=publication`,
`RELEASE_TAG`, and `GITHUB_REPOSITORY`.

## Behavioral upgrade guidance

- API compatibility checks protect exported identifiers in the stable surface,
  not every runtime contract or operator-facing default.
- API compatibility checks still cover the full `ports` package, including the
  compatibility-sensitive billing and database-stats exports described above.
- Review [docs/release-notes.md](docs/release-notes.md)
  on every upgrade for behavior changes around health endpoint exposure,
  scheduler observability, transaction cleanup, and `contrib` migration state
  handling.
- New helper packages added to support internal refactors should be treated as
  unstable unless they are listed in the stable API surface above.

## Related policies

- Panic usage is governed by `PANIC_POLICY.md`.
