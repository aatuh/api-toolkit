# Examples

- `examples/minimal`: simple JSON API with validation errors
- `examples/secure-profile`: hardened middleware profile with metrics + tracing
- `examples/auth`: Clerk JWT auth with an authorizer check
- `examples/policy`: Cedar policy engine with context-based decisions
- `examples/idempotency`: idempotent checkout endpoint using in-memory store
  - For authenticated APIs, apply auth and tenant middleware before idempotency so the default request hash includes caller scope.
- `examples/webhook`: webhook receiver with signature verification
- `examples/file-upload`: multipart file upload with size limits
- `examples/pagination`: limit/offset pagination with query limits and field-level validation errors
- `examples/spec-first`: OpenAPI-driven handler skeletons with response validation
- `examples/outbound`: SSRF-guarded outbound client with retries and breakers
