# Cookbook

Runnable examples live under `contrib/examples`. From the repo root:

```sh
cd contrib
go run ./examples/<name>
```

## Idempotent write endpoint

Use the idempotency middleware to guarantee safe retries for POST/PUT/PATCH.

- Successful terminal responses are replayed for the same `Idempotency-Key`.
- The default request hash includes authenticated actor and tenant scope when present, so authenticated stacks should apply auth and tenant middleware before idempotency.
- If the downstream handler succeeds but the replay record cannot be persisted, the client receives `503 Service Unavailable` instead of an ambiguous success response.
- Buffered replay bodies are capped at `1 MiB` by default via `MaxResponseBytes`.
- Exclude streaming, upgrade, and large download routes with `ShouldHandle`; the middleware is intended for buffered JSON-style responses.
- Failed attempts do not leave the key blocked in an in-flight state; the same request can retry immediately with the same key.

- Example: `contrib/examples/idempotency`

```sh
cd contrib
go run ./examples/idempotency
```

Try it:

```sh
curl -s -X POST http://localhost:8080/checkout \
  -H "Idempotency-Key: key-1" \
  -d '{"amount":500,"currency":"eur"}'
```

## Webhook receiver with signature verification

Use HMAC verification and strict body size caps.

- Example: `contrib/examples/webhook`

```sh
cd contrib
go run ./examples/webhook
```

## File upload endpoint

Use `maxbody` to cap request size and stream multipart data.

- Example: `contrib/examples/file-upload`

```sh
cd contrib
go run ./examples/file-upload
```

## Pagination (limit/offset)

Use `querylimits` to bound list queries and return next offsets.

- The example keeps `querylimits` as a coarse guard while returning the same field-level validation shape as `ParseListQuery` for invalid `limit` values.

- Example: `contrib/examples/pagination`

```sh
cd contrib
go run ./examples/pagination
```
