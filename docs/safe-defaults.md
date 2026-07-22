# Safe Defaults Audit

Audience: maintainers and application teams deciding which api-toolkit defaults
can be applied broadly and which controls need route-specific review.

Safe default means the package either fails closed before application side
effects, stays inert until explicitly configured, or exposes an explicit
opt-out for routes where the default would be unsafe. Fail-open behavior must be
intentional, documented, and backed by tests or contract checks.
The Evidence column names where each default is tested or contract-checked.

## Default Posture

| Surface | Default behavior | Failure posture | Evidence |
| --- | --- | --- | --- |
| `github.com/aatuh/api-toolkit/v4/httpx/recover` | Converts request-path panics into Problem Details responses. | Fail-closed for HTTP response shape; panic details are not exposed. | Direct tests and panic policy docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/auth/apikey` | Requires verifier success before protected handlers run. | Fail-closed on missing, invalid, revoked, or insufficient-scope keys. | Direct tests and security docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/auth/authz` | Requires role or authorization checks before protected handlers run. | Fail-closed on missing identity, missing role, or route validation errors. | Direct tests and route validation docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/jwt` | Requires configured issuer, audience, algorithm, and key material for protected routes. | Fail-closed on malformed, expired, wrong-audience, or unverifiable tokens. | Direct tests and generated-service auth docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/auth/tenant` | Compares configured tenant sources before handlers run. | Fail-closed on missing or mismatched required tenant sources. | Direct tests and tenant middleware docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/deprecation` | Adds standards-oriented deprecation headers when configured. | Inert until configured; invalid configuration fails during construction where applicable. | Direct tests and deprecation docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/idempotency` | Reserves unsafe writes only when middleware is applied and storage is configured. | Fail-closed on required missing keys, storage reservation conflicts, and replay mismatches. | Direct tests and idempotency security docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/json` | Enforces JSON content-type rules when applied. | Fail-closed on body-bearing write requests with missing or invalid content types. | Direct tests and cookbook examples. |
| `github.com/aatuh/api-toolkit/v4/middleware/maxbody` | Bounds request body bytes when applied. | Fail-closed on oversized request bodies before downstream parsing. | Direct tests and benchmark baseline. |
| `github.com/aatuh/api-toolkit/v4/middleware/querylimits` | Bounds query size and list parameters when applied. | Fail-closed on malformed or over-limit query input. | Direct tests and benchmark baseline. |
| `github.com/aatuh/api-toolkit/v4/middleware/ratelimit` | Enforces configured request limits with bounded keys. | Fail-closed when the limiter rejects a request; dev bypasses require explicit dangerous-bypass configuration. | Direct tests and bypass docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/secure` | Adds security headers when applied. | Fail-safe header defaults; caller owns CSP and route-specific policy. | Direct tests and security docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/timeout` | Cooperative timeout is safe for broad use; hard timeout requires explicit finite-route wrapping. | Cooperative mode cancels context; `HardTimeout.WrapRoute` fails closed for streaming, SSE, websocket, large-download, and optional-writer capabilities. | Direct tests, determinism check, and hard-timeout docs. |
| `github.com/aatuh/api-toolkit/v4/middleware/trace` | Validates or creates bounded trace identifiers when applied. | Fail-closed on invalid trace headers when validation is configured; otherwise normalizes request context. | Direct tests and request logging docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/clerk` | Validates Clerk token material before handlers run. | Fail-closed unless explicit trusted-proxy dev bypass is enabled. | Direct tests and bypass docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/devheaders` | Development-only identity source. | Fail-closed in production and unless explicit dangerous-bypass env is enabled. | Direct tests and bypass docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/oidc` | Validates OIDC/JWKS token material before handlers run. | Fail-closed on invalid issuer, audience, algorithm, JWKS, or token state. | Direct tests and generated-service auth docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/cors` | Applies explicit CORS policy. | Fail-closed for credentialed wildcard configurations; app owns browser origin policy. | Direct tests and browser-session docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/metrics` | Records bounded HTTP metric labels when applied. | Inert until mounted; labels must stay route-pattern and status-class bounded. | Direct tests and metrics docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/openapi` | Request validation can run broadly; response validation is opt-in and route-filtered. | Request validation fails closed; response validation must fail closed only on finite responses selected by `openapi.ResponseValidationOptions.ShouldValidate`. | Direct tests and OpenAPI docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/oteltrace` | Tracing is disabled by default in generated services. | Fail-closed at startup when tracing is enabled without a configured exporter endpoint. | Direct tests and generated-service shutdown docs. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/requestlog` | Emits bounded structured request metadata. | Inert until applied; raw bodies, secrets, tokens, and unbounded labels stay out of default logs. | Direct tests, metrics docs, and security docs. |

## Route Review Rules

Apply these before using a middleware globally:

- Prefer fail-closed request validation before application side effects for
  authentication, authorization, tenant checks, JSON content-type enforcement,
  request-size limits, query limits, and idempotency requirements.
- Treat fail-open behavior as a deliberate exception. Document the route, the
  reason, and the compensating control in the route contract or service docs.
- Do not apply response-buffering middleware globally to streaming responses,
  server-sent events, websocket upgrades, large downloads, or handlers that need
  optional `http.ResponseWriter` interfaces.
- Keep dev bypasses behind explicit dangerous-bypass env vars, trusted-proxy
  checks, and production startup refusal where the generated profile provides
  one.
- Keep observability labels bounded: method, route pattern, status class, and
  outcome enums are acceptable; raw paths, request bodies, secrets, tokens,
  idempotency keys, tenant-controlled object keys, and provider payloads are
  not.

The detailed placement matrix lives in [middleware-safety.md](middleware-safety.md).
