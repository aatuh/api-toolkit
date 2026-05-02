## Executive summary
This audit reviewed the current `/home/aatu/projects/saas/api-toolkit` repository after the v37 remediation pass and commit. Scope was the full codebase, with focused attention on dynamic vulnerability and disposition coverage, contrib drift disposition/enforcement, release artifact verification, retained log review, reviewer path consolidation, clean publication evidence, and remaining v3 debt in billing, database stats, response writer, and idempotency compatibility.

The repository is now in a materially stronger release posture than the v37 audit described. Before this audit directory was created, clean publication evidence passed without the dirty-tree override: `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` produced `release-check-summary.json` with `status=passed`, `publication_eligible=true`, `git_state.dirty=false`, and zero staged, unstaged, untracked, or deleted files.

Dynamic disposition handling has also improved. Release evidence now extracts imported-only vulnerability IDs from the retained `govulncheck` log, compares them with `docs/vulnerability-dispositions.tsv`, fails when missing or expired disposition issues exist, and records the result in `vulnerability_evidence`. The current evidence reports `0` called vulnerabilities, `3` imported-but-not-called IDs (`GO-2026-4762`, `GO-2026-4771`, `GO-2026-4772`), and `0` missing or expired vulnerability dispositions.

Contrib drift is still correctly report-only rather than a stable API promise, but it is now much more reviewable. The current drift report found 5 drift packages: 4 compatible and 1 incompatible (`github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/devheaders`). Release evidence compares those packages and drift statuses with `docs/contrib-api-drift-dispositions.tsv`; the current summary reports `0` missing and `0` expired contrib drift dispositions, and `CONTRIB_RELEASE_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-release-notes-check` passed with package-tied release-note acknowledgement.

The main remaining risks are no longer basic release-evidence absence or dirty publication evidence. They are sharper release assurance gaps: online GitHub attestation verification is still optional/manual when `RELEASE_TAG` is not set, `release-artifact-verify` should assert more summary invariants, and retained log verification should cross-check the summary's log paths with the archive rather than only checking a fixed log-name list.

The main architectural debt remains intentionally deferred to v3. `ports/billing.go` still contains provider-shaped deprecated exports, `ports.DatabasePool.Stat` and `ports.DatabaseStats` still keep pgx-shaped compatibility in the generic pool contract, `response_writer` remains a stable legacy package used by contrib middleware, and tokenless idempotency release remains a mixed-version compatibility path even though token-aware release is now preferred and adapter-backed.

## Scorecard
| Dimension                              | Score | Notes |
|----------------------------------------|------:|-------|
| Architecture & boundaries              |  8/10 | Root/contrib boundaries are explicit, root production code is guarded from contrib imports, and release/versioning docs classify stable and compatibility-only surfaces. Provider-shaped billing, driver-shaped database stats, legacy `response_writer`, and tokenless idempotency compatibility keep this below 9. |
| SOLID / cohesion / coupling            |  8/10 | Most ports are narrow and adapter-neutral, and compatibility-sensitive exceptions are documented. Remaining stable v2 surfaces still mix generic ports with provider/driver-specific shapes. |
| Correctness & robustness               |  9/10 | `make finalize`, release API compatibility, contrib drift reporting, contrib release-note checks, and clean release evidence all passed. Release evidence now fails closed on dirty publication provenance and disposition issues. |
| Security                               |  9/10 | `govulncheck` reports `0` called vulnerabilities, gosec reports `0` issues, imported-only vulnerability dispositions are owner/expiry tracked, and missing/expired disposition issues fail release evidence. Optional online attestation verification keeps this below 10. |
| Test effectiveness                     |  9/10 | The repository has broad unit, race, fuzz-smoke, docs contract, API compatibility, contrib review, lint, gosec, and vulnerability checks. The next improvement is stronger contract coverage for release-summary/verifier invariants and parser edge cases. |
| Change safety & backward compatibility |  9/10 | Stable core API compatibility passed against `v2.0.1`; contrib drift remains report-only but summarized and disposition-checked. Remaining v3 cleanup debt constrains future breaking changes. |
| Operability & observability            |  8/10 | Release evidence captures command lines, durations, exit codes, retained logs, git state, tool versions, vulnerability disposition status, contrib drift status, artifact expectations, and checksums. Post-upload attestation verification and archive/summary cross-checking still need tightening. |
| Clarity & developer experience         |  8/10 | `VERSIONING.md`, release runbook/review docs, dependency-risk docs, ports-surface docs, and the v3 roadmap are discoverable and concrete. The reviewer path still spans several files and commands. |
| Extensibility                          |  8/10 | Preferred v2 alternatives exist for billing compatibility, database snapshots, token-aware idempotency release, checked authz construction, and checked list parsing. Extensibility remains limited by stable v2 compatibility surfaces until v3 cleanup executes. |
| Overall                                |  8/10 | Strong release-readiness and compatibility posture, with remaining work focused on publication assurance hardening and executing documented v3 compatibility cleanup. |

