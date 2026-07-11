// Package auditpostgres provides a supported-adapter Postgres audit recorder.
//
// Use New with a contracts.DatabasePool and Options when a service wants audit.Event
// writes stored in Postgres. Store implements audit.Recorder, validates table
// names before SQL construction, clones metadata through the audit contract, and
// can reuse a transaction from request context.
//
// HealthChecker reports database readiness through the same pool. Keep raw
// secrets, request bodies, and high-cardinality identifiers out of audit
// metadata before calling Record.
package auditpostgres
