# Panic Policy

This library avoids panics in production code paths. Panics crash the host
process and are only acceptable for explicit opt-in helpers or truly
unrecoverable invariants.

## Allowed

- `Must*` helpers that wrap a non-`Must` variant and are intended for startup
  wiring or tests.
- Internal invariants (e.g., impossible states, compile-time constants that
  fail validation).
- Tests that verify invariants or guardrails.

## Not allowed

- Input validation errors.
- Environment, configuration, or network failures.
- Any error a caller can reasonably handle.

## Guidelines

- Prefer typed or sentinel errors for recoverable failures.
- Every `Must*` function must have a non-`Must` alternative that returns `error`.
- If a panic is required for an invariant, keep the scope private and document
  the invariant in code.

## HTTP Recovery Contract

- `httpx/recover` converts uncommitted handler panics into a `500` Problem
  Details response and logs the panic.
- If a handler panics after response headers or body bytes have already been
  committed, the recovery middleware must not preserve a misleading partial
  success response.
- In that committed-response case, recovery should log the panic and abort the
  request so the server treats it as a failed response instead of a successful
  truncated one.

## Stable Package Panic Audit

Every package-level `panic`, `recover`, `template.Must`, or
`regexp.MustCompile` use in stable or compatibility-only root packages must be
listed here. Startup-only `Must` calls are allowed only for static templates or
regular expressions that cannot depend on request input.

| Source | Behavior | Policy | Evidence |
| --- | --- | --- | --- |
| `endpoints/docs/docs.go` | `must` | Static HTML templates are parsed at package initialization; request input cannot affect these templates. | `endpoints/docs/handlers_test.go` verifies served docs output and escaping. |
| `httpx/recover/recover.go` | `recover` | Request-path panics are recovered before response commit and converted to Problem Details. | `httpx/recover/recover_test.go` and `httpx/recover/example_test.go`. |
| `httpx/recover/recover.go` | `panic` | `http.ErrAbortHandler` is rethrown for explicit aborts or committed-response panics so partial successes are not preserved. | `httpx/recover/recover_test.go` covers abort propagation and committed-response behavior. |
| `middleware/idempotency/idempotency.go` | `recover` | Downstream handler panics trigger reservation cleanup before propagation to the caller's recovery layer. | `middleware/idempotency/idempotency_test.go` covers retry after panic cleanup. |
| `middleware/idempotency/idempotency.go` | `panic` | The original downstream panic is rethrown after cleanup so idempotency does not swallow application failures. | `middleware/idempotency/idempotency_test.go` asserts the first request still panics. |
| `middleware/idempotency/legacy_compatibility.go` | `recover` | Compatibility telemetry sink panics are contained so observability callbacks do not fail requests. | `middleware/idempotency/idempotency_test.go` covers sink panic containment. |
| `middleware/idempotency/outcome.go` | `recover` | Outcome callback panics are contained and logged so user hooks cannot break request handling. | `middleware/idempotency/idempotency_test.go` covers outcome behavior and logging paths. |
| `middleware/timeout/timeout.go` | `recover` | Hard-timeout child goroutine panics are converted to deterministic timeout-panic Problem Details before response commit; late panics after timeout are dropped. | `middleware/timeout/timeout_test.go`, `docs/security.md`, and `docs/metrics.md`. |
| `scheduler/scheduler.go` | `recover` | Scheduled job panics are recorded as failed runs with stack logging, and the scheduler continues future intervals. | `scheduler/scheduler_test.go` covers panic recording and continued execution. |
| `specs/problems.go` | `must` | The component-name regexp is a static compile-time invariant. | `specs/registry_test.go` and `specs/schema_test.go` exercise generated Problem Details components. |
