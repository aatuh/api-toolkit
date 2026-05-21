# Coverage Hardening Backlog

Audience: maintainers raising maturity evidence for high-risk v3 packages.

This backlog tracks package-specific coverage floors that should be raised only
after behavior tests are merged. Do not raise a threshold just to make the
number look mature; each increase needs tests that exercise security,
operational, or compatibility behavior that users rely on.

| Floor variable | Package | Current focus before raising |
| --- | --- | --- |
| `AUTH_JWT_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/middleware/auth/jwt` | Keep behavior tests for issuer and audience rejection, algorithm allowlists, required claims, expired and not-yet-valid tokens, JWKS key misses, skip-header controls, trusted-proxy bypass rejection, subject context propagation, and JWKS health. |
| `ENDPOINTS_DOCS_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/endpoints/docs` | Keep behavior tests for strict first-party docs HTML, provider format gating, disabled OpenAPI formats, YAML/JSON content types, escaped template values, and docs route registration. |
| `HEALTH_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/endpoints/health` | Keep behavior tests for liveness/readiness separation, dependency state changes, timeout/error mapping, public detailed-health redaction, admin-only detailed output, dependency checker options, and scheduler updates. |
| `HTTPX_IDENTITY_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/httpx/identity` | Keep behavior tests for trusted proxy parsing, forwarded host/scheme handling, request IDs, exact-address trusted proxies, invalid proxy inputs, and hostile header fallback. |
| `HTTPX_RECOVER_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/httpx/recover` | Keep behavior tests for uncommitted panic Problem Details, committed-response aborts, nil logger behavior, stack logging controls, and `http.ErrAbortHandler` propagation. |
| `JSON_MIDDLEWARE_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/middleware/json` | Keep behavior tests for body-bearing method detection, unsupported media Problem Details, structured suffix media types, strict decoder nil-body handling, unknown-field rejection, and successful decode paths. |
| `MAXBODY_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/middleware/maxbody` | Keep behavior tests for positive limits, nil middleware/body behavior, middleware adapter wrapping, and oversized request reads. |
| `QUERYLIMITS_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/middleware/querylimits` | Keep behavior tests for parameter count, key/value length, limit parsing, custom limit names, custom error writers, and nil middleware behavior. |
| `SECURITYPROFILE_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/securityprofile` | Keep behavior tests for auth requirements, allowlists, custom error writers, dev-bypass fail-closed behavior, route overrides, hard-timeout streaming escapes, and OWASP limits. |
| `CONTRIB_PGXPOOL_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool` | Add or keep behavior tests for startup validation, ping/readiness behavior, plain-value and legacy stats snapshots, close semantics, acquire failures, and safe error surfaces. |
| `CONTRIB_OPENAPI_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi` | Keep behavior tests for request validation failures, response validation opt-in, Problem Details error mapping, route failure mapping, streaming-route opt-out, large-response bypass, file loading, and safe response buffering. |
| `CONTRIB_METRICS_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` | Keep behavior tests for bounded labels, status/byte recording, optional response-writer interfaces, informational statuses, committed-state behavior, and unsupported interface fallbacks. |
| `CONTRIB_OTELTRACE_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/middleware/oteltrace` | Keep behavior tests for trace/correlation headers, route naming, span status mapping, status/byte recording, optional response-writer interfaces, informational statuses, and unsupported interface fallbacks. |
| `CONTRIB_REQUESTLOG_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog` | Keep behavior tests for redaction, bounded labels, panic-after-commit status reporting, status/byte recording, optional response-writer interfaces, informational statuses, and unsupported interface fallbacks. |
| `CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery` | Add or keep behavior tests for signing, endpoint policy, retry classification, transport failures, safe error surfaces, async payload redaction, tenant mismatch rejection, replay safety, and low-cardinality metrics. |
| `CONTRIB_WEBHOOKDELIVERYPOSTGRES_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres` | Add or keep behavior tests for endpoint lookup, secret resolution, enqueue transaction boundaries, attempt recording, replay scheduling, table-name validation, missing-row behavior, sanitized persistence, and readiness health behavior. |

Review rule: raise only after behavior tests are merged, the focused package
test passes locally, and `GOWORK=off GOTOOLCHAIN=local make coverage-check`
passes with the new floor. Use `reference-service-coverage` for generated
service diagnostics; do not fold app-owned generated code into the toolkit
root/contrib aggregate thresholds.
