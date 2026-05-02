# Versioning and Stability

This project follows semantic versioning for the core module
`github.com/aatuh/api-toolkit/v2`. From v1 onward, we treat the packages listed
below as stable: any breaking change requires a major version bump. Stability
does not imply every exported identifier is perfectly adapter-neutral; some v2
surfaces are explicitly classified as compatibility-sensitive so the public docs
do not over-claim genericity. `docs/package-classification.tsv` classifies every
root and contrib package so new public packages cannot appear without an
explicit API and test-coverage classification.

## Stable API surface (core module)

All exported identifiers in these packages are considered stable:

- `github.com/aatuh/api-toolkit/v2/authorization`
- `github.com/aatuh/api-toolkit/v2/compat/billing`
- `github.com/aatuh/api-toolkit/v2/email`
- `github.com/aatuh/api-toolkit/v2/endpoints/docs`
- `github.com/aatuh/api-toolkit/v2/endpoints/health`
- `github.com/aatuh/api-toolkit/v2/endpoints/list`
- `github.com/aatuh/api-toolkit/v2/endpoints/pprof`
- `github.com/aatuh/api-toolkit/v2/endpoints/version`
- `github.com/aatuh/api-toolkit/v2/fielderrors`
- `github.com/aatuh/api-toolkit/v2/httpx`
- `github.com/aatuh/api-toolkit/v2/httpx/identity`
- `github.com/aatuh/api-toolkit/v2/httpx/recover`
- `github.com/aatuh/api-toolkit/v2/middleware/auth/authz`
- `github.com/aatuh/api-toolkit/v2/middleware/auth/jwt`
- `github.com/aatuh/api-toolkit/v2/middleware/auth/tenant`
- `github.com/aatuh/api-toolkit/v2/middleware/idempotency`
- `github.com/aatuh/api-toolkit/v2/middleware/json`
- `github.com/aatuh/api-toolkit/v2/middleware/maxbody`
- `github.com/aatuh/api-toolkit/v2/middleware/querylimits`
- `github.com/aatuh/api-toolkit/v2/middleware/ratelimit`
- `github.com/aatuh/api-toolkit/v2/middleware/secure`
- `github.com/aatuh/api-toolkit/v2/middleware/timeout`
- `github.com/aatuh/api-toolkit/v2/middleware/trace`
- `github.com/aatuh/api-toolkit/v2/ports`
- `github.com/aatuh/api-toolkit/v2/response_writer`
- `github.com/aatuh/api-toolkit/v2/scheduler`
- `github.com/aatuh/api-toolkit/v2/scheduler/migrations`
- `github.com/aatuh/api-toolkit/v2/securityprofile`
- `github.com/aatuh/api-toolkit/v2/specs`
- `github.com/aatuh/api-toolkit/v2/swagstub`

## Compatibility-sensitive stable sub-surfaces

These exports remain part of the v2 compatibility promise, but they are not the
recommended model for new generic boundary design:

- `github.com/aatuh/api-toolkit/v2/ports` billing contracts in
  `ports/billing.go` are stable in v2 but intentionally Stripe-shaped today.
  They are deprecated in favor of `github.com/aatuh/api-toolkit/v2/compat/billing`,
  which is the explicit v2 compatibility import path for that model.
- `github.com/aatuh/api-toolkit/v2/ports` database stats contracts in
  `ports/database.go`, including `DatabasePool.Stat` and `DatabaseStats`, are
  stable in v2 but intentionally mirror pgxpool-style counters today.
- `github.com/aatuh/api-toolkit/v2/response_writer` is also stable but legacy.

Compatibility-sensitive means:

- Minor and patch releases must preserve these APIs unless a security fix
  requires otherwise.
- New design work should prefer narrower, plain-value, or app-owned contracts
  instead of widening these surfaces further.
- For the existing hosted-checkout and invoicing model, new v2 code should
  import `github.com/aatuh/api-toolkit/v2/compat/billing` instead of importing
  billing contracts from `ports`.
- The repository should document the migration path before any future major
  cleanup. The current plan lives in `docs/ports-surface.md`.
- The current v3 cleanup checklist covers the deprecated `ports/billing.go`
  aliases, `DatabasePool.Stat`/`DatabaseStats`, and the legacy
  `response_writer` package.

## Experimental or unstable surfaces

- `github.com/aatuh/api-toolkit/v2/middleware/auth/shared` is an implementation-sharing package for auth middleware and is not part of the stable compatibility promise.
- `github.com/aatuh/api-toolkit/v2/response_writer` is legacy, but it remains part of the stable compatibility promise until explicitly removed in a major release.
- Examples, docs, and tooling are not API commitments.
- Any package explicitly documented as experimental is unstable.

