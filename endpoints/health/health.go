package health

import (
	"context"
	"fmt"
	"reflect"
	"strings"
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
	manager, err := NewManager(DefaultConfig())
	if err != nil {
		return NewManagerWithConfig(DefaultConfig())
	}
	manager.RegisterChecker(NewBasicChecker())
	return manager
}

// NewManager creates a validated health manager and returns the concrete type.
func NewManager(config Config) (*Manager, error) {
	config = normalizeManagerConfig(config)
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return newManager(config), nil
}

// NewManagerWithConfig creates a health manager without validation.
//
// Deprecated: use NewManager. This compatibility wrapper preserves v4's
// unchecked construction behavior; new applications should handle validation
// errors during startup instead.
func NewManagerWithConfig(config Config) *Manager {
	return newManager(config)
}

func newManager(config Config) *Manager {
	config = normalizeManagerConfig(config)
	return &Manager{
		config:   config,
		checkers: make(map[string]Checker),
		cache:    make(map[string]Result),
	}
}

// NewWithConfig creates a health manager with custom configuration.
//
// Deprecated: use NewManager. This compatibility wrapper preserves v4's
// unchecked construction behavior.
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

// RegisterCheckerChecked registers a checker without replacing an existing
// checker. It rejects nil and empty-name checkers so startup wiring fails
// closed instead of silently accepting an invalid probe.
func (m *Manager) RegisterCheckerChecked(checker Checker) error {
	if m == nil {
		return fmt.Errorf("health manager is nil")
	}
	if isNilChecker(checker) {
		return fmt.Errorf("health checker must not be nil")
	}
	name := checker.Name()
	if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name {
		return fmt.Errorf("health checker name must not be empty or contain surrounding whitespace")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.checkers[name]; exists {
		return fmt.Errorf("health checker %q is already registered", name)
	}
	m.checkers[name] = checker
	return nil
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
		overallStatus = StatusUnhealthy
	}

	response := DetailedResponse{
		Status:    overallStatus,
		Timestamp: m.now(),
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
		overallStatus = StatusUnhealthy
	}

	response := DetailedResponse{
		Status:    overallStatus,
		Timestamp: m.now(),
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
			Timestamp: m.now(),
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
		Timestamp: m.now(),
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
			if age := m.now().Sub(cached.Timestamp); age >= 0 && age < m.config.CacheDuration {
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
			Timestamp: m.now(),
		}
	}

	// Perform check
	result := m.runChecker(ctx, checker)

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
			Timestamp: m.now(),
		}
	}

	result := m.runChecker(ctx, checker)

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
		response.Timestamp = m.now()
	}
	m.snapshotMu.Lock()
	m.snapshot = response
	m.snapshotOK = true
	m.snapshotMu.Unlock()
}

func (m *Manager) now() time.Time {
	if m != nil && m.config.Clock != nil {
		return m.config.Clock.Now()
	}
	return time.Now()
}

func (m *Manager) runChecker(ctx context.Context, checker Checker) Result {
	started := m.now()
	if err := ctx.Err(); err != nil {
		return m.timeoutResult(started, err)
	}
	results := make(chan Result, 1)
	go func() {
		results <- checker.Check(ctx)
	}()

	select {
	case result := <-results:
		result.Duration = m.now().Sub(started)
		result.Timestamp = m.now()
		return result
	case <-ctx.Done():
		return m.timeoutResult(started, ctx.Err())
	}
}

func (m *Manager) timeoutResult(started time.Time, err error) Result {
	message := "health check timed out"
	if err != nil {
		message = err.Error()
	}
	return Result{
		Status:    StatusUnhealthy,
		Message:   message,
		Timestamp: m.now(),
		Duration:  m.now().Sub(started),
	}
}

func isNilChecker(checker Checker) bool {
	if checker == nil {
		return true
	}
	value := reflect.ValueOf(checker)
	kind := value.Kind()
	return (kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface ||
		kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice) && value.IsNil()
}
