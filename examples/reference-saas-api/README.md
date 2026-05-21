# Generated api-toolkit Full SaaS API

Generated profile: `saas-api-full`.
Generated auth mode: `api-key`.

This service is app-owned generated code. Keep the toolkit module replacements,
runtime policy, and generated assets under your application's review process.

## Quickstart

Prerequisites: Go 1.25.x and Make. Docker is only needed for
`make integration-check` and optional MinIO-backed object storage evidence.

1. Copy `.env.example` into your local environment mechanism and keep the
   checked-in file placeholder-only.
2. Run the deterministic local checks:

```sh
make test
make openapi-check
make contracts-lint
make contracts-diff
```

3. Start the API locally:

```sh
go run ./cmd/api
```

Expected result: the public listener serves `/livez` and the admin listener
serves authenticated health, metrics, and pprof routes when `ADMIN_ADDR` is set.

## Configuration

`.env.example` documents local defaults only. Replace every production secret
outside Git and keep real values in your runtime secret manager.

| Key | Local default | Production requirement |
| --- | --- | --- |
| `ENV` | `development` | Set to your deployed environment name. |
| `API_ADDR` | `:8080` | Bind the public API listener. |
| `ADMIN_ADDR` | `:9090` | Bind the admin listener on an internal-only network. |
| `DATABASE_URL` | empty | Required for Postgres persistence. Startup opens a pgx pool, checks required platform tables, and readiness reflects database health when this is set. |
| `REDIS_ADDR` | `localhost:6379` | Required when cache, rate limit, or idempotency stores use Redis. |
| `CACHE_STORE` | `memory` | Use `redis` for shared production cache state. |
| `RATE_LIMIT_STORE` | `memory` | Use `redis` for shared production rate limits. |
| `RATE_LIMIT_KEY_PREFIX` | `ratelimit:` | Use a service-specific prefix when Redis is shared. |
| `IDEMPOTENCY_STORE` | `memory` | Use `redis` for shared production idempotency. |
| `IDEMPOTENCY_KEY_PREFIX` | `idempotency:` | Use a service-specific prefix when Redis is shared. |
| `OPENAPI_REQUEST_VALIDATION` | `true` | Keep enabled unless a route has a documented app-owned exception. |
| `OPENAPI_RESPONSE_VALIDATION` | `true` | Usually disable in production unless the latency and error-mode tradeoff is accepted. |
| `ASYNC_WORKER_ENABLED` | `true` | Set `false` on API deployments when dedicated worker replicas run `cmd/worker`. |
| `OBJECT_STORE` | `memory` | Use `s3` when object data must persist outside process memory. |
| `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET` | local starter values | Required when `OBJECT_STORE=s3`. |
| `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | empty | Required secrets when S3-compatible credentials are used. |
| `API_KEY` | `local-dev-key` | Bootstrap setup credential only; rotate and replace with generated scoped API keys after setup. |
| `API_ACTOR_ID` | empty | Set a production actor identity for bootstrap requests. |
| `API_KEY_PEPPER` | empty | Required high-entropy secret for managed API-key hashing in production. |
| `WEBHOOK_SECRET_KEY` | empty | Required when `DATABASE_URL` is set; must be a 32-byte raw or base64-encoded key for encrypting webhook endpoint signing secrets at rest. |
| `ADMIN_KEY` | `local-admin-key` | Required for `X-Admin-Key` on admin health, metrics, and pprof routes. |

## Quality Checks

| Command | Purpose |
| --- | --- |
| `make test` | Tidy modules and run unit tests. |
| `make openapi-check` | Verify the OpenAPI golden file. |
| `make contracts-lint` | Lint the OpenAPI contract. |
| `make contracts-diff` | Compare the current OpenAPI contract with `OPENAPI_BASE`. |
| `make client-check` | Regenerate and compare the typed Go client. |
| `make asset-check` | Validate generated observability and deployment assets. |
| `make observability-check` | Validate dashboard and alert-rule assets. |
| `make deploy-check` | Validate Helm, Kubernetes, and Terraform starter assets. |
| `make provider-check` | Run fake-provider tests and replay checked-in provider fixtures when provider workflows are generated. |
| `make integration-check` | Opt-in Docker smoke check for Postgres, Redis, API, worker, migrations, tenant routes, idempotency, outbox/webhooks, object readback, audit writes, admin health, admin metrics, admin pprof, and public admin-route isolation. |
| `make finalize` | Deterministic local gate: format, test, build, OpenAPI, contracts, asset checks, and clean. It does not run Docker or live provider checks. |

## Runtime Surface

| Area | Generated behavior |
| --- | --- |
| Persistence | Postgres stores tenants, API keys, widgets, operations, outbox, audit, webhook delivery state, and object metadata. |
| Redis | Redis is used for shared idempotency, rate limiting, and cache state in production. Local development uses `CACHE_STORE=memory`, `RATE_LIMIT_STORE=memory`, and `IDEMPOTENCY_STORE=memory` unless you opt into Redis. |
| Service bootstrap | The generated binary uses `bootstrap.NewAPIService` for public/admin listeners, graceful shutdown, background workers, strict middleware order validation, and safe system endpoint mounting. |
| Worker | `cmd/worker` runs background jobs without serving public HTTP traffic. |
| Migrations | `cmd/migrate` applies and checks contrib migrator-compatible SQL files under `migrations/`. Docker Compose and Kubernetes assets run it before API/worker startup. |
| Health | `/livez` is a process liveness probe and never checks Postgres, Redis, or S3; `/readyz` reflects configured dependencies. |
| OpenAPI | Runtime OpenAPI request validation is enabled by default. Response validation is enabled in development/test or when `OPENAPI_RESPONSE_VALIDATION=true`. |
| Metrics and pprof | The public router emits bounded Prometheus HTTP request metrics, and `/metrics` is served only from the admin router. The admin router mounts real Go pprof handlers behind `X-Admin-Key`; the public router does not mount pprof when `ADMIN_ADDR` is set. |
| Audit | Write routes record audit events with redaction-safe metadata; raw API-key secrets, invitation tokens, webhook signing secrets, and idempotency keys are not audit metadata. |
| API surface | The generated HTTP layer starts with organization creation/listing, member listing, invitation creation/acceptance, tenant isolation, tenant-scoped idempotent widget writes, async widget imports with pollable operation state, outbound webhook endpoint/delivery/replay routes, and strict tenant-scoped object storage routes. API-key, JWT, Clerk, and OIDC modes are wired with fail-closed startup validation. |
| Sample domain | Widgets are sample app-owned domain code. Replace or complement them with product resources generated by `api-toolkit generate resource`. |

## Production Caveats

`make integration-check` is opt-in and starts Postgres and Redis through Docker
Compose, applies the generated migration, hydrates module sums with
`go mod tidy`, runs `go test ./...`, starts the worker and API on localhost,
and performs HTTP smoke checks for liveness, readiness, OpenAPI, auth failure,
tenant routes, managed API-key auth, idempotent widget writes, ETag conflict
handling, async operation polling, outbox completion/retry behavior, webhook
delivery/replay, object write/readback, audit writes, admin health, admin
metrics, admin pprof, and public admin-route isolation. Set
`INTEGRATION_OBJECT_STORE=s3` to include MinIO-backed S3 object storage in the
same script. The default finalize target stays local and deterministic.

Admin routes are intended for a separate listener when `ADMIN_ADDR` is set.
Keep `/health/detailed`, `/metrics`, and `/debug/pprof/` behind admin
authentication and network isolation.

Unsafe write routes require `Idempotency-Key`. Organization-scoped routes
require `X-Tenant-ID` to match the organization path parameter.

API-key mode keeps `API_KEY` as a bootstrap setup credential and verifies
generated scoped API keys through the API-key service after setup. Bootstrap
requests use `API_ACTOR_ID` for production actor identity; in non-production
only, tests and local tools may send `X-Actor-ID` before a generated API key
exists.

Do not copy live API keys, admin keys, webhook secrets, S3 credentials, provider
secrets, invitation tokens, idempotency keys, or raw callback bodies into logs,
tickets, traces, metrics, or release evidence.
