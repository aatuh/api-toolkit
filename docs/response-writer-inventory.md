# Response Writer Removal Record

The public `github.com/aatuh/api-toolkit/v4/response_writer` compatibility-only
package was removed from the v3 stable core surface. New response helpers
should use `httpx`, and middleware that needs capture should use package-local
response recorders.

## Current Repository Dependents

No current root or contrib runtime package imports
`github.com/aatuh/api-toolkit/v4/response_writer`.

Cleared imports:

| Path | Previous use | Replacement |
| --- | --- | --- |
| `httpx/recover/recover.go` | Wrapped `http.ResponseWriter` so panic recovery could observe whether a response was already written. | Package-local response recorder in `httpx/recover`. |
| `contrib/middleware/requestlog/requestlog.go` | Captured status and bytes for request logging. | Package-local response recorder in `contrib/middleware/requestlog`. |
| `contrib/middleware/metrics/metrics.go` | Captured status and bytes for metrics labels. | Package-local response recorder in `contrib/middleware/metrics`. |
| `contrib/middleware/oteltrace/oteltrace.go` | Captured status for OpenTelemetry span attributes. | Package-local response recorder in `contrib/middleware/oteltrace`. |

## Guardrail

Keep docscheck guardrails active so examples and public code snippets do not
teach the removed package as the preferred JSON, error, or capture helper.
