# Release Compatibility Runbook

Audience: release operators and reviewers who need the canonical command
sequence, evidence expectations, artifact verification path, and baseline
maintenance rule.

Supported v2 release baseline: `v2.0.1`.

Supported Go toolchain: Go 1.25.x for root and contrib. Release and reviewer
commands use `GOTOOLCHAIN=local` to ensure the module `go` directives and
GitHub Actions setup stay compatible with the provisioned local toolchain.

## Baseline maintenance rule

This runbook owns the supported `API_BASE_REF` baseline for v2 releases. When
the supported baseline changes, update this line first, then update command
examples in `README.md`, `VERSIONING.md`, `docs/release-review.md`,
`docs/release-notes.md`, scripts that mention the supported baseline, and any
release evidence fixtures in the same change. Do not introduce a second
baseline table in another document.

## Commands

| Command | Intent | Output |
| --- | --- | --- |
| `GOTOOLCHAIN=local make finalize` | Local implementation gate before committing changes; may rewrite formatted Go files and module files. | Pass/fail local quality signal, not release evidence. |
| `GOTOOLCHAIN=local make audit-check` | Non-mutating reviewer/audit gate. | Pass/fail review signal, not release evidence. |
| `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make reviewer-gate` | Non-mutating reviewer gate plus release evidence policy preflight. | Runs `make audit-check` and fails the release evidence policy preflight on a dirty tree unless local-audit override is intentionally used outside publication review. |
| `make api-check` | Local compatibility helper with fallback base selection. | Pass/fail or skip local compatibility signal. |
| `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-api-check` | Release API compatibility only; fails closed without an explicit supported baseline. | API compatibility evidence for the stable core package list. |
| `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-check` | Full release readiness. | Pass/fail release-readiness evidence. |
| `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` | Clean-tree publication evidence gate. | Writes `release-check-summary.json` schema v2, `.ci-result/release-evidence/logs/*.log`, and `.ci-result/release-evidence/release-evidence-logs.tgz`; this is the only local command acceptable before publishing. |
| `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` | Local dirty-tree audit evidence. | Writes the same evidence files but records `publication_eligible=false` and `provenance_policy.mode=local_audit`; not acceptable before publishing. |
| `RELEASE_SUMMARY=release-check-summary.json make release-review-summary` | Single reviewer summary. | Prints publication eligibility, git state, provenance policy, vulnerability and contrib dispositions, artifact expectations, retained log archive path, and a reject/accept decision from the summary. |
| `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-api-drift-report` | Selected contrib API drift signal from `docs/contrib-api-drift-packages.txt`. | Prints contrib API drift without making contrib stable; fails on supported-adapter incompatible drift. |
| `CONTRIB_RELEASE_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-release-notes-check` | Review gate for supported contrib adapter/integration/middleware behavior notes. | Fails when behavior files changed without `docs/release-notes.md`. |
| `make release-artifact-verify-fixture` | Synthetic local verifier fixture. | Builds a throwaway release asset bundle and runs the local verifier path; this is not publication verification. |
| `RELEASE_ASSET_DIR=/path/to/assets RELEASE_ARTIFACT_VERIFY_MODE=publication RELEASE_TAG=vX.Y.Z GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify` | Publication draft release artifact verification. | Verifies expected asset names, summary publication invariants, `release-asset-manifest.tsv` checksums, retained log archive contents from summary log paths, SBOM signatures/certificates, and online GitHub provenance attestations. |

## Clean-worktree release preflight

Use this command sequence before publishing:

1. `GOTOOLCHAIN=local make finalize` before committing implementation work when it is safe to allow formatting and module tidying.
2. `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make reviewer-gate` for non-mutating reviewer checks in a shared or dirty worktree.
3. `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` only from a clean worktree to produce local publication evidence.
4. `RELEASE_SUMMARY=release-check-summary.json make release-review-summary` to print the summary decision fields from one command.
5. `make release-artifact-verify-fixture` only when an auditor wants to exercise local verifier behavior without draft release assets.
6. `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` only for local audit context; never publish from this evidence.
7. After the tag workflow uploads and attests the draft release assets, download the draft release assets and run `RELEASE_ASSET_DIR=/path/to/assets RELEASE_ARTIFACT_VERIFY_MODE=publication RELEASE_TAG=vX.Y.Z GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify`.

