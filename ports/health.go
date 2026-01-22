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
type HealthManager interface {
	RegisterChecker(checker HealthChecker)
	RegisterCheckers(checkers ...HealthChecker)
	GetLiveness(ctx context.Context) HealthResult
	GetReadiness(ctx context.Context) HealthResult
	GetHealth(ctx context.Context) HealthResponse
	GetDetailedHealth(ctx context.Context) DetailedHealthResponse
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
