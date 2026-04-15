# Spec-first example

This example shows a minimal spec-first workflow:

- Define the OpenAPI source of truth in `openapi.json`.
- Generate handler skeletons + error types.
- Implement handlers and wire middleware.

Generate the skeletons:

```bash
go generate ./...
```

Files of interest:

- `openapi.json`: source of truth.
- `spec_gen.go`: generated routes/constants/handlers/errors.
- `main.go`: wiring with OpenAPI request + response validation.
- `handlers.go`: handler implementations.

Notes:

- The generator emits `StatusError` types for non-2xx responses, including optional field errors for validation cases.
- Error responses are documented and returned as `application/problem+json` Problem Details payloads.
- Response validation is enabled to catch mismatched handler output in dev/test.
