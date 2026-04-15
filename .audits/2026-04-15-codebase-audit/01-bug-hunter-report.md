# Bug Hunter Report

## Summary
I audited the root module and `contrib` module in audit mode, starting from `README.md`, `Makefile`, `docs/security.md`, and `docs/architecture.md`, then focused on central runtime packages: `middleware/*`, `httpx/*`, `endpoints/*`, selected `authorization/*`, and the outbound HTTP / OpenAPI contrib paths.

I ran the existing repo tests at the smallest useful scope: `go test ./...`, `cd contrib && go test ./...`, plus targeted package tests for `middleware/json` and `httpx/*`. I also used throwaway repro programs outside the repo to verify suspected bugs. Three real bugs were confirmed.

## Confirmed bugs
1. Severity: Medium
Confidence: High
Bug class: API contract / input validation bypass
Affected files/functions: `middleware/json/json.go:39`, `middleware/json/json.go:73`
Why it is a bug: `isJSON` accepts any content type containing the substring `application/json`, so invalid media types such as `text/application/json` are treated as valid JSON. That defeats the middleware's advertised content-type enforcement.
Evidence or repro: A temporary repro against the current package returned `status=204` for a `POST` with `Content-Type: text/application/json`. Current tests do not cover this path; `middleware/json/json_test.go:9` only checks nil middleware passthrough.
Minimal fix direction: Parse the media type with `mime.ParseMediaType` and accept only `application/json` or a valid `application/*+json` type.
Permanent test added or still needed: Still needed. Add a unit test that rejects `text/application/json` and accepts `application/json; charset=utf-8` and `application/problem+json`.

2. Severity: High
Confidence: High
Bug class: Reliability / response corruption after panic
Affected files/functions: `httpx/recover/recover.go:14`
Why it is a bug: The recovery middleware always writes a `500` Problem response after a panic, even if the wrapped handler has already written headers or body bytes. Once the response is committed, that produces a mixed or corrupted response rather than a clean `500`.
Evidence or repro: A repro handler wrote `200 OK` plus `"partial:"` and then panicked. Final response was `status=200`, `content-type="application/problem+json"`, and a body shaped like `partial:{...500 problem json...}\n`.
Minimal fix direction: Wrap the writer and track whether headers or body were already written. Only write the Problem body when nothing has been committed; otherwise log and abort without appending a second payload.
Permanent test added or still needed: Still needed. Add tests for panic-before-write and panic-after-partial-write.

3. Severity: Low
Confidence: High
Bug class: Reliability / nil dependency panic in exported API
Affected files/functions: `endpoints/docs/handlers.go:17`, `endpoints/docs/handlers.go:35`, `endpoints/docs/handlers.go:49`, `endpoints/docs/handlers.go:93`
Why it is a bug: `NewHandler(nil)` succeeds, but every handler method dereferences `h.manager` without validation. That turns a constructor-time configuration mistake into a runtime panic on the first request.
Evidence or repro: A temporary repro using `docs.NewHandler(nil)` and `OpenAPIHandler` panicked; when wrapped in the repo's recovery middleware it returned `500 internal-error`. The panic source is the unconditional calls like `h.manager.ServeOpenAPI(w, r)`.
Minimal fix direction: Reject a nil manager in `NewHandler`, or make the methods fail closed with a deterministic `500` or `404` instead of panicking.
Permanent test added or still needed: Still needed. Add constructor and handler tests for nil manager behavior.

## Unproven risks
1. Risk level: Medium
Why it is suspicious: Both buffered response writers used by idempotency and OpenAPI response validation only implement the base `http.ResponseWriter` methods and omit optional interfaces such as `http.Flusher`, `http.Hijacker`, `http.Pusher`, and `io.ReaderFrom`: `response_writer/capture.go:10`, `middleware/idempotency/idempotency.go:237`, `contrib/middleware/openapi/response_capture.go:8`, `contrib/middleware/openapi/openapi.go:172`. Handlers that rely on flushing, hijacking, or zero-copy streaming will likely break when wrapped.
What evidence is missing: I did not run a live SSE, WebSocket, or streaming repro through these middlewares.
Fastest next step to prove or dismiss it: Add a tiny handler that type-asserts `http.Flusher` and run it through both middlewares.

2. Risk level: Medium
Why it is suspicious: The same nil-dependency pattern exists in other endpoint handlers, especially `endpoints/health`, which also stores an interface and dereferences it in request handlers without constructor validation.
What evidence is missing: I did not run a separate repro for `health.NewHandler(nil)`.
Fastest next step to prove or dismiss it: Repeat the `docs` repro with `health.NewHandler(nil)` and each public handler.

3. Risk level: Medium
Why it is suspicious: `middleware/auth/jwt` is a high-risk package with nontrivial token parsing, claim validation, skip-header bypass logic, and JWKS handling, but it currently has no direct tests. That is a meaningful coverage gap around a trust boundary.
What evidence is missing: I did not produce a failing auth repro; this is a testability and assurance risk, not a confirmed defect.
Fastest next step to prove or dismiss it: Add focused unit tests for malformed `Authorization` headers, required-claim enforcement, and trusted-proxy skip-header handling.

## Verification log
Commands run:
- `sed -n '1,240p' README.md`
- `sed -n '1,220p' Makefile`
- `sed -n '1,220p' docs/security.md`
- `sed -n '1,220p' docs/architecture.md`
- `go test ./...`
- `cd contrib && go test ./...`
- `go test ./middleware/json ./httpx ./httpx/identity ./httpx/recover`

Key outputs:
- `go test ./...`: all root package tests passed.
- `cd contrib && go test ./...`: all contrib package tests passed.
- `go test ./endpoints/docs`: no tests.
- `go test ./httpx/recover`: no tests.

Manual repros run:
- JSON middleware repro with `Content-Type: text/application/json` returned `status=204`, confirming invalid media type acceptance.
- Recover middleware repro returned `status=200` with a partial body followed by appended `500` Problem JSON, confirming response corruption after partial writes.
- Docs handler repro using `docs.NewHandler(nil)` panicked and returned a recovered `500`, confirming the nil-dependency crash path.

Tests added or updated:
- None. Findings-only audit.

Temporary artifacts:
- All throwaway repro files were created under `mktemp -d` outside the repo and removed after execution.
