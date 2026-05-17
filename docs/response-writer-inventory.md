# Response Writer Compatibility Inventory

`github.com/aatuh/api-toolkit/v3/response_writer` is retained for v2 source
compatibility only. New response helpers should use `httpx` or package-local
capture code instead of teaching this legacy package as the preferred path.

## Current repository dependents

No current root or contrib runtime package imports
`github.com/aatuh/api-toolkit/v3/response_writer`. The public package remains
in place as a compatibility-only v2 surface for external callers.

Cleared v39 imports:

| Path | Previous use | Replacement |
| --- | --- | --- |
| `httpx/recover/recover.go` | Wrapped `http.ResponseWriter` so panic recovery could observe whether a response was already written. | Package-local response recorder in `httpx/recover`. |
| `contrib/middleware/requestlog/requestlog.go` | Captured status and bytes for request logging. | Package-local response recorder in `contrib/middleware/requestlog`. |
| `contrib/middleware/metrics/metrics.go` | Captured status and bytes for metrics labels. | Package-local response recorder in `contrib/middleware/metrics`. |
| `contrib/middleware/oteltrace/oteltrace.go` | Captured status for OpenTelemetry span attributes. | Package-local response recorder in `contrib/middleware/oteltrace`. |

The `response_writer` package files and tests remain the compatibility surface
itself. They are allowed to document the legacy API, but examples and new
guidance should not import the package as the preferred JSON, error, or capture
helper.

## V3 treatment

- Keep the package source-compatible for v2 callers.
- Keep `middleware/idempotency` on its package-local response capture helper.
- Keep root and contrib runtime imports on package-local or `httpx` capture
  helpers before removing the public legacy package in v3.
- Keep docscheck guardrails active so new examples and public code snippets do
  not teach `response_writer` as the preferred helper.
