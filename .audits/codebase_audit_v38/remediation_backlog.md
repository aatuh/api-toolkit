# Backlog

Project: api-toolkit codebase audit v38 remediation

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Close publication artifact and attestation gaps [ ]

Description: Make draft-release verification publication-grade by proving uploaded assets, signatures, retained logs, summary invariants, and online attestations before a release is published.

### Ticket E1-T1 - Add post-upload draft release verification [ ]

Description: Add an automated or documented command path that downloads draft release assets after upload and runs `make release-artifact-verify` with `RELEASE_TAG` and `GITHUB_REPOSITORY` set.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- The post-upload path should verify GitHub provenance attestations, not only local asset shape.

### Ticket E1-T2 - Require publication mode for online attestation verification [ ]

Description: Add an explicit publication verification mode so missing `RELEASE_TAG` fails when the verifier is used for release publication, while preserving local/offline verification for audits.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- A mode flag such as `RELEASE_ARTIFACT_VERIFY_MODE=publication` is acceptable if documented.

### Ticket E1-T3 - Validate release summary publication invariants [ ]

Description: Extend `scripts/release_artifact_verify.sh` to parse `release-check-summary.json` and require `status=passed`, `publication_eligible=true`, clean git state, expected `api_base_ref`, zero disposition issues, and valid artifact expectations.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer JSON parsing with clear failure messages over grep-only checks.

## Epic E2 - Harden retained log and evidence parser contracts [ ]

Description: Make release evidence resilient to tool-output changes and ensure retained logs match the summary rather than a fixed historical list only.

### Ticket E2-T1 - Cross-check summary log paths against retained archive [ ]

Description: Update artifact verification so every `checks[].log_path` and `contrib_drift.artifact_path` in `release-check-summary.json` must exist inside `release-evidence-logs.tgz`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the fixed required-log list only as a backward-compatible minimum if still useful.

### Ticket E2-T2 - Add govulncheck parser fixture coverage [ ]

Description: Add contract tests for vulnerability evidence parsing, including called vulnerabilities, imported-only vulnerabilities, no vulnerabilities, and unexpected output where imported count and parsed IDs disagree.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- The release evidence writer should fail closed if imported-only count is positive but no IDs can be parsed.

### Ticket E2-T3 - Add contrib drift parser fixture coverage [ ]

Description: Add contract tests for contrib drift parsing, including compatible drift, incompatible drift, no drift, skipped packages, and malformed report output.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- The parser should never silently classify unknown package drift as compatible.

## Epic E3 - Consolidate reviewer release path [ ]

Description: Reduce release-review friction by making the evidence, disposition, artifact, and v3-debt checks reachable through one clear reviewer path.

### Ticket E3-T1 - Add a reviewer summary command [ ]

Description: Add or extend a make target that prints the key release-review decision fields from `release-check-summary.json`, including publication eligibility, git state, vulnerability dispositions, contrib drift dispositions, artifact expectations, and retained log archive path.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- This should complement, not replace, `docs/release-review.md`.

### Ticket E3-T2 - Normalize release docs around the single reviewer path [ ]

Description: Update `docs/release-runbook.md`, `docs/release-review.md`, and `VERSIONING.md` so they point to the same ordered reviewer workflow without duplicating conflicting command guidance.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep dirty local audit guidance distinct from publication evidence guidance.

## Epic E4 - Prepare response writer and idempotency v3 cleanup [ ]

Description: Reduce v3 migration risk by separating new response capture and token-aware idempotency paths from legacy v2 compatibility surfaces.

### Ticket E4-T1 - Inventory remaining `response_writer` dependents [ ]

Description: Document all root and contrib imports of `response_writer`, classify which are compatibility-only, and identify the desired replacement path for each before v3.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Current contrib middleware imports should not block v2, but they should have explicit v3 treatment.

### Ticket E4-T2 - Add guardrails for new `response_writer` usage [ ]

Description: Add docscheck or static checks that prevent new docs/examples or non-compat code from teaching `response_writer` as the preferred response helper.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- The check should allow the package docs and v3 compatibility roadmap to mention the legacy surface.

### Ticket E4-T3 - Define tokenless idempotency sunset evidence [ ]

Description: Convert the roadmap's tokenless release sunset criteria into executable evidence requirements, including adapter contract status, telemetry labels, and the support-window signal needed before v3 removal.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Do not remove v2 compatibility in this ticket.

## Epic E5 - Prepare billing and database stats v3 cleanup [ ]

Description: Keep v2 source compatibility intact while making the major-version removal path for provider-shaped billing and driver-shaped database stats executable.

### Ticket E5-T1 - Add billing compatibility usage guardrails [ ]

Description: Add checks that discourage new examples and docs from importing deprecated `ports` billing types when `compat/billing` or app-owned ports are the intended replacement.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Preserve deprecated aliases for v2 callers.

### Ticket E5-T2 - Add database stats direct-use guardrails [ ]

Description: Add docscheck or static checks that examples and new generic observability guidance prefer `DatabasePoolSnapshotProvider`, `SnapshotDatabasePoolStats`, or adapter `StatSnapshot()` over direct `DatabaseStats` usage.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep adapter-level compatibility wrappers until v3.

### Ticket E5-T3 - Add v3 removal checklist tests [ ]

Description: Add tests or docs contracts that require each v3 roadmap surface to name current v2 API, preferred v2 API, v3 action, required tests, and removal condition before the v3 branch starts.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- This keeps future v3 cleanup auditable instead of prose-only.
