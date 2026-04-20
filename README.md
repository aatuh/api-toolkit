# api-toolkit

## Overview

Reusable building blocks for Go HTTP APIs that enforce Dependency
Inversion. Your application depends on stable `ports` interfaces, while
this toolkit ships adapters for popular libraries. Result: fewer direct
third‑party imports in your app, easier testing, and cleaner wiring.

Agent‑friendly by design: clear interfaces, small packages, sensible
defaults, and predictable wiring.

## Design goals

- Interfaces first: depend on `github.com/aatuh/api-toolkit/ports`.
- Adapters for third-party libs live in `github.com/aatuh/api-toolkit-contrib`.
- Consistent errors via RFC‑9457 (Problem Details).
- Small, composable middlewares with simple constructor options.

## Modules

- Core: `github.com/aatuh/api-toolkit` (stable ports, middleware, `httpx`, and endpoints with minimal framework coupling)
- Contrib: `github.com/aatuh/api-toolkit-contrib` (adapters, integrations, tooling)

## Documentation

- Getting started: `docs/getting-started.md`
- Hexagonal mapping: `docs/architecture.md`
- Security posture: `docs/security.md`
- Security policy: `SECURITY.md`
- Versioning and stability: `VERSIONING.md`
- Release notes: `docs/release-notes.md`
- Panic policy: `PANIC_POLICY.md`
- Metrics naming + labels policy: `docs/metrics.md`
- Cookbook: `docs/cookbook.md`

## Features (core)

- Ports (interfaces)
  - Logger, Clock, IDGen
  - HTTPRouter, HTTPMiddleware, HTTPClient, RateLimiter
  - CORS handler, Security headers
  - Database: Pool, Tx, Rows, Row, Result, Stats
  - Health: Manager, Checkers, Results, Summaries
  - Docs: Manager, Info, Version, OpenAPI
  - Validator
  - MetricsRecorder (pluggable, with No-op default)

- HTTP middleware
  - `middleware/json`, `middleware/timeout`, `middleware/maxbody`
  - `middleware/querylimits`, `middleware/ratelimit`, `middleware/idempotency`
  - `middleware/secure`, `middleware/trace`, `middleware/auth/authz`, `middleware/auth/jwt`

## JWT auth (core + contrib)

Use `middleware/auth/jwt` for generic JWT validation (JWKS, issuer, audience, alg allowlist)
and `contrib/integrations/auth/jwt` for env loading.

Common env vars:
- `JWT_AUTH_ENABLED`, `JWT_JWKS_URL`, `JWT_ISSUER`, `JWT_AUDIENCE`
- `JWT_ALLOWED_ALGORITHMS`, `JWT_ALLOWED_CLOCK_SKEW`
- `JWT_JWKS_REFRESH_INTERVAL`, `JWT_JWKS_REFRESH_TIMEOUT`
- `JWT_SKIP_HEADER_ENABLED`, `JWT_SKIP_HEADER_NAME`, `JWT_SKIP_TRUSTED_PROXIES`
- `JWT_ALLOW_DANGEROUS_DEV_BYPASSES`
- Optional claim requirements: `JWT_REQUIRE_SUBJECT`, `JWT_REQUIRE_EXPIRATION`,
  `JWT_REQUIRE_ISSUED_AT`, `JWT_REQUIRE_NOT_BEFORE`

- HTTP helpers
  - `httpx`, `httpx/identity`, `httpx/recover`
  - `response_writer` (legacy)
  - `endpoints/list`, `endpoints/docs`, `endpoints/health`, `endpoints/pprof`, `endpoints/version`

- Security profiles
  - `securityprofile`: compose security posture defaults (limits, headers, auth)

- Other core utilities
  - `authorization`, `scheduler`, `specs`, `swagstub`, `email`, `fielderrors`

## Features (contrib)

- Environment & Config
  - `adapters/envvar`, `config`

- Logging
  - `adapters/logzap`

