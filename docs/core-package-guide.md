# Core Package Decision Guide

Audience: adopters deciding which root package to import first.

`VERSIONING.md` and `docs/package-classification.tsv` remain the source of
truth for stability. This guide explains adoption choices; it does not create a
new compatibility list.

Dependency note legend:

- `root-only`: root module packages and the Go standard library.
- `auth/JWK`: package imports JWT/JWK verification dependencies.
- `test-only`: intended for tests, not production runtime.
- `compat`: stable for v3 compatibility but not recommended for new generic
  design.

## Recommended Core Matrix

| Package | Use case | Use when | Do not use when | Stability | Dependency note | Example |
| --- | --- | --- | --- | --- | --- | --- |
| `httpx` | JSON and Problem Details responses. | You need small response helpers. | A framework already owns response serialization. | stable | root-only | [example](../httpx/example_test.go) |
| `fielderrors` | Structured validation field errors. | You need stable field/code/message triples. | Errors are purely internal. | stable | root-only | [example](../fielderrors/example_test.go) |
| `binding` | Decode JSON, query, and path inputs. | You want bounded decoding and validation Problem Details. | A full validator/binder stack already owns this. | stable | root-only | [example](../binding/example_test.go) |
| `queryparams` | Sort and filter parsing. | Collection endpoints need predictable query rules. | The API has no list endpoints. | stable | root-only | [example](../queryparams/example_test.go) |
| `upload` | Multipart upload validation. | You accept finite multipart uploads. | You stream large uploads directly to object storage. | stable | root-only | [example](../upload/example_test.go) |
| `middleware/maxbody` | Request body size limits. | Any finite request body needs a cap. | The route has no body or streams by design. | stable | root-only | [example](../middleware/maxbody/example_test.go) |
| `middleware/json` | JSON content-type and decoder guardrails. | Routes accept JSON bodies. | The route accepts non-JSON media types. | stable | root-only | [example](../middleware/json/example_test.go) |
| `middleware/querylimits` | Query count and size limits. | Query strings are user-controlled. | Upstream infrastructure enforces equivalent limits. | stable | root-only | [example](../middleware/querylimits/example_test.go) |
| `middleware/timeout` | Request deadlines. | You need cooperative deadlines or finite-response hard timeouts. | The route is streaming, SSE, websocket, or a large download. | stable | root-only | [example](../middleware/timeout/example_test.go) |
| `middleware/secure` | Security response headers. | You want conservative headers on API responses. | Browser policy is owned by another gateway. | stable | root-only | [example](../middleware/secure/example_test.go) |
| `middleware/trace` | Request and correlation IDs. | You need trace IDs without a full tracing stack. | OpenTelemetry middleware already owns IDs. | stable | root-only | [example](../middleware/trace/example_test.go) |
| `middleware/deprecation` | Deprecation and Sunset headers. | A route has a migration timeline. | The API has no deprecated routes. | stable | root-only | [example](../middleware/deprecation/example_test.go) |
| `middleware/ratelimit` | Rate-limit headers and in-process limiting. | You need simple quota guardrails. | Distributed quotas must be exact across nodes. | stable | root-only | [example](../middleware/ratelimit/example_test.go) |
| `middleware/idempotency` | Idempotency key and replay middleware. | Finite write routes need replay protection. | The route streams responses or has app-owned semantics. | stable | root-only | [example](../middleware/idempotency/example_test.go) |
| `idempotent` | Idempotency HTTP contract helpers. | You need headers and request hashing without middleware. | A full store/middleware already owns the flow. | stable | root-only | [example](../idempotent/example_test.go) |
| `webhooks` | HMAC webhook verification. | You verify raw webhook bodies. | Provider-specific SDKs own signature semantics. | stable | root-only | [example](../webhooks/example_test.go) |
| `httpcache` | ETag and Last-Modified helpers. | Read routes need conditional response behavior. | Caching is entirely gateway-owned. | stable | root-only | [example](../httpcache/example_test.go) |
| `negotiation` | Accept/content-type negotiation. | Routes support multiple media types. | Every route is fixed JSON. | stable | root-only | [example](../negotiation/example_test.go) |

## Auth And Identity Matrix

| Package | Use case | Use when | Do not use when | Stability | Dependency note | Example |
| --- | --- | --- | --- | --- | --- | --- |
| `authorization` | Principal, tenant, and scope context. | Middleware needs a shared request identity model. | Identity is app-owned and not shared. | stable | root-only | [example](../authorization/example_test.go) |
| `middleware/auth/apikey` | API key authentication. | Local or service-to-service APIs use API keys. | OAuth/OIDC is the primary identity source. | stable | root-only | [example](../middleware/auth/apikey/example_test.go) |
| `middleware/auth/authz` | Role authorization. | Routes need simple role checks. | Policy engines or ABAC own authorization. | stable | root-only | [example](../middleware/auth/authz/example_test.go) |
| `contrib/middleware/auth/jwt` | JWT/JWK bearer validation. | You need local JWT/JWKS validation. | A provider integration or gateway already validates tokens. | contrib | auth/JWK | [example](../contrib/middleware/auth/jwt/example_test.go) |
| `middleware/auth/tenant` | Tenant header/context checks. | Tenant identity must be explicit per request. | Tenancy is derived only from server-side session state. | stable | root-only | [example](../middleware/auth/tenant/example_test.go) |
| `contrib/oauth2` | Provider-neutral claims and scopes. | You need typed bearer-token claims. | Provider SDK types are sufficient and isolated. | contrib | contrib | [example](../contrib/oauth2/example_test.go) |
| `httpx/identity` | Client IP and identity helpers. | Middleware needs trusted proxy-aware identity. | Identity is enforced only by upstream infrastructure. | stable | root-only | [example](../httpx/identity/example_test.go) |

