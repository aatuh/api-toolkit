// Package health provides health utilities.
//
// HTTP contract:
//   - Liveness and readiness should reflect configured checker state rather
//     than silently succeeding on empty probe sets.
//   - Detailed health output is intended for explicitly enabled operational
//     use and may expose dependency-level status details.
//   - Custom managers can opt into detailed route exposure and cached snapshot
//     middleware behavior by implementing ports.DetailedHealthManager and
//     ports.CachedHealthManager in addition to ports.HealthManager.
//   - When caching is enabled, cached checker results may be reused across
//     liveness, readiness, and detailed responses until CacheDuration expires.
package health