- HTTP Router & Middleware
  - `adapters/chi`: router and helpers as `ports.HTTPRouter` / `ports.HTTPMiddleware`
  - `middleware/cors`, `middleware/requestlog`, `middleware/metrics`
  - `middleware/openapi`, `middleware/oteltrace`
  - `middleware/auth/clerk`, `middleware/auth/devheaders`

- Outbound HTTP
  - `adapters/httpclient`, `telemetry`

- Distributed stores
  - `adapters/ratelimitredis`: Redis rate limiter implementing `ports.RateLimiter`
  - `adapters/idempotencyredis`: Redis idempotency store implementing `ports.IdempotencyStore`
  - `adapters/idempotency`: in-memory idempotency store for dev/test

- Database
  - `adapters/pgxpool`, `adapters/txpostgres`
  - `migrator`, `adapters/migrate`

- IDs & Time
  - `adapters/ulid`, `adapters/uuid`, `adapters/clock`

- Validation
  - `adapters/validation`

- Email extras
  - `email/markdown`, `email/noop`

- Misc
  - `countrycodes`

- Integrations
  - `integrations/*` convenience wrappers for Stripe/Resend/Clerk/Postgres

## Package map

Core (`github.com/aatuh/api-toolkit`):
- `ports` — core interfaces for all boundaries
- `middleware/*` — json, timeout, maxbody, querylimits, ratelimit, idempotency, secure, trace, auth/authz, auth/jwt
- `httpx`, `httpx/identity`, `httpx/recover` — JSON + error helpers and panic recovery
- `response_writer` — legacy success JSON writer
- `endpoints/*` — docs, health, pprof, version, list helpers
- `authorization`, `securityprofile`, `specs`, `swagstub`, `scheduler`, `email`, `fielderrors`

Contrib (`github.com/aatuh/api-toolkit-contrib`):
- `adapters/*` — envvar, chi, logzap, pgxpool, txpostgres, redis, stripe, resend, httpclient, ids, clock, validation
- `middleware/*` — cors, requestlog, metrics, openapi, oteltrace, auth/clerk, auth/devheaders
- `bootstrap`, `config`, `telemetry`, `migrator`
- `email/markdown`, `email/noop`, `countrycodes`
- `integrations/*` — optional wrappers for Stripe/Resend/Clerk/Postgres adapters

## Adapters vs integrations

- `api-toolkit-contrib/adapters/*` are concrete implementations of `ports` and are intended to be stable.
- `api-toolkit-contrib/integrations/*` are convenience wrappers for quick wiring; they may change faster than core adapters.
Recommended: import adapters directly and use integrations only when you want convenience defaults.

## Stability

- Stable (core): `ports`, `middleware/*`, `httpx`, `endpoints/*`, `authorization`, `scheduler`, `email`
- Stable (contrib): `adapters/*`, `bootstrap`
- Experimental/legacy: `integrations/*` (contrib), `securityprofile`, `specs`, `swagstub`, `response_writer`

## Quickstart (wiring in main)

```go
// Logger and config (contrib)
log := logzap.NewProduction()            // ports.Logger
cfg := config.MustLoadFromEnv()          // panics on missing or invalid env

// Router and middleware profile (contrib + core)
r := chi.New()                           // ports.HTTPRouter
profile, err := bootstrap.ProfileStrictAPI(log)
if err != nil { /* handle */ }
profile.Apply(r)

// Health and docs (core)
health.NewBasicHandler().RegisterRoutes(r)
docs.NewHandler(docs.New()).RegisterRoutes(r)
```

## Health endpoint contract

The health package exposes separate liveness, readiness, and detailed health
behaviors:

- Liveness and readiness are expected to reflect configured checker state and
  should not silently report healthy when no probe checks are configured.
- Detailed health output is an operator-focused surface because it can include
  dependency-level status and check details.
- `ports.HealthCheckConfig.EnableDetailed` is the switch that controls whether
  HTTP packages should expose detailed health responses at all.
- Missing checker registrations or invalid probe wiring should fail closed and
  surface as unhealthy state rather than as synthetic success.
