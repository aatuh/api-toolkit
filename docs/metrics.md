# Metrics Naming and Labels

Audience: operators and developers defining HTTP metrics, labels, dashboards,
and custom recorder behavior.

Keep HTTP metrics predictable and low-cardinality so dashboards stay usable
and storage costs remain bounded.

## Standard HTTP metrics

The contrib metrics middleware emits:

- `http_requests_total` (counter)
- `http_request_duration_seconds` (histogram)

Required labels:

- `method` (uppercase standard HTTP method, `OTHER` for non-standard methods,
  or `UNKNOWN` when missing)
- `route` (router pattern, not raw path)
- `status` (HTTP status code as string, or `0` when missing or invalid)

## Label policy

Do:

- Use route patterns like `/v1/users/{id}` instead of raw paths.
- Keep labels to small, bounded sets.
- Prefer stable enums (method, status, route) over user input.
- Use fallbacks like `unknown` or `0` when data is missing.

Avoid:

- Raw paths, query strings, or request bodies.
- User IDs, tenant IDs, API keys, or IP addresses.
- Request IDs, trace IDs, or span IDs.
- User-Agent, Referer, or error messages.

## Middleware behavior

The metrics middleware derives `route` from the router pattern and defaults
missing routes to `unknown`. The Prometheus recorder canonicalizes HTTP methods
and status codes before writing series. This keeps cardinality stable by design.

## Bootstrap defaults

The contrib bootstrap helpers keep metrics opt-in:

- `bootstrap.ProfileStrictAPI` and `bootstrap.ProfileDev` use a No-op metrics
  recorder unless you pass `bootstrap.WithMetricsRecorder(...)`.
- `bootstrap.MountSystemEndpoints` preserves v2 convenience behavior and only
  mounts `specs.Metrics` when you set `SystemEndpoints.Metrics` explicitly.
- Prefer `bootstrap.MountSystemEndpointsToWithAdmin` for new system endpoint
  wiring so metrics are mounted behind an explicit admin or internal-network
  wrapper.
- `bootstrap.PrometheusMetricsHandler()` is the convenience helper for the
  standard Prometheus HTTP handler.

Example:

```go
err := bootstrap.MountSystemEndpointsToWithAdmin(router, bootstrap.SystemEndpoints{
	Metrics: bootstrap.PrometheusMetricsHandler(),
}, bootstrap.SystemEndpointAdminOptions{
	RequireAdmin: requireAdmin,
})
if err != nil {
	return err
}
```

## Custom metrics

If you add custom metrics, keep label sets small and bounded. If you need
higher-cardinality data, emit it to structured logs or tracing instead of
metrics.

## Idempotency compatibility telemetry

Normal idempotency request outcomes can be observed with
`middleware/idempotency.Options.OnOutcome`. `OutcomeEvent.MetricLabels()` exposes
only:

- `method`: standard HTTP method or `OTHER`
- `store_class`: `memory`, `redis`, `sql`, `custom`, or `unknown`
- `outcome`: a stable idempotency outcome enum such as `completed_stored`,
  `completed_released`, `replayed`, `conflict`, `in_flight`, `ambiguous`,
  `fail_open`, or `persistence_failed`
- `status_class`: `1xx`, `2xx`, `3xx`, `4xx`, `5xx`, or `none`

Outcome events intentionally omit raw paths, query strings, request IDs, tenant
IDs, idempotency keys, bodies, and error strings.

The idempotency mixed-version compatibility metric label contract is bounded:

- `method`: standard HTTP method or `OTHER`
- `store_class`: `memory`, `redis`, `sql`, `custom`, or `unknown`
- `outcome`: `legacy_in_flight_fallback_entered`,
  `legacy_in_flight_fallback_recovered`,
  `legacy_in_flight_fallback_rejected`,
  `legacy_in_flight_fallback_unknown`, or `unknown`

Do not add raw paths, query strings, request IDs, tenant IDs, idempotency keys,
key hashes, or error strings to metric labels. `LegacyInFlightCompatibilityEvent`
keeps `Path`, `Key`, `StoreType`, and `Error` for structured logs or traces where
operators can sample, redact, and restrict access.

`LegacyInFlightCompatibilityRawKey` remains disabled by default. Enable raw-key
output only for short, access-controlled incident review, and prefer hashed keys
or redacted values for normal operations.

Adapter-level legacy idempotency recovery events follow the same privacy
posture: memory and Redis adapter events hash the `Key` field by default,
populate `KeyHash` for correlation, and leave `RawKey` empty unless the
adapter-specific raw-key opt-in is explicitly enabled for incident review.

Compatibility telemetry delivery is best-effort. Async mode uses a bounded queue
and worker pool; when the queue is full, events are dropped and a warning records
the cumulative drop count and queue size.
