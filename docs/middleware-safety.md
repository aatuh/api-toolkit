# Middleware Safety Matrix

Audience: application developers composing api-toolkit middleware around
finite JSON routes, streaming routes, admin endpoints, and generated services.

Use this page as the route review checklist. The table explicitly separates
safe global middleware from route-specific middleware, calls out middleware
forbidden for streaming, and lists required opt-outs.

## Placement Matrix

| Middleware or control | Safe global middleware | Route-specific middleware | Forbidden for streaming | Required opt-outs |
| --- | --- | --- | --- | --- |
| `middleware/secure` | Yes. Security headers are safe to apply broadly. | Add route-specific CSP or download headers where needed. | No. | None by default. |
| `middleware/trace` | Yes, when trace IDs are bounded and validation behavior is understood. | Use stricter validation on public APIs that accept external trace headers. | No. | Do not log raw unbounded request metadata. |
| `middleware/maxbody` | Yes for APIs with a common maximum request size. | Use route-specific limits for upload or import routes. | No for inbound request bodies. | Multipart upload routes need a limit sized for expected uploads. |
| `middleware/querylimits` | Yes for conventional APIs. | Tune collection endpoints that intentionally accept wider filters. | No. | Document any high-limit route. |
| `middleware/json` | Yes for JSON-only APIs. | Apply only to routes that should reject non-JSON writes. | No for response streaming, but do not require JSON on non-JSON upload/download routes. | Exclude multipart, binary, webhook raw-body, and health routes where JSON content-type is not the contract. |
| `middleware/auth/apikey`, `middleware/auth/jwt`, `middleware/auth/tenant`, `middleware/auth/authz` | Safe globally only when every public route should be protected. | Prefer route groups for mixed public/private APIs. | No. | Public health, docs, login, callback, or webhook receiver routes need explicit allow-listing and a separate threat review. |
| `middleware/ratelimit` | Safe globally when keys are bounded and shared state is appropriate. | Use route-specific limits for expensive writes, login, webhooks, and background callbacks. | Usually no, but long-lived streams need separate quotas. | Do not key on raw tokens, raw paths, request bodies, idempotency keys, or provider payloads. Empty keys share the anonymous bucket; use `DecisionLimiter` with a distributed adapter for cross-replica quotas. |
| `middleware/idempotency` | No for a whole API. | Required on unsafe writes where retry safety matters. | Yes when it captures responses for replay. | Use `Options.ShouldHandle` and tenant-scoped storage keys; exclude streaming, server-sent events, websocket, large-download, and non-finite response routes. |
| `middleware/timeout` cooperative deadlines | Yes when handlers respect `ctx.Done()`. | Tune deadlines by route class. | No. | Long-running exports, streams, and provider callbacks may need route-specific deadlines. |
| `middleware/timeout` hard timeout | No. Use cooperative `NewPropagator` globally. | Use `HardTimeout.WrapRoute(..., RouteCapabilityFiniteJSON)` only for finite JSON routes that need a wall-clock response cutoff. | Yes for streaming, server-sent events, websocket, large downloads, and handlers requiring `http.Flusher`, `http.Hijacker`, `http.Pusher`, or `io.ReaderFrom`. | Capability validation rejects unsafe hard-timeout routes; use `securityprofile.StreamingRouteOverride` or leave those routes unwrapped. |
| `contrib/middleware/openapi` request validation | Safe globally when route contracts are complete. | Use route policy metadata for public/admin/unsafe-write differences. | No. | Keep routes without stable request contracts out until specified. |
| `contrib/middleware/openapi` response validation | No for a whole API by default. | Use in tests, development, or selected finite production routes. | Yes when response validation buffers responses. | Use `openapi.ResponseValidationOptions.ShouldValidate` to skip streaming, upgrade, large-download, and optional-writer routes. |
| `contrib/middleware/cors` | Safe globally only for browser APIs with one explicit origin policy. | Prefer route groups when admin, public browser, and machine API routes differ. | No. | Never use wildcard credentials; admin endpoints should normally avoid browser CORS. |
| `contrib/middleware/metrics` | Yes when labels are route-pattern bounded and the metrics endpoint is protected. | Add route-specific outcome labels only from enums. | No. | Do not expose `/metrics` publicly when labels or dependency names are sensitive. |
| `contrib/middleware/requestlog` | Yes when the log schema is redaction-safe. | Add route-specific fields only from allow-listed bounded values. | No. | Never log request/response bodies, raw auth headers, API keys, idempotency keys, provider payloads, or raw object keys. |
| `contrib/middleware/oteltrace` | Yes when tracing is intentionally enabled and the exporter endpoint is trusted config. | Disable or sample differently for high-volume routes. | No. | Keep raw tokens, request bodies, provider payloads, and tenant-controlled values out of span attributes. |
| `contrib/middleware/auth/clerk`, `contrib/middleware/auth/oidc`, `contrib/middleware/auth/devheaders` | Safe globally only for fully protected APIs. | Prefer route groups for public callbacks, webhooks, health, and docs. | No. | Dev-header auth is local-development only and must fail closed in production. |

## Recommended Order

1. Recovery and trace context.
2. Security headers and request bounds.
3. JSON/content-type checks for JSON route groups.
4. Authentication and tenant extraction.
5. Authorization and route policy validation.
6. Rate limiting with bounded keys.
7. Idempotency only for unsafe write routes.
8. Request validation before handlers.
9. Response validation only for finite responses selected by
   `openapi.ResponseValidationOptions.ShouldValidate`.
10. Cooperative timeout broadly; hard timeout only through explicit
    `HardTimeout.WrapRoute` for finite JSON responses. Streaming and optional
    writer routes must retain their original writer or use
    `securityprofile.StreamingRouteOverride`.

Streaming, SSE, websocket, and large-download routes should be documented in
the route contract with `x-api-toolkit-streaming` or equivalent local metadata.
Those routes must not depend on response buffering, replay capture, or response
validation to finish correctly.

Use the exact review label "streaming, SSE, websocket, and large-download" when
documenting these opt-outs in package or service release notes.

## Executable Evidence

The unsafe global composition cases are test-covered so the guidance does not
depend on documentation alone:

- `middleware/timeout`: `TestHardTimeoutGlobalCompositionBreaksLargeStreamingRouteAndOptOutPreservesIt`
  proves hard-timeout buffering is unsafe for large streaming-style responses
  and that route-level opt-out preserves the original writer.
- `middleware/idempotency`: `TestShouldHandleOptOutPreservesOptionalResponseWriterInterfaces`
  and `TestIdempotencyMarksAmbiguousStateWhenResponseExceedsBufferLimit` prove
  `Options.ShouldHandle` skips streaming routes and handled oversized replay
  captures fail closed.
- `contrib/middleware/openapi`: `TestResponseValidationCanSkipStreamingRoutes`
  and `TestResponseValidationCanBypassLargeStreamingResponses` prove
  `openapi.ResponseValidationOptions.ShouldValidate` skips streaming and
  large-response routes while leaving request validation in place.
