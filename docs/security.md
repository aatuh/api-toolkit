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
  outbound client from `github.com/aatuh/api-toolkit/contrib/v2/adapters/httpclient`
  for any target that can be influenced outside trusted config.
- Migration reruns fail closed when a previous commit outcome is uncertain, so
  non-idempotent DDL is not retried blindly.

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

- API key auth extracts credentials and enforces verifier decisions. Store,
  hash, rotate, and revoke keys in application-owned verifier code.
- Generated full-profile API-key management returns raw key secrets only from
  the create response, stores only peppered SHA-256 hashes, displays non-secret
  prefixes, tracks `last_used_at` during verification, and requires admin role
  checks for create/list/revoke. API-key auth mode keeps `API_KEY` as a
  bootstrap setup credential and verifies generated keys through the generated
  API-key service when the bootstrap key does not match. Generated keys are
  scope-checked, bound to their organization, and rejected after revocation.
  Set `API_KEY_PEPPER` explicitly in production and keep raw key secrets out of
  logs, metrics, Problem Details, and examples.
- Generated full-profile invitation services return raw invitation tokens only
  from the creation use case, store token hashes, reject wrong or replayed
  tokens, and require role checks before invitation creation. Do not log raw
  invitation tokens or include them in Problem Details responses.
- Generated full-profile organization and invitation HTTP routes require
  authentication, `Idempotency-Key` on unsafe writes, role-aware application
  checks, and matching `X-Tenant-ID`/organization path values for
  organization-scoped routes. API-key full-profile scaffolds use
  `API_ACTOR_ID` for bootstrap-key actor identity; the `X-Actor-ID` request
  header is only a non-production fallback for exercising role flows before a
  generated scoped API key exists.
- Generated full-profile async widget imports keep operation polling
  tenant-scoped, reuse `Idempotency-Key` for replay-safe acceptance, and store
  sanitized operation failure problems. Do not put raw request payloads,
  provider errors, secrets, tenant identifiers, or idempotency keys into async
  logs, metrics labels, Problem Details, or operation results.
- Generated full-profile Postgres startup checks should fail closed before
  serving production traffic when `DATABASE_URL` is configured but the database
  is unreachable or required tables are missing. Readiness and admin detailed
  health expose only generic unavailable responses; keep DSNs, SQL errors, and
  migration details in operator logs, not HTTP Problem Details.
- Generated full-profile audit hooks record write actions without raw API-key
  secrets, invitation tokens, idempotency keys, authorization headers, request
  payloads, or provider errors. Treat audit metadata as allow-listed operational
  context only; anything credential-shaped should be dropped before recording.
- Tenant middleware compares configured tenant sources before handlers run. Use
  `RequireAllSources` for routes that must prove a request header or URL tenant
  matches the authenticated tenant scope.
- Generated full-profile JWT, Clerk, and OIDC modes validate issuer, audience,
  allowed algorithms, and JWKS material before handlers run. Tenant-scoped
  routes fail closed when the bearer token tenant claim does not match
  `X-Tenant-ID`; protected writes also require the route-specific scope.
- OAuth2 helpers standardize claims and scopes only after a validator verifies
  issuer, audience, expiry, JWKS material, and tenant mapping. The supported
  contrib OIDC middleware provides provider-neutral JWKS validation and claim
  mapping for API bearer tokens; keep issuer, audience, discovery URL, and JWKS
  URL as trusted operator configuration and do not enable skip headers outside
  explicit development bypass policy.
- Upload helpers reject malformed, oversized, missing, or disallowed multipart
  files. Scan and persist untrusted content outside core helpers.
- Webhook helpers verify inbound signatures and optional replay windows. The
  supported outbound webhook delivery package signs tenant-scoped events,
  rejects unsafe endpoint headers, keeps endpoint signing secrets out of async
  job payloads, and records only sanitized attempt results. The Postgres
  adapter loads signing material through an application-owned resolver instead
  of storing raw endpoint secrets in webhook endpoint rows. Keep receiver
  allow-lists, SSRF-aware transports, duplicate suppression, provider schemas,
  and endpoint-secret rotation in application-owned code.
