## Executive summary
This audit reviewed `/home/aatu/projects/saas/api-toolkit` after the v38 remediation pass, parser fix, and clean release-evidence commit. The review covered the full repository with emphasis on release evidence, contrib drift parsing and dispositions, vulnerability dispositions, artifact verification, retained logs, reviewer path consolidation, and remaining v3 compatibility debt.

The release posture is now materially stronger than the v38 audit. Before creating this audit directory, `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence` passed cleanly with `publication_eligible=true`, `provenance_policy.status=passed`, `git_state.dirty=false`, and zero staged, unstaged, untracked, or deleted files. `RELEASE_SUMMARY=release-check-summary.json make release-review-summary` also returned `review_decision: accept-local-evidence`.

The parser fix is effective for the current mixed contrib drift output. `make contrib-api-drift-report` reported 5 drift packages: 4 compatible and 1 incompatible. The incompatible `github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/devheaders` package had both incompatible comparability changes and compatible added fields, and `release-check-summary.json` recorded that package as `status=incompatible`.

Dynamic vulnerability and contrib disposition coverage now fails closed enough for release review. The current evidence reports 0 called vulnerabilities, 3 imported-but-not-called IDs (`GO-2026-4762`, `GO-2026-4771`, `GO-2026-4772`), and 0 missing or expired vulnerability dispositions. Contrib drift disposition coverage reports 0 missing and 0 expired dispositions, and `contrib-release-notes-check` passed with package-tied acknowledgement for incompatible report-only drift.

The previous release artifact and retained-log assurance gaps are largely remediated. `scripts/release_artifact_verify.sh` now verifies required asset names, checksum manifest contents, summary invariants, summary-driven retained log paths, SBOM signatures, and publication-mode GitHub attestations. The release workflow uploads draft assets, attests the summary and SBOMs, downloads the uploaded draft release assets, and runs `make release-artifact-verify` in publication mode.

The main remaining engineering risk is now mostly planned v3 debt rather than release-evidence absence. Provider-shaped billing types remain deprecated but stable in `ports`, database stats still expose pgx-shaped compatibility, `response_writer` remains a legacy stable surface, and tokenless idempotency release remains available for mixed-version recovery.

## Scorecard
| Dimension                              | Score | Notes |
|----------------------------------------|------:|-------|
| Architecture & boundaries              |  8/10 | Root and contrib boundaries are explicit, stable surfaces are documented, and production root imports from contrib are guarded. Provider-shaped billing, driver-shaped database stats, legacy `response_writer`, and tokenless idempotency compatibility still keep the design from being fully clean hexagonal architecture. |
| SOLID / cohesion / coupling            |  8/10 | Most ports are small and adapter-neutral, and compatibility exceptions are named. The remaining stable v2 compatibility surfaces still couple generic contracts to provider, driver, or legacy HTTP helper shapes. |
| Correctness & robustness               |  9/10 | `make finalize`, release API compatibility, contrib drift reporting, contrib release-note checks, release evidence, and reviewer summary all passed. Release evidence now rejects dirty publication provenance and disposition gaps. |
| Security                               |  9/10 | `govulncheck` reports 0 called vulnerabilities, gosec reports 0 issues, imported-only vulnerabilities are dynamically dispositioned, and publication asset verification now checks signatures and attestations. This stays below 10 because known imported-only vulnerable modules remain in the dependency graph. |
| Test effectiveness                     |  9/10 | Unit, race, fuzz-smoke, docs contracts, API compatibility, release evidence parser contracts, artifact verifier contracts, lint, gosec, and vulnerability checks are all active. One useful addition is an explicit mixed same-package contrib drift parser fixture. |
| Change safety & backward compatibility |  9/10 | Stable core API compatibility passed against `v2.0.1`; contrib drift remains report-only but is summarized, disposition-checked, and release-note-gated when incompatible. Remaining v3 debt is documented and intentionally deferred. |
| Operability & observability            |  9/10 | Release evidence captures command lines, durations, exit codes, log paths, git state, vulnerability evidence, contrib drift evidence, artifact expectations, checksums, and reviewer-summary output. Publication artifact verification is now workflow-backed. |
| Clarity & developer experience         |  9/10 | `docs/release-review.md` and `make release-review-summary` consolidate the reviewer path, and `VERSIONING.md`, runbook, release notes, dependency-risk, ports-surface, and v3 roadmap explain the current state clearly. |
| Extensibility                          |  8/10 | Preferred v2 alternatives exist for billing, database snapshots, token-aware idempotency, checked authz, checked list parsing, and `httpx`. Extensibility remains constrained until the compatibility shims can be removed or moved in v3. |
| Overall                                |  9/10 | Strong release-readiness and compatibility posture with remaining work concentrated in dependency burn-down, contrib drift hardening, and planned v3 cleanup. |

Confidence: high

## Findings by severity
### Critical
- None.

### High
- None.

