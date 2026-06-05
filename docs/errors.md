# Error Taxonomy

Audience: API consumers and maintainers who need predictable failure handling.

`api-toolkit` uses three error layers:

1. Go errors for package and adapter callers.
2. Field errors for validation failures.
3. RFC 9457 Problem Details for HTTP responses.

## Error Kinds

| Kind | Examples | Matching rule | HTTP mapping |
| --- | --- | --- | --- |
| Sentinel errors | `httpx.ErrUnauthorized`, `httpx.ErrForbidden`, `ports.ErrResourceMissing`, `operations.ErrInvalidTransition` | Use `errors.Is`. | Map through `httpx.ProblemFromError`, package-specific writers, or app-owned handlers. |
| Typed errors | `httpx.HTTPError`, `httpx.ProblemError`, `routecontracts.CoverageError` | Use `errors.As`. | Preserve status, code, and safe detail when exposed. |
| Field errors | `fielderrors.FieldErrors` and providers implementing `fielderrors.Provider` | Use provider extraction or package helpers. | Map to validation Problem Details with field/code/message entries. |
| Wrapped operational errors | `fmt.Errorf("lookup widget: %w", err)` | Match wrapped cause with `errors.Is` or `errors.As`. | Expose generic Problem Details unless the wrapped cause is safe. |
| Configuration errors | invalid `Options`, missing stores, invalid secrets, bad limits | Treat as startup or construction failures. | Do not expose raw config values to clients. |

## Problem Details Rules

- Use `httpx.WriteProblem` or package-specific writers for HTTP errors.
- Keep client-facing detail stable and safe.
- Put field validation details under field errors, not string-concatenated
  messages.
- Do not expose secrets, provider payloads, DSNs, SQL, token values, webhook
  signing material, or raw upstream error bodies.
- Keep metrics and logs low-cardinality; log full causes only where operator
  controls and redaction policy allow it.

## Matching Examples

```go
if errors.Is(err, httpx.ErrForbidden) {
	// return or write a 403 Problem Details response
}

var problem *httpx.ProblemError
if errors.As(err, &problem) {
	// inspect problem.StatusCode and problem.Problem
}
```

## Package Mapping

| Package family | Preferred mapping |
| --- | --- |
| `binding`, `upload`, `queryparams` | Field errors to validation Problem Details. |
| `httpx` | Sentinel and typed errors to Problem Details catalog entries. |
| `middleware/auth/*` | Authentication failures to 401, authorization failures to 403, no raw verifier errors. |
| `middleware/idempotency` | Missing keys to 400, conflicts/replays to conflict-oriented Problem Details, store failures to generic 5xx/503. |
| `webhooks` | Signature failures to 401, malformed payloads to 400, verifier internals to generic 5xx. |
| `operations` | Missing operation to 404, invalid transitions as Go errors for callers. |
