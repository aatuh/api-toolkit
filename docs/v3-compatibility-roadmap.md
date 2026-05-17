# V3 Compatibility Roadmap

Audience: maintainers planning major-version cleanup while preserving v2 source
compatibility until a v3 branch exists.

This document keeps v2 compatibility shims explicit so new code can use the
preferred APIs while v2 source compatibility remains intact.

## V3 removal matrix

| Surface | Current v2 API | Preferred v2 API | V3 action | Required tests | Removal condition |
| --- | --- | --- | --- | --- | --- |
| Provider-shaped billing ports | Deprecated exports in `ports/billing.go`, including `ports.PaymentProvider`, `ports.BillingProvider`, `ports.CheckoutSessionRequest`, invoice types, portal flow types, and webhook types. | `github.com/aatuh/api-toolkit/v3/compat/billing`, or app-owned billing ports when the hosted-checkout model is not a fit. | Remove billing exports from generic `ports` or keep them only in an explicit compatibility package. | `compat/billing` parity tests against the deprecated `ports` aliases; docscheck guardrails that examples do not teach deprecated `ports` billing types. | A major version is available, migration notes point callers to `compat/billing` or app-owned ports, and release notes cover the extraction. |
| Driver-shaped database stats | `ports.DatabasePool.Stat` and `ports.DatabaseStats` with pgx-style counters. | `ports.DatabasePoolSnapshotProvider`, `ports.SnapshotDatabasePoolStats`, `ports.SnapshotDatabaseStats`, and adapter `StatSnapshot()` methods. | Remove `DatabasePool.Stat` from the generic pool contract and keep driver-shaped counters in adapters or compatibility packages. | Root snapshot tests and pgxpool adapter snapshot tests; docscheck guardrails that examples prefer snapshots over direct `DatabaseStats`. | All maintained adapters expose plain-value snapshots and docs/examples no longer rely on direct legacy stats. |
| Legacy response helpers | `github.com/aatuh/api-toolkit/v3/response_writer`, including `WriteJSON`, `WriteErr`, `Capture`, and `Writer`. | `github.com/aatuh/api-toolkit/v3/httpx` for JSON, Problem Details, and response helpers. | Retire `response_writer` from the stable core surface or move it behind an explicit compatibility path. | `httpx` examples and response tests; docscheck guardrails that examples do not import `response_writer`. | Major version migration notes identify `httpx` replacements and no current examples depend on `response_writer`. |
| Tokenless idempotency release | `ports.IdempotencyReleaser.Release(ctx, key)` remains for source compatibility. | `ports.IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)` and `ports.ReservationReleasableIdempotencyStore`. | Make token-aware release the primary middleware release contract and remove tokenless recovery from new paths. | `contrib/adapters/idempotencytest` contract coverage and `ports.IdempotencyReservationReleaser` compile examples. | Maintained stores pass token-aware release contracts, mixed-version telemetry shows no tokenless fallback use for the supported rollout window, and release notes document custom-store migration. |
| Idempotency compatibility telemetry labels | `LegacyInFlightCompatibilityEvent` keeps path, key, store type, outcome, and error fields for structured events. | `MetricLabels()` exposes only bounded `method`, `store_class`, and `outcome` labels; high-cardinality details stay in logs or traces. | Remove mixed-version compatibility metric helpers after tokenless fallback support is retired. | Metric label contract tests and metrics docs that forbid path, key, error, tenant, request, or raw user-input labels. | Fallback events remain zero for the agreed support window and dashboards no longer depend on compatibility metrics. |
| Hard-timeout response capture | `middleware/timeout.HardTimeout` buffers responses so timeout responses can win over late writes. | `Options.MaxCaptureBytes` with a safe default, route-level opt-out for streaming, and Problem Details on overflow. | Revisit whether hard timeout should be opt-in only for non-streaming routes or expose richer route-level capture policy. | Large-response, timeout, late-write, and streaming limitation docs/tests. | Streaming/SSE/websocket guidance is explicit and maintained route profiles avoid hard timeout on streaming handlers. |
| Admin endpoint registration ergonomics | Source-compatible pprof and detailed-health helpers can still mount operator-only details without an explicit wrapper. | `pprof.RegisterAdminRoutes` and `Handler.RegisterAdminDetailedHealthRoute` for new admin mounts. | Remove or further de-emphasize policy-free admin route registration from primary examples. | Docscheck guardrails that public docs/examples do not teach policy-free admin mounts. | Public docs and examples consistently separate public probes from admin-only pprof, detailed health, and metrics. |
| Unchecked authz constructor | `middleware/auth/authz.NewRequireRoleMiddleware` keeps the v2-compatible single-return constructor. | `middleware/auth/authz.NewRequireRoleMiddlewareChecked`, `ValidateRequireRoleMiddleware`, and `ValidateRequireRoleMiddlewareRoutes`. | Make startup validation mandatory in the primary constructor shape or remove unchecked construction from new docs. | Authz checked-constructor examples and route validation tests. | Major version allows constructor signature change and route validation guidance is present in release notes. |
| Checked list parser shims | `endpoints/list.ParseListQuery`, `DefaultFilterParser`, and `DefaultSortParser` keep single-return compatibility. | `ParseListQueryChecked`, `DefaultFilterParserChecked`, and `DefaultSortParserChecked`. | Prefer checked parser APIs in examples and consider de-emphasizing unchecked parser helpers. | List parser tests for checked field errors and example coverage that uses checked parsing. | Examples and docs consistently teach checked parser APIs where validation errors matter. |

