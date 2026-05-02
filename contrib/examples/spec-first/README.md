# Spec-first example

Audience: developers evaluating an OpenAPI-first workflow with generated handler
skeletons, Problem Details errors, and response validation.

This example keeps `openapi.json` as the source of truth, generates
`spec_gen.go`, implements handlers in `handlers.go`, and wires request/response
validation in `main.go`.

## Files

| File | Role |
| --- | --- |
| `openapi.json` | Authoritative OpenAPI document. |
| `main.go` | `go:generate` directive, router setup, and OpenAPI middleware wiring. |
| `spec_gen.go` | Generated routes, constants, handler interface, and status error types. |
| `handlers.go` | Hand-written implementation of the generated handler interface. |
| `handlers_test.go` | Problem Details and OpenAPI contract tests. |

## Regenerate and test

Run from this directory:

```sh
go generate ./...
go test ./...
```

Generated-file expectation: `go generate ./...` rewrites `spec_gen.go` from
`openapi.json`. Do not edit `spec_gen.go` by hand; change `openapi.json` or the
generator and regenerate.

## Run the server

```sh
go run .
```

Expected startup result: the server listens on `:8080` with OpenAPI request
validation and response validation enabled.

## Try the API

List pets:

```sh
curl -s http://localhost:8080/pets
```

Expected response shape:

```json
[{"id":"pet_1","name":"Rex"}]
```

Create a pet:

```sh
curl -s -X POST http://localhost:8080/pets \
  -H 'Content-Type: application/json' \
  -d '{"name":"Milo"}'
```

Expected response shape:

```json
{"id":"pet_2","name":"Milo"}
```

Trigger validation Problem Details:

```sh
curl -i -X POST http://localhost:8080/pets \
  -H 'Content-Type: application/json' \
  -d '{"name":"   "}'
```

Expected response shape: `400` with `Content-Type: application/problem+json`,
`detail` set to `validation failed`, and `validation.fields[0].field` set to
`name`.

Trigger conflict Problem Details:

```sh
curl -i -X POST http://localhost:8080/pets \
  -H 'Content-Type: application/json' \
  -d '{"name":"duplicate"}'
```

Expected response shape: `409` with `Content-Type: application/problem+json` and
`detail` set to `pet already exists`.

## Regeneration verification

After changing `openapi.json`, run:

```sh
go generate ./...
go test ./...
```

Review the regenerated `spec_gen.go` together with the OpenAPI change. Response
validation is intentionally enabled in this example so mismatched handler output
fails during development and tests instead of drifting from the spec.