`make api-check` is intentionally local-development oriented. It may use `GITHUB_BASE_REF`, `HEAD~1`, or skip when no base exists, so it is not release evidence.
`make release-evidence` uses `scripts/release_check_summary.sh --run` so the
summary records exact subcommands, exit codes, durations, log paths, tool
versions, git working-tree state, `publication_eligible`, contrib drift summary,
disposition manifest paths, missing/expired disposition counts, publication
artifact expectations, release log archive checksum evidence, and the explicit
baseline.
It fails publication mode when the worktree is dirty. Set
`ALLOW_DIRTY_RELEASE_EVIDENCE=1` only for a local dirty-tree audit.
Automation must require `status=passed`, `publication_eligible=true`,
`provenance_policy.status=passed`, `git_state.dirty=false`, and zero dirty
counts before accepting local publication evidence.

## Expected release-check behavior

- The command fails immediately if `API_BASE_REF` is missing.
- The command fails immediately if `API_BASE_REF` does not resolve to a local or fetched supported baseline.
- The command compares all stable packages listed in `VERSIONING.md` through `scripts/apicheck.sh`.
- The command also runs linting, vulnerability checks, gosec, build smoke tests, docs contracts, unit tests, race tests, fuzz smoke tests, and cleanup.
- The command includes `make contrib-release-notes-check`, so incompatible
  report-only contrib drift must have release-note acknowledgement before
  publication evidence can pass.
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

Local release evidence does not generate or sign SBOMs. The summary records this
as an artifact tier distinction, not as missing release work. Dirty local
evidence is allowed only with `ALLOW_DIRTY_RELEASE_EVIDENCE=1`; dirty local
evidence is rejected before publishing.

`make release-artifact-verify-fixture` is a synthetic local verifier exercise.
It creates fake SBOM/signature material and a throwaway summary so the local
path can be audited without downloaded draft release assets. It must not be used
as publication evidence. Publication verification still requires downloaded
GitHub draft release assets, `RELEASE_ARTIFACT_VERIFY_MODE=publication`,
`RELEASE_TAG`, `GITHUB_REPOSITORY`, real Sigstore certificates, and online
attestation checks.

GitHub release workflow evidence is the publication tier. The tag-driven
workflow creates a draft release after clean evidence, SBOMs, signatures, and
attestations exist; reviewers publish the draft only after inspecting the assets.
It contains the local summary plus release assets and should be the
publication-grade source:

- `sbom-root.spdx.json`
- `sbom-contrib.spdx.json`
- `sbom-root.spdx.json.sig`
- `sbom-root.spdx.json.pem`
- `sbom-contrib.spdx.json.sig`
- `sbom-contrib.spdx.json.pem`
- `release-evidence-logs.tgz`
- `release-asset-manifest.tsv`
- provenance attestations for the summary and SBOMs

Before publishing, verify the draft release asset names against
`release-check-summary.json` `publication_artifact_expectations`, verify
`release-asset-manifest.tsv` checksums, verify SBOM signatures against their
certificates, inspect the retained log archive, and confirm provenance
attestations for the summary and both SBOMs. The repository command for this is
`RELEASE_ASSET_DIR=/path/to/assets RELEASE_ARTIFACT_VERIFY_MODE=publication RELEASE_TAG=vX.Y.Z GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify`.
Publication mode fails if `RELEASE_TAG` is missing, parses
`release-check-summary.json` for publication-grade invariants, and confirms
every `checks[].log_path` plus `contrib_drift.artifact_path` exists in
`release-evidence-logs.tgz`.

## Failed compatibility check rollback path

- Do not publish the release.
- If the GitHub release was already drafted, keep it draft-only or delete the draft before assets are published.
- If a tag was pushed prematurely, leave the previous release as the supported version while the incompatible change is reverted or intentionally moved to a new major version plan.
- Rerun `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-check` after the fix and attach the generated release evidence only after it passes.
