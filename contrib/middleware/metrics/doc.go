// Package metrics provides contrib HTTP metrics middleware.
//
// The middleware records request counts and durations through a supplied
// recorder, using low-cardinality method, route, status, route-policy, and
// health-status labels. Route labels come from chi route patterns or
// net/http ServeMux request patterns, never raw paths when the router exposes a
// matched pattern. RoutePolicyLabels exposes the same bounded route-policy
// label shape for custom recorders. IdempotencyOutcomeHook and
// HardTimeoutEventHook wire bounded idempotency and hard-timeout event labels to
// supported recorders. See docs/metrics.md for label policy and bootstrap
// defaults.
package metrics
