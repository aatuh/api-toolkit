// Package metrics provides contrib HTTP metrics middleware.
//
// The middleware records request counts and durations through a supplied
// recorder, using low-cardinality method, route, status, route-policy, and
// health-status labels. IdempotencyOutcomeHook wires bounded idempotency
// outcome labels to supported recorders. See docs/metrics.md for label policy
// and bootstrap defaults.
package metrics
