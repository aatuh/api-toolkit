package health

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Manager implements ManagerContract for managing health checks.
type Manager struct {
	config     Config
	clock      Clock
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
	manager := newManager(DefaultConfig())
	manager.RegisterChecker(NewBasicChecker())
	return manager
}

// NewManager constructs a validated health manager. Callers must provide a
// positive timeout, a positive cache duration when caching is enabled, and at
// least one liveness or readiness checker name. Use DefaultConfig as the
// explicit starting point for production configuration.
func NewManager(config Config) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return newManager(config), nil
}

// NewManagerWithConfig creates a legacy-compatible health manager.
//
// Deprecated: use NewManager, which validates configuration at construction.
func NewManagerWithConfig(config Config) *Manager {
	manager, err := NewManager(config)
	if err == nil {
		return manager
	}
	return newManager(normalizeLegacyConfig(config))
}

func newManager(config Config) *Manager {
	config = normalizeManagerConfig(config)
	return &Manager{
		config:   config,
		clock:    config.Clock,
		checkers: make(map[string]Checker),
		cache:    make(map[string]Result),
	}
}

// NewWithConfig creates a legacy-compatible health manager with custom configuration.
//
// Deprecated: use NewManager, which returns validation errors to the caller.
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

// RegisterChecker registers a single health checker and preserves the legacy
// no-error API. New callers should use RegisterCheckerChecked.
func (m *Manager) RegisterChecker(checker Checker) {
	_ = m.RegisterCheckerChecked(checker)
}

// RegisterCheckerChecked registers a checker after validating its identity.
// It rejects nil, empty-name, and duplicate checkers rather than silently
// changing the health contract at runtime.
func (m *Manager) RegisterCheckerChecked(checker Checker) error {
	if m == nil {
		return errors.New("health manager is nil")
	}
	if nilChecker(checker) {
		return errors.New("health checker is nil")
	}
	name := strings.TrimSpace(checker.Name())
	if name == "" {
		return errors.New("health checker name is empty")
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
	if len(checkerNames) == 0 {
		response := DetailedResponse{
			Status:    StatusUnhealthy,
			Timestamp: m.now(),
			Checks:    map[string]Result{},
			Summary:   Summary{},
		}
		m.storeSnapshot(Response{Status: response.Status, Timestamp: response.Timestamp})
		return response
	}

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
	if len(checkerNames) == 0 {
		response := DetailedResponse{
			Status:    StatusUnhealthy,
			Timestamp: m.now(),
			Checks:    map[string]Result{},
			Summary:   Summary{},
		}
		m.storeSnapshot(Response{Status: response.Status, Timestamp: response.Timestamp})
		return response
	}

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
			if m.now().Sub(cached.Timestamp) < m.config.CacheDuration {
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
	start := m.now()
	result := m.runChecker(ctx, checker)
	result.Duration = m.now().Sub(start)
	result.Timestamp = m.now()

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

	start := m.now()
	result := m.runChecker(ctx, checker)
	result.Duration = m.now().Sub(start)
	result.Timestamp = m.now()

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
	if m == nil || m.clock == nil {
		return time.Now()
	}
	return m.clock.Now()
}

func (m *Manager) runChecker(ctx context.Context, checker Checker) Result {
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- checker.Check(ctx)
	}()

	select {
	case result := <-resultCh:
		return result
	case <-ctx.Done():
		message := "health check canceled before it returned"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			message = "health check timed out before it returned: " + context.DeadlineExceeded.Error()
		}
		return Result{Status: StatusUnhealthy, Message: message, Timestamp: m.now()}
	}
}

func nilChecker(checker Checker) bool {
	if checker == nil {
		return true
	}
	value := reflect.ValueOf(checker)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
