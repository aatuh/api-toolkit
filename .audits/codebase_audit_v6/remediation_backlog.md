# Backlog

Project: api-toolkit

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Fix health semantics and probe contracts [x]

Description: Make health endpoints reflect real service state, honor advertised configuration, and fail safely when probe configuration is incomplete.

### Ticket E1-T1 - Define the health behavior contract [x]

Description: Write down the intended behavior for empty liveness and readiness sets, detailed health exposure, missing checkers, and cache use so implementation and documentation share one contract.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Touch the minimal doc set needed to define the contract before deeper code changes land.
- Affected areas: `ports/health.go`, `docs/*`, `README.md`.

### Ticket E1-T2 - Enforce detailed health gating [x]

Description: Change health route registration and handler behavior so `EnableDetailed` is actually enforced and detailed dependency output is not exposed when the flag is disabled.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Preserve backward compatibility where possible, but prefer safer defaults over silent exposure.
- Affected areas: `endpoints/health/handlers.go`, `endpoints/health/health.go`.

### Ticket E1-T3 - Fail safely on invalid probe configuration [x]

Description: Stop returning `healthy` for empty readiness or liveness check lists and make nil checker registration fail predictably instead of panicking.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Pick one explicit outcome for invalid config and keep it consistent across manager and handlers.
- Affected areas: `endpoints/health/health.go`, `endpoints/health/handlers.go`.

### Ticket E1-T4 - Add regression coverage for probe semantics [x]

Description: Add table-driven tests covering detailed health enablement, empty check sets, missing checker names, nil registration, and caching behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Lock behavior down before any follow-on observability changes depend on it.
- Affected areas: `endpoints/health/*_test.go`.

## Epic E2 - Harden transaction and migration failure handling [ ]

Description: Make transaction cleanup and migration state tracking safe under timeout, cancellation, and ambiguous commit outcomes.

### Ticket E2-T1 - Make transaction rollback cleanup robust [x]

Description: Change `txpostgres.WithinTx` so deferred rollback uses a short-lived non-cancelable cleanup context instead of the caller's canceled context.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep commit and rollback semantics explicit and easy to read.
- Affected areas: `contrib/adapters/txpostgres/txpostgres.go`.

### Ticket E2-T2 - Add cancellation and timeout tests for transaction cleanup [x]

Description: Add focused tests that simulate canceled and timed-out contexts so rollback behavior is verified rather than assumed.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer deterministic fakes over slow database-backed tests for this behavior.
- Affected areas: `contrib/adapters/txpostgres/*_test.go`.

### Ticket E2-T3 - Introduce explicit handling for ambiguous migration commits [ ]

Description: Redesign migrator commit-failure handling so a failed `Commit` is not automatically treated as a definite non-commit, and persist enough state to avoid blind re-application.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- This ticket may require a migration metadata schema change or a stronger verification step after commit failure.
- Affected areas: `contrib/migrator/migrator.go`, migration metadata table behavior, docs.

### Ticket E2-T4 - Block reruns when migration outcome is uncertain [ ]

Description: Update pending migration selection so previously ambiguous runs stop the process with a clear operator action instead of re-running non-idempotent DDL.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Favor explicit operator-visible failure over implicit retry.
- Affected areas: `contrib/migrator/migrator.go`, migrator tests, release notes.

### Ticket E2-T5 - Add fault-injection tests for migrator commit ambiguity [ ]

Description: Add tests that simulate execution success followed by commit failure and verify that the runner records and reacts to the outcome safely.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the test harness precise enough to reproduce the exact ambiguous path.
- Affected areas: `contrib/migrator/*_test.go`.

## Epic E3 - Surface scheduler persistence failures [ ]

Description: Make scheduler run-recording failures visible so operators can trust job history and startup decisions.

### Ticket E3-T1 - Define scheduler recorder failure policy [ ]

Description: Choose and document the intended behavior when `Recorder.Record` fails, including logging, callback hooks, and whether failures should affect scheduler control flow.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- The preferred outcome is visibility without turning the scheduler into a fragile single point of failure.
- Affected areas: `scheduler/scheduler.go`, docs.

### Ticket E3-T2 - Implement recorder failure observability [ ]

