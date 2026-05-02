# Examples

Audience: developers looking for runnable api-toolkit examples with the command,
endpoint, expected result, and safety caveat visible before opening source.

Run commands from the `contrib` module unless noted:

```sh
cd contrib
```

| Example | Task | Command | Required environment | Endpoint | Expected result | Safety caveat |
| --- | --- | --- | --- | --- | --- | --- |
| `api-key` | API key authentication with scoped routes. | `go run ./examples/api-key` | Demo key is `demo-admin-key`. | `GET /admin` with `X-API-Key: demo-admin-key`. | `200` JSON principal and scopes; invalid or missing keys return Problem Details. | Store only hashed keys, rotate keys, and bind scopes/tenants in the verifier. |
| `minimal` | Strict JSON write endpoint with validation errors. | `go run ./examples/minimal` | None. | `POST /widgets` | `201` JSON widget for valid input; Problem Details for invalid input. | Keep strict decoding and validation at the API edge. |
| `secure-profile` | Hardened middleware profile with secure headers and tracing. | `go run ./examples/secure-profile` | None. | `GET /hello` | `200` JSON `{"status":"ok"}`. | Cross-origin isolation can break embeds or unmanaged assets. |
| `auth` | Clerk JWT middleware plus explicit admin authorization. | `go run ./examples/auth` | Replace placeholder Clerk issuer, JWKS URL, and audience. | `GET /admin` | `200` JSON role for an authorized JWT; `401` or `403` otherwise. | Placeholder identity-provider values are not production configuration. |
| `policy` | Cedar policy engine authorization. | `go run ./examples/policy` | None; demo policy is embedded. | `GET /docs/{id}` | `200` JSON document id when policy context allows. | Fail startup on invalid or missing production policies. |
| `idempotency` | Idempotent checkout-style write using in-memory storage. | `go run ./examples/idempotency` | None; fake billing provider is local. | `POST /checkout` | `200` checkout session; same key and payload replay the stored response. | Use durable storage and place auth or tenant middleware before idempotency in authenticated APIs. |
| `webhook` | HMAC-verified webhook receiver with size cap. | `go run ./examples/webhook` | Demo secret is `demo-secret`. | `POST /webhooks/payment` | `202` JSON acknowledgement for a valid signature. | Store real webhook secrets securely and add replay protection in application code. |
| `file-upload` | Multipart upload with request size limits. | `go run ./examples/file-upload` | Local file for upload testing. | `POST /upload` | `200` JSON file metadata. | Scan and store untrusted uploads outside the hot request path. |
| `pagination` | Limit and offset pagination with field-level errors. | `go run ./examples/pagination` | None. | `GET /items?limit=3&offset=2` | `200` JSON items and optional `next_offset`; invalid limits return validation Problem Details. | Keep maximum limits aligned with backing-store capacity. |
| `spec-first` | OpenAPI source, generated handlers, and response validation. | `go generate ./examples/spec-first && go run ./examples/spec-first` | Run generation after editing `openapi.json`. | `GET /pets`, `POST /pets` | JSON pets for valid requests; Problem Details for validation or conflict errors. | Treat `openapi.json` as the source of truth and review `spec_gen.go` changes. |
| `outbound` | SSRF-guarded outbound client with retries, breaker, and bulkhead. | `go run ./examples/outbound` | Replace `api.example.com` for a real upstream. | No local endpoint; program calls `GET https://api.example.com/health`. | Successful outbound response body is closed, or the program logs the guarded failure. | Do not widen host, port, or retry policies without replay and SSRF review. |
