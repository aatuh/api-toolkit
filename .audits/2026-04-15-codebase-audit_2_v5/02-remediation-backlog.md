# Backlog

Project: api-toolkit

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Fix panic failure semantics in the default HTTP stack [ ]

Description: Ensure panics are surfaced as failed requests instead of leaking partial success responses through the default recovery and bootstrap path.

### Ticket E1-T1 - Redefine the committed-response panic contract [x]

Description: Decide and document how `httpx/recover` should behave when a panic happens after headers or body bytes have already been written.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- the current behavior preserves a partial `200 OK` response
- prefer a contract that fails the request instead of preserving misleading success

### Ticket E1-T2 - Implement the new recovery behavior [x]

Description: Update `httpx/recover` so panics after partial writes no longer leave clients with a truncated success response.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep panic values out of client responses
- keep operator-visible failure signals intact

### Ticket E1-T3 - Add recovery regression coverage [ ]

Description: Add focused tests for panic-before-write, panic-after-header, panic-after-body, and default bootstrap stack behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- replace the current test that blesses partial success on panic
- assert client-visible behavior, not only internal logging

### Ticket E1-T4 - Update panic-handling documentation [ ]

Description: Refresh package docs and any examples so the published recovery contract matches the implemented behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- document both safety and operator expectations
- keep the wording concrete and behavior-based

## Epic E2 - Restore observability for panicking requests [ ]

Description: Make sure the default HTTP profile still emits request logs and request metrics when handlers panic.

### Ticket E2-T1 - Choose the panic-safe middleware emission strategy [ ]

Description: Decide whether request logging and metrics should move outside recovery, emit through `defer`, or use another explicit pattern that preserves observability on panic paths.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- avoid duplicating request logs or double-counting metrics
- keep the normal non-panic path simple

### Ticket E2-T2 - Implement panic-safe request logging [ ]

Description: Update `contrib/middleware/requestlog` and the default bootstrap ordering so panicking requests still generate one useful access log entry.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep header redaction and trace fields intact
- prefer one clear error record over multiple partial signals

### Ticket E2-T3 - Implement panic-safe HTTP metrics [ ]

Description: Update `contrib/middleware/metrics` and the default bootstrap ordering so panicking requests still increment counters and record durations with the final visible status.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- preserve existing label policy
- avoid counting a single request more than once

### Ticket E2-T4 - Add observability regression tests [ ]

Description: Add tests that prove panicking requests still emit one access log and one metrics observation in the default profile.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- test the default strict profile, not only isolated middleware
- assert emitted behavior rather than internal call order

## Epic E3 - Tighten request-hardening defaults [ ]

Description: Make the repository’s strict-profile and strict-mode promises match actual runtime behavior for JSON enforcement and browser-facing defaults.

### Ticket E3-T1 - Require explicit JSON content types in strict mode [ ]

Description: Update `middleware/json` so body-bearing `POST`, `PUT`, and `PATCH` requests in strict mode fail when `Content-Type` is missing or non-JSON.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep bodyless write requests working as intended
- preserve support for `application/*+json`

### Ticket E3-T2 - Expand JSON middleware coverage and docs [ ]

Description: Add tests for missing `Content-Type` cases and update security documentation so the published strict-mode contract is exact.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- docs should describe actual behavior, not intention
- keep examples aligned with strict profile usage

### Ticket E3-T3 - Replace wildcard CORS in the strict profile [ ]

Description: Change `ProfileStrictAPI` so production-style composition does not silently inherit `AllowedOrigins: ["*"]`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- prefer explicit caller choice over permissive hidden defaults
- consider compatibility impact for current examples and consumers

### Ticket E3-T4 - Add strict-profile browser-surface tests and migration notes [ ]

Description: Add tests covering strict-profile CORS behavior and document any compatibility-sensitive changes for downstream users.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- avoid silent behavior changes for existing consumers
- include upgrade guidance in release notes if needed

## Epic E4 - Fail fast on invalid runtime configuration [ ]

Description: Make startup configuration parsing consistent so malformed bool and int values do not silently revert to defaults.

### Ticket E4-T1 - Add parse-error aggregation for bool and int env vars [ ]

Description: Extend `contrib/config.Loader` so bool and int reads record invalid input errors the same way duration parsing already does.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- preserve default behavior for missing env vars
- only invalid present values should fail the load

### Ticket E4-T2 - Add config loader regression tests [ ]

Description: Add focused tests for malformed bool, int, and duration env values and for mixed valid plus invalid startup configurations.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep tests local to config and loader packages
- assert both returned errors and resulting field values

### Ticket E4-T3 - Align config examples and startup helpers with fail-fast behavior [ ]

Description: Update config-facing docs, examples, and any helper wrappers so they describe and rely on explicit startup failure for invalid configuration.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the docs focused on operator outcomes
- do not leave stale examples that imply silent fallback is acceptable

## Epic E5 - Raise confidence in security-sensitive contrib packages [ ]

Description: Add direct coverage and edge-case hardening around the payment, telemetry, and runtime-config contrib packages that currently rely too much on indirect confidence.

### Ticket E5-T1 - Add direct Stripe webhook and billing tests [ ]

Description: Add direct tests for `contrib/adapters/stripe` covering webhook verification, dev bypass conditions, secure defaults, and core billing request translation.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover both signed and skipped verification paths
- assert the exact conditions under which dev bypass is allowed

### Ticket E5-T2 - Normalize Stripe adapter error behavior [ ]

Description: Audit the Stripe billing helpers so all externally returned provider errors follow one explicit normalization strategy.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- some methods already normalize Stripe errors and some do not
- keep public behavior consistent across checkout, billing, and retrieval helpers

### Ticket E5-T3 - Add direct tests for telemetry and config edge behavior [ ]

Description: Add focused tests for `contrib/telemetry` and any remaining config edge paths so runtime setup behavior is explicitly verified.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- prefer behavior-level assertions over transport or SDK implementation details
- cover insecure endpoint and timeout configuration branches where present

### Ticket E5-T4 - Publish a verified post-remediation baseline [ ]

Description: After the backlog is complete, rerun the repository quality gate and record the resulting verified state in a fresh audit note.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- record the exact commands used for final verification
- keep the post-remediation note factual and short
