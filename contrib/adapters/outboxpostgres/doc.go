// Package outboxpostgres provides a supported-adapter Postgres transactional outbox.
//
// Use New with a contracts.DatabasePool and Options when async workers need an
// async.Store backed by Postgres. Store preserves enqueue, lease, complete,
// retry/dead-letter, stuck-lease recovery, table-name validation, and readiness
// health behavior.
//
// Events must pass validation before enqueue. Failure messages are kept
// low-secret and lease completion requires the current lease owner so stale
// workers cannot complete another worker's job.
package outboxpostgres
