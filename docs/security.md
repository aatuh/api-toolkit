# Security Posture

Audience: application developers and operators configuring secure defaults,
dangerous bypasses, trusted proxies, health detail, and docs surfaces.

This project ships secure-by-default middleware, opt-in controls, and
documentation aligned with OWASP API Security guidance.

## Defaults and Hardening

Core defaults aim to be safe, deterministic, and explicit:

- Problem Details responses (RFC 9457) for consistent errors.
- Strict JSON content-type enforcement when enabled, including rejection of
  body-bearing write requests that omit `Content-Type`.
- Detailed dependency health output is opt-in; public health surfaces should
  keep `EnableDetailed` disabled unless operators explicitly need it.
- Docs HTML defaults to a first-party static page; the CDN-backed Swagger UI
  surface requires explicit opt-in.
- Request body limits via `middleware/maxbody`.
- Query limits via `middleware/querylimits`.
- Security headers via `middleware/secure`.
- Trace context validation via `middleware/trace`.
- API key middleware, OAuth2 helpers, upload decoding, and webhook verification
  provide transport contracts; applications still own secret storage, provider
  validation, content scanning, replay databases, and authorization policy.
- HTTP health-check URLs are trusted application configuration; do not derive
  them from request parameters or tenant-controlled input. Use an SSRF-guarded
  outbound client from `github.com/aatuh/api-toolkit/contrib/v4/adapters/httpclient`
  for any target that can be influenced outside trusted config.
- Migration reruns fail closed when a previous commit outcome is uncertain, so
  non-idempotent DDL is not retried blindly.

For a per-middleware fail-open and fail-closed matrix, use
`docs/safe-defaults.md`. For global versus route-specific placement, streaming
opt-outs, and response-buffering caveats, use `docs/middleware-safety.md`. For
header, body, JSON, query, multipart, replay-capture, and hard-timeout capture
sizes, use `docs/input-size-threat-review.md`.

## Bypass and Dev-Only Controls

Some features include explicit bypasses for local development only:

- Rate limit skip header: only honored when `AllowDangerousDevBypasses`
  is true and the request comes from a trusted proxy. Env wiring uses
  `RATE_LIMIT_SKIP_ENABLED`, `RATE_LIMIT_SKIP_HEADER`,
  `RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES`, and `TRUSTED_PROXIES`.
- Generated services use `RATE_LIMIT_STORE=memory` for local development.
  When `ENV=production`, generated services default to
  `RATE_LIMIT_STORE=redis` and require `RATE_LIMIT_REDIS_ADDR` or `REDIS_ADDR`
  so rate-limit state is shared across instances.
- Generated full-profile protected routes use hashed rate-limit keys derived
  from actor, tenant, method, and route path. Keep rate-limit labels and Redis
  keys bounded; do not include raw bearer tokens, API keys, idempotency keys,
  object keys, webhook URLs, request bodies, or provider payloads.
- Generated services set `OTEL_TRACING_ENABLED=false` by default. When tracing
  is enabled, `OTEL_EXPORTER_OTLP_ENDPOINT` is required and the OpenTelemetry
  tracer provider is closed through the service shutdown hooks.
- JWT auth skip header: same trusted-proxy restriction. Env wiring uses
  `JWT_SKIP_HEADER_ENABLED`, `JWT_SKIP_HEADER_NAME`,
  `JWT_SKIP_TRUSTED_PROXIES`, and `JWT_ALLOW_DANGEROUS_DEV_BYPASSES`.
- Clerk auth skip header: same proxy restriction. Env wiring uses
  `CLERK_SKIP_HEADER_ENABLED`, `CLERK_SKIP_HEADER_NAME`,
  `CLERK_SKIP_TRUSTED_PROXIES`, and `CLERK_ALLOW_DANGEROUS_DEV_BYPASSES`.
- Dev header auth fallback: requires explicit dangerous-bypass opt-in and only
  trusts headers from configured trusted proxies. Env wiring uses
  `DEV_AUTH_FALLBACK_ENABLED`, `DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES`,
  and `DEV_AUTH_TRUSTED_PROXIES`.
