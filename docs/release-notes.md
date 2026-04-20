# Release Notes

## 2026-04-19

- `contrib/middleware/auth/devheaders` now requires explicit dangerous-bypass opt-in and trusted-proxy configuration before it will honor debug auth headers.
- Health endpoints now fail closed on empty or miswired liveness/readiness probe sets, and HTTP handlers only expose detailed dependency output when `ports.HealthCheckConfig.EnableDetailed` is explicitly enabled.
- `contrib/adapters/txpostgres.WithinTx` now attempts deferred rollback with a bounded cleanup context even when the caller context is already canceled or timed out.
- `contrib/adapters/txpostgres` now fails closed with `ErrPoolNotConfigured` when callers forget to wire a database pool, instead of panicking on nil-pool use.
- `contrib/migrator` now records commit-acknowledgement failures as `uncertain` and blocks later runs when a prior migration record is still `started` or `uncertain`.
- `scheduler.Runner` now persists final run records through a bounded cleanup context so graceful shutdown does not drop `LastFinished` updates for jobs that already completed.
- `scheduler.Runner` now surfaces recorder persistence failures through structured logs and optional `SetRecorderFailureHandler` callbacks without changing the completed job result or schedule cadence.
- JWT and Clerk middleware now share internal auth/JWKS validation primitives with no intended public API or configuration change.

### Upgrade notes

- If you previously enabled `devheaders` without explicitly opting into dangerous bypasses or without trusted-proxy configuration, startup will now fail fast until you set both intentionally.
- If you had tests or thin wiring paths that called `txpostgres.New(nil)` or `txpostgres.FromCtx(..., nil)`, they now return `ErrPoolNotConfigured` instead of panicking.
- If a deployment previously canceled scheduler job contexts during graceful shutdown, completed jobs now get a short recorder-persistence window before exit so restart-time suppression remains accurate.
- If operators previously relied on `/health` or equivalent routes exposing dependency-level detail by default, set `EnableDetailed` explicitly during wiring; otherwise only basic probes should remain visible.
- If your deployment workflow retried migrations automatically after commit errors, stop doing that. Inspect the database state and reconcile `schema_migrations` before rerunning when a migration is recorded as `started` or `uncertain`.
- If you need alerting when scheduler run history cannot be persisted, wire `SetRecorderFailureHandler` or monitor the new recorder-failure log events; job completion alone no longer implies recorder persistence succeeded.
- JWT and Clerk integrations should be behaviorally equivalent to their prior public APIs, but custom wrappers that depended on edge-case differences in bearer parsing, claim requirements, or skip-header handling should be revalidated.

## 2026-04-15

- Idempotency middleware now releases failed reservations after downstream `5xx` responses and panics, so retries with the same payload and `Idempotency-Key` are not blocked behind a stale in-flight record.
- Idempotency middleware now fails closed with `503 Service Unavailable` when it cannot persist a completed replay record, and it stores an ambiguous state for that key instead of reopening it for another execution.
- Idempotency middleware now includes authenticated actor and tenant scope in the default request hash, preventing cross-principal or cross-tenant replays from reusing the same key and payload.
- Idempotency middleware now caps buffered replay bodies at `1 MiB` by default and returns `503 Service Unavailable` plus an ambiguous key state when a handled response exceeds the replay buffer limit.
- `scheduler.Runner` now recovers scheduled-job panics, logs and records them as failed runs, and keeps future intervals alive instead of letting one bad job crash the process.
- `scheduler.Runner` now prevents the same job name from overlapping with itself across duplicate `Start` calls or duplicate scheduling of the same job.
- `bootstrap.ProfileStrictAPI` no longer enables wildcard CORS by default; browser-facing cross-origin access now requires an explicit `WithCORSOptions(...)` allowlist.
- `contrib/config.LoadFromEnv` now treats invalid present bool and int values as startup errors instead of silently falling back to defaults.
- Docs endpoints now return `404` when the HTML docs surface is disabled or when no authoritative OpenAPI document is available.
- `DocsConfig.EnableJSON` and `DocsConfig.EnableYAML` now control which discovered OpenAPI formats may be served on the configured docs path.
- Multi-source migrator loading now documents its actual contract: duplicate version+direction pairs are rejected.
- The pagination example now returns one field-level validation shape for invalid `limit` inputs even when `querylimits` rejects the request before the handler.

### Upgrade notes

- If clients previously saw `409 Conflict` after a failed idempotent write, retry behavior has changed: the same payload and `Idempotency-Key` can now be retried immediately after downstream `5xx` and panic paths, but not after completed-response persistence failures or replay-buffer overflows.
- If clients previously received the original success response even though completion persistence failed, they now receive `503 Service Unavailable` and the key remains blocked in an ambiguous state until it expires or is reconciled.
- If authenticated middleware previously ran after idempotency, default caller scoping will not apply. Move auth and tenant middleware earlier in the stack to keep replay protection scoped per caller.
- If a route can stream, hijack, upgrade, or return large bodies, exclude it with `ShouldHandle` or raise `MaxResponseBytes`; otherwise oversized handled responses now fail closed with `503 Service Unavailable` and block same-key retries for the key lifetime.
- If a scheduled job panic previously terminated the process, that failure is now contained and surfaced through scheduler logging and run recording instead.
- If application code called `scheduler.Runner.Start` more than once or reused the same job name across duplicate schedules, those executions no longer overlap. Validate any workload that previously relied on concurrent execution of the same named job.
- If browser clients previously relied on `ProfileStrictAPI` to emit `Access-Control-Allow-Origin: *`, they must now set an explicit allowlist with `WithCORSOptions(...)` during bootstrap.
- If deployment environments previously contained malformed bool or int values such as `MIGRATE_ON_START=maybe`, startup now fails fast instead of silently using defaults. Validate env files and secrets before rollout.
- Docs handlers no longer return a synthetic OpenAPI document when no authoritative spec exists. Expect `404` for disabled docs surfaces and for missing OpenAPI files unless a real document is configured.
- `DocsConfig.EnableJSON` and `DocsConfig.EnableYAML` now control which discovered OpenAPI formats can be served. Verify custom docs paths and any YAML-based docs setup during upgrade.