## V3 owner checklist

Each compatibility-sensitive surface needs an owner-ready removal note before a
major version branch removes it.

| Surface | Removal trigger | Required tests | Release-note requirements |
| --- | --- | --- | --- |
| Provider-shaped billing ports | v3 branch opens after callers have a documented `compat/billing` or app-owned port migration path. | Alias parity tests and docscheck guardrails that guides/examples avoid deprecated billing ports. | `docs/release-notes.md` must include extraction impact, replacement imports, and upgrade notes. |
| Driver-shaped database stats | All maintained adapters expose plain-value snapshot APIs and examples avoid direct legacy stats. | Root snapshot tests, pgxpool snapshot tests, and docscheck rules that examples prefer snapshots. | Release notes must call out generic pool contract removal and adapter snapshot replacements. |
| Legacy response helpers | New docs/examples use `httpx` for success, error, capture, and recovery guidance; idempotency response capture has a non-legacy migration path. | `httpx` response tests, idempotency replay/capture tests, and docscheck rules that examples avoid the legacy package. | Release notes must map legacy helper names and idempotency capture behavior to `httpx` or internal replacements. |
| Tokenless idempotency release | Mixed-version rollout telemetry shows the tokenless recovery path is no longer needed and maintained stores implement token-aware release. | `contrib/adapters/idempotencytest` contract coverage for token-aware release, stale legacy cleanup, token mismatch handling, and adapter-specific observability labels. | Release notes must describe token-aware release requirements for custom stores, legacy telemetry removal, and rollback expectations for mixed-version deployments. |
| Unchecked authz constructor | Constructor signature changes are allowed in the v3 branch. | Checked-constructor and route-validation tests, including startup failure behavior. | Release notes must describe startup validation and rollback guidance. |
| Checked list parser shims | Guides/examples consistently use checked parser APIs where field errors matter. | Checked parser field-error tests and example coverage. | Release notes must identify unchecked helper de-emphasis or removal and checked replacements. |

Owners should update `docs/release-notes.md`, this roadmap, and docscheck
requirements in the same change that removes or de-emphasizes a compatibility
surface.

## V3 migration notes for compatibility removals

These notes prepare consumers for a future major version. They do not remove v2
compatibility symbols.

| V2 compatibility surface | Preferred v2 migration target | V3 migration note |
| --- | --- | --- |
| Deprecated provider-shaped billing exports in `ports/billing.go`. | Use `github.com/aatuh/api-toolkit/v3/compat/billing` for the existing hosted-checkout model, or define an app-owned billing port for provider-neutral code. | Expect generic `ports` billing exports to move out of core or be removed; map each used symbol to `compat/billing` or to your app-owned port before upgrading. |
| `DatabasePool.Stat` and `DatabaseStats`. | Use `DatabasePoolSnapshotProvider`, `SnapshotDatabasePoolStats`, `SnapshotDatabaseStats`, or adapter `StatSnapshot()` methods. | Expect driver-shaped counters to leave the generic pool contract; observability code should consume plain-value snapshots. |
| `response_writer` helpers and wrappers. | Use `httpx` for JSON and Problem Details responses, and package-local capture wrappers for middleware internals. | Expect the public legacy helper package to be removed or moved behind explicit compatibility; root and contrib runtime code no longer depends on it. |
| Tokenless idempotency `Release(ctx, key)`. | Implement `ports.IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)` and pass `contrib/adapters/idempotencytest` contracts. | Expect token-aware release to become the primary store contract after mixed-version fallback telemetry reaches zero for the agreed support window. |

## Executable v3 evidence requirements

The removal matrix above is guarded by docscheck so every listed surface keeps a
current v2 API, preferred v2 API, v3 action, required tests, and removal
condition. The following evidence names are the minimum release-review signals
that must exist before a v3 branch removes compatibility code.

