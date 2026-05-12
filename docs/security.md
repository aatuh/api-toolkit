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
- Tenant middleware compares configured tenant sources before handlers run. Use
  `RequireAllSources` for routes that must prove a request header or URL tenant
  matches the authenticated tenant scope.
- OAuth2 helpers standardize claims and scopes only after an app-owned validator
  verifies issuer, audience, expiry, JWKS material, and tenant mapping.
- Upload helpers reject malformed, oversized, missing, or disallowed multipart
  files. Scan and persist untrusted content outside core helpers.
- Webhook helpers verify signatures and optional replay windows. Keep replay
  stores, duplicate suppression, delivery retries, and provider schemas in
  application code.
- Cookbook recipes for these helpers live in `docs/cookbook.md`; keep this
  security document focused on ownership boundaries and production caveats.

## OWASP Mapping (Resource Consumption)

The toolkit provides concrete controls for resource limits:

- Timeouts: `securityprofile.WithTimeout` and `RouteOverride.Timeout` apply cooperative request context deadlines; `securityprofile.WithHardTimeout` and `middleware/timeout.NewHard` add a hard wall-clock response cutoff that writes a 504 Problem Details timeout response and discards late handler writes
- Hard-timeout capture limits: `securityprofile.WithHardTimeoutMaxCaptureBytes`, `RouteOverride.HardTimeoutMaxCaptureBytes`, and `middleware/timeout.Options.MaxCaptureBytes` bound buffered non-streaming responses
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
these knobs do not make streaming routes safe. Handler panics inside hard
timeout are contained in the child goroutine. Before the timeout response wins,
the middleware returns a deterministic 500 Problem Details response; after the
timeout response wins, late panics are dropped with late writes.

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
- Adapter legacy idempotency recovery events hash keys by default. Enable the
  explicit raw-key option only for short, access-controlled incident review.
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
