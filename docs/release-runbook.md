# Release Compatibility Runbook

Audience: release operators and reviewers who need the canonical command
sequence, evidence expectations, artifact verification path, and baseline
maintenance rule.

V4 release publication is paused while the
[v4 release-identity incident](release-incident-v4-release-identity.md) is
open. Do not use `v4.0.0` or `v4.0.1` as `API_BASE_REF` for a new release until
the incident records `VERIFIED_V4_BASE_REF`. The v4 major-release evidence
compared against `v3.1.2` when recording intentional v3-to-v4 breakage
evidence.

Supported Go toolchains: Go 1.25.x is the minimum line and Go 1.26.x is the
current tested line for root and contrib. Release preflight requires both
matrix jobs to pass before the Go 1.26.x release-evidence job starts. Release
and reviewer commands use `GOTOOLCHAIN=local` to ensure the module `go`
directives and GitHub Actions setup stay compatible with the provisioned local
toolchain.

## Baseline maintenance rule

This runbook owns the supported `API_BASE_REF` baseline for releases. For v4
patch and minor releases, set it only to `VERIFIED_V4_BASE_REF` from the open
incident; a latest-published-tag fallback is forbidden. The v4 major-release
evidence used `API_BASE_REF=v3.1.2` as major-version transition evidence. When
the supported v4 baseline changes,
update this line first, then update command examples in `README.md`,
`VERSIONING.md`, `docs/release-review.md`, `docs/release-notes.md`, scripts that
mention the supported baseline, and any release evidence fixtures in the same
change. Do not introduce a second baseline table in another document.

## Release candidate flow

Minor releases that touch the stable surface must publish at least one release
candidate before the final stable tag. Use `vX.Y.0-rc.1` for the first
candidate, then `vX.Y.0-rc.2`, `vX.Y.0-rc.3`, and so on for replacement
candidates. Do not delete or retag release candidates after publication.

A stable-surface touch includes any new stable root package, newly exported
stable identifier, stable package behavior change that adopters may notice, or
compatibility-sensitive deprecation/migration helper intended for the next
minor release. Patch releases and minor releases with no stable-surface touch
may skip the release-candidate path only when the release review records
`No stable surface touch`.

Release candidates use the same clean release evidence path as stable releases:

1. Keep `API_BASE_REF` set to the latest published stable v4 tag, not to a
   previous release candidate.
2. Run `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make release-evidence` from a
   clean worktree.
3. Tag the candidate as `vX.Y.0-rc.1`.
4. Let `.github/workflows/release.yml` create a draft GitHub prerelease with
   `prerelease: true`.
5. Verify draft assets with
   `RELEASE_ASSET_DIR=/path/to/assets RELEASE_TAG=vX.Y.0-rc.1 GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify`.
6. Publish the draft as a prerelease, collect adopter and maintainer feedback,
   and record any required fixes in `docs/release-notes.md`.
7. For each replacement candidate, rerun clean evidence and tag the next
   `vX.Y.0-rc.N`; never mutate an existing RC tag.
8. Promote only by rerunning clean evidence and tagging the final stable
   `vX.Y.0` after release review accepts the candidate feedback.

Do not advance the supported v4 baseline to an RC tag. The baseline moves only
after the final stable `vX.Y.0` release is published.

## Commands

