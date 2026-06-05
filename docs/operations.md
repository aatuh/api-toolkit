# Health And Admin Operations Guide

Audience: operators and developers wiring public health, detailed health,
metrics, pprof, and admin endpoints.

The operating model is public probes for load balancers and operator-only
detail for humans and internal automation.

## Endpoint Split

| Endpoint class | Public route | Admin/internal route | Production rule |
| --- | --- | --- | --- |
| Liveness | `/livez` or equivalent | Optional duplicate | Process-only. Do not attach Postgres, Redis, S3, provider, or outbound checks. |
| Readiness | `/readyz` or equivalent | Optional duplicate | Dependency-aware. Fail closed when required dependencies are unavailable or migrations are pending. |
| Detailed health | No | `/health/detailed` or equivalent | Operator-only. May expose dependency names, states, messages, and durations. |
| Metrics | No | `/metrics` | Operator-only or internal scrape path. Labels must be bounded. |
| Pprof | No | `/debug/pprof/` | Admin/internal only through `pprof.RegisterAdminRoutes` or equivalent wrapper. |
| Docs/OpenAPI | Public only when intended | Optional admin copy | Avoid exposing internal-only operations. |

## Safe Mounting

Prefer helpers that require an explicit admin wrapper:

- `Handler.RegisterPublicRoutesTo` for public liveness/readiness,
- `Handler.RegisterAdminDetailedHealthRoute` for detailed health,
- `pprof.RegisterAdminRoutes` for pprof,
- `bootstrap.MountSystemEndpointsToWithAdmin` when mounting health, metrics, and
  pprof together.

Generated full-profile services can use a separate admin listener through
`bootstrap.APIServiceConfig.AdminAddr`. Keep that listener on private networking
or behind explicit admin authentication.

## Network Policy

- Public ingress should reach public API routes and public probes only.
- Admin routes should be reachable only from operator networks, private
  services, or authenticated internal gateways.
- Metrics scrapers should not require broad public ingress.
- Pprof should never be reachable from internet-facing paths.
- Detailed health should not be called directly by browser, mobile, or desktop
  clients.

## Fail-Closed Checks

Readiness should fail when:

- required checkers are not registered,
- configured dependency checks fail,
- Postgres migrations are pending or uncertain,
- Redis-backed runtime state is required but unavailable,
- object storage or provider dependencies are required for startup and fail
  their configured checks.

Liveness should stay process-only so orchestrators do not restart healthy
processes just because an external dependency is down.

## Metrics and Logs

Use route patterns, method, status, status class, and bounded outcome labels.
Never put tenant IDs, user IDs, API keys, admin keys, idempotency keys, raw
paths, query strings, request bodies, provider payloads, dependency DSNs, or
object keys into metrics labels.

## Verification

- public route cannot reach detailed health, metrics, or pprof,
- admin route without auth fails closed,
- public `/livez` stays independent from dependency failures,
- public `/readyz` reflects required dependency failures,
- detailed health includes dependency status only on admin/internal routes,
- metrics labels use route patterns, not raw paths,
- pprof requires the admin wrapper.
