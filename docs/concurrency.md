# Concurrency Safety

Audience: API consumers deciding whether values can be reused across requests.

Default rule: construct middleware, registries, codecs, and adapters during
startup, then treat them as immutable unless the package explicitly documents
mutation or synchronization.

## Safety Matrix

| Surface | Concurrency posture |
| --- | --- |
| `httpx`, `fielderrors`, `binding`, `queryparams`, `negotiation`, `upload` helpers | Stateless helpers are safe to call concurrently when caller-provided readers and requests are not shared unsafely. |
| Middleware constructed with `New` | Safe for concurrent request use after construction unless injected dependencies are not safe. |
| `middleware/ratelimit` in-process state | Shared state is synchronized; distributed exactness requires an adapter. |
| `middleware/idempotency` | Middleware is safe after construction; store implementations own their own concurrency safety. |
| `middleware/timeout` hard timeout capture | Per-request capture state and handler goroutine are request-scoped; event hooks must be concurrency-safe and non-blocking. A handler can continue after its timeout response until it honors cancellation or returns. |
| `routecontracts.Registry` and `specs.Registry` | Build during startup; avoid mutating while serving traffic. |
| `endpoints/health` | Checker registration should happen during startup; checker implementations own concurrency safety. |
| `scheduler` | Recorder, logger, and last-run provider implementations own shared-state safety. |
| `ports` interfaces | Implementations own concurrency guarantees and must document them. |
| Generated scaffold code | App-owned; generated defaults are tested, but product additions must document their own safety. |

## Review Rules

- Do not mutate config structs after passing them to constructors unless the
  package explicitly copies the data.
- Do not share `*http.Request`, request bodies, multipart readers, response
  writers, or test recorders across goroutines without app-owned synchronization.
- Hooks and callbacks may run on request goroutines; they must avoid blocking
  and must be safe under concurrent calls.
- Caches, token resolvers, JWKS providers, Redis/Postgres adapters, and
  generated service stores must document whether they are safe for concurrent
  use and who owns `Close`.
- Run `make test-race` when shared middleware state, registries, stores, or
  generated service wiring changes.