## Contrib module policy

The contrib module is outside the stable API compatibility promise for v2.
`make release-api-check` covers only the core module
`github.com/aatuh/api-toolkit/v2`; it does not cover
`github.com/aatuh/api-toolkit/contrib/v2`.

`docs/package-classification.tsv` classifies contrib packages as experimental,
wrapper-only, test-only, example-only, generated, tooling, or excluded. Adapters
and middleware in contrib are supported as maintained implementations, but their
exported Go API may change in minor releases when adapter dependencies,
provider behavior, or security requirements make that necessary. Integrations
are convenience wrappers and have no independent compatibility contract beyond
the adapter behavior they delegate to. Examples, generated example code,
commands, and test-support packages are not public API commitments.

If a contrib package is promoted to stable in a future release line, add a
`release-contrib-api-check` gate before changing its classification to stable.
Until then, `make contrib-api-drift-report` is intentionally report-only: it
compares selected high-use contrib adapters and integrations against an explicit
baseline so maintainers can review drift, release notes, and migration guidance
without turning contrib into stable API. Local release evidence archives this
report and summarizes compatible and incompatible drift counts for reviewer
visibility.

Contrib adapter and integration changes that alter public behavior should update
`docs/release-notes.md`. `make contrib-release-notes-check` is a lightweight
review gate for that policy and requires explicit acknowledgement when
report-only drift is incompatible; it is not a substitute for human
compatibility judgment. `make release-check` and `make release-evidence` include
that gate so unacknowledged incompatible report-only drift cannot be published by
the normal release path.

## Deprecation policy

- Use `// Deprecated:` Go doc comments with a replacement when possible.
- Deprecated APIs remain for at least one minor release unless a security fix
  requires removal.

## API compatibility checks

CI runs `scripts/apicheck.sh` to detect incompatible changes in the stable
packages. Breaking changes must coincide with a major version bump.

Command intent is deliberately split. `make api-check` is a local compatibility helper. `make release-api-check` fails closed unless `API_BASE_REF` names an available supported baseline. `make release-check` is the release-readiness gate.
`make release-evidence` runs the release-readiness subchecks through the evidence
writer and writes `release-check-summary.json` schema v2 with per-check command
lines, exit codes, durations, log paths, tool versions, git working-tree state,
contrib drift summary, vulnerability evidence, and artifact tier metadata.
Publication evidence must come from a clean worktree:
`API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence`. A local dirty-tree audit must opt in with `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence`
and is not acceptable before publishing.

| Command | Intent | Mutates files | Requires explicit release baseline |
| --- | --- | --- | --- |
| `make finalize` | Local implementation quality gate before committing code. | Yes: may run formatting and module tidy steps. | No |
| `make audit-check` | Non-mutating reviewer or audit gate. | No | No |
| `make api-check` | Local compatibility helper; accepts `API_BASE_REF`, then falls back to `GITHUB_BASE_REF`, then `HEAD~1`, then skip. | No | No |
| `make release-api-check` | Fail-closed API compatibility check for the stable core package list. | No | Yes: use `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-api-check` |
| `make release-check` | Full release-readiness gate. | No | Yes: use `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-check` |
| `make release-evidence` | Clean-tree publication evidence gate plus local `release-check-summary.json` evidence generation. | Writes `release-check-summary.json` and `.ci-result/release-evidence/logs/*.log`. | Yes: use `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` |
| `ALLOW_DIRTY_RELEASE_EVIDENCE=1 make release-evidence` | Local dirty-tree audit evidence only. | Writes the same evidence files but records `provenance_policy.mode=local_audit`. | Yes, and not acceptable before publishing. |
| `make contrib-api-drift-report` | Report-only contrib API drift signal for selected high-use contrib packages. | No | Yes: use `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-api-drift-report` |
| `make contrib-release-notes-check` | Review gate requiring release notes when contrib adapter or integration behavior files change. | No | No, but accepts `CONTRIB_RELEASE_BASE_REF` or `API_BASE_REF` |

## Behavioral upgrade guidance

- API compatibility checks protect exported identifiers in the stable surface,
  not every runtime contract or operator-facing default.
- API compatibility checks still cover the full `ports` package, including the
  compatibility-sensitive billing and database-stats exports described above.
- Review [docs/release-notes.md](/home/aatu/projects/saas/api-toolkit/docs/release-notes.md)
  on every upgrade for behavior changes around health endpoint exposure,
  scheduler observability, transaction cleanup, and `contrib` migration state
  handling.
- New helper packages added to support internal refactors should be treated as
  unstable unless they are listed in the stable API surface above.

## Related policies

- Panic usage is governed by `PANIC_POLICY.md`.
