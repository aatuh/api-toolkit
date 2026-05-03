# Cookbook

Audience: developers applying api-toolkit to common HTTP API tasks. Recipes are
either runnable contrib examples or inline package sketches. Runnable examples
name a `contrib/examples/...` source and a run command; inline sketches show the
minimum package usage to adapt inside an application.

Run runnable examples from the contrib module unless a recipe says otherwise:

```sh
cd contrib
go run ./examples/<name>
```

## Recipe index

| Task area | Recipes |
| --- | --- |
| Getting started | [Minimal JSON write endpoint](#minimal-json-write-endpoint), [Hardened middleware profile](#hardened-middleware-profile), [Safe system endpoint mounting](#safe-system-endpoint-mounting) |
| Request validation and uploads | [Validated JSON and query request](#validated-json-and-query-request), [File upload endpoint](#file-upload-endpoint), [Multipart upload validation](#multipart-upload-validation) |
| Authentication and authorization | [Authenticated admin route](#authenticated-admin-route), [API key route](#api-key-route), [Provider-neutral OAuth2 scopes](#provider-neutral-oauth2-scopes), [Policy-engine authorization](#policy-engine-authorization) |
| Route contracts and OpenAPI | [Contract-first route registration](#contract-first-route-registration), [Operation-derived route policies](#operation-derived-route-policies), [OpenAPI components and security schemes](#openapi-components-and-security-schemes), [Reusable Problem Details components](#reusable-problem-details-components), [Contract tests for routes and OpenAPI](#contract-tests-for-routes-and-openapi) |
| Collection and async semantics | [Pagination with limit and offset](#pagination-with-limit-and-offset), [Collection query semantics](#collection-query-semantics), [Cursor-paginated list](#cursor-paginated-list), [Async operation polling](#async-operation-polling), [Idempotent write endpoint](#idempotent-write-endpoint), [Idempotent create and async replay contracts](#idempotent-create-and-async-replay-contracts) |
| Webhooks, outbound calls, and HTTP semantics | [Webhook receiver with signature verification](#webhook-receiver-with-signature-verification), [Signed outbound webhook request](#signed-outbound-webhook-request), [Webhook replay and delivery contracts](#webhook-replay-and-delivery-contracts), [Guarded outbound HTTP client](#guarded-outbound-http-client), [Conditional GET and update](#conditional-get-and-update), [Runtime deprecation headers](#runtime-deprecation-headers), [Standard rate-limit headers](#standard-rate-limit-headers) |
| Tests and clients | [HTTP API test assertions](#http-api-test-assertions), [API client helpers](#api-client-helpers) |

## Minimal JSON write endpoint

Purpose: accept JSON, validate fields, and return Problem Details for bad input.

- Source type: Runnable contrib example (`contrib/examples/minimal`).

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

## Validated JSON and query request

Purpose: bind request input into typed structs while preserving the toolkit's Problem Details validation shape.

- Source type: Inline sketch using the stable `binding` package.

- Prerequisites: none.
- Example source: use `github.com/aatuh/api-toolkit/v2/binding` in any HTTP handler.
- Request:

```sh
curl -s -X POST 'http://localhost:8080/widgets?dry_run=true' \
  -H 'Content-Type: application/json' \
  -d '{"name":"starter","quantity":1}'
```

- Handler sketch:

```go
type createWidgetBody struct {
	Name     string `json:"name" required:"true"`
	Quantity int    `json:"quantity" required:"true"`
}

type createWidgetQuery struct {
	DryRun bool `query:"dry_run"`
}

body, err := binding.DecodeJSON[createWidgetBody](r, binding.JSONConfig{})
if err != nil {
	binding.WriteValidationProblem(w, err)
	return
}
query, err := binding.DecodeQuery[createWidgetQuery](r, binding.QueryConfig{})
if err != nil {
	binding.WriteValidationProblem(w, err)
	return
}
```

- Expected response shape: invalid input returns `application/problem+json` with `validation.fields`.
- Production caveat: keep route-specific semantic validation in the application layer; binding handles transport shape, conversion, and required-field errors.

## Hardened middleware profile

Purpose: apply the strict API profile with secure headers, timeout, tracing, and explicit system endpoint choices.

- Source type: Runnable contrib example (`contrib/examples/secure-profile`).

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

- Source type: Runnable contrib example (`contrib/examples/auth`).

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

## Safe system endpoint mounting

Purpose: keep public probes separate from operator-only pprof, detailed health,

- Source type: Inline sketch.
and metrics routes. Web, mobile, and desktop clients should never need direct
access to operator-only endpoints.

- Prerequisites: provide an application-owned `requireAdmin` middleware or
  internal-network wrapper.
- Public routes: mount liveness and readiness on the public mux.
- Admin routes: mount detailed health with `RegisterAdminDetailedHealthRoute`
  and pprof with `pprof.RegisterAdminRoutes`; mount metrics only on the
  protected admin mux or behind equivalent upstream network policy.

```go
publicMux.HandleFunc("/livez", healthHandler.LivenessHandler)
publicMux.HandleFunc("/readyz", healthHandler.ReadinessHandler)

if err := healthHandler.RegisterAdminDetailedHealthRoute(adminRouter, requireAdmin); err != nil {
	return err
}
if err := pprof.RegisterAdminRoutes(adminRouter, requireAdmin); err != nil {
	return err
}
adminMux.Handle("/metrics", requireAdmin(metricsHandler))
```

## API key route

Purpose: authenticate service-to-service or automation calls with API keys and explicit scopes.

- Source type: Runnable contrib example (`contrib/examples/api-key`).

- Prerequisites: use the demo key only for local testing.
- Example source: `contrib/examples/api-key`.
- Run command: `cd contrib && go run ./examples/api-key`.
- Request:

```sh
curl -s http://localhost:8080/admin \
  -H 'Authorization: ApiKey demo-admin-key' \
  -H 'Accept: application/json'
```

- Route contract: the example registers the `/admin` operation once through `routecontracts`, generates the `AdminResponse` schema from a Go struct, applies `ApiKeyAuth` scope metadata, enforces `Accept` negotiation, decodes query input with `binding`, uses catalog-backed Problem Details for validation errors, and serves the generated document at `/openapi.json`.
- Expected response shape: `200` JSON with `principal_id` and `scopes`; missing or invalid keys return Problem Details `401`, and missing scopes return Problem Details `403`.
- Production caveat: keep key storage, hashing, rotation, and revocation in application-owned verifier code; the stable middleware extracts credentials, calls the verifier, writes auth context, and enforces scopes.

## Contract-first route registration

Purpose: keep runtime route registration, OpenAPI operation metadata, schema components, content negotiation, and error behavior in one declaration path.

- Source type: Inline sketch.

- Prerequisites: use a router that supports the common `Get`, `Post`, `Put`, and `Delete` registration methods.
- Packages: `github.com/aatuh/api-toolkit/v2/routecontracts`, `github.com/aatuh/api-toolkit/v2/specs`, `github.com/aatuh/api-toolkit/v2/negotiation`, and `github.com/aatuh/api-toolkit/v2/httpx`.
- Handler sketch:

```go
specRegistry := specs.NewRegistry(specs.Info{Title: "Widget API", Version: "v1"})
_ = specs.RegisterSchemaFrom[widgetResponse](specRegistry, "Widget", specs.SchemaOptions{})

contracts := routecontracts.NewRegistry(router, specRegistry)
_ = contracts.Get("/widgets/{id}", specs.Operation{
	Summary: "Read widget",
	Responses: map[int]specs.Response{
		http.StatusOK: {
			Content: map[string]specs.MediaType{
				"application/json": {SchemaRef: "#/components/schemas/Widget"},
			},
		},
	},
}, widgetHandler, negotiation.RequireAccept("application/json"))
```

- Expected response shape: the router serves the handler and `/openapi.json` can describe the same method, path, responses, schemas, and media types.
- Production caveat: keep generated schemas simple and review OpenAPI diffs when changing public response structs.

## Contract tests for routes and OpenAPI

Purpose: catch drift between route registration, OpenAPI operations, response metadata, security metadata, and typed problem catalogs.

- Source type: Inline test sketch.

- Prerequisites: register routes through `routecontracts` and operations through `specs.Registry`.
- Package: `github.com/aatuh/api-toolkit/v2/contracttest`.
- Test sketch:

```go
contracttest.AssertRegistryValid(t, routeRegistry)
contracttest.AssertRouteCoverage(t, routeRegistry, http.MethodGet, "/widgets")
contracttest.AssertOperationHasResponse(t, specRegistry, http.MethodGet, "/widgets", http.StatusOK)
contracttest.AssertOperationHasSecurity(t, specRegistry, http.MethodGet, "/widgets", "ApiKeyAuth")
contracttest.AssertProblemCatalogHas(t, httpx.DefaultProblemCatalog(), httpx.ProblemCode(httpx.TypeBadRequest))
```

- Expected result: tests fail when a runtime route, documented response, security requirement, or problem code is missing.
- Production caveat: golden OpenAPI tests should review diffs intentionally; the helper does not auto-update golden files.

## Runtime deprecation headers

Purpose: signal deprecated or sunsetting routes at runtime without changing response behavior.

- Source type: Inline sketch.

- Prerequisites: choose the deprecation and sunset dates from your published migration policy.
- Package: `github.com/aatuh/api-toolkit/v2/middleware/deprecation`.
- Handler sketch:

```go
mw, _ := deprecation.New(deprecation.Config{
	DeprecatedAt: time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
	SunsetAt:     time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
	Links: []deprecation.Link{{
		URL: "https://developer.example.com/deprecations/widgets-v1",
	}},
})
router.Get("/v1/widgets", mw.Handler(widgetsHandler).ServeHTTP)
```

- Expected response shape: unchanged route body plus `Deprecation`, `Sunset`, and `Link` headers.
- Production caveat: headers are only hints; keep the human migration guide accurate and monitor deprecated-route traffic separately.

## Policy-engine authorization

Purpose: authorize a route through a Cedar policy engine with request context.

- Source type: Runnable contrib example (`contrib/examples/policy`).

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

- Source type: Runnable contrib example (`contrib/examples/idempotency`).

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

- Source type: Runnable contrib example (`contrib/examples/webhook`).

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

- Expected response shape: `202` JSON `{"status":"accepted"}` for a valid signature.
- Production caveat: keep provider secrets in secret storage, reject unsigned or oversized payloads before JSON parsing, and add replay protection in application code.

## Signed outbound webhook request

Purpose: build a JSON event request with an HMAC signature that a `webhooks` receiver can verify.

- Source type: Inline sketch with receiver code related to `contrib/examples/webhook`.

- Prerequisites: share the demo secret only for local testing.
- Example source: `contrib/examples/webhook`.
- Handler sketch:

```go
signer, _ := webhooks.NewHMACSHA256Signer(webhooks.HMACSignerConfig{Secret: []byte("demo-secret")})
req, err := webhooks.BuildSignedRequest(ctx, webhooks.OutgoingEvent[paymentEvent]{
    ID:      "evt_123",
    Type:    "payment.succeeded",
    Payload: event,
}, webhooks.SignedRequestConfig{
    URL:    "https://api.example.com/webhooks/payment",
    Signer: signer,
})
```

- Expected request shape: `POST` JSON with `X-Signature`, `X-Webhook-Event-ID`, and `X-Webhook-Timestamp` headers.
- Production caveat: delivery scheduling, retries, replay protection, and idempotency remain application-owned.

## File upload endpoint

Purpose: cap request size and stream multipart upload metadata without buffering the full file in memory.

- Source type: Runnable contrib example (`contrib/examples/file-upload`).

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

- Source type: Runnable contrib example (`contrib/examples/pagination`).

- Prerequisites: none.
- Example source: `contrib/examples/pagination`.
- Run command: `cd contrib && go run ./examples/pagination`.
- Request:

```sh
curl -s 'http://localhost:8080/items?limit=3&offset=2'
```

- Expected response shape: `200` JSON with `items` and optional `next_offset`.
- Production caveat: keep maximum limits low enough for the backing store and prefer checked parser APIs when validation errors matter.

## Collection query semantics

Purpose: parse list endpoint query parameters for sorting, filtering, sparse fieldsets, and includes before applying application-owned query logic.

- Source type: Inline sketch.

- Prerequisites: define allowed sort and filter fields for the route.
- Package: `github.com/aatuh/api-toolkit/v2/queryparams`.
- Request:

```sh
curl -s 'http://localhost:8080/widgets?sort=name,-created_at&filter[status]=active&filter[created_at][gte]=2026-01-01&fields=id,name&include=owner'
```

- Handler sketch:

```go
sorts, err := queryparams.ParseSort(r.URL.Query(), queryparams.SortConfig{AllowedFields: []string{"name", "created_at"}})
if err != nil {
    binding.WriteValidationProblem(w, err)
    return
}
filters, err := queryparams.ParseFilters(r.URL.Query(), queryparams.FilterConfig{Fields: []queryparams.FilterField{
    {Name: "status"},
    {Name: "created_at", Operators: []queryparams.FilterOperator{queryparams.FilterOperatorGreaterThanOrEqual}},
}})
if err != nil {
    binding.WriteValidationProblem(w, err)
    return
}
```

- Expected behavior: unknown sort fields, filter fields, or filter operators return field-level validation errors.
- Production caveat: parsed query parameters are transport contracts only; translate them to SQL, search, or service-layer requests in application code.

## Async operation polling

Purpose: return `202 Accepted` for long-running work and expose a pollable operation resource.

- Source type: Inline sketch.

- Prerequisites: provide an application-owned operation store.
- Package: `github.com/aatuh/api-toolkit/v2/operations`.
- Handler sketch:

```go
operations.WriteAccepted(w, operations.AcceptedConfig{
    ID:         "op_123",
    Location:   "/operations/op_123",
    RetryAfter: 5 * time.Second,
})

router.Get("/operations/{id}", operations.PollHandler(operations.PollConfig[result]{
    Store: store,
    OperationID: func(r *http.Request) string {
        return chi.URLParam(r, "id")
    },
}).ServeHTTP)
```

- Expected response shape: accepted writes return `202` with `Location`; poll responses return operation state `pending`, `running`, `succeeded`, `failed`, or `canceled`.
- Production caveat: workers, queues, persistence, cancellation, and retry policy remain application-owned.

## Conditional GET and update

Purpose: use ETags and Last-Modified validators for cache-friendly reads and optimistic concurrency on writes.

- Source type: Inline sketch.

- Prerequisites: compute a stable representation validator from your resource version or response body.
- Package: `github.com/aatuh/api-toolkit/v2/httpcache`.
- Handler sketch:

```go
validators := httpcache.Validators{
	ETag:         httpcache.StrongETag(widget.Version),
	LastModified: widget.UpdatedAt,
}
if decision := httpcache.EvaluateRead(r, validators); decision.NotModified {
	httpcache.WriteNotModified(w, validators)
	return
}
httpcache.SetValidators(w, validators)
httpx.WriteJSON(w, http.StatusOK, widget)
```

- Expected response shape: matching `If-None-Match` or `If-Modified-Since` returns `304` with validators and no body; failed write preconditions can return `412` Problem Details.
- Production caveat: only use strong ETags for write preconditions when they represent the exact mutable resource version.

## Cursor-paginated list

Purpose: return signed cursor metadata for stable forward pagination without exposing database offsets.

- Source type: Inline sketch.

- Prerequisites: provide an application secret for HMAC signing; rotate it through your normal secret-management process.
- Example source: combine `github.com/aatuh/api-toolkit/v2/endpoints/list` with your list handler.
- Request:

```sh
curl -s 'http://localhost:8080/items?limit=25&cursor=<cursor>'
```

- Handler sketch:

```go
codec, _ := list.NewHMACCursorCodec([]byte("replace-with-secret"))
query, err := list.ParseCursorQueryChecked(r, list.CursorQueryConfig{
	DefaultLimit: 25,
	MaxLimit:     100,
	Codec:        codec,
})
if err != nil {
	httpx.WriteProblem(w, httpx.Problem{
		Type:   "https://example.com/problems/invalid-query",
		Title:  "Invalid query",
		Status: http.StatusBadRequest,
	})
	return
}

nextCursor, _ := codec.Encode(map[string]string{"after_id": "item_123"}, time.Now().Add(15*time.Minute))
httpx.WriteJSON(w, http.StatusOK, list.NewCursorResponse(items, list.CursorMeta{
	Limit:      query.Limit,
	NextCursor: nextCursor,
}))
```

- Expected response shape: `200` JSON with `items` and cursor `meta`, including `limit` and optional `next_cursor`.
- Production caveat: cursor values are signed but not encrypted; keep sensitive data out of cursor payloads.

## Spec-first handlers

Purpose: generate handler skeletons from OpenAPI and validate requests and responses.

- Source type: Runnable contrib example (`contrib/examples/spec-first`).

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

## OpenAPI components and security schemes

Purpose: register reusable schemas, responses, and security schemes for generated route contracts.

- Source type: Inline sketch.

- Prerequisites: keep schema names stable once clients consume the generated OpenAPI document.
- Package: `github.com/aatuh/api-toolkit/v2/specs`.
- Registry sketch:

```go
registry := specs.NewRegistry(specs.Info{Title: "Widget API", Version: "v1"})
registry.RegisterSchema("Widget", map[string]any{"type": "object"})
registry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{
	Type: "apiKey",
	Name: "X-API-Key",
	In:   "header",
})
registry.RegisterResponse("Problem", specs.Response{
	Description: "Problem Details",
	Content: map[string]specs.MediaType{
		"application/problem+json": {SchemaRef: "#/components/schemas/Problem"},
	},
})
```

- Expected response shape: `/openapi.json` includes deterministic `components.schemas`, `components.responses`, and `components.securitySchemes`.
- Production caveat: route registration and runtime middleware still need to be kept in sync by tests or review.

## Guarded outbound HTTP client

Purpose: show SSRF host allowlisting, retry budget controls, circuit breaking, and bulkhead limits.

- Source type: Runnable contrib example (`contrib/examples/outbound`).

- Prerequisites: replace `api.example.com` with an allowed real upstream before expecting a successful request.
- Example source: `contrib/examples/outbound`.
- Run command: `cd contrib && go run ./examples/outbound`.
- Request example: the program issues a guarded `GET https://api.example.com/health`.
- Expected response shape: no local HTTP endpoint; success means the outbound call returns and the response body closes.
- Production caveat: do not broaden `AllowedHosts`, `AllowedPorts`, or retryable methods without reviewing replay and SSRF risk.

## Operation-derived route policies

Purpose: keep runtime middleware aligned with the route contract instead of repeating deprecation, negotiation, auth, and quota metadata in handlers.

- Source type: Inline sketch.

- Prerequisites: register routes through `routecontracts.NewRegistryWithOptions`.
- Packages: `github.com/aatuh/api-toolkit/v2/routecontracts`, `github.com/aatuh/api-toolkit/v2/routepolicy`, and `github.com/aatuh/api-toolkit/v2/specs`.
- Handler sketch:

```go
policy := routepolicy.New(routepolicy.Config{
	EnableDeprecation: true,
	EnableNegotiation: true,
	EmitPolicyExtension: true,
	Auth: func(op specs.Operation) (func(http.Handler) http.Handler, error) {
		return requireScopes(op.Scopes...), nil
	},
})
contracts := routecontracts.NewRegistryWithOptions(router, specRegistry, routecontracts.Options{
	Policies: []routecontracts.Policy{policy},
})
err := contracts.Get("/widgets", specs.Operation{
	Summary:    "List widgets",
	Scopes:     []string{"widgets:read"},
	Deprecated: true,
	Responses: map[int]specs.Response{
		200: {Content: map[string]specs.MediaType{"application/json": {SchemaRef: "#/components/schemas/WidgetList"}}},
	},
}, listWidgets)
```

- Expected behavior: policy middleware is derived from the operation and route-specific middleware can still wrap it when needed.
- Production caveat: keep storage-backed auth, idempotency, and quota decisions in application-owned middleware factories; `routepolicy` only derives when to apply them.

## Reusable Problem Details components

Purpose: publish the same typed Problem Details catalog in runtime error handling and OpenAPI components.

- Source type: Inline sketch.

- Prerequisites: create or use an `httpx.ProblemCatalog`.
- Packages: `github.com/aatuh/api-toolkit/v2/httpx` and `github.com/aatuh/api-toolkit/v2/specs`.
- Registry sketch:

```go
registry := specs.NewRegistry(specs.Info{Title: "Widget API", Version: "v1"})
specs.RegisterProblemCatalog(registry, httpx.DefaultProblemCatalog())
operation := specs.Operation{Responses: map[int]specs.Response{
	400: specs.ValidationProblemResponse("Invalid request"),
	500: specs.ProblemResponse("Internal error"),
}}
```

- Expected behavior: `components.schemas.Problem`, `components.schemas.ValidationProblem`, and catalog-backed response components appear deterministically only after explicit registration.
- Production caveat: keep stable machine-readable problem codes in the catalog before exposing them in published client contracts.

## Standard rate-limit headers

Purpose: expose quota information to API clients without changing limiter storage or decision ownership.

- Source type: Inline sketch.

- Prerequisites: use `middleware/ratelimit`.
- Package: `github.com/aatuh/api-toolkit/v2/middleware/ratelimit`.
- Middleware sketch:

```go
limiter, err := ratelimit.New(ratelimit.Options{
	Capacity:     100,
	RefillRate:   10,
	HeaderConfig: ratelimit.DefaultHeaderConfig(),
})
if err != nil {
	return err
}
handler := limiter.Handler(next)
```

- Expected behavior: allowed and denied responses include standard quota headers when header emission is enabled; existing limiter behavior is unchanged when `HeaderConfig` is left empty.
- Production caveat: distributed quota accounting still belongs in an app-owned or adapter-owned limiter.

## Idempotent create and async replay contracts

Purpose: standardize idempotency-key validation, request hashing, conflict responses, and replayed `202 Accepted` responses.

- Source type: Inline sketch.

- Prerequisites: use application-owned storage or `middleware/idempotency` for reservation/replay ownership.
- Package: `github.com/aatuh/api-toolkit/v2/idempotent`.
- Handler sketch:

```go
key, err := idempotent.RequireKey(r, "")
if err != nil {
	httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: "Missing idempotency key", Detail: err.Error()})
	return
}
hash := idempotent.RequestHash(r, body)
if conflictsExistingRequest(key, hash) {
	idempotent.WriteConflict(w, "idempotency key was reused with a different request")
	return
}
idempotent.WriteAcceptedReplay(w, idempotent.AsyncConfig{ID: "op_123", Location: "/operations/op_123", RetryAfter: 2 * time.Second})
```

- Expected behavior: clients can distinguish new accepts, replayed accepts, and conflicting request reuse with consistent headers and Problem Details.
- Production caveat: persistence, key TTLs, and response replay storage remain application or middleware concerns.

## Webhook replay and delivery contracts

Purpose: require event ids, reject timestamps outside a replay window, and document outbound delivery attempts/results.

- Source type: Inline sketch.

- Prerequisites: configure a webhook verifier and timestamp-producing sender.
- Package: `github.com/aatuh/api-toolkit/v2/webhooks`.
- Receiver sketch:

```go
receiver := webhooks.Receiver[myEvent]{Config: webhooks.ReceiverConfig[myEvent]{
	Verifier: verifier,
	Replay: webhooks.ReplayConfig{
		Tolerance:      5 * time.Minute,
		RequireEventID: true,
	},
	Handle: handleEvent,
}}
```

- Expected behavior: missing event ids, missing timestamps, invalid timestamps, and timestamps outside the configured skew window return Problem Details before the event is handled.
- Production caveat: replay databases, duplicate suppression, delivery queues, and retry persistence remain outside core.

## Multipart upload validation

Purpose: decode multipart forms with required files, size limits, content-type allowlists, and Problem Details field errors.

- Source type: Inline sketch.

- Prerequisites: clients send `multipart/form-data`.
- Package: `github.com/aatuh/api-toolkit/v2/upload`.
- Handler sketch:

```go
form, err := upload.DecodeMultipart(r, upload.Config{
	MaxRequestBytes:     20 << 20,
	MaxFileBytes:        upload.MaxFileBytes(5 << 20),
	RequiredFiles:       []string{"avatar"},
	AllowedContentTypes: upload.AllowedContentTypes("image/png", "image/jpeg"),
})
if err != nil {
	upload.WriteValidationProblem(w, err)
	return
}
file, err := upload.RequireFile(form, "avatar")
if err != nil {
	upload.WriteValidationProblem(w, err)
	return
}
```

- Expected behavior: missing, oversized, malformed, or disallowed files return `application/problem+json` with validation field errors.
- Production caveat: virus scanning, object storage, image transcoding, and persistence stay in application code.

## Provider-neutral OAuth2 scopes

Purpose: keep provider-specific token validation outside core while standardizing claims, scope checks, and OpenAPI security schemes.

- Source type: Inline sketch.

- Prerequisites: provide an application-owned `oauth2.Validator` implementation.
- Package: `github.com/aatuh/api-toolkit/v2/oauth2`.
- Handler sketch:

```go
token, ok := oauth2.BearerToken(r)
if !ok {
	httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: "Unauthorized"})
	return
}
claims, err := validator.ValidateToken(r.Context(), token)
if err != nil || oauth2.RequireScopes(claims, "widgets:write") != nil {
	httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: "Forbidden"})
	return
}
ctx := authorization.WithActor(r.Context(), claims.Actor())
ctx = authorization.WithScope(ctx, claims.AuthorizationScope())
```

- Expected behavior: OAuth2 scope enforcement and route contract security metadata can share the same provider-neutral scope strings.
- Production caveat: JWKS fetching, provider quirks, token caching, and issuer-specific validation remain application or adapter concerns.

## HTTP API test assertions

Purpose: make application tests assert API contracts without reimplementing Problem Details, pagination, and OpenAPI golden checks.

- Source type: Inline test sketch.

- Prerequisites: use `httptest.ResponseRecorder`.
- Package: `github.com/aatuh/api-toolkit/v2/apitest`.
- Test sketch:

```go
recorder := httptest.NewRecorder()
handler.ServeHTTP(recorder, request)
apitest.AssertProblem(t, recorder, http.StatusBadRequest)
apitest.AssertValidationFields(t, recorder, "name")
apitest.AssertRateLimitHeaders(t, recorder)
apitest.AssertOpenAPIGolden(t, generated, golden)
```

- Expected behavior: failed assertions report the mismatched HTTP contract directly in the test failure.
- Production caveat: these helpers are for tests only; runtime code should use `httpx`, `binding`, `upload`, `operations`, and the middleware packages directly.

## API client helpers

Purpose: consume api-toolkit-shaped APIs without generating a service-specific SDK.

- Source type: Inline client sketch.

- Prerequisites: use `net/http` clients.
- Package: `github.com/aatuh/api-toolkit/v2/apiclient`.
- Client sketch:

```go
client := &http.Client{Transport: apiclient.APIKeyTransport{Key: apiKey}}
result, resp, err := apiclient.DoJSON[widgetResponse](ctx, client, http.MethodPost, endpoint, createWidgetRequest{Name: "starter"})
if problem, ok := err.(*apiclient.ProblemError); ok {
	log.Printf("api problem: %s", problem.Problem.Title)
}
if retryAfter, ok := apiclient.RetryAfter(resp.Header); ok {
	_ = retryAfter
}
```

- Expected behavior: clients can decode Problem Details, set precondition headers, sign webhook requests, add API keys, and iterate cursor pages with small composable helpers.
- Production caveat: retries, persistence, service-specific resources, and generated SDKs remain outside core.