| Command | Intent | Output |
| --- | --- | --- |
| `GOTOOLCHAIN=local make finalize` | Local implementation gate before committing changes; may rewrite formatted Go files and module files. | Pass/fail local quality signal, not release evidence. |
| `GOTOOLCHAIN=local make audit-check` | Non-mutating reviewer/audit gate. | Pass/fail review signal, including `make actions-audit` for pinned GitHub Actions and generated workflow templates, not release evidence. |
| `GOTOOLCHAIN=local make coverage-check` | High-risk package coverage gate. | Pass/fail signal for aggregate coverage and package-specific floors in `scripts/coverage_check.sh`; writes `.ci-result/coverage/summary.md` and `.ci-result/coverage/package-summary.tsv`; use `docs/coverage-hardening-backlog.md` before raising JWT, health/readiness, pgxpool, OpenAPI validation, webhook delivery, or other high-risk floors. |
| `COVERAGE_TREND_RELEASE=vX.Y.Z COVERAGE_TREND_COMMIT=$(git rev-parse --verify HEAD) GOTOOLCHAIN=local make coverage-trend-record` | Record the checked package coverage snapshot in the release pull request before tagging. | Updates `docs/coverage-trend.tsv` and the generated `docs/coverage-trend.md`; `make coverage-trend-check` verifies both files before the release commit is tagged. |
| `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make dependency-report` | Dependency footprint and diff review. | Writes `.ci-result/dependencies/summary.md`, root/contrib module lists, minimal-core package footprint, and added/removed module lists when `API_BASE_REF` is set. Vulnerability status remains owned by `make vuln` and `release-check-summary.json`. |
| `GOTOOLCHAIN=local make actions-audit` | GitHub Actions and generated workflow template audit. | Non-mutating check for full-SHA `uses:` refs, same-line version comments, stale/deprecated action comments, workflow permissions, and generated checkout/setup-go template versions. Follow `docs/governance.md` Action Pin Refresh Policy before merging an action update. Included in `make audit-check`, not release evidence by itself. |
| `GOTOOLCHAIN=local make timeout-determinism-check` | Focused timeout determinism gate. | Repeats the hard-timeout late-write test under normal and race runs, then runs the root timeout/idempotency/rate-limit/scheduler race subset. Included in `make audit-check`, not release evidence by itself. |
| `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make reviewer-gate` | Non-mutating reviewer gate plus release evidence policy preflight. | Runs `make audit-check` and fails the release evidence policy preflight on a dirty tree unless local-audit override is intentionally used outside publication review. |
| `make api-check` | Local compatibility helper with fallback base selection. | Pass/fail or skip local compatibility signal. |
| `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make release-api-check` | Release API compatibility only; fails closed without an explicit supported baseline. | API compatibility evidence for the stable core package list. |
| `API_ADDITIONS_BASE_REF=v4.0.0 GOTOOLCHAIN=local make api-additions-check` | Review gate for new stable exported identifiers. | Fails when additions lack source doc comments, current API inventory entries, compile-checked examples or exact exceptions, and package-tied release notes. |
| `GOTOOLCHAIN=local make v3-readiness-check` | Focused compatibility-sensitive surface guardrails. | Verifies the v3 roadmap, replacement guidance, docs/examples, and release-note requirements for known cleanup surfaces. |
| `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make release-check` | Full release readiness. | Pass/fail release-readiness evidence. |
| `GOTOOLCHAIN=local make full-profile-scaffold-check` | Focused generated `saas-api-full` release signal. | Generates the full scaffold through the CLI tests, then verifies generated `go test ./...`, contracts lint/diff, OpenAPI 3.1, checked-in typed Go client regeneration, resource generation, provider flags, worker wiring, and generated integration workflow assets. |
| `GOTOOLCHAIN=local make generated-integration-check` | Optional Docker-backed generated `saas-api-full` integration evidence. | Generates a temporary full scaffold, runs generated tests, the generated service build target, contract checks, OpenAPI/client checks, and `make integration-check` with Postgres and Redis. Writes `.ci-result/generated-integration/status`, `.ci-result/generated-integration/summary.json`, and the integration log. Not part of `finalize`. |
| `GOTOOLCHAIN=local make generated-integration-check-minio` | Optional Docker-backed generated full scaffold integration evidence with MinIO. | Runs the same generated integration path with the MinIO profile explicitly enabled, including the build, OpenAPI golden, health/docs, write-route, and idempotency replay checks recorded in `summary.json`. Not part of `finalize`. |
| `GOTOOLCHAIN=local make generated-soak-check` | Optional nightly generated full-profile soak evidence. | Generates a temporary `saas-api-full` service, injects an in-process `go test -race` soak that checks `runtime.NumGoroutine` growth, then repeats Docker-backed `make integration-check` cycles for connection-leak evidence. Writes `.ci-result/generated-soak/status`, `summary.json`, `race-soak.log`, `build-and-contracts.log`, and one integration log per cycle. Not part of `finalize`. |
| `GOTOOLCHAIN=local make generated-failure-check` | Optional nightly generated full-profile chaos/failure evidence. | Generates API-key and JWT `saas-api-full` services, injects hermetic failure tests for Redis down, Postgres down, expired API keys, bad JWKS endpoints, and slow downstream hard-timeout behavior, then writes `.ci-result/generated-failure/status`, `summary.json`, and per-auth failure logs. Not part of `finalize`. |
| `GOTOOLCHAIN=local make generated-upgrade-compat-check` | Optional generated-service upgrade compatibility evidence from published v3 baselines. | Generates `saas-api-full` from `GENERATED_UPGRADE_COMPAT_REFS` defaulting to `v3.0.0 v3.1.2`, replaces toolkit modules with the workspace, then runs `go mod tidy`, generated tests, OpenAPI/client checks, contracts lint, and contracts diff for each ref. `GENERATOR_REF` remains a single-ref alias. Not part of `finalize`. |
| `GOTOOLCHAIN=local make upgrade-smoke-check` | Downstream root-core upgrade smoke from the latest v3 tag. | Copies `internal/compatfixtures/rootcore/upgrade_smoke_test.go` into a temporary fixture module pinned to `UPGRADE_SMOKE_BASE_REF` defaulting to `v3.1.2`, runs stable root package tests, replaces `github.com/aatuh/api-toolkit/v4` with the current checkout, tidies, and runs the tests again. CI runs this in the API compatibility job. |
| `GOTOOLCHAIN=local make reference-service-check` | Optional checked-in reference service evidence. | Verifies `examples/reference-saas-api` as an app-owned `saas-api-full` consumer without Docker. Not part of `finalize`; Docker-backed runtime evidence stays in the service-owned `integration-check`. |
| `GOTOOLCHAIN=local make reference-service-coverage` | Optional checked-in reference service coverage diagnostic. | Writes `.ci-result/coverage/reference-service.func` and `.ci-result/coverage/reference-service-summary.md` without folding generated app code into root/contrib aggregate coverage thresholds. Not part of `finalize`. |
| `GOTOOLCHAIN=local make reference-service-load` | Optional checked-in reference service load-smoke baseline. | Runs the reference-service router in-process, writes `.ci-result/reference-service-load/status`, `summary.json`, `summary.md`, and `load-smoke.log`, and records latency, throughput, memory, allocations, and expected missing-API-key failure behavior. Not part of `finalize`. |
| `GOTOOLCHAIN=local make reference-service-evidence` | Optional recorded reference service evidence. | Runs `reference-service-check`, writes `.ci-result/reference-service/status`, `.ci-result/reference-service/summary.json`, and logs. Set `REFERENCE_SERVICE_DOCKER=1` to also run the service-owned Docker `integration-check`; set `REFERENCE_SERVICE_MINIO=1` only when object-storage integration evidence is in scope. Not part of `finalize`. |
| `make github-governance-check` | Optional authenticated GitHub repository settings verification. | Uses `gh api` to verify branch protection, required checks, the sole-maintainer PR/no-bypass rulesets, CodeQL merge protection, force-push/deletion protection, and root `v*` plus contrib `contrib/v*` tag rulesets when `gh` is installed and authenticated; skips cleanly otherwise. |
| `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make release-evidence` | Clean-tree publication evidence gate. | Writes `release-check-summary.json` schema v2, including `api_compatibility.previous_tag`, checked packages, incompatible-change count, ignored-exception count, the expected SPDX-derived root/contrib dependency license report assets, and the generated `release-api-check.log` report path, plus `.ci-result/release-evidence/logs/*.log` and `.ci-result/release-evidence/release-evidence-logs.tgz`; this is the only local command acceptable before publishing. |
| `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make release-evidence` | Local dirty-tree audit evidence. | Writes the same evidence files but records `publication_eligible=false` and `provenance_policy.mode=local_audit`; not acceptable before publishing. |
| `RELEASE_SUMMARY=release-check-summary.json make release-review-summary` | Single reviewer summary. | Prints publication eligibility, git state, provenance policy, vulnerability and contrib dispositions, dependency-license report names, artifact expectations, retained log archive path, and a reject/accept decision from the summary. |
| `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make contrib-api-drift-report` | Selected contrib API drift signal from `docs/contrib-api-drift-packages.txt`. | Prints contrib API drift without making contrib stable; fails on supported-adapter incompatible drift. |
| `CONTRIB_RELEASE_BASE_REF=v4.0.0 GOTOOLCHAIN=local make contrib-release-notes-check` | Review gate for supported contrib adapter/integration/middleware/tooling behavior notes. | Fails when behavior files or package-owned runtime assets changed without `docs/release-notes.md`. |
| `make release-artifact-verify-fixture` | Synthetic local verifier fixture. | Builds a throwaway release asset bundle and runs the local verifier path; this is not publication verification. |
| `RELEASE_ASSET_DIR=/path/to/assets RELEASE_TAG=vX.Y.Z GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify` | Publication draft release artifact verification. | Verifies a supported release-tag identifier, expected asset names, summary publication invariants, `release-asset-manifest.tsv` checksums, retained log archive contents from summary log paths, SBOM signatures/certificates, and online GitHub provenance attestations for every draft release asset. |