Confidence: high

## Findings by severity
### Critical
- None.

### High
- None.

### Medium
- Online provenance attestation verification is still optional outside the tag workflow path. Evidence: `scripts/release_artifact_verify.sh` returns success without checking GitHub attestations when `RELEASE_TAG` is unset, printing guidance instead. `.github/workflows/release.yml` runs `make release-artifact-verify` before uploading draft release assets and before `actions/attest-build-provenance`; docs instruct reviewers to rerun with `RELEASE_TAG` after downloading assets. Risk: the local verifier is strong for checksums, retained logs, and SBOM signatures, but a human can still skip the online attestation verification step before publishing the draft release.
- `release-artifact-verify` verifies asset presence, manifest checksums, log archive contents, and SBOM signatures, but it does not assert key `release-check-summary.json` invariants. Evidence: the verifier greps for expected asset and subject names, then validates checksums, log archive entries, and cosign signatures. It does not parse and require `status=passed`, `publication_eligible=true`, `provenance_policy.status=passed`, `git_state.dirty=false`, the expected `api_base_ref`, zero disposition issue counts, or `sbom_status=generated_and_signed` when SBOM assets are present. Risk: a stale or semantically non-publishable summary could pass bundle verification if it is internally checksummed with the rest of the downloaded assets.
- Retained log review is present but not fully tied to summary check records. Evidence: `release-artifact-verify` checks `release-evidence-logs.tgz` for fixed required log names, including `contrib-api-drift-report.log`; `release-check-summary.json` independently records each check's `log_path`. The verifier does not parse the summary and confirm every `checks[].log_path` and the `contrib_drift.artifact_path` exist in the archive. Risk: a future check rename or added release check can produce summary/log drift without verifier failure.
- Dynamic vulnerability disposition coverage is much improved but depends on text parsing of `govulncheck` verbose output. Evidence: `scripts/release_check_summary.sh` extracts IDs using the `Vulnerability #... GO-...` line shape and separately extracts imported/called counts from prose. Current evidence is consistent, but there is no explicit invariant that imported-only count greater than zero requires a non-empty ID list. Risk: a future govulncheck output format change could reduce disposition coverage unless parser contract tests catch it.
- Remaining v3 compatibility debt is documented but still structurally present. Evidence: `ports/billing.go` keeps deprecated provider-shaped billing exports, `ports.DatabasePool.Stat` and `ports.DatabaseStats` remain in `ports/database.go`, `response_writer` remains a stable legacy package, and `ports.IdempotencyReleaser.Release(ctx, key)` remains for tokenless release compatibility. Risk: these surfaces keep backward compatibility intact for v2 but continue to constrain new design and require careful v3 removal sequencing.