- Stripe webhook verification skip in adapters: intended for development only.
  Env wiring uses `STRIPE_WEBHOOK_SKIP_VERIFY` and `STRIPE_WEBHOOK_DEV_MODE`.

Do not enable dev bypasses in production environments.

## Threat Model (Short)

The full review map is [Security Threat Model](threat-model.md). This short
section keeps the operational assumptions visible inside the security posture
guide.
Use [package security review](security-review.md) to record the required
threat, input, secret, authorization, DoS, data-leakage, and observability
evidence for each affected package.

Assume:

- Public endpoints are reachable from untrusted networks.
- Clients can send malformed or oversized requests.
- Attackers will probe authentication, authorization, and rate limits.

Goal:

- Fail safe by default and make bypasses explicit and restricted.
- Bound resource usage (CPU, memory, network) per request.
- Preserve cleanup and operator visibility even during timeout, cancellation,
  or partial infrastructure failure paths.
- Keep auditability via structured logs and stable errors.

## Security-sensitive helper ownership

| Area | Toolkit/generated responsibility | Application/operator responsibility |
| --- | --- | --- |
| API keys | Core API-key auth extracts credentials and enforces verifier decisions. Generated full-profile API-key management returns raw key secrets only from create responses, stores peppered SHA-256 hashes, displays non-secret prefixes, tracks `last_used_at`, scope-checks keys, and rejects revoked keys. | Store, rotate, and revoke keys in application-owned verifier code. Set `API_KEY_PEPPER` explicitly in production and keep raw keys out of logs, metrics, Problem Details, examples, and release evidence. |
| Invitations and tenant routes | Generated invitation services store token hashes, reject wrong or replayed tokens, require role checks, and return raw invitation tokens only from creation. Organization-scoped routes require authentication, `Idempotency-Key` on unsafe writes, role checks, and matching `X-Tenant-ID`/organization path values. | Do not log raw invitation tokens. Use `API_ACTOR_ID` for bootstrap-key actor identity; `X-Actor-ID` is only a non-production fallback before generated scoped API keys exist. |
| Idempotency | Generated unsafe writes use `Options.RequireKey`, return Problem Details 400 when required keys are missing, and use `TenantScopedStorageKeyFunc()` for tenant-aware hashed storage keys. `IDEMPOTENCY_STORE=redis` keeps replay state outside the process for multi-instance services. | Keep logs and metrics limited to bounded outcome labels. Do not record raw `Idempotency-Key` values or replay response bodies. Adapter legacy idempotency recovery events hash keys by default; raw keys should stay disabled in production. |
| Rate limits, cache, and Redis | Generated services default to memory for local development and use Redis when `CACHE_STORE=redis`, `RATE_LIMIT_STORE=redis`, `IDEMPOTENCY_STORE=redis`, or production defaults select it. Rate-limit keys are derived from actor, tenant, method, and route path. | Use service-specific Redis prefixes. Keep cache/rate-limit keys low sensitivity and never store API-key secrets, invitation tokens, webhook signing secrets, provider secrets, object keys, or raw request payloads in shared cache values. |
| Postgres and migrations | When `DATABASE_URL` is configured, generated startup checks open a pgx pool, check required platform tables, and fail readiness/admin health closed if Postgres is unavailable. Generated migration `down` commands require `ALLOW_DANGEROUS_MIGRATION_DOWN=true` plus the CLI guard. | Keep DSNs, SQL errors, migration details, and Terraform state in operator-only logs or secret stores. Review `migrate plan` and `migrate verify` before rollout; do not enable dangerous down guards in shared or production environments. |
| Auth providers and OAuth/OIDC | Generated JWT, Clerk, and OIDC modes validate issuer, audience, algorithms, and JWKS material before handlers run. Tenant-scoped bearer routes fail closed on tenant mismatch. OAuth2 helpers standardize claims only after validator checks. | Keep issuer, audience, discovery URL, and JWKS URL as trusted operator configuration. Do not enable skip headers outside explicit development bypass policy. |
| Webhooks and providers | Core webhook helpers verify signatures and replay windows. Generated outbound webhook routes omit signing secrets from list, delivery, replay, audit, and Problem Details surfaces. With `DATABASE_URL`, `WEBHOOK_SECRET_KEY` encrypts endpoint signing secrets before persistence. Optional provider workflows verify signed callbacks before tenant-scoped audit writes. | Keep receiver allow-lists, SSRF-aware transports, duplicate suppression, provider schemas, endpoint rotation, and provider API keys app-owned. Do not put callback payloads, endpoint URLs, receiver errors, or provider secrets in Problem Details, metrics labels, OpenAPI examples, or audit metadata. |
| Audit and observability | Generated audit hooks record write actions with redaction-safe metadata. Webhook/request-log hooks expose bounded event type, outcome, and status-class labels. `OTEL_TRACING_ENABLED=false` by default; when tracing is enabled, `OTEL_EXPORTER_OTLP_ENDPOINT` is required and the tracer provider is closed during shutdown. | Treat audit metadata as allow-listed operational context only. Keep tenant IDs, endpoint IDs, delivery IDs, URLs, request bodies, secrets, raw provider errors, and unbounded user input out of logs, traces, metrics labels, dashboards, and release evidence. |
| Uploads and object storage | Upload helpers reject malformed, oversized, missing, or disallowed multipart files. Object helpers validate bucket/key references, content-type, size, and secret-shaped metadata. Generated object routes reject traversal-shaped, nested, hidden-file-shaped, unsupported, and oversized keys. | Scan and persist untrusted content outside core helpers. Keep S3 endpoints trusted operator configuration, not tenant input, and avoid logging object keys when they can encode personal data. |
| Browser sessions | The generated `saas-web` profile is isolated from API-first scaffolds and emits HttpOnly/Secure/SameSite cookies, CSRF middleware, Redis-backed session-store boundaries, OIDC callback state validation, safe browser CORS, and production `SESSION_SECRET` checks. | Keep raw session IDs and CSRF tokens out of logs, metrics, audit metadata, OpenAPI examples, and Problem Details. Browser CORS must never use wildcard credentials. |
| Deployment starters | Generated observability, Helm, Kubernetes, and Terraform starters document expected checks and placeholders. | Treat starters as templates. Keep Terraform state, Helm values, Kubernetes Secret manifests, DSNs, API keys, provider tokens, and object-store credentials out of source control. |

