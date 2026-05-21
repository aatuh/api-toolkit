# Kubernetes Starter

These manifests are direct Kubernetes starters for the generated API server,
worker, migration Job, public Service, internal admin Service, NetworkPolicy,
HPA, PDB, ConfigMap, and Secret placeholder.

## Prerequisites

- A target namespace and image pull access for the generated application image.
- Postgres and Redis endpoints reachable from the namespace.
- A Secret created from `secret.example.yaml` with placeholder values replaced
  outside Git.

## Required Configuration

| File | Required review |
| --- | --- |
| `configmap.yaml` | Public/admin bind addresses, Redis-backed store modes, validation flags, worker enablement, and object-store mode. |
| `secret.example.yaml` | `database-url`, `redis-addr`, `api-key`, `admin-key`, `api-key-pepper`, and `webhook-secret-key` placeholders. |
| `deployment.yaml` | API image, read-only filesystem, probes, secret references, and `ASYNC_WORKER_ENABLED=false` when workers run separately. |
| `worker-deployment.yaml` | Dedicated worker image, security context, and dependency secrets. |
| `migration-job.yaml` | Migration image and command to run before API/worker rollout. |
| `network-policy.yaml` | Public port, admin port, DNS, Postgres, Redis, and external HTTPS egress policy. |

## Validate

```sh
kubectl apply --dry-run=server -f deploy/kubernetes/
kubectl rollout status deployment/api
kubectl rollout status deployment/api-worker
kubectl get job api-migrate
```

Use a client inside the cluster or a controlled port-forward to verify `/livez`
and `/readyz`. Keep admin checks on the internal admin Service.

## Admin Isolation

`admin-service.yaml` is ClusterIP and annotated internal-only. Restrict access
to operator namespaces; do not route the admin port through the public Service
or ingress.

## Non-goals

These manifests do not install Postgres, Redis, S3-compatible storage, ingress,
certificates, external DNS, or secret-manager integrations.
