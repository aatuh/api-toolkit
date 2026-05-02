# Package Documentation Standard

Audience: maintainers updating public Go package docs for stable core packages,
compatibility surfaces, and high-use contrib adapters or integrations.

## Minimum package-doc content

Each public `doc.go` should answer these questions in plain Go doc prose:

| Field | Minimum expectation |
| --- | --- |
| Purpose | What task or boundary the package owns. |
| Primary abstractions | The main types, constructors, middleware, helpers, or adapters a caller starts with. |
| Stability status | Whether the package is stable core, compatibility-only, supported-adapter contrib, experimental contrib, wrapper-only, test-only, generated, tooling, or excluded. |
| Common construction path | The normal constructor or setup path when one exists. |
| Safety caveats | Fail-closed behavior, dangerous bypasses, compatibility-sensitive surfaces, or production-only constraints. |
| Examples | Pointer to `docs/cookbook.md`, `contrib/examples/README.md`, or a package-specific example when useful. |

Keep the package comment concise. Do not turn package docs into a second README.
Use links to canonical docs when the topic is already covered elsewhere.

## Placeholder inventory remediated in this pass

The v1 documentation audit found placeholder comments such as `Package X
provides X utilities`. This remediation replaced or added package docs for the
following groups.

Stable core and compatibility packages:

- `authorization/doc.go`
- `email/doc.go`
- `endpoints/list/doc.go`
- `endpoints/version/doc.go`
- `fielderrors/doc.go`
- `httpx/identity/doc.go`
- `middleware/auth/jwt/doc.go`
- `middleware/auth/tenant/doc.go`
- `middleware/json/doc.go`
- `middleware/maxbody/doc.go`
- `middleware/ratelimit/doc.go`
- `middleware/secure/doc.go`
- `middleware/trace/doc.go`
- `scheduler/migrations/doc.go`
- `securityprofile/doc.go`
- `swagstub/doc.go`

High-use contrib adapters, middleware, integrations, and wrappers:

- `contrib/adapters/cedar/doc.go`
- `contrib/adapters/clock/doc.go`
- `contrib/adapters/idempotency/doc.go`
- `contrib/adapters/logzap/doc.go`
- `contrib/adapters/migrate/doc.go`
- `contrib/adapters/pgxpool/doc.go`
- `contrib/adapters/resend/doc.go`
- `contrib/adapters/stripe/doc.go`
- `contrib/adapters/ulid/doc.go`
- `contrib/adapters/uuid/doc.go`
- `contrib/countrycodes/doc.go`
- `contrib/email/markdown/doc.go`
- `contrib/email/noop/doc.go`
- `contrib/integrations/auth/clerk/doc.go`
- `contrib/integrations/auth/devheaders/doc.go`
- `contrib/integrations/auth/jwt/doc.go`
- `contrib/integrations/pgxpool/doc.go`
- `contrib/integrations/resend/doc.go`
- `contrib/integrations/stripe/doc.go`
- `contrib/integrations/txpostgres/doc.go`
- `contrib/middleware/auth/devheaders/doc.go`
- `contrib/middleware/cors/doc.go`
- `contrib/middleware/metrics/doc.go`
- `contrib/middleware/oteltrace/doc.go`
- `contrib/scheduler/postgres/doc.go`
- `contrib/telemetry/doc.go`

## Quality gate

`GOTOOLCHAIN=local make docs-check` includes a docscheck rule that fails when a
public `doc.go` reintroduces the placeholder `Package X provides X utilities`
shape.
