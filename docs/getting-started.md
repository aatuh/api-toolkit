# Getting Started (10 minutes)

Audience: new users who want a runnable production-oriented API skeleton with
the standard service wiring already in place.

## 1) Generate a service

```sh
go run github.com/aatuh/api-toolkit/contrib/v2/cmd/api-toolkit@latest new service \
  --module example.com/my-api \
  --profile saas-api \
  --dir my-api
cd my-api
cp .env.example .env
go mod tidy
```

Expected result: `my-api` contains a Go module, chi-based API service,
OpenAPI golden, Makefile, Dockerfile, Compose file, pinned GitHub Actions CI,
tests, and a local `.env` file.

## 2) Verify the scaffold

```sh
make finalize
make openapi-check
make contracts-lint
make contracts-diff
```

Expected result: tests pass, the binary builds, the generated OpenAPI golden is
current, and route contract linting accepts the default production policies.

## 3) Run it

```sh
go run .
```

Try the public health and OpenAPI routes from another shell:

```sh
curl -s http://localhost:8080/readyz
curl -s http://localhost:8080/docs/openapi.json
```

Try the default protected write route:

```sh
curl -s -X POST http://localhost:8080/widgets \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: local-dev-key' \
  -H 'X-Tenant-ID: tenant_1' \
  -H 'Idempotency-Key: demo-1' \
  -d '{"name":"starter"}'
```

Expected result: `GET /readyz` returns ready health, `GET /docs/openapi.json`
returns the generated OpenAPI document, and `POST /widgets` returns a created
widget. Repeating the same write with the same `Idempotency-Key` replays the
stored response.

## 4) Inspect operator routes

```sh
curl -s -H 'X-Admin-Key: local-admin-key' http://localhost:8080/health/detailed
curl -s -H 'X-Admin-Key: local-admin-key' http://localhost:8080/metrics
curl -s -H 'X-Admin-Key: local-admin-key' http://localhost:8080/debug/pprof/
```

The scaffold keeps detailed health, metrics, and pprof behind the generated
admin middleware. The public routes remain limited to readiness, liveness,
version, and OpenAPI/docs.

## 5) Choose production settings

The default `saas-api` profile starts with API-key auth for local development.
For production, set explicit non-default `API_KEY`, `ADMIN_KEY`,
`IDEMPOTENCY_STORE=redis`, and `RATE_LIMIT_STORE=redis`. JWT and Clerk modes are
available from the generator:

```sh
api-toolkit new service --module example.com/my-api --profile saas-api --auth jwt
api-toolkit new service --module example.com/my-api --profile saas-api --auth clerk
```

For the heavier production foundation, use the `saas-api-full` profile:

```sh
api-toolkit new service \
  --module example.com/my-api \
  --profile saas-api-full \
  --auth api-key
```

It keeps this quick-start profile small and starts a Postgres + Redis oriented
tenant foundation with migrations, hexagonal package boundaries, OpenAPI
contracts, generated smoke tests, durable async/outbox workers, audit logs,
outbound webhook delivery, object storage hooks, OpenAPI 3.1, a checked-in typed
Go client, opt-in Docker integration checks, Docker Compose, and base Kubernetes
assets.

Useful full-profile follow-up commands:

```sh
make client-check
make resource-check
make migrate-check
```

Add app-specific tenant resources from inside a generated full-profile project:

```sh
api-toolkit generate resource \
  --name project \
  --plural projects \
  --tenant-scoped \
  --crud \
  --postgres \
  --soft-delete \
  --etag \
  --audit \
  --webhooks
```

Provider starters stay opt-in:

```sh
api-toolkit new service \
  --module example.com/my-api \
  --profile saas-api-full \
  --with stripe-billing \
  --with resend-email \
  --with clerk-webhooks
```

See [full-service-scaffold.md](full-service-scaffold.md) for the full runtime,
migration, deployment, provider, and resource-generation contract.

Next: use [cookbook.md](cookbook.md) for focused patterns and
[../contrib/examples/README.md](../contrib/examples/README.md) for runnable
example applications.
