// Package cacheredis provides a supported-adapter Redis cache store.
//
// Use New with a redis.UniversalClient and Options to satisfy cache.Store for
// generated services or app-owned cache integrations. The adapter applies an
// optional key prefix, default TTL, miss handling, delete semantics, and
// defensive byte-slice copies.
//
// HealthChecker checks Redis readiness without promoting cache keys to metrics
// or logs. Keep prefixes service-specific when a Redis deployment is shared.
package cacheredis
