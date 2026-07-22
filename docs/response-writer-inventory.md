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

## Checked Response-write Policy

`httpx.WriteJSONChecked` and `httpx.WriteProblemChecked` are the repository's
internal response-writing boundary. Runtime packages must use them directly or
through a package-local helper that applies their write-failure policy. The
older void writers remain source-compatible public wrappers for application
code and compatibility examples; they are not for new internal runtime paths.

Each checked write needs an explicit failure policy. A handler can stop its
terminal response path once the connection may be committed. Middleware with an
existing error hook must report write failures through that hook: idempotency
uses `FailurePolicy.OnError`/`Options.OnError`, and rate limiting uses
`Options.OnError`. No package-level response-write logger is used.

`docscheck` rejects direct void-writer calls from production Go files, while a
template-source guard covers generated service handlers as well. This keeps the
compatibility boundary narrow and prevents a failed response write from
accidentally starting a second response.