| Surface | Evidence source | Required signal | Acceptance gate |
| --- | --- | --- | --- |
| Provider-shaped billing ports | `compat/billing` tests, docscheck legacy-code-snippet guardrails, and release notes. | Deprecated `ports/billing.go` usage appears only in compatibility aliases/tests or migration prose. | New examples use `compat/billing` or app-owned ports, and release notes map each removed ports symbol. |
| Driver-shaped database stats | Root snapshot tests, pgxpool adapter tests, docscheck legacy-code-snippet guardrails, and release notes. | Direct `DatabaseStats` usage appears only in compatibility adapters/tests or migration prose. | Maintained adapters expose plain-value snapshots and examples use `DatabasePoolSnapshotProvider`, `SnapshotDatabasePoolStats`, or adapter `StatSnapshot()`. |
| Legacy response helpers | `docs/response-writer-inventory.md`, response tests, idempotency capture tests, and docscheck inventory checks. | No root or contrib runtime imports of `response_writer` remain; the public package is retained only as the v2 compatibility surface. | No docs/examples import `response_writer` as preferred guidance, and package-local or `httpx` replacements cover current internal capture needs. |
| Tokenless idempotency release | `contrib/adapters/idempotencytest` contracts, adapter test output, telemetry dashboards, and release notes. | `adapter_contract_status=passed`, both maintained stores (`contrib/adapters/idempotency` and `contrib/adapters/idempotencyredis`) implement token-aware release, store telemetry labels `legacy_in_flight_recovered` and `legacy_in_flight_token_mismatch` remain explicit, and middleware telemetry labels `legacy_in_flight_fallback_entered`, `legacy_in_flight_fallback_recovered`, `legacy_in_flight_fallback_rejected`, and `legacy_in_flight_fallback_unknown` remain available through the rollout. | The support-window signal shows zero tokenless fallback events for the agreed support window, dashboards no longer depend on legacy labels, and release notes document custom-store migration and rollback. |
| Unchecked authz constructor | Checked-constructor tests, route validation tests, docscheck guidance checks, and release notes. | Startup validation examples use checked constructors or route validation helpers. | Primary v3 constructor behavior is validated at startup and fail-closed request behavior remains covered. |
| Checked list parser shims | Checked parser field-error tests, docs examples, and release notes. | Examples use checked parser APIs where validation errors matter. | Release notes identify unchecked helper de-emphasis or removal and checked replacements. |

## V3 implementation tracks

Keep v2 source compatibility intact until the v3 branch. These tracks are
execution notes for the major-version branch, not permission to break v2.

### Provider-shaped billing ports removal

1. Inventory all repository and example imports of deprecated `ports/billing.go`
   exports and keep `compat/billing` parity tests green before branching.
2. In the v3 branch, remove billing exports from generic `ports` or move the
   existing hosted-checkout model fully behind a compatibility package.
3. Update examples to use app-owned billing ports unless the
   `compat/billing` hosted-checkout model is intentional.
4. Add release notes mapping each removed `ports` billing symbol to
   `compat/billing` or to app-owned ports.

### Database stats compatibility removal

1. Confirm maintained adapters expose plain-value snapshots through
   `DatabasePoolSnapshotProvider`, `SnapshotDatabasePoolStats`, or adapter
   `StatSnapshot()` methods.
2. In the v3 branch, remove `DatabasePool.Stat` from the generic pool contract
   and keep driver-shaped counters inside adapters or compatibility packages.
3. Update observability examples to depend on snapshot values instead of
   `DatabaseStats`.
4. Keep root snapshot tests and pgxpool snapshot tests as the migration gate.

### Legacy response writer removal

1. Confirm docs and examples use `httpx` for JSON success, Problem Details,
   response capture, and wrapper behavior.
2. Keep `middleware/idempotency` on its package-local response capture helper
   instead of importing `response_writer`. Keep replay semantics, response-size
   limits, ambiguous-state handling, and header allow/deny behavior under tests.
3. Keep `httpx/recover` and maintained contrib middleware on package-local
   response recorders instead of importing `response_writer`.
4. In the v3 branch, remove `response_writer` from the stable core surface or
   move it into an explicitly named compatibility path.
5. Map `response_writer.WriteJSON` to `httpx.WriteJSON`, `WriteErr` to
   `httpx.WriteProblem`, and capture/wrapper helpers to the equivalent `httpx`
   response helpers in release notes.
6. Keep `httpx` examples, idempotency replay tests, and response tests as the
   removal gate.

### Compatibility shim removal

1. Make token-aware idempotency release the primary v3 store contract and remove
   tokenless release fallback from new middleware paths after mixed-version
   telemetry is no longer needed.