## Endpoint And Contract Matrix

| Package | Use case | Use when | Do not use when | Stability | Dependency note | Example |
| --- | --- | --- | --- | --- | --- | --- |
| `endpoints/health` | Liveness, readiness, and detailed health. | Services need public probes and operator detail. | A platform probe system owns all health behavior. | stable | root-only | [example](../endpoints/health/example_test.go) |
| `endpoints/version` | Version endpoint. | Build metadata should be visible to operators. | Version is exposed by another control plane. | stable | root-only | [example](../endpoints/version/example_test.go) |
| `endpoints/docs` | OpenAPI/docs endpoint. | You serve generated docs from the service. | Docs are hosted elsewhere. | stable | root-only | [example](../endpoints/docs/example_test.go) |
| `endpoints/pprof` | Admin pprof mounting. | Operators need profiling behind explicit admin controls. | pprof is globally disabled or gateway-owned. | stable | root-only | [example](../endpoints/pprof/example_test.go) |
| `endpoints/list` | List response and cursor helpers. | Collection APIs need stable list metadata. | List shape is provider-specific. | stable | root-only | [example](../endpoints/list/example_test.go) |
| `specs` | OpenAPI schema and registry helpers. | You register route metadata in code. | You are OpenAPI-first with external generation only. | stable | root-only | [example](../specs/example_test.go) |
| `routecontracts` | Handler registration and contract checks. | You want runtime route metadata and testable contracts. | Router-specific tooling owns all contracts. | stable | root-only | [example](../routecontracts/example_test.go) |
| `routepolicy` | Runtime policy from operation metadata. | OpenAPI metadata should drive middleware policy. | Policies are configured independently. | stable | root-only | [example](../routepolicy/example_test.go) |
| `securityprofile` | Bundled API security defaults. | You want a documented middleware profile. | You need route-specific custom composition only. | stable | root-only | [example](../securityprofile/example_test.go) |
| `operations` | Async operation responses. | Writes return `202 Accepted` and pollable status. | Work completes synchronously. | stable | root-only | [example](../operations/example_test.go) |

## Support, Test, And Compatibility Matrix

| Package | Use case | Use when | Do not use when | Stability | Dependency note | Example |
| --- | --- | --- | --- | --- | --- | --- |
| `apiclient` | Client-side Problem Details and precondition helpers. | You build small stdlib clients. | Generated clients own all behavior. | stable | root-only | [example](../apiclient/example_test.go) |
| `apitest` | HTTP assertion helpers. | Tests need deterministic API response checks. | The production runtime needs assertions. | stable | test-only | [example](../apitest/example_test.go) |
| `contracttest` | Route/OpenAPI contract assertions. | Tests need route contract checks. | Production code needs runtime behavior. | stable | test-only | [example](../contracttest/example_test.go) |
| `compatkit` | Downstream HTTP compatibility checks. | Service tests need reusable upgrade evidence against a handler or base URL. | Package-local unit tests or direct contracttest assertions are enough. | experimental | test-only | [example](../compatkit/example_test.go) |
| `httpx/recover` | Panic recovery middleware. | You need deterministic Problem Details for panics. | Another recovery layer already owns panic handling. | stable | root-only | [example](../httpx/recover/example_test.go) |
| `email` | Email sender contract types. | You need a tiny app-owned email boundary. | Provider-specific workflow belongs in app code. | stable | root-only | [example](../email/example_test.go) |
| `scheduler` | Scheduler abstractions and recorder helpers. | Background jobs need toolkit-compatible contracts. | A workflow engine owns scheduling. | stable | root-only | [example](../scheduler/example_test.go) |
| `ports` | Generic logger, clock, and ID contracts. | You need an adapter-neutral utility shared across independent packages. | The contract belongs to HTTP, persistence, or a domain package. | stable | root-only | [example](../ports/example_test.go) |
| `compat/billing` | Hosted-checkout compatibility model. | Existing v3 billing compatibility is required. | You design new billing workflows. | compatibility-only | compat | [example](../compat/billing/example_test.go) |
| `scheduler/migrations` | Migration compatibility asset. | Existing migration compatibility needs it. | New migration orchestration is app-owned. | compatibility-only | compat | [example](../scheduler/migrations/example_test.go) |
| `swagstub` | Tooling compatibility shim. | Existing v3 tooling expects it. | New runtime code needs docs behavior. | compatibility-only | compat | [example](../swagstub/example_test.go) |
