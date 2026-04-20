# Metrics Naming and Labels

Keep HTTP metrics predictable and low-cardinality so dashboards stay usable
and storage costs remain bounded.

## Standard HTTP metrics

The contrib metrics middleware emits:

- `http_requests_total` (counter)
- `http_request_duration_seconds` (histogram)

Required labels:

- `method` (uppercase HTTP method)
- `route` (router pattern, not raw path)
- `status` (HTTP status code as string)

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
missing routes to `unknown`. This keeps cardinality stable by design.

## Bootstrap defaults

The contrib bootstrap helpers keep metrics opt-in:

- `bootstrap.ProfileStrictAPI` and `bootstrap.ProfileDev` use a No-op metrics
  recorder unless you pass `bootstrap.WithMetricsRecorder(...)`.
- `bootstrap.MountSystemEndpoints` only mounts `specs.Metrics` when you set
  `SystemEndpoints.Metrics` explicitly.
- `bootstrap.PrometheusMetricsHandler()` is the convenience helper for the
  standard Prometheus HTTP handler.

Example:

```go
bootstrap.MountSystemEndpoints(r, bootstrap.SystemEndpoints{
	Metrics: bootstrap.PrometheusMetricsHandler(),
})
```

## Custom metrics

If you add custom metrics, keep label sets small and bounded. If you need
higher-cardinality data, emit it to structured logs or tracing instead of
metrics.
