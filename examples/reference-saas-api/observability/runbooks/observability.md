# Observability Runbook

The generated bundle assumes bounded labels only: route, method, code class,
dependency, job kind, state, and outcome. tenant IDs are intentionally not
metric labels.

## SLO Defaults

- API availability: readiness succeeds and 5xx rate stays below 2%.
- API latency: p95 latency per route remains inside the service-owned objective.
- Async processing: outbox pending jobs drain and dead letters are investigated.
- Webhook delivery: dead letters trigger operator review and replay after the receiver is fixed.

## Verification

Run deterministic asset checks before editing dashboards or rules:

```sh
make observability-check
make asset-check
```

After deployment, confirm:

- The public API exports bounded HTTP metrics by route, method, and code class.
- `/metrics`, `/health/detailed`, and `/debug/pprof/` are reachable only through
  the admin listener or internal admin Service.
- Alert rules load successfully in the target Prometheus-compatible system.
- The dashboard imports without adding tenant, user, email, API-key,
  idempotency-key, or raw path labels.

## Triage

- For readiness failures, inspect Postgres and Redis health before restarting workloads.
- For idempotency or rate-limit spikes, verify client retry behavior and route policies.
- For admin endpoint isolation, confirm metrics, pprof, and detailed health are reachable only through the internal admin service or admin listener.
- For outbox backlog or webhook dead letters, inspect worker health, receiver errors, retry classification, and replay safety before manual replay.
- For high 5xx rates, compare route, code class, dependency health, recent deploys, and migration status. Keep user-specific values out of labels and incident notes unless there is an explicit support need.

## Evidence To Record

| Evidence | Record |
| --- | --- |
| Asset validation | `make observability-check` or `make asset-check` result. |
| Dashboard review | Dashboard version, data source, imported panel count, and confirmation that labels stay bounded. |
| Alert review | Rule file version, loaded rule group names, and firing/resolved state. |
| Admin isolation | Network path used to reach admin metrics, detailed health, and pprof. |
| Incident | Time window, route/code-class/dependency labels, action taken, and whether any secret or personal data was deliberately excluded. |

## Privacy And Secret Handling

Do not add tenant IDs, user IDs, emails, API keys, admin keys, webhook secrets,
idempotency keys, raw URLs, request bodies, provider payloads, or object keys to
metric labels, traces, dashboard variables, alert annotations, or screenshots
used as release evidence.
