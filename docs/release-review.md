# Release Review Checklist

Audience: release reviewers who need a short checklist before accepting local
publication evidence or publishing a draft release.

Use this as the short reviewer path before publishing a release.

- Run the command path in `docs/release-runbook.md`; `make finalize` is an
  implementation gate, not release evidence.
- Accept only clean publication evidence before publishing:
  `API_BASE_REF=v3.0.1 GOTOOLCHAIN=local make release-evidence` for v3 patch
  and minor releases.
- A local dirty-tree audit may use
  `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v3.0.1 GOTOOLCHAIN=local make release-evidence`,
  but it is not acceptable before publishing; dirty local evidence is rejected before publishing.
- First v3 major-release evidence may use `API_BASE_REF=v2.1.0` only as
  documented v2-to-v3 transition evidence; later v3 releases compare against
  the latest published v3 tag.
- Read `release-check-summary.json` and confirm `api_base_ref`, `commit`,
  `git_state`, `publication_eligible`, `provenance_policy`, check statuses,
  tool versions, `vulnerability_evidence`, `contrib_drift`,
  `full_profile_scaffold_evidence`, and artifact tier status.
- Run `RELEASE_SUMMARY=release-check-summary.json make release-review-summary`
  to print the same decision fields from one command before walking the detailed
  evidence.
- If draft release assets are unavailable and you only need to exercise verifier
  behavior locally, run `make release-artifact-verify-fixture`. Treat it as a
  synthetic fixture only, not publication evidence.
- Review `.ci-result/release-evidence/logs/contrib-api-drift-report.log` and the
  `contrib_drift` summary; compare drift packages with
  `docs/contrib-api-drift-dispositions.tsv`. Supported-adapter incompatible
  drift is gate-enforced; experimental and wrapper-only drift remains
  report-only review evidence. This does not make contrib stable.
- Use `docs/release-manifests.md` for the human guide to
  `docs/package-classification.tsv`, `docs/contrib-api-drift-packages.txt`,
  `docs/contrib-api-drift-dispositions.tsv`, and
  `docs/vulnerability-dispositions.tsv`.
- Check `.ci-result/release-evidence/logs/vuln.log` and
  `docs/dependency-risk.md` plus `docs/vulnerability-dispositions.tsv` when
  `vulnerability_evidence` reports imported-but-not-called findings.
- Treat local evidence carefully: only clean publication evidence is acceptable
  before publishing. GitHub release workflow evidence is the publication-grade source
  for signed SBOMs, certificates, and provenance attestations.
- Check `docs/release-notes.md` for behavior changes, upgrade notes, and any
  incompatible contrib drift acknowledgement.
- Check `VERSIONING.md` for the stable core package list and command intent.
- Check `docs/package-classification.tsv` for public package API and test
  posture changes.
- Check `docs/ports-surface.md` and `docs/v3-compatibility-roadmap.md` for
  completed v3 cleanup decisions and remaining compatibility-sensitive
  guardrails.
- Download the GitHub draft release assets into one directory and run
  `RELEASE_ASSET_DIR=/path/to/assets RELEASE_ARTIFACT_VERIFY_MODE=publication RELEASE_TAG=vX.Y.Z GITHUB_REPOSITORY=aatuh/api-toolkit make release-artifact-verify`.
  Publication mode requires online GitHub provenance attestation verification.

## Dirty-tree decision

Inspect `release-check-summary.json` `status`, `publication_eligible`,
`provenance_policy`, and `git_state`:

| State | Decision |
| --- | --- |
| `status=passed`, `publication_eligible=true`, `provenance_policy.status=passed`, `dirty=false`, and all staged, unstaged, untracked, and deleted counts are `0` | Accept as local publication evidence if every check passed and artifact review is complete. |
| `dirty=true` with any staged, unstaged, untracked, or deleted count | Reject for publication. Use only as local dirty-tree audit evidence when `provenance_policy.mode=local_audit`. |
| `ALLOW_DIRTY_RELEASE_EVIDENCE=1` present in `evidence_command` | Treat as local audit evidence, not release evidence. |

## Publication artifact review

Use `release-check-summary.json` `publication_artifact_expectations` as the
asset name checklist. Local evidence is complete only for checks and logs; the
GitHub draft release is the publication-grade source for signed artifacts.

Required draft release assets:

- `release-check-summary.json`
- `release-evidence-logs.tgz`
- `release-asset-manifest.tsv`
- `sbom-root.spdx.json`
- `sbom-contrib.spdx.json`
- `sbom-root.spdx.json.sig`
- `sbom-root.spdx.json.pem`
- `sbom-contrib.spdx.json.sig`
- `sbom-contrib.spdx.json.pem`

Verification checklist:

- Confirm the draft release assets match the expected names in
  `publication_artifact_expectations.github_draft_release_assets`.
- Verify `release-asset-manifest.tsv` with `sha256sum -c` so changed or missing
  assets are detected before publishing.
- Verify SBOM signatures against their certificates before publishing. The
  release workflow and `make release-artifact-verify` use
  `COSIGN_CERTIFICATE_IDENTITY_REGEXP=^https://github.com/aatuh/api-toolkit/\.github/workflows/release\.yml@refs/tags/v.*$`
  and `COSIGN_CERTIFICATE_OIDC_ISSUER=https://token.actions.githubusercontent.com`.
- Confirm GitHub provenance attestations exist for `release-check-summary.json`,
  `sbom-root.spdx.json`, and `sbom-contrib.spdx.json`.
- Download `release-evidence-logs.tgz` and confirm it contains the logs named by
  the summary check records. `make release-artifact-verify` now enforces every
  `checks[].log_path` and `contrib_drift.artifact_path` from the summary.

## Vulnerability and contrib disposition review

- `vulnerability_evidence.called_vulnerability_count` must be `0`.
- Every `vulnerability_evidence.imported_not_called_ids` entry must exist in
  `docs/vulnerability-dispositions.tsv` with owner, review date, expiry, and
  upgrade trigger; `missing_disposition_count` and `expired_disposition_count`
  must both be `0`.
- `contrib_drift.incompatible_drift_count>0` requires a package-tied release
  note and a matching non-expired row in
  `docs/contrib-api-drift-dispositions.tsv`; `contrib_drift.packages`,
  `missing_disposition_count`, and `expired_disposition_count` are the summary
  source of truth.
- `full_profile_scaffold_evidence.scaffold_validation.status` must be `passed`
  for publication evidence. Also review
  `openapi_31_full_scaffold.status`, `typed_client_generation.status`,
  `resource_generator_check.status`, `provider_flag_generation.status`,
  `asset_validation.status`, `worker_check.status`, and
  `integration_workflow.status`; these are covered by the same release-blocking
  scaffold contract. `integration_check.status` is opt-in and non-blocking
  unless release reviewers explicitly ran generated Docker integration checks
  and set `FULL_PROFILE_INTEGRATION_CHECK_STATUS`.

## Known v3 cleanup status

| Surface | Status for v3 release review | Reference |
| --- | --- | --- |
| `compat/billing` | Explicit hosted-checkout compatibility package; provider-shaped billing no longer belongs in generic `ports`. | `docs/ports-surface.md` |
| Database pool stats | Generic code should use plain-value snapshot APIs or adapter-owned stats; driver-shaped counters stay out of generic contracts. | `docs/ports-surface.md` |
| Legacy response helpers | Public `response_writer` was removed from v3; use `httpx` and package-local recorders. | `docs/v3-compatibility-roadmap.md` |
| Idempotency release | Token-aware reservation release is the supported path; release evidence keeps redaction and replay behavior visible. | `docs/v3-compatibility-roadmap.md` |
| Authz constructors and list parsers | Checked startup validation and checked parser APIs are the documented paths for new code. | `docs/v3-compatibility-roadmap.md` |

## Evidence examples

| Summary state | Reviewer action |
| --- | --- |
| `status=passed`, `publication_eligible=true`, `provenance_policy.status=passed`, `git_state.dirty=false` | Accept as local publication evidence after artifact and release-note review. |
| `status=passed`, `publication_eligible=false`, `provenance_policy.status=allowed_dirty_local_audit` | Use for audit context only; rerun clean before publishing. |
| `contrib_drift.incompatible_drift_count>0` | Confirm supported-adapter incompatible drift was resolved or explicitly accepted by a major-release policy decision; for report-only experimental or wrapper-only drift, confirm `docs/release-notes.md` acknowledges the affected package. |
| `vulnerability_evidence.imported_not_called_vulnerability_count>0` | Review `docs/dependency-risk.md`, `docs/vulnerability-dispositions.tsv`, and the vuln log; do not fail solely because findings are imported but not called. |
| `sbom_status=not_generated` in local evidence | Expected locally; verify draft-release SBOMs, signatures, certificates, and attestations in GitHub before publishing. |
