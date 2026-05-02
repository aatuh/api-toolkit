# Backlog

Project: api-toolkit codebase audit v39 remediation

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Burn down imported-only vulnerability dispositions [x]

Description: Reduce recurring release-review risk by upgrading, isolating, or explicitly re-dispositioning the imported-only govulncheck findings now tracked in release evidence.

### Ticket E1-T1 - Map current imported-only advisory ownership [x]

Description: Document the current owning modules, affected packages, fixed versions, and dependency paths for `GO-2026-4762`, `GO-2026-4771`, and `GO-2026-4772` before changing dependencies.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Start from `release-check-summary.json`, `.ci-result/release-evidence/logs/vuln.log`, `docs/dependency-risk.md`, and `docs/vulnerability-dispositions.tsv`.

### Ticket E1-T2 - Upgrade or isolate pgx imported-only advisories [x]

Description: Move the contrib dependency graph away from the pgx versions associated with `GO-2026-4771` and `GO-2026-4772`, or document why the upgrade must wait.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Include pgxpool, txpostgres, migrator, scheduler/postgres, and related adapter tests in the validation scope.

### Ticket E1-T3 - Upgrade or isolate grpc imported-only advisory [x]

Description: Move the contrib dependency graph away from the grpc version associated with `GO-2026-4762`, or document the blocker and expiry date for the disposition.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Check OpenTelemetry, grpc-gateway, and generated example dependency paths before forcing a direct version.

### Ticket E1-T4 - Refresh vulnerability dispositions after dependency work [x]

Description: Update `docs/vulnerability-dispositions.tsv` and `docs/dependency-risk.md` so they match the post-upgrade govulncheck output.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- If an advisory remains imported-only, keep a non-expired owner, reviewed date, expiry, and upgrade trigger.

## Epic E2 - Harden contrib drift evidence and current drift handling [x]

Description: Preserve the parser fix and make current report-only contrib drift easier for maintainers and consumers to review.

### Ticket E2-T1 - Add mixed same-package drift parser fixture [x]

Description: Extend `scripts/release_evidence_parser_contract_test.sh` with a fixture where one package has both `Incompatible changes:` and `Compatible changes:` and assert the summary status is `incompatible`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Model the fixture on the current `middleware/auth/devheaders` drift shape.

### Ticket E2-T2 - Decide devheaders incompatible drift outcome [x]

Description: Decide whether to restore comparability for `contrib/v2/middleware/auth/devheaders` exported types, keep the incompatible report-only drift with explicit migration notes, or move the breaking shape to a new API.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- If the drift remains, keep `docs/release-notes.md` and `docs/contrib-api-drift-dispositions.tsv` package-tied and non-expired.

### Ticket E2-T3 - Add consumer-facing contrib drift guidance [x]

Description: Add a short migration note for consumers who treated maintained contrib middleware or adapters as stable despite the report-only contrib policy.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the policy clear: this improves migration guidance and does not make contrib part of the stable v2 API promise.

## Epic E3 - Prepare v3 compatibility cleanup [x]

Description: Reduce future major-version risk by moving new examples, tests, and internal usage away from compatibility-sensitive v2 surfaces.

### Ticket E3-T1 - Strengthen billing compatibility guardrails [x]

Description: Ensure examples and new docs teach `compat/billing` or app-owned billing ports instead of deprecated `ports/billing.go` symbols.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Add docscheck coverage if deprecated billing symbols can appear in preferred examples unnoticed.

### Ticket E3-T2 - Reduce database stats compatibility usage [x]

Description: Move examples and adapter guidance toward `DatabasePoolSnapshotProvider`, `SnapshotDatabasePoolStats`, `SnapshotDatabaseStats`, and adapter `StatSnapshot()` methods.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Do not remove `DatabasePool.Stat` in v2; this ticket is preparation for v3.

### Ticket E3-T3 - Inventory and migrate response_writer internal users [x]

Description: Use `docs/response-writer-inventory.md` to move root and contrib internal response capture toward `httpx` or package-local helpers where source compatibility allows.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the public `response_writer` package intact for v2 callers until the v3 branch.

### Ticket E3-T4 - Expand token-aware idempotency release evidence [x]

Description: Ensure every maintained idempotency store has token-aware release contract coverage, legacy missing-token cleanup coverage, token mismatch coverage, and clear telemetry labels.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Preserve mixed-version recovery behavior in v2 while collecting evidence needed to remove tokenless release in v3.

### Ticket E3-T5 - Prepare v3 migration notes for compatibility removals [x]

Description: Draft migration notes that map deprecated billing, database stats, response writer, and tokenless idempotency release surfaces to preferred replacements.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the notes as preparation only; do not remove v2 compatibility symbols in this ticket.

## Epic E4 - Improve local release artifact audit reproducibility [x]

Description: Make it easier for auditors to exercise release artifact verification behavior locally without confusing synthetic checks with publication-grade GitHub draft release verification.

### Ticket E4-T1 - Add a documented verifier fixture command [x]

Description: Add a small documented command or script that builds a synthetic release asset bundle and runs the verifier in local mode, using the existing contract-test fixture behavior as the model.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- The command must clearly state that publication verification still requires downloaded GitHub draft release assets, `RELEASE_ARTIFACT_VERIFY_MODE=publication`, `RELEASE_TAG`, and online attestation checks.

### Ticket E4-T2 - Keep release-review-summary in release review flow [x]

Description: Ensure `docs/release-review.md`, `docs/release-runbook.md`, and release workflow comments continue to point reviewers through `make release-review-summary` before artifact verification.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- This preserves the reviewer path consolidation achieved after v38.
