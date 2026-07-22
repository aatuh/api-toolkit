# Provider Adapter Maturity Matrix

Audience: adopters deciding whether a contrib adapter is suitable for
production use.

Contrib adapters are outside the stable core API promise. The maturity tier is
machine-owned by `docs/package-classification.tsv`; behavior evidence is owned
by `docs/supported-adapter-contracts.tsv`,
`docs/supported-adapter-test-realism.tsv`, and
`docs/contrib-api-drift-packages.txt`.

## Maturity Matrix

| Area | Packages | Tier | Production posture |
| --- | --- | --- | --- |
| Postgres (PostgreSQL 18) | `adapters/pgxpool`, `adapters/txpostgres`, `adapters/migrate`, `adapters/auditpostgres`, `adapters/operationpostgres`, `adapters/outboxpostgres`, `adapters/webhookdeliverypostgres`, `scheduler/postgres` | supported-adapter | Use after reviewing schema ownership, migrations, readiness checks, table-name validation, and backup/restore. Real contracts run through `make test-postgres`. |
| Redis | `adapters/cacheredis`, `adapters/idempotencyredis`, `adapters/ratelimitredis` | supported-adapter | Use for shared production cache, idempotency, and rate-limit state with service-specific prefixes and bounded keys. |
| Stripe | `adapters/stripe`, `integrations/stripe` | supported-adapter | Use after provider account setup, webhook signing, secret storage, and billing-domain review. |
| Resend | `adapters/resend`, `integrations/resend` | supported-adapter | Use after consent, suppression, template, provider quota, and live-delivery review. |
| Clerk and OIDC | `middleware/auth/clerk`, `integrations/auth/clerk`, `middleware/auth/oidc`, `integrations/auth/oidc` | supported-adapter | Use with trusted issuer, audience, JWKS/discovery URL, algorithm, tenant/scope claim, and rotation tests. |
| OpenTelemetry | `telemetry`, `middleware/oteltrace` | supported-adapter | Use when exporter endpoints are trusted config and span attributes stay low-cardinality and redacted. |
| CORS | `middleware/cors` | supported-adapter | Use with explicit browser origin policy; never wildcard credentialed admin routes. |
| Validation | `adapters/validation` | supported-adapter | Use for transport validation when field names, codes, and safe messages match application expectations. |
| HTTP client | `adapters/httpclient` | supported-adapter | Use for SSRF-guarded outbound calls with explicit host, scheme, port, redirect, retry, breaker, and timeout policy. |
| Policy engines | `adapters/cedar`, `adapters/opa` | supported-adapter | Use after policy lifecycle, malformed response, diagnostic error, and fail-closed startup review. |
| Wrapper-only helpers | `adapters/clock`, `adapters/ulid`, `adapters/uuid`, `email/noop` | wrapper-only | Use only when smoke coverage is enough because behavior belongs to the delegated dependency or no-op implementation. |
| Experimental helpers | `adapters/idempotency`, `countrycodes`, `email/markdown` | experimental | Use with app-owned compatibility expectations until promoted. |
| Tooling and generated code | `cmd/api-toolkit`, generated packages, examples | tooling/generated/example-only | Pin CLI versions in CI and review generated diffs; do not treat examples as compatibility promises. |

The currently supported PostgreSQL major is **18**. `make test-postgres` runs
the real contract harness only against the dedicated loopback endpoint
`127.0.0.1:54329` with the `api_toolkit_test` user and database. The harness
creates a database and schema per test, refuses remote, standard-port, or
non-test DSNs, and removes the ephemeral database during cleanup. Start the
matching local container before invoking the target:

```sh
docker run --rm -d --name api-toolkit-postgres-test \
  -e POSTGRES_USER=api_toolkit_test \
  -e POSTGRES_PASSWORD=api_toolkit_test \
  -e POSTGRES_DB=api_toolkit_test \
  -p 54329:5432 postgres:18-alpine
until docker exec api-toolkit-postgres-test \
  pg_isready -U api_toolkit_test -d api_toolkit_test; do sleep 1; done
GOWORK=off GOTOOLCHAIN=local make test-postgres
docker stop api-toolkit-postgres-test
```

## Review Checklist

- Confirm the package tier in `docs/package-classification.tsv`.
- Confirm direct tests, package docs, and behavior contract evidence.
- Confirm drift coverage in `docs/contrib-api-drift-packages.txt`.
- Confirm realistic evidence in `docs/supported-adapter-test-realism.tsv`.
- Confirm provider-specific secrets, credentials, account setup, and live checks
  are app-owned.
- Confirm metrics, logs, traces, OpenAPI examples, and Problem Details do not
  expose provider secrets or raw provider payloads.
