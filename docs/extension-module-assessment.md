# Extension Module Assessment

Audience: maintainers deciding whether a provider, database, router,
observability, or tooling family should leave the contrib module.

This assessment is a release-boundary decision, not an adoption claim. It uses
repository evidence available on 2026-07-11. External adopter counts and
family-specific release-cadence data are not available in this repository, so
they must not be inferred from package count, test coverage, or dependency
count.

## Evidence Baseline

`API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make dependency-report` recorded:

| Module | Direct requirements | Indirect requirements | Build-list dependencies |
| --- | ---: | ---: | ---: |
| Root | 3 | 1 | 4 |
| Contrib | 30 | 37 | 123 |

The same report found that the minimal-core path reaches zero third-party
packages. `make dependency-boundary-check` confirms root stable packages do not
import provider SDKs, database drivers, router adapters, telemetry exporters,
or generated applications. This boundary is already the desired v3 behavior.

Supported-adapter ownership, behavior contracts, test realism, and release
drift coverage are recorded in these source-of-truth files:

- `docs/package-owners.tsv`
- `docs/supported-adapter-contracts.tsv`
- `docs/supported-adapter-test-realism.tsv`
- `docs/contrib-api-drift-packages.txt`

The dependency report compared with `v3.1.2` shows 10 contrib build-list
additions and 10 removals. That is a module-wide maintenance signal, not proof
that any individual family needs independent versioning.

## Decision Rule

An extension family may move to a same-repository module only when all of these
are evidenced:

1. An accountable maintainer and release owner accepts the new module.
2. At least two independent adopters or a documented, recurring family-specific
   release-cadence conflict makes coordinated contrib releases materially worse.
3. The family has an import migration guide, package classification and owner
   rows, contract and test-realism records, and an API-drift baseline.
4. A dependency report shows the split improves the installation or review
   boundary without adding a root dependency or a root compatibility wrapper.
5. The module has a versioning, vulnerability-review, and release-note policy.

An external repository requires the same evidence plus an explicit decision for
discoverability, ownership transfer, issue tracking, security reporting, and
cross-repository release coordination.

## Candidate Assessment

| Family | Current ownership and evidence | Dependency and release evidence | Decision | Reassess when |
| --- | --- | --- | --- | --- |
| Postgres | `contrib-maintainers`; supported adapters include `pgxpool`, `txpostgres`, `migrate`, audit, operations, outbox, webhook delivery, and scheduler storage. Contract and realism records cover fake-DB and scheduled real-service evidence. | pgx is contrib-owned; the report is module-wide and shows no independent Postgres cadence or external adopter evidence. | Defer. Keep in the contrib module. | Postgres releases or integration evidence repeatedly block unrelated contrib releases, and an owner accepts a dedicated module release process. |
| Redis | `contrib-maintainers`; `cacheredis`, `idempotencyredis`, and `ratelimitredis` are supported adapters with miniredis and scheduled real-service evidence. | Redis remains absent from root; no independent adoption or cadence evidence exists. | Defer. Keep in the contrib module. | Redis release, incident, or support cadence diverges from other adapters and a migration path is tested. |
| Stripe | `contrib-maintainers`; adapter and integration rows are supported and use hermetic provider fixtures with optional sandbox checks. | Stripe SDK risk and versioning are already isolated to contrib; there is no evidence that a separate release line improves adoption or review. | Defer. Keep in the contrib module. | Provider API changes repeatedly require urgent, independent releases with an accountable Stripe module owner. |
| OpenTelemetry and observability | `contrib-maintainers`; telemetry, OTEL trace middleware, metrics, request logging, and Zap integration are contrib-owned with direct tests and drift coverage. | Exporter and telemetry dependencies are absent from root; no separate adoption or cadence evidence exists. | Defer. Keep in the contrib module. | Exporter dependency churn or observability release cadence blocks unrelated adapters and a dedicated module can keep imports coherent. |
| Provider auth | `contrib-maintainers` for Clerk, OIDC, and provider integrations; the root JWT/JWK split has a separate `core-maintainers` decision record. | Provider auth is already optional in contrib. Root JWT/JWK graph cost is an auth-module question, not evidence for splitting provider adapters now. | Defer provider-auth extraction; retain the separate v4 auth split decision. | The root auth split is approved and provider-auth users have tested old-to-new import migrations. |
| Router and browser adapters | `contrib-maintainers`; Chi and CORS are supported-adapter paths with direct in-process tests. | Router dependencies are absent from root; no router-specific cadence or adopter evidence supports a separate module. | Defer. Keep in the contrib module. | A router family needs independent versioning or a release owner documents sustained independent adoption. |

## Result

No provider family is approved for same-repository module extraction or an
external repository. This rejects speculative splits while preserving the
existing optional contrib boundary. E2-T2 therefore has no approved adapter
module to extract in v3; any future extraction must satisfy this assessment and
ship v3 compatibility wrappers and compile-checked migration examples.

CLI and scaffold ownership are assessed separately in
`docs/cli-scaffold-identity.md`; they remain contrib tooling and do not expand
the root stable API promise.

Related documents:

- `docs/provider-adapter-split.md`
- `docs/auth-dependency-split.md`
- `docs/dependency-footprint.md`
- `docs/v4-plan.md`