2. Change or de-emphasize unchecked authz construction so startup validation is
   the normal v3 path while preserving fail-closed request behavior.
3. Teach checked list parser APIs in examples and remove unchecked parser
   helpers only after release notes identify checked replacements.
4. Keep adapter contract tests, authz checked-constructor tests, route
   validation tests, and checked list parser field-error tests as the v3 gate.

## Idempotency release semantics

Current v2 compatibility state:

- `ports/idempotency.go` keeps `IdempotencyReleaser.Release(ctx, key)` as the
  source-compatible tokenless release contract.
- `ports.IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)` is the preferred token-aware release path for new stores.
- `middleware/idempotency` uses token-aware release when the store implements
  `IdempotencyReservationReleaser`, and preserves legacy tokenless recovery
  telemetry for mixed-version rollouts.
- `contrib/adapters/idempotencytest` owns reusable adapter contract coverage for
  token-aware release, completed-record preservation, ambiguous-record
  preservation, legacy tokenless recovery, and token mismatch handling.
- Maintained contrib stores are `contrib/adapters/idempotency` and
  `contrib/adapters/idempotencyredis`; both pass the shared contract suite.
- Store-level migration telemetry labels are `legacy_in_flight_recovered` and
  `legacy_in_flight_token_mismatch`.
- Redis `ReleaseReservation` uses an atomic compare-and-delete operation so a
  stale releaser cannot delete a successor in-flight reservation after expiry or
  replacement.

Preferred v2 guidance:

- New stores should implement `ports.ReservationReleasableIdempotencyStore`.
- Legacy tokenless records should remain a narrow mixed-version recovery path,
  with hashed-key telemetry enabled by default.
- Adapter recovery telemetry should keep raw-key output disabled by default and
  use the explicit raw-key option only for short, access-controlled incident
  review.
- Rollouts should align `InFlightTTL` values and use the startup preflight
  checks in `middleware/idempotency` before enabling strict failure behavior.

Concrete v3 sunset criteria for tokenless `Release(ctx, key)`:

- Every maintained adapter must implement
  `ports.IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)` and
  pass the shared adapter contract for token mismatch, missing-token legacy
  cleanup, completed-record preservation, and ambiguous-record preservation.
- Operators must have a mixed-version rollout window with telemetry for
  `legacy_in_flight_fallback_entered`, `legacy_in_flight_fallback_recovered`,
  `legacy_in_flight_fallback_rejected`, and `legacy_in_flight_fallback_unknown`.
- Tokenless fallback removal can proceed only after fallback events remain at
  zero for the agreed support window across maintained deployments, dashboard
  and alert rules no longer depend on legacy labels, and release notes document
  custom-store migration and rollback expectations.

V3 cleanup checklist:

- [ ] Make token-aware release the primary release contract after every
  maintained adapter passes the shared token-aware release contract suite.
- [ ] Remove the need for tokenless `Release(ctx, key)` from new middleware
  paths.
- [ ] Retire legacy tokenless in-flight recovery once v2 mixed-version rollout
  telemetry shows no fallback events for the agreed support window and
  dashboards no longer depend on legacy labels.
- [ ] Keep adapter contract tests as the source of truth for release semantics,
  including token mismatch, missing-token legacy cleanup, completed-record
  preservation, and ambiguous-record preservation.
- [ ] Remove compatibility telemetry that only exists for tokenless recovery
  after the legacy path is gone.
- [ ] Publish release notes that tell custom stores to implement
  `ports.IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)`
  before upgrading.

## Authz constructor validation

Current v2 compatibility state:

- `middleware/auth/authz.NewRequireRoleMiddleware` keeps the v2-compatible
  single-return constructor and fails closed at request time when configuration
  is invalid.
- `middleware/auth/authz.NewRequireRoleMiddlewareChecked` is the preferred
  constructor for new code because it returns startup configuration errors.
- `ValidateRequireRoleMiddleware` and `ValidateRequireRoleMiddlewareRoutes`
  let applications validate route registrations during bootstrap.
- `contrib/adapters/chi` provides route-scanning helpers for chi applications.

Preferred v2 guidance:

- New code should prefer `NewRequireRoleMiddlewareChecked`.
- Applications should validate protected route registrations during startup and
  fail before serving traffic when authz middleware is malformed or missing.
- Keep the single-return constructor only where source compatibility with
  existing v2 callers matters.

V3 cleanup checklist:

- [ ] Remove or de-emphasize unchecked constructor paths from new documentation.
- [ ] Consider making constructor validation mandatory by returning errors from
  the primary constructor.
- [ ] Keep route validation helpers for applications that need registry-wide
  startup checks.
- [ ] Preserve fail-closed request behavior even when startup validation is
  accidentally skipped.
