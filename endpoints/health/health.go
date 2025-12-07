package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/ports"
)

// Manager implements ports.HealthManager for managing health checks.
type Manager struct {
	config     ports.HealthCheckConfig
	checkers   map[string]ports.HealthChecker
	cache      map[string]ports.HealthResult
	cacheMutex sync.RWMutex
	mu         sync.RWMutex
}

// New creates a new health manager with default configuration.
func New() ports.HealthManager {
	return NewWithConfig(ports.HealthCheckConfig{
		Timeout:         5 * time.Second,
		CacheDuration:   5 * time.Second,
		EnableCaching:   true,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"database", "basic"},
	})
}

// NewWithConfig creates a new health manager with custom configuration.
func NewWithConfig(config ports.HealthCheckConfig) ports.HealthManager {
	return &Manager{
		config:   config,
		checkers: make(map[string]ports.HealthChecker),
		cache:    make(map[string]ports.HealthResult),
	}
}

// RegisterChecker registers a single health checker.
func (m *Manager) RegisterChecker(checker ports.HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkers[checker.Name()] = checker
}

// RegisterCheckers registers multiple health checkers.
func (m *Manager) RegisterCheckers(checkers ...ports.HealthChecker) {
	for _, checker := range checkers {
		m.RegisterChecker(checker)
	}
}

// GetLiveness performs liveness checks.
func (m *Manager) GetLiveness(ctx context.Context) ports.HealthResult {
	return m.performChecks(ctx, m.config.LivenessChecks)
}

// GetReadiness performs readiness checks.
func (m *Manager) GetReadiness(ctx context.Context) ports.HealthResult {
	return m.performChecks(ctx, m.config.ReadinessChecks)
}

// GetHealth performs basic health checks.
func (m *Manager) GetHealth(ctx context.Context) ports.HealthResponse {
	result := m.GetReadiness(ctx)
	return ports.HealthResponse{
		Status:    result.Status,
		Timestamp: result.Timestamp,
		Message:   result.Message,
	}
}

// GetDetailedHealth performs detailed health checks.
func (m *Manager) GetDetailedHealth(ctx context.Context) ports.DetailedHealthResponse {
	m.mu.RLock()
	checkerNames := make([]string, 0, len(m.checkers))
	for name := range m.checkers {
		checkerNames = append(checkerNames, name)
	}
	m.mu.RUnlock()

	checks := make(map[string]ports.HealthResult)
	summary := ports.HealthSummary{Total: len(checkerNames)}

	for _, name := range checkerNames {
		result := m.performCheck(ctx, name)
		checks[name] = result

		switch result.Status {
		case ports.HealthStatusHealthy:
			summary.Healthy++
		case ports.HealthStatusUnhealthy:
			summary.Unhealthy++
		case ports.HealthStatusDegraded:
			summary.Degraded++
		case ports.HealthStatusUnknown:
			summary.Unknown++
		}
	}

	// Determine overall status
	var overallStatus ports.HealthStatus
	if summary.Unhealthy > 0 {
		overallStatus = ports.HealthStatusUnhealthy
	} else if summary.Degraded > 0 {
		overallStatus = ports.HealthStatusDegraded
	} else if summary.Healthy > 0 {
		overallStatus = ports.HealthStatusHealthy
	} else {
		overallStatus = ports.HealthStatusUnknown
	}

	return ports.DetailedHealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    checks,
		Summary:   summary,
	}
}

// performChecks performs multiple health checks.
func (m *Manager) performChecks(ctx context.Context, checkerNames []string) ports.HealthResult {
	if len(checkerNames) == 0 {
		return ports.HealthResult{
			Status:    ports.HealthStatusHealthy,
			Message:   "No checks configured",
			Timestamp: time.Now(),
		}
	}

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	results := make([]ports.HealthResult, 0, len(checkerNames))

	for _, name := range checkerNames {
		result := m.performCheck(checkCtx, name)
		results = append(results, result)

		// If any check is unhealthy, return immediately
		if result.Status == ports.HealthStatusUnhealthy {
			return result
		}
	}

	// Determine overall status
	var overallStatus ports.HealthStatus
	var messages []string

	for _, result := range results {
		switch result.Status {
		case ports.HealthStatusUnhealthy:
			overallStatus = ports.HealthStatusUnhealthy
			if result.Message != "" {
				messages = append(messages, result.Message)
			}
		case ports.HealthStatusDegraded:
			if overallStatus != ports.HealthStatusUnhealthy {
				overallStatus = ports.HealthStatusDegraded
				if result.Message != "" {
					messages = append(messages, result.Message)
				}
			}
		case ports.HealthStatusHealthy:
			if overallStatus == "" {
				overallStatus = ports.HealthStatusHealthy
			}
		}
	}

	var message string
	if len(messages) > 0 {
		message = fmt.Sprintf("Issues: %s", fmt.Sprintf("%v", messages))
	}

	return ports.HealthResult{
		Status:    overallStatus,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// performCheck performs a single health check with caching.
func (m *Manager) performCheck(ctx context.Context, name string) ports.HealthResult {
	// Check cache first
	if m.config.EnableCaching {
		m.cacheMutex.RLock()
		if cached, exists := m.cache[name]; exists {
			if time.Since(cached.Timestamp) < m.config.CacheDuration {
				m.cacheMutex.RUnlock()
				return cached
			}
		}
		m.cacheMutex.RUnlock()
	}

	// Get checker
	m.mu.RLock()
	checker, exists := m.checkers[name]
	m.mu.RUnlock()

	if !exists {
		return ports.HealthResult{
			Status:    ports.HealthStatusUnknown,
			Message:   fmt.Sprintf("Checker '%s' not found", name),
			Timestamp: time.Now(),
		}
	}

	// Perform check
	start := time.Now()
	result := checker.Check(ctx)
	result.Duration = time.Since(start)
	result.Timestamp = time.Now()

	// Cache result
	if m.config.EnableCaching {
		m.cacheMutex.Lock()
		m.cache[name] = result
		m.cacheMutex.Unlock()
	}

	return result
}
