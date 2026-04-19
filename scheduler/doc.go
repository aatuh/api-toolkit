// Package scheduler provides scheduler utilities.
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
package scheduler
