package health

import (
	"context"
	"net/http"
	"time"
)

// Checker defines the package-local health check contract.
type Checker interface {
	Name() string
	Check(ctx context.Context) Result
}

// Status represents a health check status.
type Status string

const (
	// StatusHealthy indicates all checks are passing.
	StatusHealthy Status = "healthy"
	// StatusUnhealthy indicates one or more checks failed.
	StatusUnhealthy Status = "unhealthy"
	// StatusDegraded indicates partial degradation.
	StatusDegraded Status = "degraded"
	// StatusUnknown indicates indeterminate health.
	StatusUnknown Status = "unknown"
)

// Result represents one health check result.
type Result struct {
	Status    Status        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Details   interface{}   `json:"details,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// Response represents the overall health response.
type Response struct {
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// DetailedResponse represents detailed health output with individual checks.
type DetailedResponse struct {
	Status    Status            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]Result `json:"checks"`
	Summary   Summary           `json:"summary"`
}

// Summary provides a summary of health check results.
type Summary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Degraded  int `json:"degraded"`
	Unknown   int `json:"unknown"`
}

// Config defines health manager configuration.
type Config struct {
	Timeout         time.Duration `json:"timeout"`
	CacheDuration   time.Duration `json:"cache_duration"`
	EnableCaching   bool          `json:"enable_caching"`
	EnableDetailed  bool          `json:"enable_detailed"`
	LivenessChecks  []string      `json:"liveness_checks"`
	ReadinessChecks []string      `json:"readiness_checks"`
}

// ManagerContract defines the package-local health manager contract.
type ManagerContract interface {
	RegisterChecker(checker Checker)
	RegisterCheckers(checkers ...Checker)
	GetLiveness(ctx context.Context) Result
	GetReadiness(ctx context.Context) Result
	GetHealth(ctx context.Context) Response
	GetDetailedHealth(ctx context.Context) DetailedResponse
}

// DetailedManager is an optional detailed health capability contract.
type DetailedManager interface {
	DetailedHealthEnabled() bool
}

// CachedManager exposes a reusable health snapshot.
type CachedManager interface {
	CachedHealth() (Response, bool)
}

// RouteRegistrar is the minimal health route registration contract.
type RouteRegistrar interface {
	Get(pattern string, h http.HandlerFunc)
}

// DatabasePool is the narrow database capability used by health probes.
type DatabasePool interface {
	Ping(ctx context.Context) error
}

// DatabasePoolSnapshot captures optional plain-value connection-pool stats.
type DatabasePoolSnapshot struct {
	AcquireCount  int64
	AcquiredConns int32
	IdleConns     int32
	MaxConns      int32
	TotalConns    int32
}

// DatabasePoolSnapshotProvider exposes an optional pool snapshot.
type DatabasePoolSnapshotProvider interface {
	StatSnapshot() DatabasePoolSnapshot
}

// SnapshotDatabasePoolStats returns an optional pool snapshot.
func SnapshotDatabasePoolStats(pool DatabasePool) DatabasePoolSnapshot {
	if snapshotter, ok := pool.(DatabasePoolSnapshotProvider); ok {
		return snapshotter.StatSnapshot()
	}
	return DatabasePoolSnapshot{}
}