Tenant middleware compares configured tenant sources before handlers run. Use
`RequireAllSources` for routes that must prove a request header or URL tenant
matches the authenticated tenant scope. Cookbook recipes for these helpers live
in `docs/cookbook.md`; scaffold-specific setup and operations live in
`docs/full-service-scaffold.md`, `docs/reference-service.md`, and
`examples/reference-saas-api/`.

## Security-Sensitive Production Settings

| Surface | Required production setting | Canonical guide |
| --- | --- | --- |
| Auth | Configure API-key, JWT, OIDC, or Clerk issuer/audience/JWKS values from trusted operator config; disable dangerous bypasses in production. | `docs/auth.md` |
| Tenant | Require authenticated tenant scope and compare required path/header/token sources before handlers run. | `docs/auth.md` |
| Admin endpoints | Keep detailed health, metrics, and pprof behind admin auth or internal network policy. | `docs/operations.md` |
| Pprof | Use `pprof.RegisterAdminRoutes` or `bootstrap.MountSystemEndpointsToWithAdmin`; do not mount pprof on public routers. | `docs/operations.md` |
| Metrics | Use bounded route-pattern labels and keep tenant IDs, user IDs, API keys, admin keys, idempotency keys, raw paths, and provider payloads out of labels. | `docs/metrics.md` |
| Idempotency | Require `Idempotency-Key` on unsafe writes, use durable storage in multi-instance services, hash tenant-scoped storage keys, and exclude streaming responses. | `docs/idempotency.md` |
| Webhooks | Verify signatures before trusting payloads, cap request bodies, add replay protection, and keep signing secrets out of responses, logs, metrics, and OpenAPI examples. | `docs/cookbook.md#webhook-receiver-with-signature-verification` |
| OpenAPI validation | Keep request validation on when contracts are complete; route-filter response validation for finite responses only. | `docs/openapi-workflow.md` |

