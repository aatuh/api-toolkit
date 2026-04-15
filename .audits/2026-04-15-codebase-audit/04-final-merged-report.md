# Final Merged Audit Report

## Executive summary
This report merges three passes over the repository:
- `bug-hunter`
- `rest-api-expert`
- `code-review-audit`

Across all three passes, the same pattern held: the repository is architecturally decent, but the highest-risk problems live at the HTTP edge and in default composition. The codebase is reusable in principle, yet several default behaviors are not stable enough to serve as a production baseline without further tightening.

The strongest shared conclusion is that this is not a repo with generic architectural collapse. It has a workable ports-and-adapters core, small packages, and good low-level ergonomics. The failure mode is sharper: public HTTP behavior is inconsistent between middleware, helpers, bootstrap defaults, and examples, and some composition-level bugs escaped because the current tests focus more on leaf packages than on integration points.

Verification completed during the audit included `make test`, `make test-race`, `go test ./...`, `cd contrib && go test ./...`, and targeted external repros for suspected defects. All standard tests passed. Several issues below were confirmed despite that green baseline.

## Combined findings
### Confirmed defects
- Duplicate default metrics registration can panic on repeated initialization. Evidence: `contrib/middleware/metrics/metrics.go:91`, `contrib/bootstrap/profile.go:161`, `contrib/bootstrap/profile.go:252`, `contrib/bootstrap/http.go:47`. Verified with a temporary repro that calling the default recorder constructor twice against the default registerer panics.
- Recovery middleware corrupts already-started responses. Evidence: `httpx/recover/recover.go:14`. Verified with a repro that wrote `200 OK` and partial body before panic; the middleware appended `500` Problem JSON to the committed response.
- JSON media-type validation is broken. Evidence: `middleware/json/json.go:73`. Verified with a repro that `Content-Type: text/application/json` is accepted.
- Docs handlers can panic from nil-manager construction. Evidence: `endpoints/docs/handlers.go:17`, `endpoints/docs/handlers.go:35`, `endpoints/docs/handlers.go:49`, `endpoints/docs/handlers.go:93`. Verified with a repro using `docs.NewHandler(nil)`.

### Contract issues
- Strict JSON mode returns plain-text `415` instead of Problem Details and applies content-type enforcement too broadly. Evidence: `middleware/json/json.go:46`, `contrib/bootstrap/profile.go:189`.
- Authn and authz semantics are inconsistent across JWT, role, and tenant middleware. Evidence: `middleware/auth/jwt/middleware.go:134`, `middleware/auth/authz/require_role.go:40`, `middleware/auth/tenant/tenant.go:76`, `middleware/auth/tenant/tenant.go:167`.
- The spec-first example documents an error schema that does not match actual runtime behavior. Evidence: `contrib/examples/spec-first/openapi.json:25`, `contrib/examples/spec-first/openapi.json:112`, `contrib/middleware/openapi/error_mapping.go:42`, `httpx/field_errors.go:29`.
- List parsing defaults malformed inputs instead of rejecting them, while the public pagination example rejects them with `400`. Evidence: `endpoints/list/list.go:73`, `endpoints/list/list.go:148`, `contrib/examples/pagination/main.go:62`.

### Structural and operational concerns
- Timeout handling is documented as a resource-limit control, but the implementation only sets a context deadline and does not enforce response termination for non-cooperative handlers. Evidence: `middleware/timeout/timeout.go:10`, `README.md:374`, `docs/security.md:44`. Verified with a repro.
- `ProfileStrictAPI` does not include query-limit middleware despite the docs positioning query limits as baseline hardening. Evidence: `contrib/bootstrap/profile.go:206`, `docs/security.md:48`.
- Error responses do not include correlation data even though request logging and tracing do.
- Important packages still lack direct tests: `contrib/bootstrap`, `middleware/auth/jwt`, `httpx/recover`, `endpoints/docs`, and `scheduler`.

## Merged assessment
### What is strong
- Ports and adapters are mostly placed correctly.
- Package boundaries are readable and pragmatic.
- Problem Details support is fundamentally sound.
- JWT hardening and outbound SSRF controls are better than average.
- Route-based metrics and structured request logging are the right default observability primitives.

### What is weak
- Client-visible contract ownership is diffuse.
- Defaults are not yet "boring" enough to be trusted blindly.
- Examples and docs sometimes contradict actual runtime semantics.
- Several composition-level failures are currently untested.

## Priority fix order
1. Fix the default metrics initialization panic.
2. Fix panic recovery so it never appends error payloads after the response is committed.
3. Fix JSON media-type parsing and normalize `415` responses to RFC 9457.
4. Standardize `401` versus `403` semantics across auth-related middleware.
5. Align the spec-first example and public docs with actual error envelopes.
6. Decide and document the real timeout model, then implement it consistently.
7. Add integration tests for bootstrap, recovery, docs handlers, and JWT middleware.

## Final verdict
Overall quality: mixed but salvageable.

This is a better codebase than its current audit score suggests, because the underlying structure is workable. The priority work is concentrated and practical: fix four confirmed defects, converge on one explicit REST contract, and add tests around the exact places where downstream services will trust the toolkit most.
