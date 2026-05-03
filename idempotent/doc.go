// Package idempotent standardizes HTTP contracts for idempotent API workflows.
//
// The package is stable core API. It does not store idempotency keys or replay
// responses; middleware/idempotency or application code owns reservation,
// persistence, locking, TTLs, and response replay. idempotent provides the
// shared HTTP pieces that handlers, route contracts, and tests can agree on.
//
// Start with RequireKey to extract the Idempotency-Key header and RequestHash to
// compare method, target, and body bytes. Use ConflictProblem, ReplayProblem,
// WriteConflict, WriteAcceptedReplay, and OperationExtensions to keep conflict,
// replay, accepted-operation, and OpenAPI behavior consistent across create,
// update, patch, and asynchronous workflows.
//
// Keep keys scoped to tenant and principal in storage, reject reused keys with
// mismatched hashes, and avoid using this package alone as a replay store. For
// examples, see docs/cookbook.md.
package idempotent
