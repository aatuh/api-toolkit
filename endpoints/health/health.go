package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager implements ManagerContract for managing health checks.
type Manager struct {
	config     Config
	checkers   map[string]Checker
	cache      map[string]Result
	cacheMutex sync.RWMutex
	mu         sync.RWMutex
	snapshot   Response
	snapshotMu sync.RWMutex
	snapshotOK bool
}

// New creates a new health manager with default configuration.
func New() ManagerContract {
	manager := NewManagerWithConfig(Config{
		Timeout:         5 * time.Second,
		CacheDuration:   5 * time.Second,
		EnableCaching:   true,
		EnableDetailed:  false,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(NewBasicChecker())
	return manager
}

// NewManagerWithConfig creates a new health manager and returns the concrete type.
func NewManagerWithConfig(config Config) *Manager {
	return &Manager{
		config:   config,
		checkers: make(map[string]Checker),
		cache:    make(map[string]Result),
	}
}

// NewWithConfig creates a new health manager with custom configuration.
func NewWithConfig(config Config) ManagerContract {
	return NewManagerWithConfig(config)
}

// DetailedHealthEnabled reports whether HTTP handlers should expose detailed
// health output for this manager.
func (m *Manager) DetailedHealthEnabled() bool {
	if m == nil {
		return false
	}
	return m.config.EnableDetailed
}

// RegisterChecker registers a single health checker.
func (m *Manager) RegisterChecker(checker Checker) {
	if checker == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkers[checker.Name()] = checker
}

// RegisterCheckers registers multiple health checkers.
func (m *Manager) RegisterCheckers(checkers ...Checker) {
	for _, checker := range checkers {
		m.RegisterChecker(checker)
	}
}

// GetLiveness performs liveness checks.
func (m *Manager) GetLiveness(ctx context.Context) Result {
	result := m.performChecks(ctx, m.config.LivenessChecks)
	m.storeSnapshot(Response{
		Status:    result.Status,
		Timestamp: result.Timestamp,
		Message:   result.Message,
	})
	return result
}

// GetReadiness performs readiness checks.
func (m *Manager) GetReadiness(ctx context.Context) Result {
	result := m.performChecks(ctx, m.config.ReadinessChecks)
	m.storeSnapshot(Response{
		Status:    result.Status,
		Timestamp: result.Timestamp,
		Message:   result.Message,
	})
	return result
}

// GetHealth performs basic health checks.
func (m *Manager) GetHealth(ctx context.Context) Response {
	result := m.GetReadiness(ctx)
	response := Response{
		Status:    result.Status,
		Timestamp: result.Timestamp,
		Message:   result.Message,
	}
	m.storeSnapshot(response)
	return response
}

// GetDetailedHealth performs detailed health checks.
func (m *Manager) GetDetailedHealth(ctx context.Context) DetailedResponse {
	ctx = normalizeContext(ctx)
	m.mu.RLock()
	checkerNames := make([]string, 0, len(m.checkers))
	for name := range m.checkers {
		checkerNames = append(checkerNames, name)
	}
	m.mu.RUnlock()

	checkCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	checks := make(map[string]Result)
	summary := Summary{Total: len(checkerNames)}

	for _, name := range checkerNames {
		result := m.performCheck(checkCtx, name)
		checks[name] = result

		switch result.Status {
		case StatusHealthy:
			summary.Healthy++
		case StatusUnhealthy:
			summary.Unhealthy++
		case StatusDegraded:
			summary.Degraded++
		case StatusUnknown:
			summary.Unknown++
		}
	}

	// Determine overall status
	var overallStatus Status
	if summary.Unhealthy > 0 {
		overallStatus = StatusUnhealthy
	} else if summary.Degraded > 0 {
		overallStatus = StatusDegraded
	} else if summary.Healthy > 0 {
		overallStatus = StatusHealthy
	} else {
		overallStatus = StatusUnknown
	}

	response := DetailedResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    checks,
		Summary:   summary,
	}
	m.storeSnapshot(Response{
		Status:    response.Status,
		Timestamp: response.Timestamp,
	})
	return response
}

// RefreshAll runs all registered checks and updates the cache.
func (m *Manager) RefreshAll(ctx context.Context) DetailedResponse {
	ctx = normalizeContext(ctx)
	m.mu.RLock()
	checkerNames := make([]string, 0, len(m.checkers))
	for name := range m.checkers {
		checkerNames = append(checkerNames, name)
	}
	m.mu.RUnlock()

	checkCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	checks := make(map[string]Result)
	summary := Summary{Total: len(checkerNames)}

	for _, name := range checkerNames {
		result := m.performCheckNoCache(checkCtx, name)
		checks[name] = result

		switch result.Status {
		case StatusHealthy:
			summary.Healthy++
		case StatusUnhealthy:
			summary.Unhealthy++
		case StatusDegraded:
			summary.Degraded++
		case StatusUnknown:
			summary.Unknown++
		}
	}

	var overallStatus Status
	if summary.Unhealthy > 0 {
		overallStatus = StatusUnhealthy
	} else if summary.Degraded > 0 {
		overallStatus = StatusDegraded
	} else if summary.Healthy > 0 {
		overallStatus = StatusHealthy
	} else {
		overallStatus = StatusUnknown
	}

	response := DetailedResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    checks,
		Summary:   summary,
	}
	m.storeSnapshot(Response{
		Status:    response.Status,
		Timestamp: response.Timestamp,
	})
	return response
}

