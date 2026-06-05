// Package idempotency provides stable HTTP idempotency middleware.
// The middleware buffers responses before replay/storage and is therefore not
// suitable for streaming, hijacking, HTTP/2 push, or other handlers that rely
// on optional http.ResponseWriter interfaces. If a completed response cannot be
// persisted for replay, the middleware fails closed with 503 and stores an
// ambiguous state for that key instead of reopening it for another execution.
// Buffered responses that exceed Options.MaxResponseBytes follow the same
// ambiguous-outcome path. The default request hash includes authenticated actor
// and tenant scope when earlier middleware has populated them in request
// context. For multi-tenant APIs backed by shared idempotency storage, set
// Options.StorageKeyFunc to TenantScopedStorageKeyFunc() after auth and tenant
// middleware so the backing store receives a stable hashed key scoped by tenant
// and actor while replay responses still return the original client key.
// Set Options.RequireKey when unsafe write route contracts require an
// Idempotency-Key; when enabled, handled requests without a key fail with a
// Problem Details 400 instead of falling through to the handler.
//
// Token-aware release helpers:
//
//   - Store implementations must implement ports.IdempotencyReservationReleaser
//     so middleware releases only the current tokened in-flight reservation.
//     Maintained stores pass the token-aware adapter contract tests for token
//     mismatch, missing-token legacy cleanup, completed record preservation, and
//     ambiguous record preservation.
//   - OnLegacyInFlightCompatibility or LegacyInFlightCompatibilitySink emits
//     additive mixed-version recovery telemetry with method/path/store correlation for
//     both fallback attempts and outcomes.
//   - By default, OnLegacyInFlightCompatibility is a logger sink when unset, so
//     fallback telemetry is always emitted with low-cardinality fields and stable
//     key redaction behavior.
//   - Legacy compatibility key values are hashed by default. Set
//     LegacyInFlightCompatibilityRawKey=true to opt in to raw key emission.
//   - LegacyInFlightCompatibilityAsync disables request-path coupling to callback
//     execution in high-volume telemetry windows by using one bounded queue and
//     four workers per middleware instance. The queue holds 1024 events, drops
//     new events when full, emits a warning with dropped_events/queue_size, and
//     drains best-effort while the process is running, and emits queued events
//     with cancellation stripped from the first enqueue context so request
//     cancellation does not suppress telemetry delivery. There is no request-path
//     flush or shutdown wait; use a synchronous sink if every telemetry event
//     must be durably observed.
//   - LegacyInFlightCompatibilitySampleEvery emits one event per N emitted events
//     and is the preferred low-cost throttle for high-volume mixed-version windows.
//   - Logger and KnownInFlightTTLs can be used to run startup checks for mixed-
//     version InFlightTTL alignment. Set FailOnInFlightTTLMismatch to fail-fast
//     when rollout rules require strict enforcement.
//   - FailOnInFlightClockSkewPreflight enables strict startup governance for
//     clock-skew sensitive startup preflights while preserving advisory mode by
//     default.
//   - Use LegacyInFlightCompatibilityMetricSink for metrics-first consumers; event
//     labels are exposed through MetricLabels() and use the stable bounded schema:
//     method, store_class, and outcome.
//   - Use OnOutcome for normal request-path idempotency telemetry. OutcomeEvent
//     intentionally omits paths, keys, tenants, request IDs, body data, and raw
//     error strings; MetricLabels() exposes method, store_class, outcome, and
//     status_class.
//
// Recommended event contract:
//
// - `legacy_in_flight_fallback_entered`
// - `legacy_in_flight_fallback_recovered`
// - `legacy_in_flight_fallback_rejected`
// - `legacy_in_flight_fallback_unknown`
//
// Each event should include method/path/store_type/outcome plus an optional key and
// error payload when failures occur.
//
// Practical guidance:
//   - Keep event key hashed by default to limit cardinality.
//   - Set LegacyInFlightCompatibilitySampleEvery when compatibility traffic becomes
//     noisy during mixed-version windows to lower event volume deterministically
//     before the bounded async queue sees the event.
//   - Prefer an explicit metric sink for dashboard counters and keep logger sink
//     in place during transition.
//   - If you rely on request latency, avoid heavy synchronous callback work unless
//     LegacyInFlightCompatibilityAsync is enabled.
//   - Treat async compatibility telemetry as lossy operational evidence. It must
//     never be the only source of correctness for idempotency recovery decisions,
//     and raw keys should stay disabled in production unless a short, access-
//     controlled incident review requires them.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/idempotency`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package idempotency