### Medium
- Imported-only vulnerable dependency versions remain present even though release evidence disposition passes. Evidence: `GOTOOLCHAIN=local make finalize` ran `govulncheck`; root had no vulnerabilities, while contrib reported 0 called vulnerabilities and 3 imported-only findings: `GO-2026-4762`, `GO-2026-4771`, and `GO-2026-4772`. The generated summary records those same IDs with `missing_disposition_count=0` and `expired_disposition_count=0`. The govulncheck output listed fixes for `github.com/jackc/pgx/v5` and `google.golang.org/grpc`. Risk: not an active called-vulnerability finding, but release review still carries recurring disposition burden and future code or dependency changes could turn an imported-only advisory into a called finding.
- Remaining v3 compatibility debt is still structurally present in stable surfaces. Evidence: `ports/billing.go` keeps deprecated Stripe-shaped billing exports while `compat/billing` aliases them, `ports.DatabasePool.Stat` and `ports.DatabaseStats` remain in the generic pool contract, `response_writer` is still a stable legacy package, and `ports.IdempotencyReleaser.Release(ctx, key)` remains alongside token-aware `ReleaseReservation(ctx, key, token)`. Risk: v2 compatibility is preserved, but new design still has to route around provider-specific billing, driver-shaped database stats, legacy response capture, and tokenless recovery paths.

### Low
- The current incompatible contrib drift is correctly parsed and enforced, but it is still a consumer-facing report-only drift event. Evidence: `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-api-drift-report` reported `drift_packages=5`, `compatible_drift_packages=4`, and `incompatible_drift_packages=1`; `devheaders` contained both incompatible comparability changes and compatible added fields. `release-check-summary.json` recorded `devheaders` as `incompatible`, and `contrib-release-notes-check` passed with package-tied acknowledgement and non-expired disposition coverage. Risk: consumers may still be surprised if they have treated maintained contrib middleware as semver-stable despite the documented report-only policy.
- Parser contract coverage would be stronger with an explicit mixed same-package contrib drift fixture. Evidence: `scripts/release_evidence_parser_contract_test.sh` covers compatible-only drift, incompatible-only drift, no drift, skipped packages, malformed headings, and fail-closed govulncheck output. The live `devheaders` report validates a package with both `Incompatible changes:` and `Compatible changes:`, but the contract fixture does not make that mixed shape permanent. Risk: once the current live drift disappears, a future parser regression could lose this exact case without a fixture catching it.
- Local `make release-artifact-verify` was not run in this audit because no downloaded draft release asset directory was present. Evidence: the verifier requires `release-check-summary.json`, `release-evidence-logs.tgz`, `release-asset-manifest.tsv`, signed SBOMs, signatures, and certificates in `RELEASE_ASSET_DIR`; this local workspace only generated local release evidence. Risk: low for this audit because the release workflow now downloads uploaded draft assets and runs publication-mode verification, but a local audit cannot fully prove GitHub-hosted asset integrity without those downloaded assets.

## Hexagonal architecture verdict
What is clean: the repository has a clear two-module shape. Root packages provide stable ports, middleware, HTTP helpers, endpoints, scheduling, specs, security profile, and small utility packages. Contrib holds adapters, integrations, examples, and optional vendor dependencies. `VERSIONING.md`, `docs/package-classification.tsv`, `docs/ports-surface.md`, and docscheck gates make the stable/public boundary explicit.

What leaks across boundaries: the leaks are mostly intentional v2 compatibility surfaces. Billing in `ports` is provider-shaped, database stats mirror pgx-style counters, `response_writer` is public legacy HTTP response infrastructure, and idempotency release still supports tokenless cleanup for mixed-version rollout recovery. These are documented, but they remain real architectural constraints.

The codebase is strongly ports-and-adapters oriented, not purely hexagonal. The current v2 line makes pragmatic source-compatibility tradeoffs while documenting preferred APIs and v3 removal conditions. A future v3 can make the architecture cleaner by moving or removing the remaining compatibility surfaces.

## Test verdict
What is covered well: the audit ran `GOTOOLCHAIN=local make finalize`, `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-api-check`, `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-api-drift-report`, `CONTRIB_RELEASE_BASE_REF=v2.0.1 GOTOOLCHAIN=local make contrib-release-notes-check`, `API_BASE_REF=v2.0.1 GOTOOLCHAIN=local make release-evidence`, and `RELEASE_SUMMARY=release-check-summary.json make release-review-summary`. All returned exit 0.

The checks covered formatting, tidy, lint, govulncheck, gosec, build smoke, docs contracts, API compatibility, contrib drift reporting, release-note gating, unit tests, race tests, fuzz smoke, cleanup, release evidence generation, and reviewer-summary decision output. `make finalize` included `release artifact verifier contract tests passed` and `release evidence parser contract tests passed`.

What is weak: local publication asset verification could not be executed because the workspace did not contain downloaded GitHub draft release assets with signed SBOMs, signatures, certificates, checksum manifest, and attestations. The audit inspected the verifier, its contracts, the retained log archive, and the release workflow instead.

The tests are confidence-building rather than superficial. The remaining testing improvement is to add an explicit mixed compatible/incompatible same-package contrib drift fixture so the current live `devheaders` parser behavior remains locked after that drift is eventually resolved.

## Best next fixes
- Burn down the imported-only vulnerability dispositions by upgrading or isolating the affected contrib dependency versions, then refresh `docs/vulnerability-dispositions.tsv`.
- Add a release evidence parser contract fixture where one contrib package contains both `Incompatible changes:` and `Compatible changes:` and must be summarized as incompatible.
- Decide whether to keep, restore, or further document the incompatible report-only `devheaders` comparability drift before the next release.
- Prepare v3 cleanup by moving remaining `response_writer` usage toward `httpx` or package-local capture helpers.
- Prepare v3 cleanup by keeping new billing examples on `compat/billing` or app-owned ports and new database observability on snapshot APIs.
- Continue the token-aware idempotency migration by collecting adapter contract and telemetry evidence needed to retire tokenless release paths in v3.
