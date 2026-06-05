# Production Readiness

Audience: teams deciding where api-toolkit is ready to standardize production
Go API services and where application-owned work is still required.

api-toolkit v3 is production-credible for conventional Go JSON/HTTP APIs and
generated SaaS/API services. It is not a universal backend platform for every
transport, streaming workload, provider workflow, or organization-specific
operating model. Generated code is app-owned; api-toolkit standardizes the
HTTP/API infrastructure foundation while product logic, provider workflows, and
deployment evidence stay with each service.

For new library adopters, prefer the small stable core described in
`docs/stable-core.md`. Treat generated scaffolds and contrib adapters as
intentional adoption choices, not prerequisites for using the root module.
Use `docs/core-readiness.md` as the package-specific production checklist before
standardizing on any stable root package.

| Area | Status | What is covered | Caveats |
| --- | --- | --- | --- |
| Stable core packages | Stable SemVer surface | Ports, HTTP helpers, middleware, health, errors, idempotency, pagination, route contracts, OpenAPI helpers, webhooks, and tests listed in `VERSIONING.md`. | Breaking exported API changes require a future major version. Runtime behavior still needs release-note review. |
| Supported contrib adapters | Supported adapter tier | Postgres, Redis, chi, logging, OpenTelemetry, OpenAPI validation, bootstrap, async/outbox, audit, cache, object storage, webhook delivery, and selected auth/provider adapters with direct tests and drift evidence. | Supported-adapter incompatible drift is gate-enforced, but contrib is not part of the stable core API promise. |
| Experimental contrib packages | Maintained experimental tier | Policy-engine adapters, some config/migration helpers, country/email helpers, and development-only adapters. | Do not treat as standardized production contracts until promoted with tests, docs, health checks where relevant, drift coverage, and release notes. |
| `saas-api` scaffold | Lean starter | Runnable API-key/JWT/Clerk/dev-header service with health, OpenAPI, metrics, validation, idempotency, and safe admin endpoint defaults. | Keep app-specific persistence, membership, and workers app-owned or use `saas-api-full`. |
| `saas-api-full` scaffold | Production reference scaffold | Postgres + Redis foundation with tenancy, memberships, API keys, async/outbox, audit, webhook delivery, OpenAPI 3.1, generated clients, deployment starters, and integration-check assets. | Generated code is app-owned ordinary Go code and still needs product-specific tests, threat modeling, and operational ownership. |
| `examples/reference-saas-api` | Checked-in adoption proof | A separate generated `saas-api-full` application module verified by `make reference-service-check` and optional Docker integration. | This is release-context evidence, not a substitute for each downstream service's own load, backup/restore, incident-response, and rollout evidence. |
| Streaming, SSE, WebSockets, and large downloads | Explicit caveat | Route metadata and security profile opt-outs exist for streaming and large-response paths. | Do not apply hard-timeout response buffering, response validation, or idempotency response capture globally to these routes. Use `x-api-toolkit-streaming`, `securityprofile.StreamingRouteOverride`, and route-level middleware opt-outs. server-sent events, websocket upgrades, and custom `http.ResponseWriter` interfaces need route-specific wiring. |
| Release evidence | Publication-grade path | Clean `make release-evidence`, artifact verification, SBOMs, signatures, provenance, and release-review docs. | Dirty-tree evidence is local audit evidence only. Scheduled generated-scaffold integration is release evidence context, not a default PR blocker. |

## Package-Specific Production Checklist

The package-specific checklist source is `docs/core-readiness.md`. For each
stable or compatibility-only root package, reviewers must check:

- package docs and public guide links,
- compile-checked examples,
- direct tests and any package-specific coverage notes,
- fuzz decision for parsers, signatures, request metadata, or header logic,
- benchmark decision for hot paths or response buffering,
- compatibility status in `VERSIONING.md` and `docs/api-inventory.md`,
- security review notes for auth, tenant, replay, streaming, observability, and
  operator-only surfaces,
- production caveats that remain application-owned.

Do not treat a package as production-standardized for a service until its row in
`docs/core-readiness.md` has been reviewed against that service's route shape,
provider integrations, storage model, and deployment evidence.

## Adapter Maturity Review

The adapter maturity review is manifest-driven. Packages classified as
`supported-adapter` in `docs/package-classification.tsv` are the
evidence-complete supported adapter set: they must have direct tests, package
docs, a behavior contract in `docs/supported-adapter-contracts.tsv`, and release
drift coverage in `docs/contrib-api-drift-packages.txt`. Test-realism evidence
is tracked separately in `docs/supported-adapter-test-realism.tsv` so reviewers
can tell direct-unit, fake DB, miniredis, hermetic fixture, and scheduled/manual
real-service evidence apart without treating all adapter tests as equivalent.

Packages classified as `experimental` are intentionally not promoted. They can
be useful and maintained, but remain outside the supported-adapter tier until
their package-specific tests, docs, health behavior where relevant, drift
coverage, and release notes justify promotion. The manifests remain the source
of truth; this section exists to make that review visible during production
readiness and release review.

## Maturity Evidence Still Required Per Service

- Application-specific authorization and tenant-isolation tests.
- Load, soak, and rollback evidence in the target deployment environment.
- Provider-specific live checks when Stripe, Resend, Clerk, or other external
  systems are part of the service.
- Operational runbooks for paging, backups, restore drills, migrations, and
  incident response.
- Review of streaming, binary, multipart, and unusually large response routes.

Use the [reference service evidence template](reference-service.md#adoption-evidence-template)
when turning repository proof into downstream service adoption evidence.
