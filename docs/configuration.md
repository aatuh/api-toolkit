# Runtime Configuration Guide

Audience: operators and developers configuring generated services or contrib
runtime adapters.

Configuration is deployment-owned. The toolkit documents expected variable
names, unsafe development defaults, and startup validation patterns; production
values belong in secret stores or operator-managed configuration.

## Production Variables

| Variable | Development default | Production requirement |
| --- | --- | --- |
| `ENV` | `development` | Set to the deployed environment name. |
| `API_ADDR` | `:8080` | Bind public API listener. |
| `ADMIN_ADDR` | `:9090` in reference service | Bind admin listener on internal-only networking when admin routes are enabled. |
| `DATABASE_URL` | empty | Required when Postgres persistence, managed API keys, tenancy, audit, outbox, operations, or object metadata are enabled. |
| `REDIS_ADDR` | `localhost:6379` | Required when cache, rate-limit, or idempotency stores use Redis. |
| `CACHE_STORE` | `memory` | Use `redis` for shared production cache state. |
| `RATE_LIMIT_STORE` | `memory` | Use `redis` for shared production rate limits. |
| `RATE_LIMIT_KEY_PREFIX` | `ratelimit:` | Use a service-specific prefix when Redis is shared. |
| `IDEMPOTENCY_STORE` | `memory` | Use `redis` for shared production idempotency. |
| `IDEMPOTENCY_KEY_PREFIX` | `idempotency:` | Use a service-specific prefix when Redis is shared. |
| `OPENAPI_REQUEST_VALIDATION` | `true` | Keep enabled unless route contracts are incomplete and the exception is documented. |
| `OPENAPI_RESPONSE_VALIDATION` | development/test opt-in | Enable in production only for finite responses after latency and failure-mode review. |
| `API_KEY` | local setup value | Bootstrap setup credential only; rotate to managed scoped keys after setup. |
| `API_ACTOR_ID` | empty | Set a production actor identity for bootstrap requests. |
| `API_KEY_PEPPER` | empty | Required high-entropy secret for managed API-key hashing in production. |
| `ADMIN_KEY` | local admin value | Required for admin health, metrics, and pprof when generated admin auth is used. |
| `WEBHOOK_SECRET_KEY` | empty | Required when persisted webhook endpoint signing secrets are encrypted at rest. |
| `OTEL_TRACING_ENABLED` | `false` | Enable only with trusted exporter configuration. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | empty | Required when tracing is enabled. |

## Unsafe Development Defaults

These defaults are acceptable only for local development:

- memory cache, rate-limit, and idempotency stores,
- local bootstrap API key,
- local admin key,
- dev-header auth,
- auth skip headers,
- Stripe webhook verification skip,
- in-process object storage,
- response validation enabled without production latency review.

Production startup should fail closed when required secrets, Redis, Postgres,
tracing endpoints, issuer/audience/JWKS values, or dangerous-bypass guards are
missing or invalid.

## Startup Validation

Validate configuration before serving traffic:

- parse bool, int, duration, enum, URL, and CSV values strictly,
- reject invalid `ENV=production` dangerous bypasses,
- check Postgres and required platform tables when `DATABASE_URL` is set,
- check Redis when Redis-backed runtime state is selected,
- validate auth issuer, audience, algorithms, and JWKS/discovery URLs,
- reject missing `API_KEY_PEPPER` when managed API-key hashing is enabled,
- reject missing `WEBHOOK_SECRET_KEY` when persisted webhook secrets are used,
- require tracing exporter endpoint when tracing is enabled.