## OWASP Mapping (Resource Consumption)

The toolkit provides concrete controls for resource limits:

- Timeouts: `securityprofile.WithTimeout` and `RouteOverride.Timeout` apply cooperative request context deadlines; `securityprofile.WithHardTimeout` and `middleware/timeout.NewHard` add a hard wall-clock response cutoff that writes a 504 Problem Details timeout response and discards late handler writes
- Hard-timeout capture limits: `securityprofile.WithHardTimeoutMaxCaptureBytes`, `RouteOverride.HardTimeoutMaxCaptureBytes`, and `middleware/timeout.Options.MaxCaptureBytes` bound buffered non-streaming responses
- OpenAPI response validation: `openapi.ResponseValidationOptions.ShouldValidate`
  excludes streaming, upgrade, or large-download routes from response buffering
  while leaving OpenAPI request validation enabled
- Payload size: `securityprofile.WithMaxBodyBytes` and `RouteOverride.MaxBodyBytes`
- Query limits: `securityprofile.WithQueryLimits` and `querylimits.Options`
- Rate limits: `securityprofile.WithRateLimitOptions` and `RouteOverride.RateLimit`
- Header limits: `httpx.HeaderLimitsBalanced` + server `MaxHeaderBytes`

The detailed input-size map lives in
[input-size-threat-review.md](input-size-threat-review.md). Review it before
changing route body limits, query limits, multipart bounds, idempotency replay
capture sizes, or hard-timeout capture sizes.

`securityprofile.WithTimeout` does not force a timeout response or stop handlers
that ignore `ctx.Done()`. Use `securityprofile.WithHardTimeout`,
`middleware/timeout.NewHard`, and server read/write deadlines when you need a
hard wall-clock response limit. Hard timeout wrappers cannot stop CPU work in a
handler that ignores cancellation, but they do prevent late response writes.
They buffer responses up to a configured maximum capture size and return a
Problem Details error instead of silently truncating oversized responses. Do not
apply hard timeout globally to streaming responses, server-sent events,
websocket upgrades, or handlers that require optional `http.ResponseWriter`
interfaces such as `http.Flusher` or `http.Hijacker`. Tune capture size globally
with `securityprofile.WithHardTimeoutMaxCaptureBytes` or per route with
`RouteOverride.HardTimeoutMaxCaptureBytes` for large non-streaming responses;
these knobs do not make streaming routes safe. Use
`securityprofile.StreamingRouteOverride` for streaming, SSE, websocket, or
large-download routes that need to preserve optional writer interfaces and avoid
timeout response buffering. Handler panics inside hard timeout are contained in
the child goroutine. Before the timeout response wins, the middleware returns a
deterministic 500 Problem Details response; after the timeout response wins,
late panics are dropped with late writes.

`middleware/timeout.Options.EventHooks` exposes bounded operator
metadata for timeout, panic, and capture-overflow outcomes. The event contract
intentionally omits panic values, URL paths, query strings, request headers,
response headers, and bodies, so it is suitable for low-cardinality counters or
sanitized structured logs. Contrib services can wire the event to
`metrics.HardTimeoutEventHook` for `http_hard_timeout_events_total` and
`requestlog.HardTimeoutEventLogHook` for bounded structured log fields.

OpenAPI response validation also buffers handler responses so it can validate
the final status, headers, and body against the route contract. Do not apply it
to streaming responses, server-sent events, websocket upgrades, or routes that
need optional writer interfaces. Keep request validation enabled globally and
use `openapi.ResponseValidationOptions.ShouldValidate` to opt those routes out
of response validation.

## Admin-only endpoints and EU privacy posture

Pprof, detailed health, trace IDs, metrics labels, and request logs can expose
operational data, request metadata, dependency names, tenant identifiers, or
other personal data under an EU baseline privacy posture. Treat these as
operator-only surfaces.

