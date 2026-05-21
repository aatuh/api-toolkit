# Helm Starter

This chart packages the generated API server, worker, migration Job, admin
Service, probes, resources, NetworkPolicy, HPA, PDB, and secret/config
references. The admin Service is internal-only by default.

## Prerequisites

- Helm 3 and access to the target Kubernetes cluster.
- A built application image pushed to a registry the cluster can pull from.
- A Secret named by `secretName` that contains the generated service
  environment keys from `.env.example`.

## Required Values

| Value | Purpose |
| --- | --- |
| `image.repository` and `image.tag` | API, worker, and migration image. |
| `secretName` | Existing Secret consumed through `envFrom`. |
| `api.replicas` and `worker.replicas` | API and worker replica counts. |
| `adminService.enabled` and `adminService.port` | Internal admin listener exposure. |
| `migration.enabled` | Whether the migration Job is installed with the chart. |
| `resources` and `autoscaling` | Starter requests, limits, and HPA bounds. |

## Required Secrets

At minimum, provide `DATABASE_URL`, `REDIS_ADDR` when Redis-backed stores are
enabled, `API_KEY`, `ADMIN_KEY`, `API_KEY_PEPPER`, and `WEBHOOK_SECRET_KEY`.
Add S3 and provider secrets only when those generated features are enabled. Use
placeholders in examples; never commit live secret values.

## Validate

```sh
helm lint deploy/helm
helm template api-toolkit deploy/helm --values deploy/helm/values.yaml >/tmp/api-toolkit-rendered.yaml
kubectl apply --dry-run=server -f /tmp/api-toolkit-rendered.yaml
```

## Admin Isolation

Keep the admin Service reachable only from operator namespaces or private
networking. Do not expose `/health/detailed`, `/metrics`, or `/debug/pprof/`
through the public ingress.

## Non-goals

The chart is a starter, not a hosted platform. It does not choose an ingress
controller, certificate issuer, external secret manager, cluster autoscaler, or
cloud account layout.
