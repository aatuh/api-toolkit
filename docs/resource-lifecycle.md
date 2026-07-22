# Resource Lifecycle

Audience: maintainers and adopters wiring network, database, cache, scheduler,
timer, or background-worker behavior.

Root packages mostly expose pure helpers and HTTP middleware. Anything that
opens external resources should make ownership explicit through an interface,
constructor, `Close`, shutdown hook, or app-owned lifecycle.

## Lifecycle Matrix

| Surface | Lifecycle owner | Rule |
| --- | --- | --- |
| `ports.DatabasePool`, `ports.DatabaseConnection`, `ports.DatabaseRows`, `ports.Migrator` | Adapter or app | Implementations own `Close`, transaction cleanup, and canceled-context behavior. |
| `middleware/auth/jwt.Middleware` | App or auth integration | Call `Close` when the middleware owns background JWKS refresh resources. |
| `middleware/timeout` hard timeout | Middleware | One handler goroutine and one bounded response buffer are allocated per wrapped request. The timeout response can return while a cancellation-ignoring handler still runs; use `OnHandlerContinues` for bounded evidence and ensure application work eventually returns. Hooks must not leak goroutines. |
| `middleware/ratelimit` in-process limiter | Middleware | Cleanup intervals and state TTL are middleware-owned after construction. |
| `middleware/idempotency` stores | Store adapter | Store reservation, replay, TTL, and release semantics are adapter-owned. |
| `endpoints/health` checkers | App or adapter | Checks should be bounded and should not expose detailed dependency output publicly. |
| `scheduler` | App or adapter | Runner lifecycle, recorder cleanup, and shutdown ordering are app-owned unless generated scaffold wiring owns them. |
| `webhooks` signing and receiver helpers | App or provider adapter | Secret rotation, replay windows, and provider delivery lifecycle are app-owned. |
| `apiclient` transports | Caller | Caller owns underlying `http.RoundTripper` and `http.Client` lifetimes. |
| Generated services | Generated app | `bootstrap.NewAPIService` wires shutdown hooks, but the checked-in service owns production resources. |

## Review Rules

- Every new package that opens network connections, files, timers, goroutines,
  caches, stores, or background workers must document shutdown ownership.
- Constructors should validate required resources and fail closed on nil stores,
  missing secrets, invalid limits, or impossible lifecycle settings.
- Request cancellation should not prevent required cleanup from being attempted
  with a short bounded cleanup context.
- Do not put provider secrets, DSNs, raw SQL, raw webhook payloads, or raw token
  errors into lifecycle logs, metrics, Problem Details, or release evidence.
- Generated app resources are app-owned even when templates supply default
  shutdown hooks.
