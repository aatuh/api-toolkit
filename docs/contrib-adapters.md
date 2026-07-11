# Contrib Adapter Path

Audience: teams that already picked a stable core package and now need a
maintained adapter, integration, example, or generator from the contrib module.

Contrib is outside the stable core API promise. Supported adapters are tested,
documented, and reviewed for drift, but they are not the same compatibility tier
as root stable packages.

Install contrib only when the service needs a third-party adapter or the CLI:

```sh
go get github.com/aatuh/api-toolkit/contrib/v4
```

## Adapter decision table

| Need | Start in core | Add contrib when | Do not add contrib when |
| --- | --- | --- | --- |
| Router integration | `routecontracts`, `routepolicy`, `specs` | You want chi helpers from `contrib/adapters/chi`. | Your app-owned router already maps policies clearly. |
| Runtime OpenAPI validation | `specs`, `routecontracts` | You need `contrib/middleware/openapi` request/response validation. | You only need to publish OpenAPI metadata. |
| Metrics and logs | `middleware/trace`, `httpx/identity` | You need Prometheus, OpenTelemetry, zap, or request logging adapters. | Your platform already owns observability middleware. |
| Redis-backed behavior | `middleware/idempotency`, `middleware/ratelimit` | You need Redis stores for idempotency or rate limiting. | In-process dev/test stores are enough. |
| Postgres-backed behavior | root ports or app-owned interfaces | You need migrations, transaction, scheduler, audit, operation, or outbox adapters. | Database behavior is product-specific or provider-specific. |
| Provider integrations | `email`, `webhooks`, `compat/billing` | You need Resend, Stripe, Clerk, or provider workflow starters. | The integration shape is business-specific and better app-owned. |
| New service generation | none | You want `contrib/cmd/api-toolkit`. | You only need one root middleware package. |

## Maturity tiers

Use `docs/package-classification.tsv` as the source of truth.

| Tier | Meaning | Adoption posture |
| --- | --- | --- |
| `supported-adapter` | Maintained contrib runtime adapter with direct tests, drift review, and behavior evidence. | Acceptable for production after reviewing provider setup and package docs. |
| `experimental` | Maintained but not protected by stable API gates. | Use with app-owned compatibility expectations. |
| `wrapper-only` | Thin convenience wrapper around another adapter or library. | Expect smoke coverage, not deep behavior ownership. |
| `tooling` | CLI, generator, or developer tool. | Pin versions in CI and review generated diffs. |
| `example-only` | Runnable examples and sketches. | Copy patterns, not compatibility promises. |

## Rules

- Root stable packages must not import contrib packages.
- App-specific provider contracts should stay in app code unless the adapter has
  a documented supported-adapter tier.
- Release reviewers must check `docs/contrib-api-drift-packages.txt`,
  `docs/contrib-api-drift-dispositions.tsv`, and
  `docs/supported-adapter-contracts.tsv` before treating contrib behavior as
  release evidence.
- Generated scaffold code is app-owned even when it imports supported contrib
  adapters.

