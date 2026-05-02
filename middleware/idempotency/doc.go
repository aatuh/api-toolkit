// Package idempotency provides idempotency utilities.
// The middleware buffers responses before replay/storage and is therefore not
// suitable for streaming, hijacking, HTTP/2 push, or other handlers that rely
// on optional http.ResponseWriter interfaces. If a completed response cannot be
// persisted for replay, the middleware fails closed with 503 and stores an
// ambiguous state for that key instead of reopening it for another execution.
// Buffered responses that exceed Options.MaxResponseBytes follow the same
// ambiguous-outcome path. The default request hash includes authenticated actor
// and tenant scope when earlier middleware has populated them in request
// context.
//
// Compatibility helpers:
//
//   - Store implementations should preserve ports.IdempotencyReleaser.Release(ctx, key)
//     for v2 source compatibility and add ports.IdempotencyReservationReleaser
//     when they can safely release only the current tokened in-flight reservation.
//     The v3 sunset criteria are: maintained stores pass the token-aware adapter
//     contract tests for token mismatch, missing-token legacy cleanup, completed
//     record preservation, and ambiguous record preservation; mixed-version rollout
//     telemetry shows no tokenless fallback events for the agreed support window;
//     and release notes document migration and rollback expectations for custom
//     stores.
//   - OnLegacyInFlightCompatibility or LegacyInFlightCompatibilitySink emits
//     additive mixed-version recovery telemetry with method/path/store correlation for
//     both fallback attempts and outcomes.
//   - By default, OnLegacyInFlightCompatibility is a logger sink when unset, so
//     fallback telemetry is always emitted with low-cardinality fields and stable
//     key redaction behavior.
//   - Legacy compatibility key values are hashed by default. Set
//     LegacyInFlightCompatibilityRawKey=true to opt in to raw key emission.
//   - LegacyInFlightCompatibilityAsync disables request-path coupling to callback
//     execution in high-volume telemetry windows, at the cost of deferred error
//     surfacing in the caller path.
//     It is intentionally fire-and-forget and can start one goroutine per emitted
//     event after sampling. Use LegacyInFlightCompatibilitySampleEvery or a
//     bounded custom sink during high-volume mixed-version migrations.
//   - LegacyInFlightCompatibilitySampleEvery emits one event per N emitted events
//     and is the preferred low-cost throttle for high-volume mixed-version windows.
//   - Logger and KnownInFlightTTLs can be used to run startup checks for mixed-
//     version InFlightTTL alignment. Set FailOnInFlightTTLMismatch to fail-fast
//     when rollout rules require strict enforcement.
//   - FailOnInFlightClockSkewPreflight enables strict startup governance for
//     clock-skew sensitive startup preflights while preserving advisory mode by
//     default.
//   - Use LegacyInFlightCompatibilityMetricSink for metrics-first consumers; event
//     labels are exposed through MetricLabels() and use a stable schema:
//     method, path, store_type, outcome, key, and error.
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
//     noisy during mixed-version windows to lower event volume deterministically.
//   - Prefer an explicit metric sink for dashboard counters and keep logger sink
//     in place during transition.
//   - If you rely on request latency, avoid heavy synchronous callback work unless
//     LegacyInFlightCompatibilityAsync is enabled.
//   - Treat async compatibility telemetry as lossy operational evidence. It must
//     never be the only source of correctness for idempotency recovery decisions.
package idempotency
