// Package idempotencyredis provides the supported-adapter Redis idempotency store.
//
// Use New with a redis.UniversalClient and Options when idempotency middleware
// needs shared reservation, replay, TTL, and token-aware release behavior.
// Store implements ports.IdempotencyStore and
// ports.IdempotencyReservationReleaser for generated services and app-owned
// APIs.
//
// Legacy tokenless in-flight recovery emits bounded, hashed-key telemetry by
// default. Keep raw-key recovery disabled in production unless a short,
// access-controlled incident review requires it.
package idempotencyredis
