# Dependency Boundary

The root module is the stable core surface. It should stay free of contrib and adapter-only dependencies so consumers can import core middleware, endpoints, and ports without inheriting Redis or contrib test adapters.

## Root module direct requirements

- `github.com/MicahParks/jwkset`: required by core JWT key handling.
- `github.com/MicahParks/keyfunc/v3`: required by core JWT key resolution.
- `github.com/golang-jwt/jwt/v5`: required by core JWT middleware.
- `golang.org/x/time`: required by core rate-limit support and tracked by `go mod tidy`.

## Disallowed root adapter dependencies

These dependencies belong in `contrib/go.mod`, not root `go.mod`:

- `github.com/aatuh/api-toolkit/contrib/v2`
- `github.com/alicebob/miniredis/v2`
- `github.com/redis/go-redis/v9`

## Test ownership

- `middleware/idempotency` root tests use the package-local in-memory test store for middleware flow, migration recovery, telemetry, and rollout preflight behavior.
- `contrib/adapters/idempotencytest` owns reusable adapter release-contract coverage for token-aware release behavior, legacy tokenless recovery, completed-record preservation, ambiguous-record preservation, and token mismatch handling.
- Redis- and memory-adapter contract tests live under `contrib/adapters/...`, where Redis and miniredis are already legitimate contrib module dependencies.
