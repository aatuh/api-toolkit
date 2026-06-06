# Auth Dependency Split Decision

Audience: adopters and maintainers evaluating whether simple api-toolkit
middleware pulls in JWT/JWK dependency weight.

The v3 root module currently contains both simple HTTP guardrails and stable
auth packages. Simple packages such as `middleware/maxbody`, `httpx`, and
`binding` do not compile against JWT/JWK packages, but the root module graph
still includes JWT/JWK modules because `middleware/auth/jwt` is stable in the
same module.

## Decision

For v3:

- Keep `middleware/auth/jwt` in the root module to preserve stable import paths.
- Keep direct root requirements for `github.com/MicahParks/jwkset`,
  `github.com/MicahParks/keyfunc/v3`, and `github.com/golang-jwt/jwt/v5`
  because the stable JWT middleware owns those dependencies.
- Keep simple middleware package imports free of JWT/JWK packages.
- Document the current module-graph cost instead of pretending simple
  middleware users have a zero-auth dependency graph in v3.

For v4 planning:

- Split JWT/JWK-heavy auth packages into a dedicated auth module or another
  explicit non-core module.
- Remove JWT/JWK direct requirements from the root module when the stable JWT
  package leaves root.
- Keep API-key and tenant helpers in root only if design review confirms they
  remain small, provider-neutral HTTP guardrails.
- Provide migration notes for old root imports before the v4 branch cuts.

## Current Dependency Boundary

| Import path | Current v3 dependency behavior | V4 target |
| --- | --- | --- |
| `github.com/aatuh/api-toolkit/v3/middleware/maxbody` | Does not import JWT/JWK packages. Root module graph still includes JWT/JWK direct requirements. | Root users should not inherit JWT/JWK modules. |
| `github.com/aatuh/api-toolkit/v3/httpx` | Does not import JWT/JWK packages. Root module graph still includes JWT/JWK direct requirements. | Root users should not inherit JWT/JWK modules. |
| `github.com/aatuh/api-toolkit/v3/binding` | Does not import JWT/JWK packages. Root module graph still includes JWT/JWK direct requirements. | Root users should not inherit JWT/JWK modules. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/jwt` | Imports `keyfunc` and `jwt/v5`; owns JWK/JWT behavior. | Move to the auth module or another explicit non-core module. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/shared` | Imports `keyfunc` and `jwt/v5`; implementation-sharing package, not stable API. | Move with JWT middleware or stay internal to the auth module. |
| `github.com/aatuh/api-toolkit/v3/oauth2` | Provider-neutral scope/claim helpers, no direct JWT/JWK imports. | Review with auth split; keep root only if still provider-neutral and dependency-light. |

## Split Requirements

A v4 auth split must include:

- a design issue or decision note for the new auth module path,
- import migration examples for JWT middleware users,
- package classification and owner rows for moved packages,
- release notes that call out dependency graph impact,
- docscheck guardrails proving minimal-core examples do not use the auth module,
- release evidence showing root `go.mod` no longer directly requires JWT/JWK
  modules.

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