## Clean-worktree release preflight

Use this command sequence before publishing:

1. `GOTOOLCHAIN=local make finalize` before committing implementation work when it is safe to allow formatting and module tidying.
2. `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make reviewer-gate` for non-mutating reviewer checks in a shared or dirty worktree.
3. Run `GOTOOLCHAIN=local make coverage-check` when high-risk behavior tests or coverage floors changed; use `docs/coverage-hardening-backlog.md` as the floor-raising checklist. For every release, record the tagged-commit snapshot in the release pull request with `COVERAGE_TREND_RELEASE=vX.Y.Z COVERAGE_TREND_COMMIT=$(git rev-parse --verify HEAD) GOTOOLCHAIN=local make coverage-trend-record`, then run `GOTOOLCHAIN=local make coverage-trend-check`. Use `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make dependency-report` when dependency footprint or module diff evidence is in scope. Use `GOTOOLCHAIN=local make timeout-determinism-check` when timeout middleware behavior or race evidence changed. Use `GOTOOLCHAIN=local make reference-service-coverage` when reviewers need generated-service coverage as separate adoption evidence.
4. Optionally run `GOTOOLCHAIN=local make generated-integration-check` and, when object storage behavior is in release scope, `GOTOOLCHAIN=local make generated-integration-check-minio`.
5. Optionally run `GOTOOLCHAIN=local make generated-soak-check` when reviewers want nightly-style generated full-profile soak evidence for goroutine growth, race-prone caches, and connection leaks.
6. Optionally run `GOTOOLCHAIN=local make generated-failure-check` when reviewers want generated full-profile failure evidence for Redis down, Postgres down, expired API keys, bad JWKS endpoints, and slow downstream timeout behavior.
7. Optionally run `GOTOOLCHAIN=local make generated-upgrade-compat-check` when release reviewers want generated-service upgrade evidence from published v3 baselines.
8. Review `docs/supported-adapter-test-realism.tsv` when supported adapter behavior changes. Run `make supported-adapter-check` with any supported PostgreSQL or Redis adapter change; `real-postgres-pr` and `real-redis-pr` rows require the isolated `make test-postgres` or `make test-redis` contract in pull-request and release workflows.
9. Optionally run `GOTOOLCHAIN=local make reference-service-check`, `GOTOOLCHAIN=local make reference-service-coverage`, `GOTOOLCHAIN=local make reference-service-load`, or `GOTOOLCHAIN=local make reference-service-evidence` when release reviewers want checked-in adoption proof evidence.
10. Optionally run `make github-governance-check` when `gh` can read repository settings.
11. `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make release-evidence` only from a clean worktree to produce local publication evidence.
12. `RELEASE_SUMMARY=release-check-summary.json make release-review-summary` to print the summary decision fields from one command.
13. `make release-artifact-verify-fixture` only when an auditor wants to exercise local verifier behavior without draft release assets.
14. `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make release-evidence` only for local audit context; never publish from this evidence.
15. After the tag workflow uploads and attests the draft release assets, download the draft release assets and run `RELEASE_ASSET_DIR=/path/to/assets RELEASE_TAG=vX.Y.Z GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify`.
16. For a release candidate, use `RELEASE_TAG=vX.Y.0-rc.1` in the same publication verification command and confirm the draft release is marked as a GitHub prerelease.

