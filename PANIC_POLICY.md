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
