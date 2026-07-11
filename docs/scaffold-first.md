# Scaffold-First Path

Audience: teams starting a new service and willing to own generated application
code.

The scaffold path generates ordinary app-owned Go code. Generated services are app-owned generated code.
`api-toolkit` supplies production defaults, templates, and contract checks; the
generated service is not a hosted framework and is not regenerated blindly over
product code.

Use this path when you want a new service baseline with router wiring, health,
version, OpenAPI, auth mode, idempotency, admin routes, CI, Docker assets, and a
Makefile already in place.

Use [library-first.md](library-first.md) instead when you already have a service
or only need one middleware package.

## Generate a new service

```sh
go run github.com/aatuh/api-toolkit/contrib/v4/cmd/api-toolkit@latest new service \
  --module example.com/my-api \
  --profile saas-api \
  --dir my-api
cd my-api
cp .env.example .env
go mod tidy
```

Then follow [getting-started.md](getting-started.md) for the build, OpenAPI, and
local curl checks.

## Ownership split

| Area | Toolkit owns | Generated app owns |
| --- | --- | --- |
| Templates | Safe defaults and current scaffold shape. | Product-specific edits after generation. |
| HTTP guardrails | Middleware packages, route contract helpers, and docs. | Which routes get which middleware and opt-outs. |
| Domain code | Sample widgets only as starter shape. | Real entities, use cases, data rules, and migrations. |
| Provider workflows | Starter integration patterns. | Provider account setup, live credentials, webhook policy, billing semantics. |
| Release checks | Toolkit gates for generated defaults. | Service-specific tests, deployment gates, threat model, and runbooks. |

Generated README files must say "This service is app-owned generated code" near
the top. Scaffold docs should repeat that generated code is app-owned so users
do not confuse templates with a managed application framework.

## Profile choice

| Profile | Use for | Avoid when |
| --- | --- | --- |
| `saas-api` | Lean JSON/HTTP service starter. | You need database-backed tenancy, outbox, or provider starters on day one. |
| `saas-api-full` | Production reference scaffold with Postgres, Redis, async/outbox, audit, OpenAPI 3.1, typed clients, and deployment starters. | You want a tiny first dependency or already have platform wiring. |
| `dev-api` | Local development services with explicit debug-header auth. | Any production or shared environment. |
| `saas-web` | Browser/session service starter with cookies, CSRF, and OIDC session flow. | API-first bearer-token services. |

## Regeneration policy

- Treat generated code as app-owned source once it lands in your repository.
- Re-run generators only for intentional scaffold upgrades, then review the
  diff like application code.
- Keep toolkit module replacements, `.env.example`, generated CI, and contract
  commands until the app has an equivalent owned replacement.
- Store production secrets outside generated files; generated scaffolds use
  examples and startup validation, not committed credentials.
