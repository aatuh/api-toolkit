# Coverage Hardening Backlog

Audience: maintainers raising maturity evidence for high-risk v3 packages.

This backlog tracks package-specific coverage floors that should be raised only
after behavior tests are merged. Do not raise a threshold just to make the
number look mature; each increase needs tests that exercise security,
operational, or compatibility behavior that users rely on.

| Floor variable | Package | Current focus before raising |
| --- | --- | --- |
| `AUTH_JWT_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/middleware/auth/jwt` | Add or keep behavior tests for issuer and audience rejection, algorithm allowlists, required claims, expired and not-yet-valid tokens, JWKS key misses, skip-header controls, and trusted-proxy bypass rejection. |
| `HEALTH_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/v3/health` | Add or keep behavior tests for liveness/readiness separation, dependency state changes, timeout/error mapping, detailed health redaction, and admin-only detailed output. |
| `CONTRIB_PGXPOOL_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool` | Add or keep behavior tests for startup validation, ping/readiness behavior, plain-value stats snapshots, close semantics, and safe error surfaces. |
| `CONTRIB_OPENAPI_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi` | Add or keep behavior tests for request validation failures, response validation opt-in, Problem Details error mapping, streaming-route opt-out, and large-response bypass. |
| `CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery` | Add or keep behavior tests for signing, endpoint policy, retry classification, async payload redaction, tenant mismatch rejection, replay safety, and low-cardinality metrics. |
| `CONTRIB_WEBHOOKDELIVERYPOSTGRES_COVERAGE_MIN` | `github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres` | Add or keep behavior tests for endpoint lookup, enqueue transaction boundaries, attempt recording, replay scheduling, table-name validation, and readiness health behavior. |

Review rule: raise only after behavior tests are merged, the focused package
test passes locally, and `GOWORK=off GOTOOLCHAIN=local make coverage-check`
passes with the new floor.
