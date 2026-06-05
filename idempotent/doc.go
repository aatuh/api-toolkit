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
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/idempotent`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package idempotent
