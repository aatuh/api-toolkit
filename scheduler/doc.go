// Package scheduler provides stable background job scheduling primitives.
//
// Runner executes jobs immediately on start and then on their configured
// intervals. Panics inside scheduled jobs are recovered, logged, and recorded
// as failed runs so later intervals can continue. At most one schedule is
// active per job name, even if the same runner is started more than once or
// duplicate named jobs are configured, and same-name jobs never overlap with
// themselves while a prior run is still in flight.
//
// Recorder persistence failures are treated as observability failures rather
// than job failures. The runner should surface them through logging and any
// configured callback hook, but it should not rerun the job immediately or
// stop future intervals after the job function itself has already completed.
// Final run persistence uses a short-lived cleanup context so graceful
// shutdown can still record outcomes after the job function returns.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/scheduler`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package scheduler
