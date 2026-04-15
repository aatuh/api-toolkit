// Package scheduler provides scheduler utilities.
//
// Runner executes jobs immediately on start and then on their configured
// intervals. Jobs with the same name do not overlap with themselves, even if a
// runner is started more than once.
package scheduler
