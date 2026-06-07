# V3 Compatibility Record

Audience: maintainers reviewing the completed v3 cleanup and the guardrails
that keep migration-shaped APIs from returning to generic core surfaces.

v3 removed or relocated the known compatibility-only surfaces from the v2 line.
This document records the decision evidence and the replacement APIs that new
code should use.

## V3 Removal Matrix

| Surface | v3 status | Replacement | Required evidence |
| --- | --- | --- | --- |
| Provider-shaped billing ports | Removed from generic `ports`; historical `ports/billing.go` hosted-checkout model is explicit in `compat/billing`. | `github.com/aatuh/api-toolkit/v3/compat/billing` or app-owned billing ports. | `compat/billing` tests, docscheck legacy-code-snippet guardrails, and release notes mapping removed symbols. |
| Driver-shaped database stats | `ports.DatabasePool.Stat` and `ports.DatabaseStats` were removed from the generic pool contract. | `DatabasePoolSnapshotProvider`, `ports.SnapshotDatabasePoolStats`, `SnapshotDatabaseStats`, and adapter `StatSnapshot()` methods. | Root snapshot tests, pgxpool adapter tests, and docscheck rules that examples prefer snapshots. |
| Legacy response helpers | Public `response_writer` package removed. | `github.com/aatuh/api-toolkit/v3/httpx` and package-local response recorders. | `httpx` response tests, idempotency capture tests, and docscheck rules that examples avoid the removed package. |
| Tokenless idempotency release | Legacy `IdempotencyReleaser.Release(ctx, key)` behavior was replaced by token-aware reservation release. | `ports.IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)`. | Idempotency adapter contract tests and bounded, redacted compatibility telemetry. |
| Root rate limiter port | `ports.RateLimiter` remains available for v3 source compatibility, but new code should migrate to the package-local shim. | `middleware/ratelimit.Limiter`, an alias over the v3 port. | Compile-checked `ExampleLimiter`, API inventory, and docscheck shim coverage. |
| Unchecked authz construction | Checked startup validation is the documented path. | `middleware/auth/authz.NewRequireRoleMiddlewareChecked` and `ValidateRequireRoleMiddlewareRoutes`. | Authz checked-constructor examples and route validation tests. |
| Unchecked list parsing | Checked parser APIs are the documented path when validation matters. | `ParseListQueryChecked`, `DefaultFilterParserChecked`, and `DefaultSortParserChecked`. | Parser validation tests and docs/examples using checked helpers. |

## V3 Owner Checklist

- Keep `docs/ports-surface.md`, `VERSIONING.md`, package classification, and
  release notes aligned when stable surfaces change.
- Keep docscheck legacy-code-snippet guardrails active for removed billing,
  database stats, and response helper APIs.
- Keep replacement imports visible in user-facing docs: `compat/billing`,
  `DatabasePoolSnapshotProvider`, `SnapshotDatabasePoolStats`, adapter
  `StatSnapshot()`, `middleware/ratelimit.Limiter`, and `httpx`.
- Keep idempotency compatibility telemetry bounded: no raw paths, keys,
  provider secrets, or high-cardinality error values in labels.
- Keep streaming, SSE, websocket, and large-download timeout caveats visible in
  README, `docs/security.md`, and middleware docs.

## Remaining Guardrails

The v3 branch is allowed to remove v2 compatibility-only surfaces, but new v3
minor and patch releases must preserve the stable package list in
`VERSIONING.md`. Any future cleanup that breaks exported stable APIs requires a
new major version, release notes, API-diff evidence, and package-classification
updates.
