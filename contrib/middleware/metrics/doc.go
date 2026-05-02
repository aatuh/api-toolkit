// Package metrics provides contrib HTTP metrics middleware.
//
// The middleware records request counts and durations through a supplied
// recorder, using low-cardinality method, route, and status labels. See
// docs/metrics.md for label policy and bootstrap defaults.
package metrics