- When `EnableCaching` is true, checker results may be reused across health
  endpoints until `CacheDuration` expires.

## Problem Details (RFC 9457) and success responses

```go
// Error
httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Detail: "invalid"})

// Success
httpx.WriteJSON(w, http.StatusOK, payload)
```

Use `httpx.DefaultTypeURI` or `httpx.TypeRegistry` to keep `type` URIs consistent.

Error and trace responses can be correlated via response headers:

- `X-Request-ID` when the request already carries a request or correlation ID
- `X-Trace-ID` and `traceparent` from `middleware/trace`

## Panic recovery

- `httpx/recover` logs recovered handler panics.
- If the response is still uncommitted, it returns a `500` Problem Details response.
- If headers or body bytes have already been committed, it aborts the request instead of preserving a misleading partial success response.

### Validation error schema

Validation errors use a canonical `validation` extension with field errors.
Legacy `errors` and `field_errors` extensions are also emitted for compatibility.

```json
{
  "type": "https://api-toolkit.dev/problems/validation-error",
  "title": "Bad Request",
  "status": 400,
  "detail": "validation failed",
  "validation": {
    "fields": [
      { "field": "email", "code": "format", "message": "email is invalid" }
    ]
  }
}
```

### Validation

Use the contrib validation adapter.

```go
v := validation.New()
if err := v.ValidateStruct(ctx, &dto); err != nil {
  httpx.WriteError(w, err)
  return
}
```

## Migrations with embed

Migrations live in the contrib module.

```go
//go:embed migrations/*.sql
var fsys embed.FS

m, err := migrator.New(migrator.Options{EmbeddedFSs: []fs.FS{fsys}, Logger: log})
if err != nil { /* handle */ }
defer m.Close()
if err := m.Up("."); err != nil { /* handle */ }
```

When loading from multiple migration sources, duplicate version+direction pairs are rejected instead of being overridden silently.

If migration SQL executes but transaction commit acknowledgement is ambiguous,
the runner records the migration as `uncertain` and stops future runs until an
operator reconciles the database state and the migration record.

## Database adapters

Database adapters live in the contrib module.

```go
pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
if err != nil { /* handle */ }
tx := txpostgres.New(pool)
```

`txpostgres.WithinTx` uses the caller context for acquire, begin, and commit,
but deferred rollback cleanup switches to a short-lived context without caller
cancellation so timed-out or canceled requests still attempt to release the
transaction cleanly.
If a pool is missing, `txpostgres.New(nil)` and `txpostgres.FromCtx(..., nil)`
now fail closed with `txpostgres.ErrPoolNotConfigured` instead of panicking.

## Metrics integration

The metrics middleware lives in the contrib module.
Provide an implementation of `ports.MetricsRecorder` to record counts
and durations. Passing a nil recorder to `metricsmw.New(metricsmw.Options{})`
uses a No-op implementation.
See `docs/metrics.md` for naming and label policy.

## Request logging

The request logging middleware lives in the contrib module and emits
consistent structured fields:

- `request_id`
- `trace_id`
- `span_id`
- `route`
- `status`
- `latency_ms`
- Additional context: `method`, `path`, `bytes`, `client_ip`, `user_agent`

Header logging is opt-in and always redacts `Authorization`, `Cookie`,
`Set-Cookie`, and API key headers by default.

```go
logger := logzap.NewProduction()
mw, err := requestlog.New(logger,
	requestlog.WithRequestHeaders(),
	requestlog.WithResponseHeaders(),
	requestlog.WithRedactedHeaders("X-Session-Token"),
)
if err != nil { /* handle */ }
```

## Idempotency

