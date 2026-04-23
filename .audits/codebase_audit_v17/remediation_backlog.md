# Backlog

Project: api-toolkit codebase audit v17 remediation

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Repair idempotency failure semantics [ ]

Description: Make idempotency behavior correct under cancellation, retries, and future store implementations so payment-like flows do not get stranded by infrastructure edge cases.

### Ticket E1-T1 - Decouple idempotency cleanup from request cancellation [x]

Description: Replace request-scoped cleanup and ambiguity persistence with a bounded cleanup context so client disconnects and request timeouts do not strand reservations.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- mirror the cleanup pattern already used in `contrib/adapters/txpostgres` and `scheduler`
- cover panic, 5xx, oversized response, and store-save failure paths

### Ticket E1-T2 - Make store release semantics explicit and safe [x]

Description: Remove the hidden dependency on optional `Release` behavior by either promoting release into `ports.IdempotencyStore` or redesigning the middleware state machine so stores without delete semantics remain correct and retryable.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- add a conformance test for a store that intentionally does not implement the current optional releaser
- keep existing bundled stores passing through the new contract

### Ticket E1-T3 - Canonicalize idempotency request hashing [x]

Description: Normalize hash inputs so semantically equivalent retries produce identical hashes even when query parameter ordering or other serialization details differ.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- sort query parameters deterministically
- preserve multi-value parameter semantics
- add coverage for reordered query strings and equivalent retries

### Ticket E1-T4 - Expand idempotency regression coverage [ ]

Description: Add negative-path and contract tests for cancellation cleanup, store contract mismatch, canonical hashing, and retry windows so future changes cannot reintroduce these bugs.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- keep the tests behavior-oriented rather than asserting internal implementation details
- include at least one test that proves retries recover after request-context cancellation

## Epic E2 - Harden observability registration behavior [ ]

Description: Ensure first-party observability helpers fail predictably instead of silently dropping metrics when applications compose the library with existing collectors.

### Ticket E2-T1 - Surface Prometheus registration conflicts [ ]

Description: Change the Prometheus recorder constructor and helper functions so incompatible metric registration errors are returned or otherwise surfaced explicitly instead of being swallowed.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- preserve the current `AlreadyRegistered` reuse behavior for compatible collectors
- decide deliberately whether the new API should return an error or require explicit opt-in to silent fallback

### Ticket E2-T2 - Add metrics registration conflict tests [ ]

Description: Add tests for compatible reuse and incompatible descriptor conflicts so metrics setup behavior remains deterministic and visible.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover default and custom registerer scenarios
- verify that failure cases are observable to callers

## Epic E3 - Reduce stable boundary leakage [ ]

Description: Move the stable core closer to a genuinely adapter-neutral boundary so future evolution does not require downstream application code to speak provider or driver dialects.

### Ticket E3-T1 - Define and implement the billing-port extraction path [ ]

Description: Move Stripe-shaped billing contracts out of the stable core surface or formally deprecate them behind a compatibility package with a documented migration path.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- update `README.md`, `docs/architecture.md`, `docs/ports-surface.md`, and release notes together
- keep compatibility guarantees explicit if the extraction spans more than one release

### Ticket E3-T2 - Isolate legacy database stats compatibility from new call sites [ ]

Description: Finish the migration toward plain-value pool stats by keeping new code on snapshot APIs and containing legacy `DatabaseStats` usage to compatibility adapters.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- prefer value snapshots in health and observability call sites
- document the intended end-state clearly so adapter authors know which surface is legacy

## Epic E4 - Remove duplication and API inconsistencies [ ]

Description: Reduce future drift in security-sensitive packages and normalize edge-case behavior across public middleware adapters.

### Ticket E4-T1 - Consolidate shared JWT and Clerk middleware flow [ ]

Description: Refactor the duplicated JWT validation pipeline into shared machinery with provider-specific subject mapping hooks so future auth fixes land once.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- preserve existing public behavior and config names unless there is a deliberate migration plan
- add parity tests so JWT and Clerk continue to enforce the same skip-header and claim-validation rules

### Ticket E4-T2 - Make docs middleware nil-safe and add parity tests [ ]

Description: Align `endpoints/docs.Handler.Middleware` with the nil-safe behavior already used by sibling middleware adapters and lock that behavior in with tests.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket
- ensure the quality check covers testing, formatting, linting, API compatibility, docs checks, vulnerability checks, security checks, race checks, and fuzzing where relevant
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- mirror the nil-safe adapter pattern already used in `endpoints/health`
- keep the public middleware contract consistent across packages