// CachedHealth returns the most recent health snapshot produced by this manager.
func (m *Manager) CachedHealth() (Response, bool) {
	if m == nil {
		return Response{}, false
	}
	m.snapshotMu.RLock()
	defer m.snapshotMu.RUnlock()
	if !m.snapshotOK {
		return Response{}, false
	}
	return m.snapshot, true
}

// performChecks performs multiple health checks.
func (m *Manager) performChecks(ctx context.Context, checkerNames []string) Result {
	ctx = normalizeContext(ctx)
	if len(checkerNames) == 0 {
		return Result{
			Status:    StatusUnhealthy,
			Message:   "health check configuration is invalid: no checks configured",
			Timestamp: time.Now(),
		}
	}

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	results := make([]Result, 0, len(checkerNames))

	for _, name := range checkerNames {
		result := m.performCheck(checkCtx, name)
		results = append(results, result)

		// If any check is unhealthy, return immediately
		if result.Status == StatusUnhealthy {
			return result
		}
	}

	// Determine overall status
	var overallStatus Status
	var messages []string

	for _, result := range results {
		switch result.Status {
		case StatusUnhealthy:
			overallStatus = StatusUnhealthy
			if result.Message != "" {
				messages = append(messages, result.Message)
			}
		case StatusDegraded:
			if overallStatus != StatusUnhealthy {
				overallStatus = StatusDegraded
				if result.Message != "" {
					messages = append(messages, result.Message)
				}
			}
		case StatusUnknown:
			if overallStatus == "" || overallStatus == StatusHealthy {
				overallStatus = StatusUnknown
				if result.Message != "" {
					messages = append(messages, result.Message)
				}
			}
		case StatusHealthy:
			if overallStatus == "" {
				overallStatus = StatusHealthy
			}
		}
	}

	var message string
	if len(messages) > 0 {
		message = fmt.Sprintf("Issues: %s", fmt.Sprintf("%v", messages))
	}

	return Result{
		Status:    overallStatus,
		Message:   message,
		Timestamp: time.Now(),
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (m *Manager) cachingEnabled() bool {
	return m != nil && m.config.EnableCaching && m.config.CacheDuration > 0
}

// performCheck performs a single health check with caching.
func (m *Manager) performCheck(ctx context.Context, name string) Result {
	// Check cache first
	if m.cachingEnabled() {
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
		return Result{
			Status:    StatusUnhealthy,
			Message:   fmt.Sprintf("health check configuration is invalid: checker %q not found", name),
			Timestamp: time.Now(),
		}
	}

	// Perform check
	start := time.Now()
	result := checker.Check(ctx)
	result.Duration = time.Since(start)
	result.Timestamp = time.Now()

	// Cache result
	if m.cachingEnabled() {
		m.cacheMutex.Lock()
		m.cache[name] = result
		m.cacheMutex.Unlock()
	}

	return result
}

func (m *Manager) performCheckNoCache(ctx context.Context, name string) Result {
	// Get checker
	m.mu.RLock()
	checker, exists := m.checkers[name]
	m.mu.RUnlock()

	if !exists {
		return Result{
			Status:    StatusUnhealthy,
			Message:   fmt.Sprintf("health check configuration is invalid: checker %q not found", name),
			Timestamp: time.Now(),
		}
	}

	start := time.Now()
	result := checker.Check(ctx)
	result.Duration = time.Since(start)
	result.Timestamp = time.Now()

	if m.cachingEnabled() {
		m.cacheMutex.Lock()
		m.cache[name] = result
		m.cacheMutex.Unlock()
	}

	return result
}

func (m *Manager) storeSnapshot(response Response) {
	if m == nil {
		return
	}
	if response.Status == "" {
		response.Status = StatusUnknown
	}
	if response.Timestamp.IsZero() {
		response.Timestamp = time.Now()
	}
	m.snapshotMu.Lock()
	m.snapshot = response
	m.snapshotOK = true
	m.snapshotMu.Unlock()
}