- Generated full-profile webhook endpoint creation returns the signing secret
  once and omits it from endpoint lists, delivery lists, replay responses,
  audit metadata, and Problem Details. When `DATABASE_URL` is configured, the
  generated Postgres store requires `WEBHOOK_SECRET_KEY` and encrypts endpoint
  signing secrets before persistence so workers can sign future deliveries
  without storing raw secrets in response-visible state. Treat webhook target
  URLs as operator or tenant-admin input only; production delivery workers
  should use SSRF-aware outbound transports and receiver allow-lists where your
  threat model requires them.
- Webhook delivery metrics and request-log hooks expose only bounded event
  type, outcome, and status-class labels. Do not add tenant IDs, endpoint IDs,
  delivery IDs, URLs, request bodies, secrets, or raw receiver errors to those
  observations.
- Optional generated provider workflows are app-owned starter boundaries, not
  root-module provider dependencies. `stripe-billing` must verify signed webhook
  payloads before tenant-scoped audit writes; `resend-email` must keep raw
  invitation tokens and full recipients out of logs and audit metadata; and
  `clerk-webhooks` must reject bad signatures and tenant mismatches before sync
  hooks run. Keep provider API keys, webhook secrets, and callback payloads out
  of Problem Details, OpenAPI examples, metrics labels, and generated audit
  metadata.
- Optional generated entitlements are provider-neutral app-owned boundaries.
  Keep billing customer IDs, subscription IDs, quota counters, and usage events
  out of metric labels and public errors; billing webhooks should update
  app-owned mappings before changing tenant entitlements.
- The generated `saas-web` profile is isolated from API-first scaffolds. Cookie
  sessions must use HttpOnly, Secure, and SameSite defaults; CSRF tokens must be
  compared in constant time; login callback state must be validated before
  session rotation; and raw session IDs or CSRF tokens must not appear in logs,
  metrics, audit metadata, OpenAPI examples, or Problem Details.
- Generated migration `down` commands are guarded for local/schema-teardown
  only. Do not enable `ALLOW_DANGEROUS_MIGRATION_DOWN=true` in shared or
  production environments, and review `migrate plan`/`migrate verify` output
  before rollout.
- Generated observability, Helm, and Terraform starters are templates, not a
  substitute for environment-specific network policy and secret management.
  Keep Terraform state, Helm values, Kubernetes Secret manifests, DSNs, API keys,
  provider tokens, and object-store credentials out of source control.
- Generated full-profile cache wiring keeps local development in memory and
  uses Redis only when `CACHE_STORE=redis` or production defaults select it.
  Keep cache keys application-owned and low sensitivity; do not store API-key
  secrets, invitation tokens, webhook signing secrets, provider secrets, or raw
  request payloads in shared cache values.
- Generated full-profile idempotency wiring uses the core middleware with
  tenant-aware hashed storage keys. `IDEMPOTENCY_STORE=redis` keeps replay state
  outside the process for multi-instance services; logs and metrics must remain
  limited to bounded outcome labels and must not include raw `Idempotency-Key`
  values or replay response bodies.
- Object storage helpers validate bucket/key references, enforce content-type
  and size policy, and reject secret-shaped metadata. Treat object payloads as
  untrusted content: scan before use, keep S3 endpoints as trusted operator
  configuration rather than tenant input, and avoid logging object keys when
  they can encode user-provided names or personal data.
- Generated full-profile object routes reject traversal-shaped keys, nested
  keys, hidden-file-shaped keys, unsupported content types, and oversized
  payloads. Create/list responses and validation Problem Details do not echo
  object payloads; read responses intentionally return object content to
  authorized callers only. The generated Postgres object metadata store keeps
  tenant ID, object key, content type, size, and timestamps only; it must not
  store object payload bytes or user-supplied secret metadata.
- Cookbook recipes for these helpers live in `docs/cookbook.md`; keep this
  security document focused on ownership boundaries and production caveats.

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
