package health

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/aatuh/api-toolkit/ports"
)

// BasicChecker implements a basic health check that always returns healthy.
type BasicChecker struct{}

func NewBasicChecker() ports.HealthChecker {
	return &BasicChecker{}
}

func (c *BasicChecker) Name() string {
	return "basic"
}

func (c *BasicChecker) Check(ctx context.Context) ports.HealthResult {
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

func NewDatabaseChecker(pool ports.DatabasePool) ports.HealthChecker {
	return &DatabaseChecker{pool: pool}
}

func (c *DatabaseChecker) Name() string {
	return "database"
}

func (c *DatabaseChecker) Check(ctx context.Context) ports.HealthResult {
	start := time.Now()

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
	stats := c.pool.Stat()
	details := map[string]interface{}{
		"total_conns":    stats.TotalConns(),
		"idle_conns":     stats.IdleConns(),
		"acquired_conns": stats.AcquiredConns(),
		"max_conns":      stats.MaxConns(),
		"acquire_count":  stats.AcquireCount(),
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

func NewMemoryChecker(maxMemoryMB int64) ports.HealthChecker {
	return &MemoryChecker{maxMemoryMB: maxMemoryMB}
}

func (c *MemoryChecker) Name() string {
	return "memory"
}

func (c *MemoryChecker) Check(ctx context.Context) ports.HealthResult {
	// This is a simplified memory check
	// In a real implementation, you'd use runtime.MemStats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryMB := int64(m.Alloc / 1024 / 1024)

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

func NewCustomChecker(name string, checkFunc func(ctx context.Context) (ports.HealthStatus, string, interface{})) ports.HealthChecker {
	return &CustomChecker{
		name:      name,
		checkFunc: checkFunc,
		timeout:   5 * time.Second,
	}
}

func NewCustomCheckerWithTimeout(name string, timeout time.Duration, checkFunc func(ctx context.Context) (ports.HealthStatus, string, interface{})) ports.HealthChecker {
	return &CustomChecker{
		name:      name,
		checkFunc: checkFunc,
		timeout:   timeout,
	}
}

func (c *CustomChecker) Name() string {
	return c.name
}

func (c *CustomChecker) Check(ctx context.Context) ports.HealthResult {
	start := time.Now()

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

// CompositeChecker implements a composite health check that combines multiple checks.
type CompositeChecker struct {
	name     string
	checkers []ports.HealthChecker
}

func NewCompositeChecker(name string, checkers ...ports.HealthChecker) ports.HealthChecker {
	return &CompositeChecker{
		name:     name,
		checkers: checkers,
	}
}

func (c *CompositeChecker) Name() string {
	return c.name
}

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
