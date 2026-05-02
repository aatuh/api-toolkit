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

## OWASP Mapping (Resource Consumption)

The toolkit provides concrete controls for resource limits:

- Timeouts: `securityprofile.WithTimeout` and `RouteOverride.Timeout` apply cooperative request context deadlines
- Payload size: `securityprofile.WithMaxBodyBytes` and `RouteOverride.MaxBodyBytes`
- Query limits: `securityprofile.WithQueryLimits` and `querylimits.Options`
- Rate limits: `securityprofile.WithRateLimitOptions` and `RouteOverride.RateLimit`
- Header limits: `httpx.HeaderLimitsBalanced` + server `MaxHeaderBytes`

`securityprofile.WithTimeout` does not force a timeout response or stop handlers that ignore `ctx.Done()`.
Use server read/write deadlines or selective wrappers such as `http.TimeoutHandler` when you need a hard wall-clock response limit.

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
- [ ] Use TLS in production and prefer TLS 1.3
- [ ] Monitor for rate limiting and validation errors
