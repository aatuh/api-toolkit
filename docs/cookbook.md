# Cookbook

Runnable examples live under `contrib/examples`. From the repo root:

```sh
cd contrib
go run ./examples/<name>
```

## Idempotent write endpoint

Use the idempotency middleware to guarantee safe retries for POST/PUT/PATCH.

- Successful terminal responses are replayed for the same `Idempotency-Key`.
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

- Example: `contrib/examples/pagination`

```sh
cd contrib
go run ./examples/pagination
```
