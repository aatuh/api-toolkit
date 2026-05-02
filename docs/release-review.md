# Release Review Checklist

Use this as the short reviewer path before publishing a release.

- Run the command path in `docs/release-runbook.md`; `make finalize` is an
  implementation gate, not release evidence.
- Accept only clean publication evidence before publishing:
  `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence`.
- A local dirty-tree audit may use
  `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence`,
  but it is not acceptable before publishing; dirty local evidence is rejected before publishing.
- Read `release-check-summary.json` and confirm `api_base_ref`, `commit`,
  `git_state`, `publication_eligible`, `provenance_policy`, check statuses,
  tool versions, `vulnerability_evidence`, `contrib_drift`, and artifact tier
  status.
- Review `.ci-result/release-evidence/logs/contrib-api-drift-report.log` and the
  `contrib_drift` summary; compare drift packages with
  `docs/contrib-api-drift-dispositions.tsv`. Contrib drift is report-only and
  does not make contrib stable.
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
  compatibility-sensitive v2 surfaces and pending major-version cleanup.
- Download the GitHub draft release assets into one directory and run
  `RELEASE_ASSET_DIR=/path/to/assets make release-artifact-verify`. Set
  `RELEASE_TAG` and `GITHUB_REPOSITORY` as well when verifying GitHub provenance
  attestations online.

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
  the summary check records.

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

## Known v2 debt status

| Surface | Status for v2 release review | V3 reference |
| --- | --- | --- |
| `compat/billing` and deprecated `ports/billing.go` aliases | Stable v2 compatibility surface; new code should prefer `compat/billing` or app-owned billing ports. | `docs/v3-compatibility-roadmap.md` |
| `DatabasePool.Stat` and `DatabaseStats` | Stable v2 compatibility surface; prefer plain-value snapshot APIs. | `docs/v3-compatibility-roadmap.md` |
| `response_writer` | Stable but legacy; prefer `httpx`. | `docs/v3-compatibility-roadmap.md` |
| Tokenless idempotency release | Mixed-version v2 recovery shim; prefer token-aware reservation release. | `docs/v3-compatibility-roadmap.md` |
| Authz unchecked constructor | V2 source-compatible constructor; prefer checked startup validation. | `docs/v3-compatibility-roadmap.md` |
| List parser unchecked helpers | V2 source-compatible helpers; prefer checked parser APIs when validation matters. | `docs/v3-compatibility-roadmap.md` |

## Evidence examples

| Summary state | Reviewer action |
| --- | --- |
| `status=passed`, `publication_eligible=true`, `provenance_policy.status=passed`, `git_state.dirty=false` | Accept as local publication evidence after artifact and release-note review. |
| `status=passed`, `publication_eligible=false`, `provenance_policy.status=allowed_dirty_local_audit` | Use for audit context only; rerun clean before publishing. |
| `contrib_drift.incompatible_drift_count>0` | Confirm `docs/release-notes.md` explicitly acknowledges incompatible report-only contrib drift. |
| `vulnerability_evidence.imported_not_called_vulnerability_count>0` | Review `docs/dependency-risk.md`, `docs/vulnerability-dispositions.tsv`, and the vuln log; do not fail solely because findings are imported but not called. |
| `sbom_status=not_generated` in local evidence | Expected locally; verify draft-release SBOMs, signatures, certificates, and attestations in GitHub before publishing. |