`make api-check` is intentionally local-development oriented. It may use `GITHUB_BASE_REF`, `HEAD~1`, or skip when no base exists, so it is not release evidence.
`make release-evidence` uses `scripts/release_check_summary.sh --run` so the
summary records exact subcommands, exit codes, durations, log paths, tool
versions, git working-tree state, `publication_eligible`, contrib drift summary,
disposition manifest paths, missing/expired disposition counts,
`dependency_license_evidence`, `full_profile_scaffold_evidence`, publication artifact expectations, release log
archive checksum evidence, and the explicit baseline. Full-profile evidence
includes OpenAPI 3.1 output, typed client generation, resource generator checks,
provider-flag generation, worker wiring, optional integration workflow assets,
and non-blocking opt-in Docker integration status. If
`.ci-result/generated-integration/status` exists from `make
generated-integration-check`, release evidence records that local status and
log path; otherwise it records `not_run_opt_in`.
It fails publication mode when the worktree is dirty. Set
`ALLOW_DIRTY_RELEASE_EVIDENCE=1` only for a local dirty-tree audit.
Automation must require `status=passed`, `publication_eligible=true`,
`provenance_policy.status=passed`, `git_state.dirty=false`, and zero dirty
counts before accepting local publication evidence.

## Signing And Attestation Policy

