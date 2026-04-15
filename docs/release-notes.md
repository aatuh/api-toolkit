# Release Notes

## 2026-04-15

- Idempotency middleware now releases failed reservations after `5xx` responses, panics, and completed-response persistence failures, so retries with the same payload and `Idempotency-Key` are not blocked behind a stale in-flight record.
- `scheduler.Runner` now prevents the same job name from overlapping with itself across duplicate `Start` calls or duplicate scheduling of the same job.
- Docs endpoints now return `404` when the HTML docs surface is disabled or when no authoritative OpenAPI document is available.
- `DocsConfig.EnableJSON` and `DocsConfig.EnableYAML` now control which discovered OpenAPI formats may be served on the configured docs path.
