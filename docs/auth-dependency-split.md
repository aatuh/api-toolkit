# Auth Dependency Split Decision

Audience: adopters and maintainers evaluating auth-package dependency cost.

## Decision

V3 kept JWT/JWK middleware in the root module to preserve its stable import
path. V4 moves JWT/JWK middleware, shared JWT parsing, OAuth2 helpers, and auth
test support to `github.com/aatuh/api-toolkit/contrib/v4`. Root v4 has no direct JWT/JWK requirements.

| Import path | Ownership and dependency behavior |
| --- | --- |
| `github.com/aatuh/api-toolkit/v4/middleware/maxbody` | Root HTTP helper with no JWT/JWK dependency. |
| `github.com/aatuh/api-toolkit/v4/httpx` | Root HTTP helper with no JWT/JWK dependency. |
| `github.com/aatuh/api-toolkit/v4/binding` | Root HTTP helper with no JWT/JWK dependency. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/jwt` | Owns JWT/JWK validation and its `keyfunc` and `jwt/v5` dependencies. |
| `github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/shared` | Shares JWT/JWK implementation details across contrib auth middleware. |
| `github.com/aatuh/api-toolkit/contrib/v4/oauth2` | Owns bearer-token claim and scope helpers outside the root promise. |

Migration examples are in [migration/v4.md](migration/v4.md). Existing v3 imports remain unchanged in the v3 release line; v4 has no root aliases that
would retain the former dependency graph.

## Non-Goals

- Do not break v3 import paths to improve the dependency graph.
- Do not move provider-specific Clerk, OIDC, or session concerns into root.
- Do not hide JWT/JWK dependencies behind root aliases that keep the same module
  graph cost.
- Do not make root `authorization` or `ports` broader to compensate for the
  split; app-owned auth policy should stay app-owned.

Related documents:

- `docs/dependency-boundary.md`
- `docs/dependency-footprint.md`
- `docs/v4-plan.md`
- `docs/adr/0001-module-boundaries.md`