Release tags, signatures, and attestations have separate meanings:

See [release provenance](provenance.md) for the full consumer verification
command, trusted source-reference policy, attested asset scope, and explicit
limits of this SLSA-style provenance model.
See [reproducible build status](reproducible-builds.md) for the separate
non-claim on bit-for-bit executable reproducibility and the release assets this
workflow actually verifies.

| Item | Policy | Consumer verification |
| --- | --- | --- |
| Git release tag `vX.Y.Z` | Protected by GitHub tag rulesets for `refs/tags/v*`. The release workflow does not currently create a GPG-, SSH-, or Sigstore-signed Git tag, so do not advertise the tag itself as cryptographically signed. | Fetch the tag from GitHub, compare it with the GitHub release target, and rely on the signed and attested release assets below for artifact integrity. |
| Contrib release tag `contrib/vX.Y.Z` | Protected by GitHub tag rulesets for `refs/tags/contrib/v*`. It is not currently a cryptographically signed Git tag. | Fetch the tag from GitHub and compare it with the contrib release target before using matching release evidence. |
| SBOM payloads | `sbom-root.spdx.json` and `sbom-contrib.spdx.json` are signed with keyless Sigstore/cosign in the GitHub release workflow. | Run the per-SBOM `cosign verify-blob` commands in `SECURITY.md`, or run publication mode of `make release-artifact-verify`. |
| SBOM signature and certificate assets | `*.sig` and `*.pem` files are uploaded with the draft release and included in `release-asset-manifest.tsv`. | Verify manifest checksums and then verify the SBOM payloads against their matching signature and certificate. |
| Release asset manifest | `release-asset-manifest.tsv` records SHA-256 checksums for the uploaded release assets. | Run `sha256sum -c release-asset-manifest.tsv` from the downloaded asset directory or use `make release-artifact-verify`. |
| GitHub provenance attestations | Every draft release asset listed in `publication_artifact_expectations.github_attestation_subjects` is attested by the release workflow. | Run `RELEASE_ASSET_DIR=/path/to/assets RELEASE_TAG=vX.Y.Z GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify`; it runs `gh attestation verify` for every expected asset and source reference. |

If cryptographically signed Git tags are added later, update this section, the
release workflow, and docscheck contracts in the same change. Until then,
release trust is based on protected tag creation, clean release evidence,
manifest checksums, keyless SBOM signatures, and GitHub artifact attestations.

## Expected release-check behavior

- The command fails immediately if `API_BASE_REF` is missing.
- The command fails immediately if `API_BASE_REF` does not resolve to a local or fetched supported baseline.
- The command compares all stable packages listed in `VERSIONING.md` through `scripts/apicheck.sh`.
- The command also runs linting, vulnerability checks, gosec, build smoke tests,
  v3 readiness guardrails, docs contracts, coverage-check, unit tests, race
  tests, fuzz smoke tests, and cleanup.
- The docs contracts include `make api-additions-check`, so newly exported
  stable identifiers need doc comments, API inventory rows, compile-checked
  examples or exact exceptions, and package-tied release notes before release
  evidence can pass.
- The command includes `make contrib-release-notes-check`, so incompatible
  report-only contrib drift must have release-note acknowledgement before
  publication evidence can pass.
- The command includes `make full-profile-scaffold-check`, so the generated
  `saas-api-full` scaffold, OpenAPI/contract workflow, generated typed Go
  client, resource generator, provider flags, worker binary, and integration
  workflow stay release-visible without making Docker integration checks
  mandatory.
- The command includes `make v3-readiness-check`, so compatibility-only surface
  removal planning, replacement guidance, and release-note requirements stay
  visible before any major-version cleanup.
- For local release evidence, `make release-evidence` runs the same release
  subchecks through the evidence writer before writing `release-check-summary.json`.

## Evidence artifact tiers

Local release evidence is the developer/auditor tier. It contains:

