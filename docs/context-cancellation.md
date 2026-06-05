# Context And Cancellation

Audience: maintainers reviewing HTTP, auth, idempotency, scheduler, client, and
adapter-facing APIs.

Context ownership follows Go HTTP conventions:

- incoming request work uses `r.Context()`,
- provider, store, validator, scheduler, and client boundaries accept
  `context.Context`,
- middleware that creates deadlines must pass the derived context downstream,
- cleanup that must outlive a canceled request uses a bounded cleanup context,
- pure parsing helpers without I/O document that context is not needed.

## Package Guidance

| Area | Rule |
| --- | --- |
| HTTP handlers and middleware | Use the request context and do not replace it with `context.Background()`. |
| Auth verifiers | Accept context and honor cancellation for stores, JWKS, or provider calls. |
| Idempotency stores | Accept context for reservations, saves, replays, and releases. |
| Scheduler recorders | Accept context; completed jobs may use a bounded cleanup context for final persistence. |
| API clients | Accept request context or preserve the caller's request context. |
| Webhooks | Verification is local CPU work, but receiver handlers and outbound signers should preserve request context. |
| Route contracts and specs | Pure registration and parsing paths do not need context unless they call user code. |
| Ports | Context-bearing methods must not ignore cancellation when they perform I/O. |

## Timeout Middleware

`middleware/timeout.NewPropagator` adds a cooperative deadline and relies on
handlers to observe `ctx.Done()`. `middleware/timeout.NewHard` also buffers
finite responses and writes a timeout Problem Details response, but it cannot
stop CPU work that ignores cancellation.

Do not apply hard-timeout buffering globally to streaming, SSE, websocket, or
large-download routes.

## Review Checklist

- Does the API accept context before user-controlled I/O, network calls, locks,
  retries, or long-running work?
- Does the implementation pass `r.Context()` instead of creating background
  request contexts?
- Are cleanup contexts bounded and documented?
- Are context cancellation errors wrapped safely without leaking secrets or raw
  provider payloads?
- Are tests using `httptest.NewRequestWithContext` or explicit canceled
  contexts for cancellation-sensitive behavior?
