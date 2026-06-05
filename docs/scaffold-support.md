# Generated Scaffold Support Matrix

Audience: teams using generated services and maintainers reviewing scaffold
changes.

Generated services are app-owned generated code. api-toolkit supports the
template, defaults, and release evidence for generated outputs; the downstream
application owns product code, deployment, and local modifications after
generation.

## Support Matrix

| Area | Supported by api-toolkit | App-owned after generation | Regeneration expectation |
| --- | --- | --- | --- |
| Service templates | Current `saas-api`, `saas-api-full`, `dev-api`, and `saas-web` template shape. | Product routes, domain model, persistence rules, and business workflows. | Generate into a temporary directory and port intentional changes manually. |
| HTTP guardrails | Middleware wiring, route contracts, OpenAPI defaults, health/version/docs endpoints, and admin endpoint defaults. | Route-specific opt-outs, auth policy, tenant policy, and product-specific validation. | Review middleware and route-contract diffs before copying. |
| Generated CI and Makefile | Starter commands and pinned workflow templates. | Required checks, deployment-specific gates, and provider-live checks. | Keep generated targets until the app has equivalent owned gates. |
| `.env.example` and config | Example variable names, unsafe dev defaults, and startup validation patterns. | Real secrets, environment values, secret stores, and config rollout. | Never copy committed examples into production secret stores. |
| Deployment starters | Docker, Kubernetes, Helm, Terraform, and observability starter assets where generated. | Cluster topology, IAM, networking, backup/restore, SLOs, and incident runbooks. | Treat starter assets as templates, not managed infrastructure. |
| Reference service evidence | `examples/reference-saas-api` demonstrates a checked-in generated full profile. | Downstream services must collect their own load, backup, provider, and rollout evidence. | Use reference evidence as comparison, not as proof for your deployment. |

## What Breaks On Regeneration

Regeneration can overwrite or conflict with:

- local route additions,
- product domain files,
- migrations,
- OpenAPI golden files,
- generated client code,
- deployment starter edits,
- observability dashboards and alert rules,
- provider workflow customization,
- service-specific environment defaults.

Use a temporary generation directory and compare diffs. Do not regenerate
directly over a production service checkout unless the service has an app-owned
merge process for generated files.

## Migration Expectations

- Upgrade toolkit modules deliberately and read `docs/migration/v3.md`.
- Review generated template changes like application code.
- Keep generated app secrets, provider credentials, webhook secrets, API keys,
  admin keys, and Terraform state out of source control.
- Run generated service tests, OpenAPI checks, contract lint/diff, asset checks,
  and deployment checks that the service owns.
- Record any local scaffold divergence in the generated service repository.
