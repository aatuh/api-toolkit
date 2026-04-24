# Backlog

Project: api-toolkit codebase audit v2 remediation

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Stabilize Public Contract Metadata [x]

Description: Remove contradictions and drift in the documented stable API surface so downstream users and API checks receive one coherent compatibility contract.

### Ticket E1-T1 - Decide the `specs` stability classification [x]

Description: Choose whether `github.com/aatuh/api-toolkit/v2/specs` is stable in v2 or experimental, then update `specs/doc.go`, `VERSIONING.md`, and `README.md` so they all say the same thing.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- If `specs` stays stable, remove the experimental package-doc wording.
- If `specs` becomes experimental, remove it from stable API docs and the API compatibility package list.

### Ticket E1-T2 - Add a stable surface manifest check [x]

Description: Add a docs/tooling contract test that verifies the stable package list in `VERSIONING.md` matches the package list used by `scripts/apicheck.sh`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer parsing a simple explicit manifest format over brittle prose scraping if the current docs need a small normalization.

### Ticket E1-T3 - Keep README stability summary aligned [x]

Description: Extend the contract check or docs structure so the README stability summary cannot contradict `VERSIONING.md`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the README readable; the source of truth should remain `VERSIONING.md` or a manifest consumed by both docs and tests.

## Epic E2 - Add Missing Stable Package Coverage [x]

Description: Add focused behavior tests for stable packages that currently rely on API shape checks but have little or no direct behavioral coverage.

### Ticket E2-T1 - Test `specs.Registry` deterministic OpenAPI output [x]

Description: Add tests for `specs.NewRegistry`, operation registration, deterministic path/method ordering, default response generation, explicit response content types, request bodies, tags, descriptions, and deprecated operations.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Assert decoded JSON structures rather than raw JSON string ordering, except where deterministic behavior is the point.

### Ticket E2-T2 - Test `specs` endpoint constants and grouped endpoint values [x]

Description: Add tests that lock down health, docs, metrics, version, and pprof endpoint constants plus the grouped endpoint structs.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep these tests small; they protect stable route constants from accidental drift.

### Ticket E2-T3 - Add low-cost tests for simple stable value packages [x]

Description: Add minimal tests for stable no-test packages where behavior exists, such as `email.Message` expectations, field error helpers, and scheduler migration embedded assets.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Do not add empty coverage tests. Only test stable behavior that future changes could accidentally break.

## Epic E3 - Harden Operational Defaults [x]

Description: Tighten remaining operational edge cases around outbound clients and migration locking without changing public behavior unexpectedly.

### Ticket E3-T1 - Define safe behavior for `telemetry.WrapHTTPClient(nil)` [x]

Description: Decide whether nil input should be rejected, documented as no-timeout, or converted to a client with a default timeout; implement the chosen behavior with tests.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Preserve existing non-nil client settings by shallow copy.
- If behavior changes for nil input, document the compatibility impact in release notes.

### Ticket E3-T2 - Add configurable migrator lock timeout [x]

Description: Add an `Options` field for advisory lock wait timeout, default it to the current behavior, and cover timeout behavior in tests.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the zero value backward compatible.
- Make the timeout visible in errors where practical.

### Ticket E3-T3 - Surface migrator unlock failures without masking primary errors [x]

Description: Add logging or an optional callback for advisory unlock failures so operators can see cleanup problems while preserving the primary migration error contract.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Do not let unlock reporting hide migration execution, commit, or state-recording errors.

## Epic E4 - Improve CI And Supply Chain Hardening [ ]

Description: Make CI and release automation more tamper-resistant and easier to run in non-mutating review contexts.

### Ticket E4-T1 - Pin GitHub Actions to immutable SHAs [x]

Description: Replace version-tag action references in `.github/workflows/*.yml` with full commit SHAs and comments that record the human-readable upstream version.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Include checkout, setup-go, CodeQL, Scorecard, SBOM, cosign installer, release upload, and attestation actions.

### Ticket E4-T2 - Add Dependabot updates for GitHub Actions pins [x]

Description: Configure Dependabot or document the update workflow so pinned GitHub Actions can still be refreshed safely.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Verify the chosen Dependabot strategy works with SHA-pinned actions.

### Ticket E4-T3 - Add a non-mutating audit check target [ ]

Description: Add a Makefile target for reviewers that runs non-mutating checks such as tests, race tests, fuzz smoke, lint, vuln, gosec, API check, docs-check, and build smoke without running `fmt` or `tidy`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep `make finalize` as the implementation-quality gate.
- Name the new target clearly, for example `check` or `audit-check`.

## Epic E5 - Tighten Documentation Contracts [ ]

Description: Expand docs contract tests so security and operational docs stay synchronized with actual code behavior.

### Ticket E5-T1 - Check documented dev bypass names against code [ ]

Description: Add docscheck coverage that verifies the security docs mention the actual dangerous bypass environment variables and trusted proxy requirement.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Include rate limit, JWT, Clerk, devheaders, and Stripe webhook skip behavior.

### Ticket E5-T2 - Document non-mutating reviewer checks [ ]

Description: Add a short docs section that tells auditors and release managers which commands are non-mutating and which commands may rewrite files.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Mention that `make finalize` may run formatting and tidy steps.

### Ticket E5-T3 - Add release-note guidance for stable surface changes [ ]

Description: Add a release checklist note requiring stable surface changes, deprecations, and compatibility-sensitive updates to mention docs, API check coverage, and release notes.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the checklist short and actionable.

## Epic E6 - Reduce Compatibility Surface Drag [ ]

Description: Make the existing v2 compatibility exceptions easier to manage until a future major version can remove or reshape them.

### Ticket E6-T1 - Add tests proving new database observability uses snapshots [ ]

Description: Add or extend tests that fail if new health or observability code calls legacy `DatabasePool.Stat()` when `DatabasePoolSnapshotProvider` is available.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Preserve fallback coverage for legacy-only pool implementations.

### Ticket E6-T2 - Add a contract test for deprecated billing aliases [ ]

Description: Add a focused test or docscheck assertion that deprecated `ports/billing.go` aliases continue pointing users to `compat/billing`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- This should protect messaging, not expand the legacy billing API.

### Ticket E6-T3 - Add a v3 cleanup checklist for legacy surfaces [ ]

Description: Convert the current v3 extraction notes into a concise checklist covering `ports/billing.go`, database stats, and `response_writer`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep this as planning documentation only; do not remove v2 APIs in this ticket.
