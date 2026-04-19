// Package txpostgres provides transaction helpers over a DatabasePool.
//
// WithinTx uses the caller context for application work and commit, but
// deferred rollback cleanup switches to a short-lived context without caller
// cancellation so timed-out or canceled requests still attempt to release the
// transaction cleanly.
package txpostgres
