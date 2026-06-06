# Dependency Boundary

The root module is the stable core surface. It should stay free of contrib and adapter-only dependencies so consumers can import core middleware, endpoints, and ports without inheriting Redis or contrib test adapters.

## Executable guardrail

Run:

```sh
GOTOOLCHAIN=local make dependency-boundary-check
```

The target runs `scripts/dependency_boundary_check.sh`, which executes the
docscheck contract
`TestStableCoreDependencyBoundariesExcludeContribProvidersAndGeneratedApps`.
The check fails if a stable or compatibility-only root package imports contrib,
scaffold-only packages, provider SDKs, database drivers, router adapters,
generated app code, reference-service code, or example code.

GitHub Actions runs this target in `.github/workflows/ci.yml` as part of the
governance job, and `make docs-check` also covers the same docscheck package.

## Root module direct requirements

- `github.com/MicahParks/jwkset`: required by core JWT key handling.
- `github.com/MicahParks/keyfunc/v3`: required by core JWT key resolution.
- `github.com/golang-jwt/jwt/v5`: required by core JWT middleware.
- `golang.org/x/time`: required by core rate-limit support and tracked by `go mod tidy`.

## Disallowed root adapter dependencies

These dependencies belong in `contrib/go.mod`, not root `go.mod`:

- `github.com/aatuh/api-toolkit/contrib/v3`
- `github.com/alicebob/miniredis/v2`
- `github.com/redis/go-redis/v9`

Stable core packages must also stay free of provider or adapter SDK imports,
including:

- `github.com/stripe/stripe-go`
- `github.com/resend/resend-go`
- `github.com/jackc/pgx`
- `github.com/go-chi/chi`
- `go.opentelemetry.io/otel`

When a package genuinely needs one of these dependencies, it belongs in contrib,
generated application code, or a future module split rather than in the stable
root API surface.

`docs/provider-adapter-split.md` records the provider-adapter split decision and
the future criteria for separate provider-family modules.

## Test ownership

- `middleware/idempotency` root tests use the package-local in-memory test store for middleware flow, migration recovery, telemetry, and rollout preflight behavior.
- `contrib/adapters/healthchecktest` owns reusable adapter contract coverage for health checker names, status classification, timestamps, messages, and duration shape.
- `contrib/adapters/idempotencytest` owns reusable adapter release-contract coverage for token-aware release behavior, legacy tokenless recovery, completed-record preservation, ambiguous-record preservation, and token mismatch handling.
- `contrib/adapters/ratelimittest` owns reusable adapter contract coverage for rate limiter empty-key bypass, per-key isolation, retry-after behavior, and token refill behavior.
- Redis- and memory-adapter contract tests live under `contrib/adapters/...`, where Redis and miniredis are already legitimate contrib module dependencies.
- Maintained idempotency stores are `contrib/adapters/idempotency` and
  `contrib/adapters/idempotencyredis`; both use the shared contract suite for
  token-aware release, legacy missing-token cleanup, token mismatch handling,
  completed-record preservation, and ambiguous-record preservation.
- Store-level migration telemetry labels remain explicit:
  `legacy_in_flight_recovered` and `legacy_in_flight_token_mismatch`. Middleware
  rollout telemetry labels remain explicit:
  `legacy_in_flight_fallback_entered`,
  `legacy_in_flight_fallback_recovered`,
  `legacy_in_flight_fallback_rejected`, and
  `legacy_in_flight_fallback_unknown`.
