# Release Manifests

Audience: release reviewers and maintainers who need to interpret the
machine-readable manifests that support release and documentation checks.

## Manifest ownership

| Manifest | Owner | Review expectation |
| --- | --- | --- |
| `docs/package-classification.tsv` | Maintainers of public API and test posture. | Every root and contrib Go package has one row with API status, test status, and a rationale note. Stable and compatibility-only root packages must match `VERSIONING.md` and `scripts/apicheck.sh`. |
| `docs/contrib-api-drift-packages.txt` | Contrib maintainers. | Lists selected high-use contrib packages reviewed by `make contrib-api-drift-report`; the report is a review signal and does not make contrib stable. |
| `docs/contrib-api-drift-dispositions.tsv` | Release reviewers and package owners. | Current drift packages need status, reason, release-note acknowledgement, review date, expiry date, and owner. Incompatible drift must have a package-tied release note. |
| `docs/vulnerability-dispositions.tsv` | Security and release reviewers. | Imported-but-not-called vulnerability IDs need owning dependency, affected module/package, called status, review date, expiry date, owner, and upgrade trigger. The file can be header-only when current imported-only evidence is zero. |

## Owner and expiry rules

- `owner` must name a team or accountable maintainer group, not an individual workstation state.
- `reviewed_on` and `expires_on` use `YYYY-MM-DD`.
- Expired rows block release evidence until the owner either refreshes the review, removes obsolete rows, or upgrades the affected dependency/package.
- Missing rows for current contrib drift or imported-only vulnerabilities block release evidence through `missing_disposition_count`.
- Header-only `docs/vulnerability-dispositions.tsv` is valid only while current `govulncheck` evidence reports zero imported-but-not-called IDs.

## Reviewer workflow

1. Start with [release-review.md](release-review.md) and [release-runbook.md](release-runbook.md).
2. Inspect `release-check-summary.json` fields for `contrib_drift` and `vulnerability_evidence`.
3. Compare the current summary packages and advisory IDs with the TSV manifests.
4. Confirm all missing and expired disposition counts are `0` before accepting release evidence.
5. Check [release-notes.md](release-notes.md) for package-tied acknowledgement when `contrib_drift.incompatible_drift_count` is greater than `0`.

## Maintenance notes

- Keep package-classification notes short but specific enough for future reviewers to understand why smoke, generated, tooling, test-support, or excluded status is acceptable.
- Keep contrib drift package selection focused on high-use adapters and integrations; do not use it to imply a stable contrib API promise.
- Keep vulnerability dispositions tied to current evidence. Remove stale advisory rows after dependencies are upgraded and current evidence no longer reports the ID.
