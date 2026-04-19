# Remediation Backlog

## Epic 1: Contract Hardening For Runtime Safety
Goal: close the three confirmed correctness gaps identified in `audit.md` without broad refactoring.

- [x] Ticket 1: Enforce the documented timeout contract for detailed health passes.
  Scope: `endpoints/health`
  Acceptance:
  - `GetDetailedHealth` applies `HealthCheckConfig.Timeout` to the full detailed pass.
  - A slow checker cannot hang detailed health indefinitely when a timeout is configured.
  - Regression tests cover the timeout path.
  Quality checks:
  - `go test ./endpoints/health`
  - `make lint`
  Commit: completed in the accompanying ticket commit

- [ ] Ticket 2: Preserve upstream retry semantics for non-replayable request bodies.
  Scope: `contrib/adapters/httpclient`
  Acceptance:
  - When a retried request body cannot be replayed, the client returns the original upstream response/error instead of replacing it with `request body is not replayable`.
  - Regression tests cover retryable responses with non-replayable bodies.
  Quality checks:
  - `cd contrib && go test ./adapters/httpclient`
  - `make lint`
  Commit: pending

- [ ] Ticket 3: Fail closed on unresolved idempotency reservation collisions.
  Scope: `middleware/idempotency`
  Acceptance:
  - If `TryBegin` loses the reservation race and the store cannot confirm the existing record, the middleware does not execute the downstream handler again unless `FailOpen` is explicitly enabled.
  - Regression tests cover the collision-with-missing-record path.
  - Same-key retries receive a deterministic error response rather than duplicate execution.
  Quality checks:
  - `go test ./middleware/idempotency`
  - `go test -race ./middleware/idempotency`
  - `make lint`
  Commit: pending

## Completion Rules
- Work one ticket at a time.
- Add or update unit tests before considering a ticket complete.
- Mark the ticket done only after its quality checks pass.
- Commit after each completed ticket.
