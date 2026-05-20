# Observability Runbook

The generated bundle assumes bounded labels only: route, method, code class, dependency, job kind, state, and outcome. tenant IDs are intentionally not metric labels.

## SLO Defaults

- API availability: readiness succeeds and 5xx rate stays below 2%.
- API latency: p95 latency per route remains inside the service-owned objective.
- Async processing: outbox pending jobs drain and dead letters are investigated.
- Webhook delivery: dead letters trigger operator review and replay after the receiver is fixed.

## Operator Actions

- For readiness failures, inspect Postgres and Redis health before restarting workloads.
- For idempotency or rate-limit spikes, verify client retry behavior and route policies.
- For admin endpoint isolation, confirm metrics, pprof, and detailed health are reachable only through the internal admin service or admin listener.
