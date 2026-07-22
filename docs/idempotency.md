# Idempotency Production Guide

Audience: application developers building retry-safe unsafe HTTP writes.

Use idempotency on POST, PUT, PATCH, and DELETE routes where clients may retry
after network loss, timeouts, or gateway failures. Do not apply response replay
capture to streaming, SSE, websocket, or large-download routes.

## Route Contract

| Requirement | Production guidance |
| --- | --- |
| Required key | Set `middleware/idempotency.Options.RequireKey` for unsafe writes whose route contract requires `Idempotency-Key`. Missing keys should fail with Problem Details 400 before side effects run. |
| Request hash | Include method, route identity, and normalized request body metadata in the request hash so the same key with a different payload becomes a conflict instead of a replay. |
| Tenant scoping | Apply auth and tenant middleware before idempotency, then use `idempotency.TenantScopedStorageKeyFunc()` for shared multi-tenant stores. |
| Replay semantics | Replays should return the stored status, headers, and finite response body for the same key and request hash. |
| Conflict behavior | Same key plus different request hash should return a deterministic conflict response and must not run the handler again. |
| Streaming opt-out | Use `Options.ShouldHandle` to exclude streaming, large-download, and optional-writer routes. |
| Metrics and logs | Emit bounded outcome labels only. Do not log raw `Idempotency-Key` values or replay response bodies. |

## Storage Contract

A production store must preserve these behaviors:

- reserve a key before side effects run,
- prevent two concurrent writers from committing the same key,
- store the request hash and replay response atomically with completion,
- release abandoned reservations only when the reservation token matches,
- keep completed records until the configured TTL expires,
- preserve ambiguous or uncertain records until operator review,
- avoid raw tenant IDs, actor IDs, or idempotency keys in storage keys.

## TTL and Locking

Choose a TTL that covers client retry windows, gateway retry behavior, and
operator replay expectations. Short TTLs reduce storage use but increase the
chance that a late retry runs the write again. Long TTLs improve retry safety
but require retention review for response bodies and headers.

Locking should be token-aware. A stale handler must not release another
handler's reservation. Token mismatch should be observable through bounded
metrics or logs without exposing raw keys.

## Redis Example

Use the contrib Redis adapter for multi-instance services:

```go
store := idempotencyredis.New(client, idempotencyredis.Options{
	KeyPrefix: "my-api:idempotency:",
})
mw, err := idempotency.NewWithStore(store, idempotency.Options{
	RequireKey:     true,
	StorageKeyFunc: idempotency.TenantScopedStorageKeyFunc(),
	ShouldHandle:   idempotency.DefaultShouldHandle,
})
if err != nil {
	return err
}
```

Use service-specific Redis prefixes. Keep Redis keys hashed and bounded.
`NewWithStore` requires `idempotency.ReleasableStore`, making token-aware
reservation release a compile-time requirement. `idempotency.New` and
`Options.Store` remain available only for v4 source compatibility and are
deprecated; migrate before the next major release.

## Configuration groups

New v4 configuration should use focused groups rather than the deprecated
flat fields:

```go
mw, err := idempotency.NewWithStore(store, idempotency.Options{
	Limits: idempotency.Limits{
		MaxBodyBytes:     1 << 20,
		MaxResponseBytes: 1 << 20,
	},
	Retention: idempotency.Retention{
		CompletedTTL: 24 * time.Hour,
		InFlightTTL:  2 * time.Minute,
	},
	Failure: idempotency.FailurePolicy{
		FailOpen: false,
	},
})
if err != nil {
	return err
}
defer mw.Close()
```

`Observability` owns the logger and outcome hook. `Compatibility` contains
temporary mixed-version recovery controls. Its raw-key option is disabled by
default; keep `ExposeRawLegacyKey` off in normal operation. If
`Compatibility.LegacyAsync` is enabled, `Middleware.Close` drains its bounded
telemetry queue during graceful shutdown. Group fields with zero values retain
the package defaults. Do not set both a grouped field and its deprecated flat
equivalent: differing values fail construction with
`ErrAmbiguousConfiguration`.

## Postgres Example

Postgres idempotency storage is application-owned unless a generated service
already provides the store. Use the same contract as Redis:

- unique index on storage key,
- request hash column,
- reservation token column,
- status and finite replay response columns,
- `expires_at` for TTL,
- transaction that reserves before side effects and completes after response
  capture,
- cleanup job that removes expired completed records and leaves uncertain
  records for review.

If a write already uses Postgres transactions for product data, keep the
idempotency reservation and product write in a transaction boundary that cannot
commit product data without either completing the idempotency record or leaving
an explicit uncertain state.

## Test Matrix

- missing `Idempotency-Key` on a required route,
- same key and same request body replays,
- same key and different request body conflicts,
- concurrent same-key writes produce one winner,
- reservation release token mismatch does not delete another reservation,
- expired completed records are not replayed,
- tenant A cannot replay tenant B's key,
- streaming and large-download routes are skipped.
