// Package idempotency provides a contrib in-memory idempotency store.
//
// The memory store is useful for local development, tests, and examples. It is
// not a durable production store; production APIs should use storage with shared
// process visibility and retention aligned with retry windows.
package idempotency