- Use `Idempotency-Key` to store and replay the first non-5xx response by default.
- The default request hash includes authenticated actor and tenant scope when present, so place auth and tenant middleware before idempotency in authenticated APIs.
- If a completed response cannot be persisted, the middleware returns `503 Service Unavailable` instead of the downstream success response and records an ambiguous outcome for that key.
- Buffered replay bodies are capped at `1 MiB` by default via `MaxResponseBytes`.
- Exclude streaming, upgrade, and large download routes with `ShouldHandle`; the idempotency middleware is for normal buffered responses, not streaming transport.
- Failed attempts that end in downstream `5xx` responses or panics release their reservation so the same request can retry immediately with the same key.
- Completed-response persistence failures and replay-buffer overflows do not reopen the key. They return `503`, leave an ambiguous record behind, and block same-key retries until the record expires or is reconciled.
- Replays include `Idempotency-Replayed: true`.
- `Set-Cookie` is stripped from replayed responses unless explicitly allowed.
- Use `api-toolkit-contrib/adapters/idempotencyredis` for production storage.

## Scheduler

- `scheduler.Runner` executes each configured job immediately on start and then on its interval.
- Panics inside scheduled jobs are recovered, logged, and recorded as failed runs; the runner keeps future intervals alive instead of crashing the process.
- The runner keeps at most one active schedule per job name, so duplicate `Start` calls or duplicate named jobs do not multiply execution cadence.
- Non-overlap is enforced per job name: while one run is in flight, later ticks for that same named job are skipped.
- Recorder persistence failures are operational signals, not scheduler control-flow failures: surface them through logging or `SetRecorderFailureHandler`, but do not treat them as a reason to rerun the job immediately or stop future intervals after the job function has already finished.

## Security headers

- Header profiles: `securemw.APIOnly()`, `securemw.DocsUI()`, `securemw.WebApp()`.
  - `WebApp()` ships a strict baseline CSP; use templates to add nonces or extra sources.
- COOP/COEP/CORP are opt-in: `securemw.WithCrossOriginIsolation()` or `securemw.WithCOOP`/`WithCOEP`/`WithCORP`.
- CSP templates: `securemw.CSPTemplateWebApp` + `securemw.RenderCSPTemplate`.

```go
secure, err := securemw.New(
	securemw.WebApp(),
	securemw.WithCSPTemplateFunc(securemw.CSPTemplateWebApp, func(r *http.Request) securemw.CSPTemplateValues {
		return securemw.CSPTemplateValues{Nonce: "per-request-nonce"}
	}),
)
if err != nil { /* handle */ }
```

`DocsUI()` allows third-party CDN assets and inline styles for Swagger UI; `WebApp()` is stricter and may block inline scripts until you add nonces/hashes.
For a first-party docs surface without CDN assets or `unsafe-inline`, use `docs.NewStrict()` or `docs.NewWithConfig(ports.DocsConfig{HTMLMode: ports.DocsHTMLModeStatic})`.
Disabled docs surfaces and missing OpenAPI files return `404` instead of placeholder content.
`DocsConfig.EnableJSON` and `DocsConfig.EnableYAML` control which discovered OpenAPI formats may be served on the configured OpenAPI path.
COOP/COEP/CORP can break cross-origin embeds; enable only when you control embedded resources.

## Authorization (default deny + policy engines)

Enforce explicit allow rules per action (route) with a default-deny authorizer.

```go
allowlist := authorization.NewAllowlistAuthorizer()
_ = allowlist.AllowAny("GET /health")
_ = allowlist.AllowFunc("GET /admin", func(ctx context.Context, subject any, action string, resource any) error {
	return httpx.ErrForbidden
})
```

Policy engines (OPA/Cedar) plug in through `ports.PolicyEngine`.

```go
engine, err := opa.New(opa.Config{
	DecisionURL: "http://opa:8181/v1/data/authz/allow",
})
if err != nil { /* handle */ }

authorizer := authorization.NewPolicyAuthorizer(engine, authorization.PolicyAuthorizerOptions{
	ContextProvider: func(ctx context.Context) any {
		return map[string]any{"tenant": "acme"}
	},
})
```

## Multi-tenant scoping

The tenant middleware extracts tenant identifiers from auth claims, headers, or URL params.
It rejects mismatches and stores scope in context.

