# Backlog

Project: api-toolkit codebase audit v14 remediation

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Make observability surfaces explicit [x]

Description: Remove hidden metrics side effects from bootstrap helpers so service authors must opt into metrics exposure and global Prometheus registration.

### Ticket E1-T1 - Stop implicit Prometheus registration [x]

Description: Change `ProfileStrictAPI` and `ProfileDev` so they do not register collectors against the default Prometheus registerer unless the caller explicitly requests a Prometheus recorder.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- preserve a straightforward opt-in path for Prometheus users
- document migration behavior for existing bootstrap consumers

### Ticket E1-T2 - Make `/metrics` opt-in [x]

Description: Change `MountSystemEndpointsTo` so a nil `SystemEndpoints.Metrics` does not mount `/metrics`, and add an explicit helper or option for the Prometheus handler.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the call site easy to wire from examples and bootstrap code
- avoid silently changing unrelated health/docs/version behavior

### Ticket E1-T3 - Lock the new metrics contract with tests and docs [x]

Description: Add tests and docs that assert metrics registration and metrics exposure are explicit choices rather than default side effects.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover both zero-value bootstrap behavior and explicit opt-in wiring

## Epic E2 - Make request-path runtime behavior predictable [ ]

Description: Remove misleading or expensive default runtime behavior from timeout and health helpers.

### Ticket E2-T1 - Decide and codify timeout semantics [x]

Description: Either implement hard timeout handling with a stable response contract or rename/document the current middleware so it is clearly a context-deadline propagator only.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep compatibility impact explicit in release notes
- add tests for both cooperative and non-cooperative handlers

### Ticket E2-T2 - Remove live health probes from request middleware [ ]

Description: Redesign `health.Handler.Middleware()` so it injects cached or local state only, or deprecate it if that contract cannot be made safe.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- do not put DB or HTTP dependency checks on normal application request paths
- if deprecated, provide a clear replacement pattern

### Ticket E2-T3 - Align default health check registration with executed probes [ ]

Description: Update `NewDefaultHandler` so every registered default checker participates in the default liveness/readiness contract, or stop registering unused checkers.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- add regression tests for the final default checker set
- keep detailed health behavior consistent with the same checker inventory

## Epic E3 - Clean up bootstrap and config ergonomics [ ]

Description: Make contrib helpers easier to embed in real services by removing hidden process-control behavior and validating documented configuration constraints earlier.

### Ticket E3-T1 - Replace hard exit and panic helpers with composable variants [ ]

Description: Add error-returning alternatives for panic or exit based bootstrap helpers and steer library users toward them in docs.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep hard exit behavior only in binaries, examples, or explicitly named must or exit helpers
- document which APIs are intended for reusable library code

### Ticket E3-T2 - Remove unbounded background-context startup paths [ ]

Description: Require caller-supplied contexts or enforce explicit internal timeouts in startup constructors so database and migration startup cannot hang indefinitely.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `bootstrap.NewMigrator`, `pgxpool.New`, and `adapters/migrate.New`
- preserve ergonomic wrappers only if they remain bounded and documented

### Ticket E3-T3 - Add semantic config validation [ ]

Description: Validate documented enum-like fields such as `Env` and `LogLevel` during `LoadFromEnv`, and fail fast with aggregated startup errors.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep existing loader error aggregation behavior
- add tests for invalid semantic values and docs showing accepted values
