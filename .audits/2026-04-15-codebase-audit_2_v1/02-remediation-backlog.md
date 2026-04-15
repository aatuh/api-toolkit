# Backlog

Project: api-toolkit

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Bootstrap Config And Lifecycle Hardening [x]

Description: Make the default router bootstrap path explicit and testable, and surface server shutdown failures instead of hiding them.

### Ticket E1-T1 - Add explicit default-router config and trusted-proxy validation [x]

Description: Introduce an explicit config path for default router construction, keep the compatibility constructor, and return observable errors for invalid trusted-proxy configuration.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- preserve backward compatibility for existing callers of `NewDefaultRouter`
- add direct unit tests for invalid trusted-proxy handling and explicit-config behavior

### Ticket E1-T2 - Surface graceful shutdown failures and cover lifecycle edges [x]

Description: Refactor server startup/shutdown control so cancellation-triggered shutdown errors are returned to callers and directly unit tested.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the public API compatibility check green
- prefer injected test doubles over timing-sensitive network tests

## Epic E2 - Logging And Observability Hardening [ ]

Description: Route panic and server-error diagnostics through explicit logging policy instead of stderr and blanket stack dumps.

### Ticket E2-T1 - Make panic recovery logger-driven and configurable [ ]

Description: Add a logger-aware recovery constructor with explicit stack policy, keep the current helper for compatibility, and wire the strict bootstrap profile through the new path.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- do not leak panic values to clients
- add unit tests for logger usage and committed-response behavior

### Ticket E2-T2 - Stop unconditional 5xx stack dumping in request logs [ ]

Description: Change request logging so stack traces are opt-in for handled `5xx` responses, and add tests that prove the default behavior is quieter.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep request correlation fields intact
- avoid changing the client-visible HTTP behavior

## Epic E3 - Docs Surface And Boundary Cleanup [ ]

Description: Add a stricter documentation mode and reduce internal dependence on the most adapter-shaped legacy compatibility ports without breaking the public API.

### Ticket E3-T1 - Add a strict first-party docs mode [ ]

Description: Add a docs HTML/CSP mode that avoids external CDNs and `unsafe-inline`, and test both the default and strict modes.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the convenience Swagger UI default for existing callers
- make the stricter mode usable without third-party assets

### Ticket E3-T2 - Add compatibility-safe inward-facing routing and db-stats helpers [ ]

Description: Introduce additive helper abstractions that let internal code use smaller routing and DB-stats surfaces while preserving existing exported compatibility types.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep `api-check` green by making the cleanup additive
- update the codebase and docs to prefer the smaller compatibility-safe helpers internally
