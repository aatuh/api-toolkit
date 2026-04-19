# Backlog

Project: api-toolkit

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Harden idempotency ambiguous-outcome handling [x]

Description: Prevent idempotent writes from reopening the same key after the downstream handler has already executed but replay safety is unknown.

### Ticket E1-T1 - Block duplicate execution after ambiguous completion [x]

Description: Change the idempotency middleware so oversized buffered responses and completion-persistence failures leave an explicit ambiguous state instead of releasing the key for another execution.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `middleware/idempotency/idempotency.go`
- add or update unit tests for ambiguous retry behavior

### Ticket E1-T2 - Document idempotency retry semantics [x]

Description: Update public docs and package docs so callers understand the middleware’s behavior for large responses, persistence failures, and ambiguous outcomes.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `README.md`, `docs/release-notes.md`, and `middleware/idempotency/doc.go`

## Epic E2 - Align health endpoint contracts [ ]

Description: Make the health endpoint surface internally consistent across helpers, handlers, and published default routes.

### Ticket E2-T1 - Return Problem Details for disabled detailed health [ ]

Description: Replace the plain-text `404` in the detailed health handler with the toolkit’s RFC 9457 error envelope and add regression coverage for the response shape.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `endpoints/health/handlers.go` and `endpoints/health/handlers_test.go`

### Ticket E2-T2 - Align DefaultHealthPaths with published defaults [ ]

Description: Use the canonical `specs` endpoint constants for health helper defaults and add regression tests so the custom-route helper matches the published route surface.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `endpoints/health/handlers.go`, `endpoints/health/handlers_test.go`, and `specs/endpoints.go`