```go
tenantmw, err := tenant.New(tenant.Options{
	HeaderName: "X-Tenant-ID",
	TenantFromContext: func(ctx context.Context) (string, bool) {
		return "acme", true
	},
})
if err != nil { /* handle */ }
r.Use(tenantmw.Handler)
```

## Authentication hardening (JWT)

Clerk JWT middleware follows RFC 8725 guidance:

- Allowed signing algorithms are enforced (defaults to `RS256`).
- Required claims default to `sub` and `exp`; `iat`/`nbf` can be required.
- Bearer parsing is strict (single header, no whitespace ambiguity).
- Dev header auth fallback is development-only and now requires explicit
  dangerous-bypass opt-in plus trusted-proxy configuration before it will
  honor debug headers.

```go
requireIssuedAt := true
cfg := clerk.Config{
	AllowedClockSkew: 30 * time.Second,
	RequiredClaims: clerk.ClaimRequirements{
		RequireIssuedAt: &requireIssuedAt,
	},
}
```

## mTLS for privileged endpoints

For privileged/internal routes, require mutual TLS at the edge or service.

```go
clientCAs := x509.NewCertPool()
// load CA certs into clientCAs
srv := &http.Server{
	Addr: ":8443",
	TLSConfig: &tls.Config{
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCAs,
	},
}
```

Follow OWASP REST guidance for HTTPS/mTLS: https://cheatsheetseries.owasp.org/cheatsheets/REST_Security_Cheat_Sheet.html

## OWASP resource consumption limits

OWASP highlights "Unrestricted Resource Consumption" as a top API risk.
The toolkit maps mitigations to concrete knobs:

- Timeouts: `securityprofile.WithTimeout` and `RouteOverride.Timeout`.
- These timeouts are cooperative request context deadlines. For hard wall-clock response limits, also configure server read/write timeouts or use `http.TimeoutHandler` selectively.
- Payload caps: `securityprofile.WithMaxBodyBytes` and `RouteOverride.MaxBodyBytes`.
- Header limits: configure `http.Server.MaxHeaderBytes` with presets like `httpx.HeaderLimitsBalanced`.
  Use `httpx.HeaderLimits.Check` to enforce header counts in middleware if needed.
- Pagination and query limits: `securityprofile.WithQueryLimits` and `querylimits.Options` (`MaxParams`, `MaxLimit`).
- Rate limits: `securityprofile.WithRateLimitOptions` and `RouteOverride.RateLimit`.
- Retry budgets: cap outbound retries with `api-toolkit-contrib/adapters/httpclient` `RetryOptions`.

Use the baseline preset to keep headers + limits consistent, then override
specific routes as needed. Overrides merge with the baseline; only the
specified fields change. Patterns match exact paths or prefix matches with
a trailing `*`.

```go
maxBody := int64(8 << 20)
slow := 30 * time.Second
profile, err := securityprofile.OWASPBaseline(
	securityprofile.WithAuthCheck(authCheck),
	securityprofile.WithRouteOverrides(securityprofile.RouteOverride{
		Pattern:      "/uploads",
		Methods:      []string{http.MethodPost},
		MaxBodyBytes: &maxBody,
		Timeout:      &slow,
	}),
)
if err != nil { /* handle */ }
profile.Apply(r)
```

## OpenAPI request validation

OpenAPI request validation middleware lives in the contrib module.

```go
spec, err := openapi3.NewLoader().LoadFromFile("openapi.json")
if err != nil { /* handle */ }
validator, err := openapi.New(spec)
if err != nil { /* handle */ }
r.Use(validator.Middleware())
```

Response validation (dev/test) is opt-in.

```go
validator, err := openapi.New(spec,
	openapi.WithResponseValidation(openapi.ResponseValidationOptions{
		Enabled:      true,
		MaxBodyBytes: 1 << 20,
	}),
)
if err != nil { /* handle */ }
r.Use(validator.Middleware())
```

Response validation buffers the response (default 1 MiB when enabled), so
enable it in dev/test or for contract confidence checks.

OpenAPI validation errors map to RFC 9457 Problem Details:

