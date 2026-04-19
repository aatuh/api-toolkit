// Package health provides health utilities.
//
// HTTP contract:
//   - Liveness and readiness should reflect configured checker state rather
//     than silently succeeding on empty probe sets.
//   - Detailed health output is intended for explicitly enabled operational
//     use and may expose dependency-level status details.
//   - When caching is enabled, cached checker results may be reused across
//     liveness, readiness, and detailed responses until CacheDuration expires.
package health
