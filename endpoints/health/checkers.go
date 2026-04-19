package health

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

const defaultCustomCheckerTimeout = 5 * time.Second

// BasicChecker implements a basic health check that always returns healthy.
type BasicChecker struct{}

// NewBasicChecker returns a health checker that always reports healthy.
func NewBasicChecker() ports.HealthChecker {
	return &BasicChecker{}
}

// Name returns the checker name.
func (c *BasicChecker) Name() string {
	return "basic"
}

// Check runs the basic health check.
func (c *BasicChecker) Check(_ context.Context) ports.HealthResult {
	return ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   "Basic health check passed",
		Timestamp: time.Now(),
	}
}

// DatabaseChecker implements a database health check.
type DatabaseChecker struct {
	pool ports.DatabasePool
}

// NewDatabaseChecker returns a database ping health checker.
func NewDatabaseChecker(pool ports.DatabasePool) ports.HealthChecker {
	return &DatabaseChecker{pool: pool}
}

// Name returns the checker name.
func (c *DatabaseChecker) Name() string {
	return "database"
}

// Check runs the database health check.
func (c *DatabaseChecker) Check(ctx context.Context) ports.HealthResult {
	if c == nil || c.pool == nil {
		return ports.HealthResult{
			Status:    ports.HealthStatusUnhealthy,
			Message:   "database pool not configured",
			Timestamp: time.Now(),
		}
	}

	start := time.Now()
	ctx = normalizeContext(ctx)

	// Create context with timeout for ping
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err := c.pool.Ping(pingCtx)
	duration := time.Since(start)

	if err != nil {
		return ports.HealthResult{
			Status:    ports.HealthStatusUnhealthy,
			Message:   fmt.Sprintf("Database ping failed: %v", err),
			Timestamp: time.Now(),
			Duration:  duration,
		}
	}

	// Get pool stats for additional details
	stats := ports.SnapshotDatabaseStats(c.pool.Stat())
	details := map[string]interface{}{
		"total_conns":    stats.TotalConns,
		"idle_conns":     stats.IdleConns,
		"acquired_conns": stats.AcquiredConns,
		"max_conns":      stats.MaxConns,
		"acquire_count":  stats.AcquireCount,
	}

	return ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   "Database connection healthy",
		Details:   details,
		Timestamp: time.Now(),
		Duration:  duration,
	}
}

// MemoryChecker implements a memory usage health check.
type MemoryChecker struct {
	maxMemoryMB int64
}

// NewMemoryChecker returns a memory usage health checker.
func NewMemoryChecker(maxMemoryMB int64) ports.HealthChecker {
	return &MemoryChecker{maxMemoryMB: maxMemoryMB}
}

// Name returns the checker name.
func (c *MemoryChecker) Name() string {
	return "memory"
}

// Check runs the memory usage health check.
func (c *MemoryChecker) Check(_ context.Context) ports.HealthResult {
	// This is a simplified memory check
	// In a real implementation, you'd use runtime.MemStats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	allocMB := m.Alloc / 1024 / 1024
	memoryMB := int64(math.MaxInt64)
	if allocMB <= uint64(math.MaxInt64) {
		memoryMB = int64(allocMB)
	}

	status := ports.HealthStatusHealthy
	message := fmt.Sprintf("Memory usage: %d MB", memoryMB)

	if c.maxMemoryMB > 0 && memoryMB > c.maxMemoryMB {
		status = ports.HealthStatusUnhealthy
		message = fmt.Sprintf("Memory usage too high: %d MB (max: %d MB)", memoryMB, c.maxMemoryMB)
	} else if c.maxMemoryMB > 0 && memoryMB > c.maxMemoryMB*8/10 {
		status = ports.HealthStatusDegraded
		message = fmt.Sprintf("Memory usage high: %d MB (max: %d MB)", memoryMB, c.maxMemoryMB)
	}

	details := map[string]interface{}{
		"alloc_mb":      memoryMB,
		"max_memory_mb": c.maxMemoryMB,
		"heap_alloc":    m.HeapAlloc,
		"heap_sys":      m.HeapSys,
		"num_gc":        m.NumGC,
	}

	return ports.HealthResult{
		Status:    status,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// CustomChecker implements a custom health check with a function.
type CustomChecker struct {
	name      string
	checkFunc func(ctx context.Context) (ports.HealthStatus, string, interface{})
	timeout   time.Duration
}

// NewCustomChecker returns a checker that calls the provided function.
func NewCustomChecker(name string, checkFunc func(ctx context.Context) (ports.HealthStatus, string, interface{})) ports.HealthChecker {
	return &CustomChecker{
		name:      name,
		checkFunc: checkFunc,
		timeout:   defaultCustomCheckerTimeout,
	}
}

// NewCustomCheckerWithTimeout returns a checker with a custom timeout.
func NewCustomCheckerWithTimeout(name string, timeout time.Duration, checkFunc func(ctx context.Context) (ports.HealthStatus, string, interface{})) ports.HealthChecker {
	return &CustomChecker{
		name:      name,
		checkFunc: checkFunc,
		timeout:   normalizeCustomCheckerTimeout(timeout),
	}
}

// Name returns the checker name.
func (c *CustomChecker) Name() string {
	return c.name
}

// Check runs the custom health check.
func (c *CustomChecker) Check(ctx context.Context) ports.HealthResult {
	start := time.Now()
	if c == nil || c.checkFunc == nil {
		return ports.HealthResult{
			Status:    ports.HealthStatusUnknown,
			Message:   "custom health check not configured",
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}
	}
	if ctx == nil {
		return ports.HealthResult{
			Status:    ports.HealthStatusUnknown,
			Message:   "custom health check context is missing",
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}
	}

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	status, message, details := c.checkFunc(checkCtx)
	duration := time.Since(start)

	return ports.HealthResult{
		Status:    status,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
		Duration:  duration,
	}
}

func normalizeCustomCheckerTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultCustomCheckerTimeout
	}
	return timeout
}

