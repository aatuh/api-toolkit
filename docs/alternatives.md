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

## Comparison Examples

Each example below exposes the same `POST /widgets` intent at a different
adoption level. Use the smallest version that matches the work your service
needs to own.

### Plain chi version

Choose this when the service only needs routing and application-owned request
helpers.

```go
r := chi.NewRouter()

r.Post("/widgets", func(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var input createWidgetRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	widget, err := app.CreateWidget(r.Context(), input)
	if err != nil {
		http.Error(w, "create widget failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(widget)
})
```

### api-toolkit library version

Choose this when the app keeps its router and architecture, but wants shared
transport guardrails such as bounded JSON decoding, validation Problem Details,
and buffered JSON responses.

```go
r := chi.NewRouter()

r.Post("/widgets", func(w http.ResponseWriter, r *http.Request) {
	input, err := binding.DecodeJSON[createWidgetRequest](r, binding.JSONConfig{
		MaxBytes:      1 << 20,
		RequireObject: true,
	})
	if err != nil {
		binding.WriteValidationProblem(w, err)
		return
	}

	widget, err := app.CreateWidget(r.Context(), input)
	if err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
			Title: "Create widget failed",
		})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, widget)
})
```

### Generated scaffold version

Choose this when the team wants an app-owned starter service with chi routing,
OpenAPI checks, generated client assets, auth/idempotency wiring, deployment
starters, and repeatable local quality gates.

```sh
go run github.com/aatuh/api-toolkit/contrib/v3/cmd/api-toolkit@latest new service \
  --module example.com/widgets-api \
  --profile saas-api \
  --dir widgets-api
```

The scaffold is intentionally broader than the library. Existing services should
start with the library version unless they also want generated project structure
and operational assets.
