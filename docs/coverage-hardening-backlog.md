# Coverage Hardening Backlog

Audience: maintainers raising maturity evidence for high-risk v3 packages.

This backlog tracks package-specific coverage floors that should be raised only
after behavior tests are merged. Do not raise a threshold just to make the
number look mature; each increase needs tests that exercise security,
operational, or compatibility behavior that users rely on.

| Floor variable | Package | Current focus before raising |
| --- | --- | --- |
| `AUTH_JWT_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/middleware/auth/jwt` | Keep behavior tests for issuer and audience rejection, algorithm allowlists, required claims, expired and not-yet-valid tokens, JWKS key misses, skip-header controls, trusted-proxy bypass rejection, subject context propagation, and JWKS health. |
| `HEALTH_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/endpoints/health` | Keep behavior tests for liveness/readiness separation, dependency state changes, timeout/error mapping, public detailed-health redaction, admin-only detailed output, dependency checker options, and scheduler updates. |
| `CONTRIB_PGXPOOL_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool` | Add or keep behavior tests for startup validation, ping/readiness behavior, plain-value and legacy stats snapshots, close semantics, acquire failures, and safe error surfaces. |
| `CONTRIB_OPENAPI_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi` | Keep behavior tests for request validation failures, response validation opt-in, Problem Details error mapping, route failure mapping, streaming-route opt-out, large-response bypass, file loading, and safe response buffering. |
| `CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery` | Add or keep behavior tests for signing, endpoint policy, retry classification, transport failures, safe error surfaces, async payload redaction, tenant mismatch rejection, replay safety, and low-cardinality metrics. |
| `CONTRIB_WEBHOOKDELIVERYPOSTGRES_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres` | Add or keep behavior tests for endpoint lookup, secret resolution, enqueue transaction boundaries, attempt recording, replay scheduling, table-name validation, missing-row behavior, sanitized persistence, and readiness health behavior. |

Review rule: raise only after behavior tests are merged, the focused package
test passes locally, and `GOWORK=off GOTOOLCHAIN=local make coverage-check`
passes with the new floor.
