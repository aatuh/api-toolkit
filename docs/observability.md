# Observability Guide

Audience: operators and developers wiring metrics, logs, traces, request IDs,
and dashboards for api-toolkit services.

Observability data should be useful for operations without becoming a side
channel for secrets, tenant data, provider payloads, or unbounded user input.

## Metrics

Use the naming and label policy in `docs/metrics.md`.

Required properties:

- route labels come from route patterns, not raw paths,
- status labels are bounded status codes or status classes,
- idempotency, route-policy, hard-timeout, health, and webhook labels use stable
  outcome enums,
- tenant IDs, user IDs, API keys, admin keys, idempotency keys, request bodies,
  object keys, provider payloads, and raw errors never become labels.

## Logs

Structured logs may include request ID, bounded route pattern, method, status
class, stable outcome enum, dependency class, and sanitized operation ID.

Do not log:

- raw authorization headers,
- API keys or admin keys,
- session IDs or CSRF tokens,
- idempotency keys,
- webhook signing secrets,
- raw provider payloads,
- request or response bodies,
- tenant-controlled object keys,
- database URLs or Terraform state.

## Traces

Tracing is disabled by default in generated services. When
`OTEL_TRACING_ENABLED=true`, `OTEL_EXPORTER_OTLP_ENDPOINT` is required and must
come from trusted operator configuration.

Keep span attributes low-cardinality:

- route pattern,
- method,
- status class,
- dependency class,
- bounded outcome enum.

Do not put tenant IDs, user IDs, emails, API keys, provider IDs, object keys,
request bodies, or raw errors in span attributes.

## Correlation IDs

Use request IDs or trace IDs for correlation, but keep them out of metric
labels. Logs and traces can carry correlation IDs when access is controlled and
retention is understood.

## Dashboards and Alerts

Dashboards should focus on:

- HTTP request rate, latency, and error class by route pattern,
- readiness and health-state transitions,
- idempotency replay, conflict, in-flight, and persistence outcomes,
- rate-limit rejection rate,
- hard-timeout timeout, panic, and capture-overflow outcomes,
- webhook delivery success, retry, failure, and dead-letter outcomes,
- worker backlog and job outcome classes,
- admin route isolation checks.

Alert labels should stay bounded. Put high-cardinality context in logs or
incident notes after access review, not in metric labels.
