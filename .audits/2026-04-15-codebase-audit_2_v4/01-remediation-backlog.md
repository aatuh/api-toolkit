# Backlog

Project: api-toolkit

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Restore a trustworthy quality baseline [x]

Description: Get the repository back to a reproducible green quality gate so later remediation work can rely on one verified baseline instead of conflicting audit claims.

### Ticket E1-T1 - Fix the current local lint and formatting failures [x]

Description: Resolve the present `make lint` failures so the repository once again satisfies its advertised local quality gate.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- start with the current `gofmt`, `noctx`, and test-file `gosec` failures
- do not weaken the lint rules just to make the gate pass

### Ticket E1-T2 - Reconcile toolchain-sensitive quality behavior [x]

Description: Confirm which Go and tool versions the repository expects for local quality checks, and tighten the documentation or tooling so audit results stay reproducible across machines.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- prefer explicit toolchain expectations over audit-time assumptions
- keep CI and local workflow aligned

### Ticket E1-T3 - Record the verified post-fix baseline [x]

Description: After the gate is green again, update the audit notes or release notes so later reviews can distinguish verified repo state from stale local audit artifacts.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the note short and factual
- record the exact commands used for verification

## Epic E2 - Harden idempotency from best-effort to safe-by-default [x]

Description: Close the remaining correctness and security gaps in idempotency handling so replay behavior is explicit, bounded, and safe for authenticated multi-tenant APIs.

### Ticket E2-T1 - Define the completion-persistence failure contract [x]

Description: Decide how the middleware should behave when the downstream handler succeeds but the completed idempotency record cannot be persisted.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- choose explicitly between fail-closed behavior and documented best-effort semantics
- keep the docs and examples aligned with the final decision

### Ticket E2-T2 - Implement the chosen save-failure behavior in middleware [x]

Description: Update `middleware/idempotency` so its runtime behavior matches the chosen persistence-failure contract instead of silently returning a misleading success response.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- preserve current successful replay behavior unless the contract decision requires a change
- cover `Save` failure, retry behavior, and client-visible status/body outcomes

### Ticket E2-T3 - Scope idempotency keys to caller context by default [x]

Description: Reduce replay leakage risk by including subject or tenant context in the default idempotency keying strategy, or require explicit caller scoping in authenticated examples and docs.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep unauthenticated use cases simple
- make the secure path the obvious documented path for real APIs

### Ticket E2-T4 - Add response buffering guardrails to idempotent handlers [x]

Description: Prevent the middleware from acting as an unbounded response buffer by adding explicit size limits, streaming exclusions, or both.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- do not break existing replay for normal JSON responses
- make streaming and upgrade limitations explicit

### Ticket E2-T5 - Expand idempotency regression coverage [x]

Description: Add tests for caller scoping, save-failure behavior, replay headers, large responses, and any newly introduced exclusion or size-limit rules.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the tests local to `middleware/idempotency`
- prefer behavior-level assertions over implementation-detail checks

## Epic E3 - Make scheduler lifecycle failures non-fatal and deterministic [ ]

Description: Remove the remaining scheduler lifecycle hazards so background jobs cannot crash the process or multiply execution schedules through repeated startup calls.

### Ticket E3-T1 - Contain scheduled-job panics and surface them through logging/recording [x]

Description: Add panic recovery around job execution so one broken job cannot terminate the host process, while still recording failure details clearly.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the failure visible to operators
- do not silently swallow panic information

### Ticket E3-T2 - Make duplicate Start behavior idempotent [ ]

Description: Prevent repeated `Runner.Start` calls from creating multiple ticker loops for the same job set unless that behavior is made an explicit opt-in.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the current non-overlap guarantee intact
- favor simple runner state over distributed coordination inside the scheduler package

### Ticket E3-T3 - Add lifecycle regression tests for panic and duplicate-start paths [ ]

Description: Add focused tests proving that panics are contained and that repeated start calls do not create duplicate schedules or surprising run frequency changes.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- use synchronization instead of timing-heavy sleeps where possible
- test both operator-visible signals and execution behavior

### Ticket E3-T4 - Refresh scheduler docs and examples [ ]

Description: Update package docs and release notes so the scheduler contract covers non-overlap, panic handling, and duplicate-start behavior truthfully.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- remove claims that are not directly verified by tests
- call out any changed behavior for downstream users

## Epic E4 - Raise change safety around stateful contrib adapters [ ]

Description: Add direct coverage for the highest-risk stateful adapter packages so correctness claims are backed by tests at the persistence and integration boundaries.

### Ticket E4-T1 - Add direct tests for Redis-backed idempotency and rate limiting [ ]

Description: Cover `contrib/adapters/idempotencyredis` and `contrib/adapters/ratelimitredis` with direct tests for serialization, TTL behavior, and retry-delay semantics.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- prefer deterministic tests over broad end-to-end fixtures
- cover edge values, not just happy paths

### Ticket E4-T2 - Add direct tests for scheduler and database-facing adapters [ ]

Description: Add focused coverage for packages such as `contrib/scheduler/postgres`, `contrib/adapters/pgxpool`, and other stateful wrappers that currently rely mostly on indirect confidence.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- start with the adapters most likely to affect production state or lifecycle
- do not overfit tests to one concrete backend if a small contract-level test is enough

### Ticket E4-T3 - Document the high-risk adapter coverage policy [ ]

Description: Record which adapter categories require direct tests before new features or refactors are accepted so the same coverage gap does not reopen later.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the policy short and repo-specific
- focus on stateful, concurrency-sensitive, and external-system adapters
