# Response Writer Compatibility Inventory

`github.com/aatuh/api-toolkit/v2/response_writer` is retained for v2 source
compatibility only. New response helpers should use `httpx` or package-local
capture code instead of teaching this legacy package as the preferred path.

## Current repository dependents

| Path | Import class | Current use | V3 replacement path |
| --- | --- | --- | --- |
| `httpx/recover/recover.go` | Root compatibility bridge | Wraps `http.ResponseWriter` so panic recovery can observe whether a response was already written. | Move the small status-capture wrapper into `httpx/recover` or an internal `httpx` capture helper before retiring the public legacy package. |
| `contrib/middleware/requestlog/requestlog.go` | Contrib compatibility-only middleware implementation | Captures status and bytes for request logging. | Use a contrib-local capture wrapper or a future `httpx` capture helper that is not exposed as the legacy stable package. |
| `contrib/middleware/metrics/metrics.go` | Contrib compatibility-only middleware implementation | Captures status and bytes for metrics labels. | Use a contrib-local capture wrapper or a future `httpx` capture helper. |
| `contrib/middleware/oteltrace/oteltrace.go` | Contrib compatibility-only middleware implementation | Captures status for OpenTelemetry span attributes. | Use a contrib-local capture wrapper or a future `httpx` capture helper. |

The `response_writer` package files and tests remain the compatibility surface
itself. They are allowed to document the legacy API, but examples and new
guidance should not import the package as the preferred JSON, error, or capture
helper.

## V3 treatment

- Keep the package source-compatible for v2 callers.
- Keep `middleware/idempotency` on its package-local response capture helper.
- Replace root and contrib imports with package-local or `httpx` capture
  helpers before removing the public legacy package in v3.
- Keep docscheck guardrails active so new examples and public code snippets do
  not teach `response_writer` as the preferred helper.
