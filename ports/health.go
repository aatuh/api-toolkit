package ports

import (
	"context"
	"time"
)

// HealthChecker defines the interface for individual health checks.
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthResult
}

// HealthResult represents the result of a health check.
type HealthResult struct {
	Status    HealthStatus  `json:"status"`
	Message   string        `json:"message,omitempty"`
	Details   interface{}   `json:"details,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// HealthStatus represents the status of a health check.
type HealthStatus string

const (
	// HealthStatusHealthy indicates all checks are passing.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusUnhealthy indicates one or more checks failed.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusDegraded indicates partial degradation.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnknown indicates indeterminate health.
	HealthStatusUnknown HealthStatus = "unknown"
)

// HealthManager defines the interface for managing health checks.
//
// HTTP handler packages may opt into additional health behavior through the
// exported DetailedHealthManager and CachedHealthManager capability interfaces
// instead of relying on package-private knowledge about concrete managers.
type HealthManager interface {
	RegisterChecker(checker HealthChecker)
	RegisterCheckers(checkers ...HealthChecker)
	GetLiveness(ctx context.Context) HealthResult
	GetReadiness(ctx context.Context) HealthResult
	GetHealth(ctx context.Context) HealthResponse
	GetDetailedHealth(ctx context.Context) DetailedHealthResponse
}

// DetailedHealthManager is an optional capability for health managers that
// allows HTTP handler packages to decide whether detailed health routes should
// be registered or served.
type DetailedHealthManager interface {
	DetailedHealthEnabled() bool
}

// CachedHealthManager is an optional capability for health managers that
// exposes a reusable health snapshot for middleware and other request-path
// callers that should avoid probing dependencies inline.
type CachedHealthManager interface {
	CachedHealth() (HealthResponse, bool)
}

// HealthResponse represents the overall health response.
type HealthResponse struct {
	Status    HealthStatus `json:"status"`
	Timestamp time.Time    `json:"timestamp"`
	Message   string       `json:"message,omitempty"`
}

// DetailedHealthResponse represents a detailed health response with individual checks.
type DetailedHealthResponse struct {
	Status    HealthStatus            `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]HealthResult `json:"checks"`
	Summary   HealthSummary           `json:"summary"`
}

// HealthSummary provides a summary of all health checks.
type HealthSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Degraded  int `json:"degraded"`
	Unknown   int `json:"unknown"`
}

// HealthCheckConfig defines configuration for health checks.
//
// Contract:
//   - Timeout bounds a single liveness, readiness, or detailed health pass.
//   - CacheDuration controls how long individual checker results may be reused
//     when EnableCaching is true.
//   - EnableDetailed controls whether HTTP packages should expose detailed
//     health output that includes per-check dependency information.
//   - LivenessChecks and ReadinessChecks should name at least one checker each;
//     empty probe lists are invalid configuration and should not be reported as
//     healthy.
type HealthCheckConfig struct {
	Timeout         time.Duration `json:"timeout"`
	CacheDuration   time.Duration `json:"cache_duration"`
	EnableCaching   bool          `json:"enable_caching"`
	EnableDetailed  bool          `json:"enable_detailed"`
	LivenessChecks  []string      `json:"liveness_checks"`
	ReadinessChecks []string      `json:"readiness_checks"`
}

// HealthCheckRegistry defines the interface for registering health checks.
type HealthCheckRegistry interface {
	Register(name string, checker HealthChecker)
	Unregister(name string)
	GetChecker(name string) (HealthChecker, bool)
	ListCheckers() []string
}