### Low
- Local release artifact verification was not run in this audit because no downloaded GitHub draft release asset directory was provided. Evidence: `make release-artifact-verify` is now available and documented, but it requires `release-check-summary.json`, `release-evidence-logs.tgz`, `release-asset-manifest.tsv`, signed SBOMs, signatures, and certificates in `RELEASE_ASSET_DIR`; local `make release-evidence` does not generate signed SBOMs. Risk: low for this local audit, but publication review still needs the downloaded draft-release asset pass.
- Reviewer guidance is improved but still distributed across `docs/release-runbook.md`, `docs/release-review.md`, `VERSIONING.md`, `docs/dependency-risk.md`, `docs/ports-surface.md`, `docs/v3-compatibility-roadmap.md`, summary JSON, TSV manifests, and retained logs. Risk: low because the docs are concrete; the remaining issue is reviewer friction rather than missing evidence.
- Contrib drift remains intentionally report-only. Evidence: `VERSIONING.md` states contrib is outside the stable v2 API promise and `make contrib-api-drift-report` is report-only. Risk: low as long as release notes and dispositions stay current; consumers may still misread supported contrib adapters as semver-stable API unless release notes are explicit.

## Hexagonal architecture verdict
What is clean: the repository has a clear root/contrib module split. Stable root packages are listed in `VERSIONING.md`, package classification is tracked in `docs/package-classification.tsv`, root production code is guarded from importing contrib, and concrete third-party integrations live under contrib. Core packages such as `authorization`, `httpx`, `fielderrors`, `scheduler`, `specs`, and most middleware stay largely transport- or adapter-neutral.

What leaks across boundaries: the remaining leaks are mostly stable v2 compatibility surfaces rather than accidental new design. `ports/billing.go` is provider-shaped, `ports.DatabaseStats` is pgxpool-shaped, `response_writer` is legacy HTTP response infrastructure, and tokenless idempotency release remains a compatibility path alongside the preferred token-aware release contract.

The codebase is strongly ports-and-adapters oriented, but not purely hexagonal. The current design is pragmatic for v2: it preserves source compatibility while documenting the exceptions and adding preferred APIs. The v3 roadmap is the right place to remove or relocate the remaining compatibility leaks.

## Test verdict
Checks run before creating this audit directory:

- `GOTOOLCHAIN=local make finalize`: passed. It ran tool installation, formatting, linting, vulnerability scanning, gosec, build smoke, docs checks, tidy, unit tests, race tests, fuzz smoke, and clean.
- `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-api-check`: passed.
- `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-api-drift-report`: passed as report-only. It reported 5 drift packages, 4 compatible and 1 incompatible.
- `CONTRIB_RELEASE_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-release-notes-check`: passed. It confirmed package-tied release-note acknowledgement and non-expired disposition coverage for incompatible contrib drift.
- `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence`: passed cleanly. No `ALLOW_DIRTY_RELEASE_EVIDENCE=1` override was needed.

Current clean release evidence records `publication_eligible=true`, `git_state.dirty=false`, `called_vulnerability_count=0`, imported-only vulnerability IDs with `0` missing and `0` expired dispositions, contrib drift with `0` missing and `0` expired dispositions, and retained log paths for every release-evidence check.

What was not run: `make release-artifact-verify` was not run because this local workspace did not contain a downloaded GitHub draft-release asset directory with signed SBOMs, certificates, signatures, and `release-asset-manifest.tsv`. The release workflow does run `make release-artifact-verify` after generating those assets and before uploading them to the draft release.

## Best next fixes
- Add a post-upload draft-release verification path that downloads the draft release assets and runs `make release-artifact-verify` with `RELEASE_TAG` and `GITHUB_REPOSITORY` set, so online attestation verification is not only a manual reviewer instruction.
- Extend `release-artifact-verify` to parse `release-check-summary.json` and require publication-grade invariants: passed status, clean git state, publication eligibility, expected baseline, zero disposition issues, and generated/signed SBOM status where applicable.
- Make retained log verification summary-driven by asserting every summary log path exists in `release-evidence-logs.tgz`.
- Add parser fixture tests for `govulncheck` and contrib drift evidence so imported-only vulnerability IDs and drift package statuses cannot silently disappear after upstream output changes.
- Start executing the v3 compatibility roadmap in small preparatory steps: response capture migration away from legacy `response_writer`, billing extraction guardrails, database stats snapshot-only examples, and tokenless idempotency sunset evidence.

## Optional follow-up
- Execute the remediation backlog in `./.audits/codebase_audit_v38/remediation_backlog.md`.
