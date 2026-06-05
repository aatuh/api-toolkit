# Alternatives

Audience: Go developers deciding whether `api-toolkit` is the right dependency
for a conventional HTTP API.

`api-toolkit` is a guardrail library for JSON/HTTP APIs. It is not a router,
OpenAPI-first code generator, design-first framework, or Protobuf/RPC stack.
Use the smaller or more specialized tool when that tool is the actual problem
you need to solve.

## Use `net/http` Or Chi

Use `net/http` or chi when you mostly need routing, small middleware, or a few
application-owned helpers. `api-toolkit` does not replace routing. It adds
production guardrails around request bounds, response shapes, health endpoints,
idempotency contracts, route metadata, and security defaults.

Choose `net/http` or chi instead when:

- your service has only a few routes,
- route matching is the main problem,
- you do not need shared Problem Details, route contracts, or release evidence,
- the app can own a tiny helper more clearly than it can carry a dependency.

## Use oapi-codegen

Use oapi-codegen when OpenAPI is the central contract and the primary need is
generated Go server/client boilerplate from an OpenAPI document.

Choose oapi-codegen instead when:

- the OpenAPI document is authored before the Go routes,
- generated request/response types are the core workflow,
- API drift is managed through spec-first generation,
- runtime middleware guardrails are secondary to generated code.

## Use Goa

Use Goa when the team wants design-first API development with a DSL and generated
server, client, and documentation artifacts.

Choose Goa instead when:

- the API design language is the source of truth,
- generated transport code is expected,
- the service can adopt Goa's framework shape,
- the team wants a broad design-first platform rather than small HTTP helpers.

## Use Connect

Use Connect when the API is Protobuf/RPC-oriented and the team wants typed
contracts that work across gRPC-compatible and HTTP clients.

Choose Connect instead when:

- Protobuf messages are the source of truth,
- RPC method contracts matter more than conventional JSON/REST semantics,
- generated clients and schemas are required across languages,
- HTTP endpoint ergonomics are secondary to typed RPC compatibility.

## Use api-toolkit

Use `api-toolkit` when the app already owns its architecture and needs small,
composable HTTP/API primitives:

- bounded request bodies and query parsing,
- consistent JSON responses and RFC 9457 Problem Details,
- health/version/docs endpoints with operator caveats,
- route metadata and OpenAPI contract checks,
- idempotency and webhook verification contracts,
- security-conscious middleware defaults.

The minimal adoption path is the core module:

```sh
go get github.com/aatuh/api-toolkit/v3
```

Add contrib only when you intentionally need maintained adapters, integrations,
examples, or generated service tooling.
