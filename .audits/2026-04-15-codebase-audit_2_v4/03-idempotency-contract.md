# Idempotency Save-Failure Contract

Date: 2026-04-16

Scope: Epic E2, Ticket E2-T1

## Decision

The idempotency middleware will use a fail-closed contract when the downstream
handler succeeds but the completed idempotency record cannot be persisted.

## Client-visible behavior

- The middleware must not return the captured downstream success response.
- The middleware must return `503 Service Unavailable`.
- The response body should explain that the idempotency record could not be
  persisted.
- The same request may be retried with the same `Idempotency-Key` after the
  failed persistence path releases its reservation.

## Rationale

- Returning success after a failed completion save creates an unsafe retry
  contract: the client sees success, but the server has no durable replay record.
- A `503` makes the ambiguity explicit and preserves the middleware's
  documented "safe retry" position.
- This is stricter than the current runtime behavior and is intentionally
  backward-incompatible for callers that previously received the original success
  response on completion persistence failure.

## Follow-on work

- E2-T2 will implement the runtime change and update public docs/examples.
- E2-T3 will harden default key scoping for authenticated and multi-tenant APIs.
- E2-T4 and E2-T5 will cover buffering guardrails and regression tests.
