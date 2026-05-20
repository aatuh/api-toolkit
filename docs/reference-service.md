# Reference SaaS API Service

`examples/reference-saas-api` is the checked-in adoption proof for the
`saas-api-full` scaffold. It is generated from the same CLI path as customer
services, uses local `replace` directives back to this workspace, and is kept as
a separate Go module so it behaves like an application that consumes
api-toolkit.

The reference service is intentionally production-like rather than minimal. It
includes Postgres migrations, Redis-backed production paths, tenant membership,
API keys, idempotent widget writes, async operations, outbox-backed webhook
delivery, audit records, object storage hooks, OpenAPI 3.1, the typed Go
client, admin health/metrics/pprof isolation, Docker Compose, Kubernetes
manifests, Helm, Terraform AWS starters, and observability assets.

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

Use the generated-service upgrade compatibility check when release reviewers
need evidence that published generator output can move to the current workspace:

```sh
GOWORK=off GOTOOLCHAIN=local make generated-upgrade-compat-check
```

By default this checks `v3.0.0` and `v3.1.0`. Set
`GENERATED_UPGRADE_COMPAT_REFS="v3.1.0 vX.Y.Z"` to choose a matrix, or
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
- Docker-backed `make integration-check` result, including whether MinIO ran.
- Migration `up`, `check`, `verify`, and guarded `down` refusal result.
- Backup/restore drill notes for Postgres and object storage when the service is
  deployed beyond local CI.
- Load-smoke result with request rates, error rates, latency summary, and any
  limit reached.

Keep failures as release-context evidence until they are understood. Do not make
Docker-backed reference-service checks required PR or local finalize gates until
they are consistently fast and stable.
