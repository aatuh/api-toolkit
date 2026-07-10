# Reference SaaS API Service

`examples/reference-saas-api` is the checked-in adoption proof for the
`saas-api-full` scaffold. It is generated from the same CLI path as customer
services, uses local `replace` directives back to this workspace, and is kept as
a separate Go module so it behaves like an application that consumes
api-toolkit.

For the candid maintainer-owned account of what this evidence did and did not
establish, read the [adopter story](adopters.md). It is not an external customer
case study or a substitute for a downstream service's deployment evidence.

The reference service is intentionally production-like rather than minimal. It
includes Postgres migrations, Redis-backed production paths, tenant membership,
API keys, idempotent widget writes, async operations, outbox-backed webhook
delivery, audit records, object storage hooks, OpenAPI 3.1, the typed Go
client, admin health/metrics/pprof isolation, Docker Compose, Kubernetes
manifests, Helm, Terraform AWS starters, and observability assets.

## App-local Documentation

Use these app-owned documents when checking generated-service behavior instead
of relying only on root repository docs:

- [Reference service README](../examples/reference-saas-api/README.md)
- [Helm starter](../examples/reference-saas-api/deploy/helm/README.md)
- [Kubernetes starter](../examples/reference-saas-api/deploy/kubernetes/README.md)
- [Terraform AWS dependency starter](../examples/reference-saas-api/deploy/terraform/aws/README.md)
- [Observability runbook](../examples/reference-saas-api/observability/runbooks/observability.md)
- [Provider workflow runbook](../examples/reference-saas-api/docs/providers/provider-runbook.md)

## Local Verification

Use the root target for non-Docker evidence:

```sh
GOWORK=off GOTOOLCHAIN=local make reference-service-check
```

That target runs the reference service tests, OpenAPI golden check, contract
lint/diff, typed client regeneration check, asset check, observability check,
and deployment asset check. It is not part of default `make finalize`.

Use the evidence target when release reviewers need a recorded local artifact:

```sh
GOWORK=off GOTOOLCHAIN=local make reference-service-evidence
```

The command writes `.ci-result/reference-service/status`,
`.ci-result/reference-service/summary.json`, and logs for the checks it ran.
Set `REFERENCE_SERVICE_DOCKER=1` to include the service-owned Docker
`integration-check`; set `REFERENCE_SERVICE_MINIO=1` only when object-storage
integration evidence is in scope. The target is opt-in and not part of default
`make finalize`.

Use the coverage target when reviewers need a non-Docker coverage diagnostic for
the checked-in generated service:

```sh
GOWORK=off GOTOOLCHAIN=local make reference-service-coverage
```

The command writes `.ci-result/coverage/reference-service.func` and
`.ci-result/coverage/reference-service-summary.md`. It is reported separately
from root and contrib coverage thresholds because generated application code is
app-owned evidence, not part of the toolkit module aggregate.

Use the load-smoke target when reviewers need a local baseline for the checked-in
generated service:

```sh
GOWORK=off GOTOOLCHAIN=local make reference-service-load
```

The command runs the real reference-service router in-process with bounded
synthetic requests. It writes `.ci-result/reference-service-load/status`,
`.ci-result/reference-service-load/summary.json`,
`.ci-result/reference-service-load/summary.md`, and `load-smoke.log`. The JSON
and Markdown summaries include throughput, latency percentiles, memory deltas,
allocation deltas, expected auth failure behavior for a missing API key on
`GET /widgets`, unexpected status counts, rate-limit responses, and secret-leak
counts. The committed seed baseline is tracked in
`docs/reference-service-load-baseline.tsv`; treat it as release-review context,
not a cross-machine SLA.

Use the generated-service upgrade compatibility check when release reviewers
need evidence that published generator output can move to the current workspace:

```sh
GOWORK=off GOTOOLCHAIN=local make generated-upgrade-compat-check
```

By default this checks `v3.0.0` and `v3.1.2`. Set
`GENERATED_UPGRADE_COMPAT_REFS="v3.1.2 vX.Y.Z"` to choose a matrix, or
`GENERATOR_REF=vX.Y.Z` for the older single-ref path. Results are written under
`.ci-result/generated-upgrade-compat/` with one log per generator ref.

Use the service-owned Docker target when release reviewers need runtime
evidence:

```sh
cd examples/reference-saas-api
GOTOOLCHAIN=local make integration-check
```

`make integration-check` starts Postgres and Redis, runs migrations, starts the
API and worker, and exercises liveness, readiness, auth failure, tenant routes,
managed API-key auth, idempotency, ETag conflicts, async operation polling,
outbox behavior, webhook delivery/replay, object readback, audit writes, admin
metrics, admin pprof, and public admin-route isolation. Set
`INTEGRATION_OBJECT_STORE=s3` and the generated Compose MinIO profile when
object-storage integration evidence is in scope.

## Operational Evidence To Record

Before claiming a release has reference-service evidence, record these outcomes
in release review notes:

- `make reference-service-check` result.
- `make reference-service-evidence` summary path and status.
- `make reference-service-load` summary path, latency, throughput, memory,
  allocations, and expected failure behavior.
- Docker-backed `make integration-check` result, including whether MinIO ran.
- Migration `up`, `check`, `verify`, and guarded `down` refusal result.
- Backup/restore drill notes for Postgres and object storage when the service is
  deployed beyond local CI.
- Load-smoke result with request rates, error rates, latency summary, and any
  limit reached.

Keep failures as release-context evidence until they are understood. Do not make
Docker-backed reference-service checks required PR or local finalize gates until
they are consistently fast and stable.

## Adoption Evidence Template

Use this template when evaluating a downstream service generated from or
upgraded with api-toolkit. Repo evidence proves the scaffold path remains
usable; it does not replace deployment-owned evidence for the real service.

| Evidence item | Result |
| --- | --- |
| Setup time | Record elapsed time from generator command to first passing local check. |
| Upgrade result | Record source toolkit version, target toolkit version, changed files, and `generated-upgrade-compat-check` or service-specific upgrade output. |
| OpenAPI/client result | Record `openapi-check`, `contracts-lint`, `contracts-diff`, `client-check`, and any generated TypeScript client result. |
| Tenant isolation notes | Record tenant mismatch tests, role failures, and any app-owned authorization checks added beyond the scaffold. |
| Idempotency notes | Record unsafe-write replay behavior, conflict behavior, and storage mode used during evidence. |
| Backup/restore notes | Record Postgres backup/restore and object-store restore drill status for the target environment. |
| Load-smoke notes | Record request rate, latency, error rate, bottleneck, and whether the result was local, staging, or production-like. |
| Known pain points | Record manual edits, confusing generated boundaries, missing docs, slow checks, or operational gaps. |
