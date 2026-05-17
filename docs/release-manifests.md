# Release Manifests

Audience: release reviewers and maintainers who need to interpret the
machine-readable manifests that support release and documentation checks.

## Manifest ownership

| Manifest | Owner | Review expectation |
| --- | --- | --- |
| `docs/package-classification.tsv` | Maintainers of public API and test posture. | Every root and contrib Go package has one row with API status, test status, and a rationale note. Stable and compatibility-only root packages must match `VERSIONING.md` and `scripts/apicheck.sh`. |
| `docs/supported-adapter-contracts.tsv` | Contrib maintainers and package owners. | Every package classified as `supported-adapter` has an explicit behavior contract and evidence path for direct tests, package docs, and release drift coverage. |
| `docs/contrib-api-drift-packages.txt` | Contrib maintainers. | Lists selected high-use contrib packages reviewed by `make contrib-api-drift-report`; supported-adapter incompatible drift is gate-enforced, experimental and wrapper-only drift remains report-only review evidence, and this does not make contrib stable. |
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
- Keep experimental package notes explicit about the missing promotion evidence, such as absent shared contract suites, health checks, drift coverage, or supported-tier scope.
- Keep supported adapter contracts specific to reusable behavior, not provider marketing claims. Promotion to `supported-adapter` requires direct tests, package docs, release drift coverage, and a row in `docs/supported-adapter-contracts.tsv`.
- Keep contrib drift package selection focused on high-use adapters and integrations; supported-adapter incompatible drift is gate-enforced, and supported package-owned runtime assets plus production generator CLI behavior remain release-note reviewed, but neither rule implies a stable contrib API promise.
- Keep `saas-api-full` full-profile runtime assets release-note reviewed because changes alter the generated production foundation. The opt-in integration checks must stay documented separately from default release gates so Docker-backed Postgres, Redis, and MinIO checks do not become accidental publication prerequisites.
- Keep vulnerability dispositions tied to current evidence. Remove stale advisory rows after dependencies are upgraded and current evidence no longer reports the ID.
