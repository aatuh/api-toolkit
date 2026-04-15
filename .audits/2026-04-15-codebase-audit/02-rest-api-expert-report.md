# REST API Expert Report

## Verdict
The repository has good low-level HTTP primitives, but as a REST API toolkit its client-visible contract is not consistent enough yet. The biggest problems are not crashes; they are mismatched wire semantics across defaults, middleware, and public examples.

I ran `go test` on the HTTP-facing packages: `./httpx`, `./middleware/json`, `./middleware/idempotency`, `./middleware/querylimits`, `./middleware/ratelimit`, `./middleware/auth/tenant`, `./endpoints/list`, `./contrib/middleware/openapi`, and `./contrib/middleware/requestlog`.

## Biggest issues first
1. Default strict mode can emit plain-text `415` responses and enforce `Content-Type` too broadly.
Evidence: `contrib/bootstrap/profile.go:136`, `contrib/bootstrap/profile.go:221`, `middleware/json/json.go:46`
Why it matters: `ProfileStrictAPI` enables JSON enforcement for the default hardened stack, but `jsonmw.Handler` rejects non-JSON `Content-Type` with `http.Error(...)` instead of Problem Details. That violates the repo's own RFC 9457 consistency claim and can reject bodyless `GET` or `DELETE` requests just because a client sent an irrelevant `Content-Type`.
Risk: High
Type: Contract-breaking

2. Authn and authz boundaries are blurred into `403`.
Evidence: `middleware/auth/jwt/middleware.go:134`, `middleware/auth/authz/require_role.go:40`, `middleware/auth/tenant/tenant.go:76`, `middleware/auth/tenant/tenant.go:167`
Why it matters: JWT middleware correctly returns `401` for missing or invalid bearer tokens, but role enforcement returns `403` when no roles exist at all, and tenant enforcement returns `403` even when tenant scope is simply absent. Clients cannot reliably distinguish reauthentication from insufficient authorization.
Risk: High
Type: Contract-breaking, security-sensitive

3. The public OpenAPI and spec-first contract contradict the toolkit's actual error envelope.
Evidence: `README.md:178`, `httpx/field_errors.go:29`, `contrib/middleware/openapi/error_mapping.go:42`, `contrib/examples/spec-first/openapi.json:25`, `contrib/examples/spec-first/openapi.json:112`
Why it matters: The toolkit standard is Problem Details plus `validation.fields`, and the OpenAPI middleware emits that shape. The shipped spec-first example instead documents errors as `{code,message}` under `application/json`. That will produce wrong generated clients and wrong example APIs.
Risk: High
Type: Contract-breaking

4. List and query handling is too permissive for a reusable REST contract.
Evidence: `endpoints/list/list.go:73`, `endpoints/list/list.go:148`, `endpoints/list/list.go:205`, `contrib/examples/pagination/main.go:62`, `contrib/bootstrap/profile.go:206`, `docs/security.md:8`
Why it matters: `ParseListQuery` silently coerces malformed `limit` and `offset` into defaults and silently ignores unsupported filters and sorts. The pagination example returns `400` for invalid pagination inputs. These teach incompatible contracts. Separately, the strict profile does not apply `querylimits` at all, despite docs presenting them as a baseline hardening control.
Risk: Medium
Type: Contract-breaking for `endpoints/list`; implementation-only and operational for strict-profile omission

5. HTTP-surface observability stops at logs; error responses do not expose correlation IDs.
Evidence: `contrib/middleware/requestlog/requestlog.go:167`, `httpx/httpx.go:31`, `httpx/errors.go:82`
Why it matters: Request logging includes `request_id`, `trace_id`, and `span_id`, but `WriteProblem` and the default error mappers do not attach request or trace identifiers to the response body or headers. Client-reported failures are harder to trace operationally.
Risk: Medium
Type: Operational, implementation-only

## Concrete fixes
- Make JSON enforcement method-aware and body-aware. Only require JSON for methods and endpoints that actually consume JSON bodies, and emit `application/problem+json` for `415`.
- Standardize auth semantics: `401` for missing or invalid authentication, `403` only after identity is established but insufficient.
- Replace the spec-first example error schema with RFC 9457 Problem Details, including `application/problem+json` responses and the validation extension shape used by `httpx`.
- Decide one list-query contract and document it. For a toolkit, defaulting malformed query params is too implicit; prefer explicit validation failures with field-level errors.
- Add request correlation to the HTTP surface. Minimum: echo or generate `X-Request-ID`; better: include `request_id` in problem extensions via a wrapper or middleware-aware problem writer.
- Either add `querylimits` to `ProfileStrictAPI` or stop describing it as part of the default hardening baseline.

## What is acceptable and should not be churned
- The core Problem Details writer is solid: correct `application/problem+json`, clean standard members, and compatibility extensions for validation errors.
- Idempotency defaults are broadly sane: replay first non-`5xx`, `409` for in-flight or mismatched reuse, and `Retry-After` on contention.
- JWT hardening is good: algorithm allowlist, issuer and audience checks, and configurable required claims.
- Request logging and metrics are sensibly route-based and low-cardinality, which is the right default for HTTP observability.

## Risk level
Overall risk: Medium-high. The code is not obviously unstable, but the default and public REST contract is inconsistent in ways that will leak directly into clients, generated SDKs, and operational runbooks.
