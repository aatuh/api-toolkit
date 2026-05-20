# Generated api-toolkit Full SaaS API

Generated profile: `saas-api-full`.
Generated auth mode: `api-key`.

Run locally:

```sh
make test
go run ./cmd/api
```

Postgres stores tenants, API keys, widgets, operations, outbox, audit, webhook delivery state, and object metadata.
When `DATABASE_URL` is set, `WEBHOOK_SECRET_KEY` must be a 32-byte raw or base64-encoded key used to encrypt webhook endpoint signing secrets at rest.
Redis is used for shared idempotency, rate limiting, and cache state in production. Local development uses `CACHE_STORE=memory`, `RATE_LIMIT_STORE=memory`, and `IDEMPOTENCY_STORE=memory` unless you opt into Redis.
When `DATABASE_URL` is set, startup opens a pgx pool, checks required platform tables, and readiness reflects database health.
The generated binary uses `bootstrap.NewAPIService` for public/admin listeners, graceful shutdown, background workers, strict middleware order validation, and safe system endpoint mounting.
`cmd/worker` runs background jobs without serving public HTTP traffic. Set `ASYNC_WORKER_ENABLED=false` on API deployments when you run dedicated worker replicas.
`cmd/migrate` applies and checks contrib migrator-compatible SQL files under `migrations/`. Docker Compose and Kubernetes assets run it before API/worker startup.
`/livez` is a process liveness probe and never checks Postgres, Redis, or S3; `/readyz` reflects configured dependencies.
Runtime OpenAPI request validation is enabled by default. Response validation is enabled in development/test or when `OPENAPI_RESPONSE_VALIDATION=true`.
The public router emits bounded Prometheus HTTP request metrics, and `/metrics` is served only from the admin router.
The admin router mounts real Go pprof handlers behind `X-Admin-Key`; the public router does not mount pprof when `ADMIN_ADDR` is set.
Write routes record audit events with redaction-safe metadata; raw API-key secrets, invitation tokens, webhook signing secrets, and idempotency keys are not audit metadata.
The generated HTTP layer starts with organization creation/listing, member listing, invitation creation/acceptance, tenant isolation, tenant-scoped idempotent widget writes, async widget imports with pollable operation state, outbound webhook endpoint/delivery/replay routes, and strict tenant-scoped object storage routes. API-key, JWT, Clerk, and OIDC modes are wired with fail-closed startup validation.
Unsafe write routes require `Idempotency-Key`. Organization-scoped routes require `X-Tenant-ID` to match the organization path parameter.
API-key mode keeps `API_KEY` as a bootstrap setup credential and verifies generated scoped API keys through the API-key service after setup. Bootstrap requests use `API_ACTOR_ID` for production actor identity; in non-production only, tests and local tools may send `X-Actor-ID` before a generated API key exists.


Useful checks:

```sh
make openapi-check
make contracts-lint
make contracts-diff
make integration-check
```

`make integration-check` is opt-in and starts Postgres and Redis through Docker Compose, applies the generated migration, hydrates module sums with `go mod tidy`, runs `go test ./...`, starts the worker and API on localhost, and performs HTTP smoke checks for liveness, readiness, OpenAPI, auth failure, tenant routes, managed API-key auth, idempotent widget writes, ETag conflict handling, async operation polling, outbox completion/retry behavior, webhook delivery/replay, object write/readback, audit writes, admin health, admin metrics, admin pprof, and public admin-route isolation. Set `INTEGRATION_OBJECT_STORE=s3` to include MinIO-backed S3 object storage in the same script. The default finalize target stays local and deterministic.

Admin routes are intended for a separate listener when `ADMIN_ADDR` is set. Keep `/health/detailed`, `/metrics`, and `/debug/pprof/` behind admin authentication and network isolation.
