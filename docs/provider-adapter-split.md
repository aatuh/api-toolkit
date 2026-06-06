# Provider Adapter Split Decision

Audience: adopters and maintainers deciding where provider, database, cache,
router, telemetry, and integration dependencies belong.

api-toolkit keeps provider adapters outside the root stable core. The root
module should stay useful for existing `net/http`, chi, or app-owned router
services without inheriting Postgres, Redis, Stripe, Resend, Clerk,
OpenTelemetry, CORS, router-adapter, or generated-service dependency weight.

## Decision

For v3:

- Keep provider and infrastructure adapters in
  `github.com/aatuh/api-toolkit/contrib/v3`.
- Keep Postgres, Redis, Stripe, Resend, Clerk, OpenTelemetry, CORS, chi,
  validation, logging, metrics, and generated-service wiring outside the root
  stable API promise.
- Keep root stable packages free of provider SDKs, database drivers, router
  adapters, telemetry exporters, and generated application code.
- Use `docs/package-classification.tsv`,
  `docs/supported-adapter-contracts.tsv`,
  `docs/supported-adapter-test-realism.tsv`, and
  `docs/contrib-api-drift-packages.txt` as the evidence set for supported
  contrib adapters.

For v4 planning:

- Split high-use provider families into separate modules only when contrib
  becomes too broad for installation, review, or release cadence.
- Do not promote provider adapters into root stable core only to make imports
  shorter.
- Any provider-module split must preserve the root library-first identity and
  keep provider-specific behavior out of root `ports`.

## Current Adapter Families

| Family | Current location | Root impact | Future split trigger |
| --- | --- | --- | --- |
| Postgres | `contrib/adapters/pgxpool`, `txpostgres`, `migrate`, `auditpostgres`, `operationpostgres`, `outboxpostgres`, `webhookdeliverypostgres`, `scheduler/postgres` | No root database driver dependency. | Split if Postgres release cadence or integration tests dominate contrib. |
| Redis | `contrib/adapters/cacheredis`, `idempotencyredis`, `ratelimitredis` | No root Redis dependency. | Split if cache/idempotency/rate-limit adapter release cadence needs independent versioning. |
| Stripe and billing adapters | `contrib/adapters/stripe`, `contrib/integrations/stripe` | No root Stripe SDK dependency; root billing remains compatibility-only. | Split if provider API changes require a separate support and release stream. |
| Resend and email adapters | `contrib/adapters/resend`, `contrib/integrations/resend` | No root Resend SDK dependency. | Split if live-provider policy or dependency updates need a separate cadence. |
| Clerk, OIDC, and provider auth | `contrib/middleware/auth/*`, `contrib/integrations/auth/*` | Provider-specific auth stays out of root. | Split with the broader auth-heavy dependency plan when needed. |
| OpenTelemetry, metrics, and logging adapters | `contrib/telemetry`, `contrib/middleware/oteltrace`, `metrics`, `requestlog`, `adapters/logzap` | No root exporter, Prometheus, or zap dependency. | Split if observability dependencies become too broad for contrib. |
| Router and browser adapters | `contrib/adapters/chi`, `contrib/middleware/cors` | No root chi or CORS adapter dependency. | Split only if router or browser adapters need independent versioning. |

## Promotion Rules

A provider adapter may be promoted from experimental to supported-adapter only
when it has:

- direct tests for owned behavior,
- package docs,
- behavior contracts in `docs/supported-adapter-contracts.tsv`,
- realism evidence in `docs/supported-adapter-test-realism.tsv`,
- drift coverage when the package is high-use,
- release-note policy for behavior changes,
- clear ownership of secrets, provider account setup, live checks, and
  deployment-specific configuration by the consuming application.

Supported-adapter status does not make the package stable core.

## Non-Goals

- Do not add Postgres, Redis, Stripe, Resend, Clerk, OpenTelemetry, chi, zap, or
  provider SDK imports to root stable packages.
- Do not expose provider-specific request/response shapes through root `ports`.
- Do not treat generated service provider wiring as proof that the root module
  should own the provider abstraction.
- Do not hide provider SDK imports behind root convenience wrappers.

Related documents:

- `docs/contrib-adapters.md`
- `docs/adapter-maturity.md`
- `docs/dependency-boundary.md`
- `docs/v4-plan.md`
- `docs/adr/0001-module-boundaries.md`
