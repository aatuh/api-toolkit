# Release Notes

## 2026-04-15

- Idempotency middleware now releases failed reservations after `5xx` responses, panics, and completed-response persistence failures, so retries with the same payload and `Idempotency-Key` are not blocked behind a stale in-flight record.
- `scheduler.Runner` now prevents the same job name from overlapping with itself across duplicate `Start` calls or duplicate scheduling of the same job.
- Docs endpoints now return `404` when the HTML docs surface is disabled or when no authoritative OpenAPI document is available.
- `DocsConfig.EnableJSON` and `DocsConfig.EnableYAML` now control which discovered OpenAPI formats may be served on the configured docs path.
- Multi-source migrator loading now documents its actual contract: duplicate version+direction pairs are rejected.
- The pagination example now returns one field-level validation shape for invalid `limit` inputs even when `querylimits` rejects the request before the handler.

### Upgrade notes

- If clients previously saw `409 Conflict` after a failed idempotent write, retry behavior has changed: the same payload and `Idempotency-Key` can now be retried immediately after `5xx`, panic, or response-persistence failure paths.
- If application code called `scheduler.Runner.Start` more than once or reused the same job name across duplicate schedules, those executions no longer overlap. Validate any workload that previously relied on concurrent execution of the same named job.
- Docs handlers no longer return a synthetic OpenAPI document when no authoritative spec exists. Expect `404` for disabled docs surfaces and for missing OpenAPI files unless a real document is configured.
- `DocsConfig.EnableJSON` and `DocsConfig.EnableYAML` now control which discovered OpenAPI formats can be served. Verify custom docs paths and any YAML-based docs setup during upgrade.