| Case                      | Status      | type               |
|---------------------------|-------------|--------------------|
| Request validation error  | 400/415/422 | `validation-error` |
| Missing/invalid auth      | 401         | `unauthorized`     |
| Route not found           | 404         | `not-found`        |
| Response validation error | 500         | `internal-error`   |

Validation errors include the `validation` extension with field-level details.

## Outbound HTTP client

Outbound HTTP client lives in the contrib module. It includes retry budgets
(idempotent methods by default), optional SSRF guardrails, and resilience
controls like breakers and bulkheads.

```go
guard, err := httpclient.NewSSRFTransport(httpclient.SSRFOptions{
	AllowedHosts: []string{"api.example.com"},
	AllowedPorts: []int{443},
})
if err != nil { /* handle */ }
bulkhead, err := httpclient.NewSemaphoreBulkhead(50)
if err != nil { /* handle */ }
breaker := httpclient.NewCircuitBreaker(httpclient.CircuitBreakerOptions{})

client := httpclient.New(httpclient.Options{
	Transport: guard,
	Retry: httpclient.RetryOptions{
		MaxRetries:     2,
		MaxElapsedTime: 2 * time.Second,
		UseRetryAfter:  true,
	},
	Breaker:  breaker,
	Bulkhead: bulkhead,
})
resp, err := client.Do(req)
```

For `net/http` clients, set `CheckRedirect: guard.CheckRedirect` to ensure
redirects are re-validated and scheme changes are blocked by default. Set
`AllowRedirectSchemeChange: true` to allow http<->https redirects.

## Examples

See `contrib/examples/` for end-to-end wiring samples.

## Conventions for applications

- Import toolkit interfaces/adapters, not third‑party libs, in app code.
- Handlers: decode → validate → call service → encode.
- Prefer `httpx` for both error and success responses.

## Version and requirements

- Supported Go versions: 1.24.x
- Minimum in `go.mod`: Go 1.24.x.
- Core keeps transport and application boundaries stable and lightweight; contrib pins router, logging, database, and validation integrations.

## Go toolchain behavior

- `go` in `go.mod` sets language/module compatibility and a minimum version.
- We intentionally do not set a `toolchain` directive to avoid auto-downloads.
- If you enable toolchain auto-downloads, set `GOTOOLCHAIN=local` for locked-down or offline environments.

## Local quality tooling

- CI and reproducible local verification both target Go `1.24.x`.
- `make tools` installs pinned QA tool versions:
  - `golangci-lint v2.11.4`
  - `gosec v2.25.0`
  - `govulncheck v1.2.0`
  - `apidiff v0.0.0-20260410095643-746e56fc9e2f`
- To reproduce the repository quality gate locally, run `GOTOOLCHAIN=local make finalize`.
- If you override the `*_VERSION` Make variables, your local results may differ from CI and audited baseline runs.

### Adapter coverage policy

- Any adapter that persists or coordinates external state needs direct package tests, not only indirect coverage through examples or middleware.
- This includes Redis- and database-backed adapters, scheduler run stores, transaction/pool wrappers, and provider adapters that translate third-party responses or errors.
- Time- and state-sensitive adapter behavior must cover success, failure, and boundary cases such as TTL expiry, retry delay calculation, duplicate protection, serialization, and no-row/not-found paths.
- Wrapper-only integration packages may rely on underlying adapter tests when they add no new logic; once they add defaults, translation, or lifecycle behavior, they need their own direct tests too.
- Changes to these adapter categories should not be merged until direct tests exist in the touched package and `GOTOOLCHAIN=local make finalize` is green.

## Verify releases

Release SBOM signatures are published via Sigstore/cosign; verification steps
are documented in `SECURITY.md`. Go module consumers also benefit from the
default Go checksum database verification.

## Recommended Usage

- Wire dependencies via `ports` (core) and contrib adapters; avoid direct
  imports in the application layer.
- Prefer non‑interactive Make targets; verify with `make health`.
- Do not edit generated files; run `make codegen` when annotations
  change.
