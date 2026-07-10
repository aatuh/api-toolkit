# Adopter Story: Checked-in Reference Service

Audience: maintainers and teams deciding what the full-service scaffold proves
before they rely on it in their own service.

This is not an external customer case study. The story is about the
maintainer-owned `examples/reference-saas-api` module, generated from the
`saas-api-full` path and kept in this repository as a separate application
consumer. It is useful adoption evidence because it exercises the generator
and library as an application would. It is not an independent adopter report,
a production deployment, or evidence of customer usage.

## What Worked

- Keeping the generated service as its own Go module made the application
  boundary visible. The service can consume the workspace through local
  `replace` directives while still owning ordinary application code, tests,
  configuration, and deployment assets.
- The full-profile service puts several cross-cutting paths in one checked
  consumer: tenant membership and API keys, idempotent widget writes, Postgres
  migrations, Redis-backed paths, async operations, audit records, webhook
  delivery, and object-storage hooks.
- `make reference-service-check` verifies the application tests alongside its
  OpenAPI golden file, contract lint/diff, typed Go client regeneration,
  observability assets, and deployment assets. This catches drift that root
  package tests cannot see by themselves.
- The service exposes liveness and readiness separately and keeps detailed
  health, metrics, and pprof on the administrative surface. Docker integration
  can exercise Postgres, Redis, migrations, tenancy, API-key authentication,
  idempotency, outbox delivery, webhooks, and admin-route isolation together.

## What Hurt

- The reference service increases the review surface. A realistic application
  brings database migrations, Redis, worker behavior, deployment manifests,
  observability configuration, and generated clients that a small library user
  does not need.
- The non-Docker check is intentionally the default evidence. Docker-backed
  runtime checks require local Docker and are opt-in; object-storage checks
  require the additional MinIO profile. Passing the normal check therefore
  does not claim every runtime integration was exercised.
- Local `replace` directives are useful for repository verification but are not
  a downstream upgrade plan. A real adopter must choose released module
  versions, review generated diffs, and own changes to its product routes,
  authorization model, configuration, and deployment.
- The checked-in load smoke is a local diagnostic, not a cross-machine SLA.
  This repository has no external production load, rollback, backup/restore,
  incident-response, or provider-live evidence to report for the reference
  service.

## What Changed

- The full-profile generator now has a checked-in, separately tested
  application consumer instead of relying only on generated fixture output.
- Repository review can record deterministic application evidence with
  `make reference-service-check` and `make reference-service-evidence`, while
  `make reference-service-load` records bounded in-process load diagnostics.
- The service owns its OpenAPI contract, typed client, Docker Compose,
  Kubernetes, Helm, Terraform, and observability materials so those artifacts
  are reviewed as application artifacts rather than inferred from toolkit
  packages.
- The [reference-service guide](reference-service.md) now acts as the
  canonical evidence and operational reference. It includes the downstream
  adoption-evidence template for recording facts that this maintainer-owned
  example cannot establish.

## Evidence And Limits

| Evidence | Supports | Does not support |
| --- | --- | --- |
| `make reference-service-check` | The checked-in application, OpenAPI/client workflow, observability assets, and deployment assets remain internally consistent. | A claim that an external team deployed or operates the service. |
| `make reference-service-evidence` and the optional Docker integration check | Recorded local runtime evidence for the selected Postgres, Redis, and optional object-storage paths. | Production availability, provider behavior, or backup/restore readiness. |
| `make reference-service-load` | A bounded local router baseline, including expected authentication failure behavior. | An SLO, capacity commitment, or performance result portable to another machine or deployment. |
| A downstream service's own records | Its setup time, upgrade outcome, authorization tests, deployment, load, backup/restore, and incident evidence. | Proof that another service inherits those results merely by using api-toolkit. |

Use the [reference service evidence template](reference-service.md#adoption-evidence-template)
when converting this repository proof into a downstream adoption record. Share
public experience through the [adopter feedback template](../.github/ISSUE_TEMPLATE/adopter_review.md)
without including secrets, customer data, private URLs, or provider credentials.