- `release-check-summary.json` schema v2.
- One check record per `make release-check` subtarget.
- Command lines, exit codes, durations, log availability, log paths, tool
  versions, commit, branch or detached state, dirty flag, staged/unstaged/
  untracked/deleted counts, and `API_BASE_REF`.
- `.ci-result/release-evidence/logs/*.log` for the commands run locally.
- `.ci-result/release-evidence/release-evidence-logs.tgz` for retained log
  review in the GitHub draft release.
- `.ci-result/release-evidence/logs/contrib-api-drift-report.log` for the
  contrib drift review output; supported-adapter incompatible drift is
  gate-enforced, and this does not make contrib stable.
- `docs/contrib-api-drift-dispositions.tsv` for owner, status, review, and
  expiry disposition of current drift packages.
- `docs/release-manifests.md` for the human review guide covering package
  classification, contrib drift, contrib dispositions, and vulnerability
  dispositions.
- `.ci-result/release-evidence/logs/vuln.log` plus
  `release-check-summary.json` `vulnerability_evidence` for called and
  imported-but-not-called `govulncheck` disposition.
- `docs/vulnerability-dispositions.tsv` for owner, review, expiry, and upgrade
  triggers for imported-only vulnerability IDs.
- `vulnerability_evidence.missing_disposition_count` and
  `vulnerability_evidence.expired_disposition_count`, which must both be `0`
  before release review accepts imported-only vulnerability dispositions.
- `contrib_drift.packages`, `contrib_drift.missing_disposition_count`, and
  `contrib_drift.expired_disposition_count`, which dynamically compare the
  current drift report with `docs/contrib-api-drift-dispositions.tsv`.
- `full_profile_scaffold_evidence`, which records the release-blocking
  `full-profile-scaffold-check`, OpenAPI 3.1 full scaffold output, checked-in
  typed `client-check` signal, resource generator check, provider-flag
  generation, generated asset validation, worker check, generated integration
  workflow status, and the opt-in non-blocking `integration-check` status.

Local release evidence does not generate or sign SBOMs. The summary records this
as an artifact tier distinction, not as missing release work. Dirty local
evidence is allowed only with `ALLOW_DIRTY_RELEASE_EVIDENCE=1`; dirty local
evidence is rejected before publishing.

`make release-artifact-verify-fixture` is a synthetic local verifier exercise.
It creates fake SBOM/signature material and a throwaway summary so the local
path can be audited without downloaded draft release assets. It must not be used
as publication evidence. Publication verification still requires downloaded
GitHub draft release assets, `RELEASE_TAG`, `GITHUB_REPOSITORY`, real Sigstore
certificates, and online
attestation checks.

GitHub release workflow evidence is the publication tier. The tag-driven
workflow creates a draft release after clean evidence, SBOMs, signatures, and
provenance attestations for every draft release asset exist; reviewers publish
the draft only after inspecting the assets. It contains the local summary plus
release assets and should be the publication-grade source:

- `sbom-root.spdx.json`
- `sbom-contrib.spdx.json`
- `sbom-root.spdx.json.sig`
- `sbom-root.spdx.json.pem`
- `sbom-contrib.spdx.json.sig`
- `sbom-contrib.spdx.json.pem`
- `release-evidence-logs.tgz`
- `release-asset-manifest.tsv`
- provenance attestations for every draft release asset

Before publishing, verify the draft release asset names against
`release-check-summary.json` `publication_artifact_expectations`, verify
`release-asset-manifest.tsv` checksums, verify SBOM signatures against their
certificates, inspect the retained log archive, and confirm provenance
attestations for every subject in
`publication_artifact_expectations.github_attestation_subjects`. The repository
command for this is
`RELEASE_ASSET_DIR=/path/to/assets RELEASE_TAG=vX.Y.Z GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify`.
The command invokes `scripts/verify-release.sh`, rejects missing or unsupported
release-tag identifiers, parses
`release-check-summary.json` for publication-grade invariants, and confirms
every `checks[].log_path` plus `contrib_drift.artifact_path` exists in
`release-evidence-logs.tgz`.

## Failed compatibility check rollback path

- Do not publish the release.
- If the GitHub release was already drafted, keep it draft-only or delete the draft before assets are published.
- If a tag was pushed prematurely, leave the previous release as the supported version while the incompatible change is reverted or intentionally moved to a new major version plan.
- Rerun `API_BASE_REF=v4.0.0 GOTOOLCHAIN=local make release-check` after the fix and attach the generated release evidence only after it passes.
