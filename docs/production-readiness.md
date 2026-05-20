# Production Readiness

Audience: teams deciding where api-toolkit is ready to standardize production
Go API services and where application-owned work is still required.

api-toolkit v3 is production-credible for conventional Go JSON/HTTP APIs and
generated SaaS/API services. It is not a universal backend platform for every
transport, streaming workload, provider workflow, or organization-specific
operating model.

| Area | Status | What is covered | Caveats |
| --- | --- | --- | --- |
| Stable core packages | Stable SemVer surface | Ports, HTTP helpers, middleware, health, errors, idempotency, pagination, route contracts, OpenAPI helpers, webhooks, and tests listed in `VERSIONING.md`. | Breaking exported API changes require a future major version. Runtime behavior still needs release-note review. |
| Supported contrib adapters | Supported adapter tier | Postgres, Redis, chi, logging, OpenTelemetry, OpenAPI validation, bootstrap, async/outbox, audit, cache, object storage, webhook delivery, and selected auth/provider adapters with direct tests and drift evidence. | Supported-adapter incompatible drift is gate-enforced, but contrib is not part of the stable core API promise. |
| Experimental contrib packages | Maintained experimental tier | Policy-engine adapters, some config/migration helpers, country/email helpers, and development-only adapters. | Do not treat as standardized production contracts until promoted with tests, docs, health checks where relevant, drift coverage, and release notes. |
| `saas-api` scaffold | Lean starter | Runnable API-key/JWT/Clerk/dev-header service with health, OpenAPI, metrics, validation, idempotency, and safe admin endpoint defaults. | Keep app-specific persistence, membership, and workers app-owned or use `saas-api-full`. |
| `saas-api-full` scaffold | Production reference scaffold | Postgres + Redis foundation with tenancy, memberships, API keys, async/outbox, audit, webhook delivery, OpenAPI 3.1, generated clients, deployment starters, and integration-check assets. | Generated code is ordinary app-owned code and still needs product-specific tests, threat modeling, and operational ownership. |
| `examples/reference-saas-api` | Checked-in adoption proof | A separate generated `saas-api-full` application module verified by `make reference-service-check` and optional Docker integration. | This is release-context evidence, not a substitute for each downstream service's own load, backup/restore, incident-response, and rollout evidence. |
| Streaming, SSE, WebSockets, and large downloads | Explicit caveat | Route metadata and security profile opt-outs exist for streaming and large-response paths. | Do not apply hard-timeout response buffering, response validation, or idempotency response capture globally to these routes. Use `x-api-toolkit-streaming`, `securityprofile.StreamingRouteOverride`, and route-level middleware opt-outs. server-sent events, websocket upgrades, and custom `http.ResponseWriter` interfaces need route-specific wiring. |
| Release evidence | Publication-grade path | Clean `make release-evidence`, artifact verification, SBOMs, signatures, provenance, and release-review docs. | Dirty-tree evidence is local audit evidence only. Scheduled generated-scaffold integration is release evidence context, not a default PR blocker. |

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
