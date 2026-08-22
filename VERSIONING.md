# Versioning and Stability

Audience: API consumers and maintainers who need the stable core surface,
compatibility-sensitive exceptions, and release-command intent.

This project follows semantic versioning for the core module
`github.com/aatuh/api-toolkit/v4`. From v1 onward, we treat the packages listed
below as stable: any breaking change requires a major version bump. Stability
does not imply every exported identifier is perfectly adapter-neutral; some
surfaces are explicitly classified as compatibility-sensitive so the public docs
do not over-claim genericity. `docs/package-classification.tsv` classifies every
root and contrib package so new public packages cannot appear without an
explicit API and test-coverage classification.

The recommended adoption identity is defined in `docs/stable-core.md`: small Go
HTTP API building blocks first, optional scaffold and contrib adapters second.
Some packages below are stable because v3 already promises compatibility, but
they are not all recommended as new generic abstractions.

Public API review artifacts:

- generated inventory: `docs/api-inventory.md`
- review checklist: `docs/api-review-checklist.md`
- stable API review board process: `docs/governance.md`
- example-exception manifest for new stable exports:
  `docs/api-addition-exceptions.tsv`
- options-struct defaults and zero-value review: `docs/options-structs.md`
- deprecation policy and register: `docs/deprecations.md`

## Stable API surface (core module)

All exported identifiers in these packages are considered stable:

- `github.com/aatuh/api-toolkit/v4/apiclient`
- `github.com/aatuh/api-toolkit/v4/apitest`
- `github.com/aatuh/api-toolkit/v4/authorization`
- `github.com/aatuh/api-toolkit/v4/binding`
- `github.com/aatuh/api-toolkit/v4/compat/billing`
- `github.com/aatuh/api-toolkit/v4/contracttest`
- `github.com/aatuh/api-toolkit/v4/email`
- `github.com/aatuh/api-toolkit/v4/endpoints/docs`
- `github.com/aatuh/api-toolkit/v4/endpoints/health`
- `github.com/aatuh/api-toolkit/v4/endpoints/list`
- `github.com/aatuh/api-toolkit/v4/endpoints/pprof`
- `github.com/aatuh/api-toolkit/v4/endpoints/version`
- `github.com/aatuh/api-toolkit/v4/fielderrors`
- `github.com/aatuh/api-toolkit/v4/httpcache`
- `github.com/aatuh/api-toolkit/v4/httpx`
- `github.com/aatuh/api-toolkit/v4/httpx/identity`
- `github.com/aatuh/api-toolkit/v4/httpx/recover`
- `github.com/aatuh/api-toolkit/v4/idempotent`
- `github.com/aatuh/api-toolkit/v4/middleware/auth/apikey`
- `github.com/aatuh/api-toolkit/v4/middleware/auth/authz`
- `github.com/aatuh/api-toolkit/v4/middleware/auth/tenant`
- `github.com/aatuh/api-toolkit/v4/middleware/deprecation`
- `github.com/aatuh/api-toolkit/v4/middleware/idempotency`
- `github.com/aatuh/api-toolkit/v4/middleware/json`
- `github.com/aatuh/api-toolkit/v4/middleware/maxbody`
- `github.com/aatuh/api-toolkit/v4/middleware/querylimits`
- `github.com/aatuh/api-toolkit/v4/middleware/ratelimit`
- `github.com/aatuh/api-toolkit/v4/middleware/secure`
- `github.com/aatuh/api-toolkit/v4/middleware/timeout`
- `github.com/aatuh/api-toolkit/v4/middleware/trace`
- `github.com/aatuh/api-toolkit/v4/negotiation`
- `github.com/aatuh/api-toolkit/v4/operations`
- `github.com/aatuh/api-toolkit/v4/ports`
- `github.com/aatuh/api-toolkit/v4/queryparams`
- `github.com/aatuh/api-toolkit/v4/routecontracts`
- `github.com/aatuh/api-toolkit/v4/routepolicy`
- `github.com/aatuh/api-toolkit/v4/scheduler`
- `github.com/aatuh/api-toolkit/v4/scheduler/migrations`
- `github.com/aatuh/api-toolkit/v4/securityprofile`
- `github.com/aatuh/api-toolkit/v4/specs`
- `github.com/aatuh/api-toolkit/v4/swagstub`
- `github.com/aatuh/api-toolkit/v4/upload`
- `github.com/aatuh/api-toolkit/v4/webhooks`

## Compatibility-sensitive stable sub-surfaces

