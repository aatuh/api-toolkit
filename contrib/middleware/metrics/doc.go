// Package metrics provides contrib HTTP metrics middleware.
//
// The middleware records request counts and durations through a supplied
// recorder, using low-cardinality method, route, status, route-policy, and
// health-status labels. IdempotencyOutcomeHook and HardTimeoutEventHook wire
// bounded idempotency and hard-timeout event labels to supported recorders. See
// docs/metrics.md for label policy and bootstrap defaults.
package metrics