// CompositeChecker implements a composite health check that combines multiple checks.
type CompositeChecker struct {
	name     string
	checkers []ports.HealthChecker
}

// NewCompositeChecker returns a checker that aggregates other checkers.
func NewCompositeChecker(name string, checkers ...ports.HealthChecker) ports.HealthChecker {
	return &CompositeChecker{
		name:     name,
		checkers: checkers,
	}
}

// Name returns the checker name.
func (c *CompositeChecker) Name() string {
	return c.name
}

// Check runs the composite health check.
func (c *CompositeChecker) Check(ctx context.Context) ports.HealthResult {
	if len(c.checkers) == 0 {
		return ports.HealthResult{
			Status:    ports.HealthStatusUnknown,
			Message:   "No checkers configured",
			Timestamp: time.Now(),
		}
	}

	start := time.Now()
	results := make([]ports.HealthResult, 0, len(c.checkers))

	for _, checker := range c.checkers {
		result := checker.Check(ctx)
		results = append(results, result)

		// If any check is unhealthy, return immediately
		if result.Status == ports.HealthStatusUnhealthy {
			return ports.HealthResult{
				Status:    ports.HealthStatusUnhealthy,
				Message:   fmt.Sprintf("Composite check failed: %s", result.Message),
				Details:   map[string]interface{}{"results": results},
				Timestamp: time.Now(),
				Duration:  time.Since(start),
			}
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
		case ports.HealthStatusUnknown:
			if overallStatus == "" || overallStatus == ports.HealthStatusHealthy {
				overallStatus = ports.HealthStatusUnknown
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
		message = fmt.Sprintf("Composite check: %s", fmt.Sprintf("%v", messages))
	} else {
		message = "All composite checks passed"
	}

	return ports.HealthResult{
		Status:    overallStatus,
		Message:   message,
		Details:   map[string]interface{}{"results": results},
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}
}

// HTTPChecker implements a basic HTTP health check.
type HTTPChecker struct {
	name          string
	url           string
	method        string
	headers       map[string]string
	client        *http.Client
	timeout       time.Duration
	successStatus map[int]struct{}
	failureStatus ports.HealthStatus
}

// HTTPCheckerOption configures HTTPChecker behavior.
type HTTPCheckerOption func(*HTTPChecker)

// NewHTTPChecker returns a checker that probes an HTTP endpoint.
func NewHTTPChecker(name, url string, opts ...HTTPCheckerOption) ports.HealthChecker {
	checker := &HTTPChecker{
		name:          name,
		url:           url,
		method:        http.MethodGet,
		client:        http.DefaultClient,
		timeout:       5 * time.Second,
		successStatus: map[int]struct{}{http.StatusOK: {}},
		failureStatus: ports.HealthStatusUnhealthy,
	}
	for _, opt := range opts {
		opt(checker)
	}
	return checker
}

// WithHTTPMethod sets the HTTP method.
func WithHTTPMethod(method string) HTTPCheckerOption {
	return func(checker *HTTPChecker) {
		method = strings.TrimSpace(method)
		if method != "" {
			checker.method = strings.ToUpper(method)
		}
	}
}

// WithHTTPHeader sets a header to send with the request.
func WithHTTPHeader(key, value string) HTTPCheckerOption {
	return func(checker *HTTPChecker) {
		if checker.headers == nil {
			checker.headers = make(map[string]string)
		}
		checker.headers[key] = value
	}
}

// WithHTTPHeaders sets multiple headers to send with the request.
func WithHTTPHeaders(headers map[string]string) HTTPCheckerOption {
	return func(checker *HTTPChecker) {
		if len(headers) == 0 {
			return
		}
		if checker.headers == nil {
			checker.headers = make(map[string]string, len(headers))
		}
		for key, value := range headers {
			checker.headers[key] = value
		}
	}
}

// WithHTTPClient sets the HTTP client for the checker.
func WithHTTPClient(client *http.Client) HTTPCheckerOption {
	return func(checker *HTTPChecker) {
		if client != nil {
			checker.client = client
		}
	}
}

// WithHTTPTimeout sets the request timeout.
func WithHTTPTimeout(timeout time.Duration) HTTPCheckerOption {
	return func(checker *HTTPChecker) {
		if timeout > 0 {
			checker.timeout = timeout
		}
	}
}

// WithHTTPSuccessStatuses defines acceptable status codes.
func WithHTTPSuccessStatuses(statuses ...int) HTTPCheckerOption {
	return func(checker *HTTPChecker) {
		if len(statuses) == 0 {
			return
		}
		checker.successStatus = make(map[int]struct{}, len(statuses))
		for _, status := range statuses {
			checker.successStatus[status] = struct{}{}
		}
	}
}

// WithHTTPFailureStatus sets the health status when the check fails.
func WithHTTPFailureStatus(status ports.HealthStatus) HTTPCheckerOption {
	return func(checker *HTTPChecker) {
		if status != "" {
			checker.failureStatus = status
		}
	}
}

// Name returns the checker name.
func (c *HTTPChecker) Name() string {
	return c.name
}

// Check runs the HTTP health check.
func (c *HTTPChecker) Check(ctx context.Context) ports.HealthResult {
	if strings.TrimSpace(c.url) == "" {
		return ports.HealthResult{
			Status:    ports.HealthStatusUnknown,
			Message:   "health check URL not configured",
			Timestamp: time.Now(),
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// #nosec G704 -- the health checker only probes explicit application-configured URLs.
	req, err := http.NewRequestWithContext(checkCtx, c.method, c.url, nil)
	if err != nil {
		return ports.HealthResult{
			Status:    c.failureStatus,
			Message:   fmt.Sprintf("health check request failed: %v", err),
			Timestamp: time.Now(),
		}
	}
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	start := time.Now()
	// #nosec G704 -- outbound health checks intentionally call configured dependency endpoints.
	resp, err := c.client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return ports.HealthResult{
			Status:    c.failureStatus,
			Message:   fmt.Sprintf("health check failed: %v", err),
			Timestamp: time.Now(),
			Duration:  duration,
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	_, ok := c.successStatus[resp.StatusCode]
	if !ok {
		return ports.HealthResult{
			Status:    c.failureStatus,
			Message:   fmt.Sprintf("unexpected status %d from %s", resp.StatusCode, c.url),
			Timestamp: time.Now(),
			Duration:  duration,
			Details: map[string]interface{}{
				"status_code": resp.StatusCode,
				"url":         c.url,
			},
		}
	}

	return ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   "HTTP dependency healthy",
		Timestamp: time.Now(),
		Duration:  duration,
		Details: map[string]interface{}{
			"status_code": resp.StatusCode,
			"url":         c.url,
		},
	}
}

// PaymentProviderChecker implements a health check for payment providers.
type PaymentProviderChecker struct {
	name          string
	provider      ports.PaymentProvider
	failureStatus ports.HealthStatus
}

// PaymentProviderCheckerOption configures PaymentProviderChecker behavior.
type PaymentProviderCheckerOption func(*PaymentProviderChecker)

// NewPaymentProviderChecker returns a checker for payment providers.
func NewPaymentProviderChecker(provider ports.PaymentProvider, opts ...PaymentProviderCheckerOption) ports.HealthChecker {
	checker := &PaymentProviderChecker{
		name:          "payments",
		provider:      provider,
		failureStatus: ports.HealthStatusUnhealthy,
	}
	for _, opt := range opts {
		opt(checker)
	}
	return checker
}

// WithPaymentProviderName overrides the checker name.
func WithPaymentProviderName(name string) PaymentProviderCheckerOption {
	return func(checker *PaymentProviderChecker) {
		name = strings.TrimSpace(name)
		if name != "" {
			checker.name = name
		}
	}
}

// WithPaymentProviderFailureStatus sets the failure status.
func WithPaymentProviderFailureStatus(status ports.HealthStatus) PaymentProviderCheckerOption {
	return func(checker *PaymentProviderChecker) {
		if status != "" {
			checker.failureStatus = status
		}
	}
}

// Name returns the checker name.
func (c *PaymentProviderChecker) Name() string {
	return c.name
}

// Check runs the payment provider health check.
func (c *PaymentProviderChecker) Check(ctx context.Context) ports.HealthResult {
	start := time.Now()
	if c.provider == nil {
		return ports.HealthResult{
			Status:    ports.HealthStatusUnknown,
			Message:   "payment provider not configured",
			Timestamp: time.Now(),
		}
	}
	prices, err := c.provider.ListPrices(ctx)
	duration := time.Since(start)
	if err != nil {
		return ports.HealthResult{
			Status:    c.failureStatus,
			Message:   fmt.Sprintf("payment provider check failed: %v", err),
			Timestamp: time.Now(),
			Duration:  duration,
		}
	}

	return ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   "payment provider healthy",
		Details:   map[string]interface{}{"price_count": len(prices)},
		Timestamp: time.Now(),
		Duration:  duration,
	}
}