These exports remain compatibility-shaped, but they are not the recommended
model for new generic boundary design:

- `github.com/aatuh/api-toolkit/v4/compat/billing` keeps the hosted-checkout
  and invoicing compatibility model explicit. New applications should prefer
  app-owned billing ports unless this model is exactly what they need.
- `github.com/aatuh/api-toolkit/v4/scheduler/migrations` remains stable for v3
  migration compatibility. New applications should keep migration orchestration
  app-owned or adapter-owned unless this package's exact model is needed.
- `github.com/aatuh/api-toolkit/v4/swagstub` remains stable for v3 tooling
  compatibility. It is not a recommended runtime abstraction for new
  application code.
- `github.com/aatuh/api-toolkit/v4/ports` is intentionally limited to generic
  logger, clock, and ID utilities. HTTP, persistence, configuration, and
  domain contracts belong with their consuming package or in contrib/contracts.

Compatibility-sensitive means:

- Minor and patch releases must preserve these APIs unless a security fix
  requires otherwise.
- New design work should prefer narrower, plain-value, or app-owned contracts
  instead of widening these surfaces further.
- No new `ports` export is accepted without an accepted design note proving
  adapter neutrality, at least two real implementations, and why the application
  should not own the interface. The design note must explicitly answer why the
  application should not own the interface.
- No new stable root package or stable-package promotion is accepted without a
  public design issue, a public comment window of at least 7 calendar days, and
  maintainer approval recorded through the stable API review board process in
  `docs/governance.md`.
- For the existing hosted-checkout and invoicing model, new code should import
  `github.com/aatuh/api-toolkit/v4/compat/billing` or use app-owned ports.
- The repository should document the migration path before any future major
  cleanup. The current plan lives in `docs/ports-surface.md`.
- The v3 compatibility record is historical. V4 replacement ownership and the
  retained generic ports are recorded in `docs/v4-plan.md` and
  `docs/ports-v4-migration-ledger.tsv`.

## Experimental or unstable surfaces

- Examples, docs, and tooling are not API commitments.
- Any package explicitly documented as experimental is unstable.

## Contrib module policy

The contrib module is outside the stable API compatibility promise.
`make release-api-check` covers only the core module
`github.com/aatuh/api-toolkit/v4`; it does not cover
`github.com/aatuh/api-toolkit/contrib/v4`.

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

Compatibility-sensitive cleanup stays gated separately from the stable API
check. `make v3-readiness-check` runs focused docscheck guardrails for the
completed provider-shaped billing extraction, driver-shaped database stats,
legacy response helper removal, token-aware idempotency release, checked authz
construction, checked list parser APIs, and release-note requirements.

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
`make release-evidence` runs the release-readiness subchecks through the evidence writer and writes `release-check-summary.json` schema v2. It requires `RELEASE_TAG` to resolve to the exact `HEAD` commit and records the tag, commit/tree, default-branch ancestry, module versions, and workflow identity.
`make contrib-api-drift-report` enforces supported-adapter incompatible drift.
`make contrib-release-notes-check` is a lightweight review gate for contrib
adapter, integration, middleware, bootstrap, telemetry, and production generator
CLI behavior notes.

Publication evidence must come from a clean worktree with an explicit baseline
and an immutable release tag at `HEAD`. The tag-driven GitHub release workflow
is the trusted publication producer; local evidence is tag-binding preflight.
Use `docs/release-runbook.md` for the current command examples. The v4
major-release evidence used `API_BASE_REF=v3.1.2` as documented v3-to-v4
transition evidence; v4 patch and minor releases use the latest published v4
tag. A local dirty-tree audit must opt in with
`ALLOW_DIRTY_RELEASE_EVIDENCE=1` and an explicit `API_BASE_REF`; it is not acceptable before publishing. This is local dirty-tree audit evidence, not publication evidence.
Minor releases that touch the stable surface must publish a `vX.Y.0-rc.1`
release candidate before the final stable `vX.Y.0` tag. RC evidence still
compares against the latest published stable v4 tag, not a previous RC.

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
- API compatibility checks still cover the full stable package list, including
  compatibility-shaped packages described above.
- Review [docs/release-notes.md](docs/release-notes.md)
  on every upgrade for behavior changes around health endpoint exposure,
  scheduler observability, transaction cleanup, and `contrib` migration state
  handling.
- New helper packages added to support internal refactors should be treated as
  unstable unless they are listed in the stable API surface above.

## Related policies

- Panic usage is governed by `PANIC_POLICY.md`.
