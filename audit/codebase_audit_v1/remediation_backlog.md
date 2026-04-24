# Backlog

Project: api-toolkit codebase audit remediation

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Restore Local Quality Gate Parity [x]

Description: Fix the local CI/security workflow so contributors can run the advertised checks reliably before pushing.

### Ticket E1-T1 - Fix the local CodeQL build target [x]

Description: Update `.codeql-local-build` so it builds the root module and contrib module without invoking an invalid `@go` shell command.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Verify at minimum with `make .codeql-local-build`.

### Ticket E1-T2 - Add a smoke check for local CI build commands [x]

Description: Add a lightweight repository-native check that catches broken Makefile build command syntax without requiring a full CodeQL run.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer testing the existing target instead of duplicating build logic.

### Ticket E1-T3 - Document mutating and external-tool Makefile targets [x]

Description: Update developer docs or Makefile help text to identify targets that install tools, require tokens, require optional binaries, or mutate files.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Call out `finalize`, `ci-local`, `scorecard-local`, `sbom-local`, `fmt`, and `tidy`.

## Epic E2 - Harden Trace Middleware Edge Cases [x]

Description: Make trace correlation behavior deterministic and preserve configured sampling semantics.

### Ticket E2-T1 - Add regression tests for trace sampling fallback [x]

Description: Add tests proving that `SampledFlag` is honored when `TrustIncoming` is true and the request has no valid incoming `traceparent`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Include both absent and invalid incoming `traceparent` cases.

### Ticket E2-T2 - Fix trace sampling fallback logic [x]

Description: Change trace middleware so trusted incoming flags are used only when a valid incoming `traceparent` was accepted; otherwise use the configured `SampledFlag`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Preserve existing behavior for valid incoming trace flags.

### Ticket E2-T3 - Handle trace random source failures explicitly [x]

Description: Stop silently discarding `crypto/rand.Read` errors in trace ID and span ID generation.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the public middleware API stable unless a stronger error-returning constructor path already exists.

## Epic E3 - Harden Public API Nil and Cleanup Semantics [x]

Description: Remove small public API footguns that can produce panics or resource-release bugs in otherwise normal caller error paths.

### Ticket E3-T1 - Add scheduler nil-context regression coverage [x]

Description: Add a test that calls `scheduler.Runner.Start(nil)` and verifies the runner does not panic.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Use a short-lived test context or controllable job to avoid leaking goroutines.

### Ticket E3-T2 - Normalize nil scheduler contexts [x]

Description: Update scheduler start/run paths to normalize nil contexts consistently with other packages in the repository.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer a small helper rather than scattering nil checks.

### Ticket E3-T3 - Add txpostgres idempotent release tests [x]

Description: Add tests for double `rows.Close()` and repeated `row.Scan()` calls to define expected connection-release behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Assert that a pooled connection is released exactly once.

### Ticket E3-T4 - Make txpostgres release wrappers idempotent [x]

Description: Update `rowsWithRelease` and `rowWithRelease` so connection release can occur at most once.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Preserve the existing public `DBer` behavior.

## Epic E4 - Improve High-Risk Test Depth [x]

Description: Raise confidence in security-sensitive adapters and compatibility surfaces where current coverage is low or uneven.

### Ticket E4-T1 - Add JWT middleware edge-case tests [x]

Description: Add targeted tests for JWT claim requirements, algorithm allowlist behavior, JWKS setup failures, and optional-vs-required auth behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep tests deterministic with local JWKS fixtures.

### Ticket E4-T2 - Add Clerk auth middleware edge-case tests [x]

Description: Add tests for Clerk middleware required/optional flows, skip-header denial cases, and malformed token handling.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Reuse shared auth test helpers where possible.

### Ticket E4-T3 - Add Stripe adapter validation and webhook tests [x]

Description: Add tests for Stripe request validation, webhook verification-required behavior, safe dev skip behavior, and error normalization.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Avoid live Stripe network calls.

### Ticket E4-T4 - Add migrator uncertain-state regression tests [x]

Description: Add tests around failed commit acknowledgement, unresolved migration state, duplicate migration detection, and advisory lock failure paths.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer fake transaction hooks over requiring a live database.

### Ticket E4-T5 - Add OpenAPI middleware negative-path tests [x]

Description: Add tests for invalid request/response validation behavior, oversized response capture behavior, and error mapping.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep fixtures small and local to the package.

## Epic E5 - Tighten Operator-Facing Documentation [ ]

Description: Make security and operational assumptions explicit for users wiring the toolkit into services.

### Ticket E5-T1 - Document docs endpoint file discovery behavior [x]

Description: Explain that default OpenAPI file discovery uses fixed relative paths from the service working directory unless a provider is registered.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Mention provider registration as the preferred explicit production path.

### Ticket E5-T2 - Document pprof and detailed health exposure guidance [ ]

Description: Add operator guidance for when to mount pprof and detailed health endpoints, including expected access-control placement.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep guidance short and security-focused.

### Ticket E5-T3 - Document outbound health-check SSRF assumptions [ ]

Description: Add documentation that HTTP health check URLs are application-configured and should not be derived from untrusted input.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Cross-reference the SSRF-guarded HTTP client adapter where useful.

## Epic E6 - Preserve Architecture and Compatibility Discipline [ ]

Description: Keep future changes aligned with the documented ports/adapters boundaries and v2 compatibility promises.

### Ticket E6-T1 - Add a root-to-contrib dependency guard [ ]

Description: Add a lightweight check that fails if root-module production code imports `github.com/aatuh/api-toolkit/contrib/v2`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- A docscheck-style test is sufficient.

### Ticket E6-T2 - Add a compatibility-surface growth guard [ ]

Description: Add a docs or test check that flags new provider-shaped exports added to stable `ports` without updating versioning and ports-surface documentation.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Focus on preventing accidental boundary widening, not blocking intentional documented compatibility work.

### Ticket E6-T3 - Refresh the v3 extraction notes for compatibility-sensitive surfaces [ ]

Description: Update the documented v3 migration path for billing, database stats, and legacy response writer surfaces based on the current package state.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep v2 compatibility commitments unchanged.
