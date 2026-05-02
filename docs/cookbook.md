# Cookbook

Audience: developers applying api-toolkit to common HTTP API tasks. Full source
for each recipe lives under `contrib/examples`.

Run examples from the contrib module unless a recipe says otherwise:

```sh
cd contrib
go run ./examples/<name>
```

## Minimal JSON write endpoint

Purpose: accept JSON, validate fields, and return Problem Details for bad input.

- Prerequisites: none.
- Example source: `contrib/examples/minimal`.
- Run command: `cd contrib && go run ./examples/minimal`.
- Request:

```sh
curl -s -X POST http://localhost:8080/widgets \
  -H 'Content-Type: application/json' \
  -d '{"name":"starter","quantity":1}'
```

- Expected response shape: `201` JSON with `id`, `name`, and `quantity`.
- Production caveat: keep strict JSON decoding and field-level errors at the edge so downstream services do not need to guess validation shape.

## Hardened middleware profile

Purpose: apply the strict API profile with secure headers, timeout, tracing, and explicit system endpoint choices.

- Prerequisites: none.
- Example source: `contrib/examples/secure-profile`.
- Run command: `cd contrib && go run ./examples/secure-profile`.
- Request:

```sh
curl -i http://localhost:8080/hello
```

- Expected response shape: `200` JSON `{"status":"ok"}` plus security headers.
- Production caveat: cross-origin isolation can break embeds or CDN-backed assets; enable it only when you control embedded resources.

## Authenticated admin route

Purpose: wire Clerk JWT middleware and an explicit authorizer check.

- Prerequisites: replace the placeholder Clerk issuer, JWKS URL, and audience with a real tenant before using the route successfully.
- Example source: `contrib/examples/auth`.
- Run command: `cd contrib && go run ./examples/auth`.
- Request:

```sh
curl -i http://localhost:8080/admin \
  -H 'Authorization: Bearer <jwt>'
```

- Expected response shape: `200` JSON `{"role":"admin"}` for an allowed subject, otherwise Problem Details `401` or `403`.
- Production caveat: do not keep placeholder identity-provider URLs in deployed configuration.

## Policy-engine authorization

Purpose: authorize a route through a Cedar policy engine with request context.

- Prerequisites: none; the example embeds a demo policy.
- Example source: `contrib/examples/policy`.
- Run command: `cd contrib && go run ./examples/policy`.
- Request:

```sh
curl -s http://localhost:8080/docs/doc_123
```

- Expected response shape: `200` JSON with `id` when the policy allows the request.
- Production caveat: load policies through your deployment process and fail startup if policy parsing fails.

## Idempotent write endpoint

Purpose: use the idempotency middleware to make safe retries for POST, PUT, or PATCH operations.

- Prerequisites: none; the example uses an in-memory store and a fake billing provider.
- Example source: `contrib/examples/idempotency`.
- Run command: `cd contrib && go run ./examples/idempotency`.
- Request:

```sh
curl -s -X POST http://localhost:8080/checkout \
  -H 'Idempotency-Key: key-1' \
  -H 'Content-Type: application/json' \
  -d '{"amount":500,"currency":"eur"}'
```

- Expected response shape: `200` JSON checkout session with `id` and `url`; repeat the same request and key to replay the stored response.
- Production caveat: use a durable store such as Redis, apply auth and tenant middleware before idempotency, and exclude streaming or large-download routes with `ShouldHandle`.

## Webhook receiver with signature verification

Purpose: verify HMAC signatures, cap body size, parse the event, and return an acknowledgement.

- Prerequisites: use the demo secret only for local testing.
- Example source: `contrib/examples/webhook`.
- Run command: `cd contrib && go run ./examples/webhook`.
- Request:

```sh
body='{"id":"evt_123","type":"payment.succeeded"}'
sig=$(printf '%s' "$body" | openssl dgst -sha256 -hmac 'demo-secret' -binary | xxd -p -c 256)
curl -s -X POST http://localhost:8080/webhooks/payment \
  -H "X-Signature: $sig" \
  -H 'Content-Type: application/json' \
  -d "$body"
```

- Expected response shape: `202` JSON with `status` and `event_id`.
- Production caveat: keep provider secrets in secret storage and reject unsigned or oversized payloads before JSON parsing.

## File upload endpoint

Purpose: cap request size and stream multipart upload metadata without buffering the full file in memory.

- Prerequisites: a local file to upload.
- Example source: `contrib/examples/file-upload`.
- Run command: `cd contrib && go run ./examples/file-upload`.
- Request:

```sh
printf 'hello\n' > /tmp/api-toolkit-upload.txt
curl -s -X POST http://localhost:8080/upload \
  -F 'file=@/tmp/api-toolkit-upload.txt;type=text/plain'
```

- Expected response shape: `200` JSON with `filename`, `content_type`, and `bytes`.
- Production caveat: store uploads outside the request path when possible and scan untrusted content before later use.

## Pagination with limit and offset

Purpose: bound list queries and return field-level validation errors for invalid limits.

- Prerequisites: none.
- Example source: `contrib/examples/pagination`.
- Run command: `cd contrib && go run ./examples/pagination`.
- Request:

```sh
curl -s 'http://localhost:8080/items?limit=3&offset=2'
```

- Expected response shape: `200` JSON with `items` and optional `next_offset`.
- Production caveat: keep maximum limits low enough for the backing store and prefer checked parser APIs when validation errors matter.

## Spec-first handlers

Purpose: generate handler skeletons from OpenAPI and validate requests and responses.

- Prerequisites: run generation from the example directory before changing generated code.
- Example source: `contrib/examples/spec-first`.
- Run command: `cd contrib/examples/spec-first && go generate ./... && go run .`.
- Request:

```sh
curl -s -X POST http://localhost:8080/pets \
  -H 'Content-Type: application/json' \
  -d '{"name":"Milo"}'
```

- Expected response shape: `201` JSON pet with `id` and `name`; invalid names return `application/problem+json` with a `validation.fields` entry.
- Production caveat: treat `openapi.json` as the source of truth and review generated-file changes with the implementation change that caused them.

## Guarded outbound HTTP client

Purpose: show SSRF host allowlisting, retry budget controls, circuit breaking, and bulkhead limits.

- Prerequisites: replace `api.example.com` with an allowed real upstream before expecting a successful request.
- Example source: `contrib/examples/outbound`.
- Run command: `cd contrib && go run ./examples/outbound`.
- Request example: the program issues a guarded `GET https://api.example.com/health`.
- Expected response shape: no local HTTP endpoint; success means the outbound call returns and the response body closes.
- Production caveat: do not broaden `AllowedHosts`, `AllowedPorts`, or retryable methods without reviewing replay and SSRF risk.