- Mount pprof with `pprof.RegisterAdminRoutes` and pass an explicit admin or
  internal-network wrapper; keep legacy `pprof.RegisterRoutes` only for source
  compatibility or separately protected internal muxes.
- Mount detailed health with `Handler.RegisterAdminDetailedHealthRoute` when it
  is enabled; avoid teaching policy-free detailed-health mounts in new examples.
- Mount combined system endpoints with `bootstrap.MountSystemEndpointsToWithAdmin`
  when detailed health, pprof, or metrics are present so operator-only routes
  require an explicit wrapper.
- Prefer `bootstrap.APIServiceConfig.AdminAddr` for new generated services that
  can run a separate admin listener; public routes keep public probes/docs while
  detailed health, metrics, and pprof stay on the admin handler.
- Generated `saas-api-full` services expose `/livez` as process-only liveness
  and `/readyz` as dependency readiness. Do not attach Postgres, Redis, S3, or
  provider checks to liveness.
- Generated `saas-api-full` services enable OpenAPI request validation by
  default. Response validation is intended for tests/development or explicit
  production opt-in with `OPENAPI_RESPONSE_VALIDATION=true`.
- To migrate policy-free pprof, replace `pprof.RegisterRoutes(router)` with
  `pprof.RegisterAdminRoutes(router, requireAdmin)` and fail startup if it
  returns an error.
- To migrate policy-free detailed health, keep `/live` and `/ready` public and
  mount only the detailed endpoint on an admin router with
  `healthHandler.RegisterAdminDetailedHealthRoute(adminRouter, requireAdmin)`.
- Keep public liveness/readiness separate from detailed dependency output.
- Keep request log payloads redacted, keep metrics labels bounded, and avoid raw
  idempotency keys unless a short access-controlled incident review requires
  them.
- Generated `saas-api-full` metrics use the contrib Prometheus recorder behind
  admin authentication. HTTP metric labels come from router patterns, not raw
  paths, and generated tests check that tenant IDs, actors, API keys, admin
  keys, and idempotency keys are not exported.
- Generated `saas-api-full` pprof routes use `pprof.RegisterAdminRoutes` with
  the generated admin wrapper; the public handler does not mount pprof, and
  generated tests verify missing admin auth fails closed.
- Adapter legacy idempotency recovery events hash keys by default. Enable the
  explicit raw-key option only for short, access-controlled incident review.
- Multi-tenant APIs that share idempotency storage should configure
  `middleware/idempotency.Options.StorageKeyFunc` with
  `TenantScopedStorageKeyFunc()` after auth and tenant middleware, so storage
  keys are scoped by tenant and actor without embedding raw tenant IDs, user
  IDs, or client-supplied idempotency keys.
- For unsafe writes whose route contracts require idempotency, configure
  `middleware/idempotency.Options.RequireKey` so missing `Idempotency-Key`
  requests fail with Problem Details 400 before side effects run.
- Prefer upstream network policy plus application authorization for admin
  routes; endpoint helpers do not create legal compliance by themselves.

## Recommended Production Baseline

```go
log := logzap.NewProduction()
profile, err := securityprofile.OWASPBaseline(
	securityprofile.WithAuthCheck(func(r *http.Request) bool {
		return r.Header.Get("Authorization") != ""
	}),
	securityprofile.WithRateLimitOptions(ratelimit.Options{
		Capacity:   60,
		RefillRate: 30,
	}),
)
if err != nil { /* handle */ }

r := chi.New()
profile.Apply(r)

srv := bootstrap.HardenedServer(":8080", r, func(s *http.Server) {
	httpx.HeaderLimitsBalanced.ApplyServer(s)
})
```

## Checklist

- [ ] Enable OWASP baseline limits for context deadlines, payload, query, and rate limits
- [ ] Set MaxHeaderBytes with `httpx.HeaderLimitsBalanced` or stricter
- [ ] Require authentication by default and deny by default
- [ ] Avoid dev-only bypass headers in production
- [ ] Keep API key storage, OAuth2 provider validation, upload scanning, and
      webhook replay stores application-owned
- [ ] Use TLS in production and prefer TLS 1.3
- [ ] Monitor for rate limiting and validation errors