Description: Add structured logging and, if needed, a configurable hook so recorder write failures are surfaced with job name, timing, and persistence error context.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the logger interface small; avoid widening core interfaces unless there is a clear need.
- Affected areas: `scheduler/scheduler.go`, `contrib/scheduler/postgres`.

### Ticket E3-T3 - Add scheduler recorder failure tests [ ]

Description: Add tests that force recorder errors and verify the scheduler emits the expected logs or hook behavior without losing normal run completion behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Cover both error-returning jobs and successful jobs whose persistence write fails afterward.
- Affected areas: `scheduler/scheduler_test.go`.

## Epic E4 - Deduplicate auth and JWKS validation logic [ ]

Description: Remove security-sensitive duplication between generic JWT and Clerk middleware while preserving the public behavior that downstream services depend on.

### Ticket E4-T1 - Extract shared internal auth primitives [ ]

Description: Create an internal shared package for bearer parsing, algorithm normalization, required claim validation, and trusted-proxy skip-header handling.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep the shared layer internal so public API churn stays minimal.
- Affected areas: new internal package plus both auth middlewares.

### Ticket E4-T2 - Migrate core JWT middleware to shared primitives [ ]

Description: Refactor `middleware/auth/jwt` to use the extracted primitives without changing its public configuration or response behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Avoid mixing semantic changes with the extraction step.
- Affected areas: `middleware/auth/jwt/*`.

### Ticket E4-T3 - Migrate Clerk middleware to shared primitives [ ]

Description: Refactor `contrib/middleware/auth/clerk` to use the same shared auth and JWKS helpers while preserving Clerk-specific configuration and behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep Clerk-specific docs and env loading aligned with the refactor.
- Affected areas: `contrib/middleware/auth/clerk/*`, `contrib/integrations/auth/clerk/*`.

### Ticket E4-T4 - Add parity tests across auth implementations [ ]

Description: Add shared test cases that verify both auth implementations enforce the same bearer parsing, claim validation, skip-header, and algorithm rules.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Treat this as the guardrail that prevents future drift after the refactor lands.
- Affected areas: auth middleware test suites.

## Epic E5 - Raise coverage on exported contrib edges [ ]

Description: Add targeted tests around public contrib adapters, wrappers, and command entrypoints so repo boundary changes are safer.

### Ticket E5-T1 - Add tests for public transport and adapter shims [ ]

Description: Add focused tests for `txpostgres`, `chi`, and `middleware/cors` that validate their exported behavior and expected lifecycle semantics.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Start with public surfaces whose behavior is small enough to test cheaply.
- Affected areas: `contrib/adapters/txpostgres`, `contrib/adapters/chi`, `contrib/middleware/cors`.

### Ticket E5-T2 - Add tests for simple exported adapters with no coverage [ ]

Description: Add lightweight but meaningful tests for packages such as `logzap`, `uuid`, `ulid`, and `validation` so they have direct compatibility checks.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer behavior tests over coverage-only assertions.
- Affected areas: `contrib/adapters/logzap`, `uuid`, `ulid`, `validation`.

### Ticket E5-T3 - Add smoke tests for integrations and command packages [ ]

Description: Add low-cost smoke tests or constructor tests for exported `integrations/*` and `cmd/*` packages so public wiring entrypoints do not drift untested.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep these tests cheap enough to stay stable in CI.
- Affected areas: `contrib/integrations/*`, `contrib/cmd/*`.

## Epic E6 - Align documentation and release guidance [ ]

Description: Keep documentation, upgrade notes, and release communication aligned with the behavior changes introduced by the remediation work.

### Ticket E6-T1 - Update docs for health, transactions, migrations, and scheduler behavior [ ]

Description: Refresh README and package docs so users understand the new health semantics, transaction cleanup guarantees, migration ambiguity behavior, and scheduler recorder visibility.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep examples consistent with any changed defaults or operator guidance.
- Affected areas: `README.md`, `docs/security.md`, `docs/architecture.md`, package docs.

### Ticket E6-T2 - Record upgrade impact in release notes and versioning guidance [ ]

Description: Capture any behavior changes that downstream services must notice, especially health endpoint exposure, migration behavior, and auth refactoring with no intended API change.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API checks, race tests, fuzz smoke tests, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- If any change is behavior-breaking, make the upgrade path explicit rather than implied.
- Affected areas: `docs/release-notes.md`, `VERSIONING.md`.
