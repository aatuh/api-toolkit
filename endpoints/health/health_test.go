package health

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestProbeChecksReturnUnhealthyForInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config ports.HealthCheckConfig
		check  func(*Manager, context.Context) ports.HealthResult
	}{
		{
			name: "liveness with no configured checks",
			config: ports.HealthCheckConfig{
				Timeout:        time.Second,
				LivenessChecks: nil,
			},
			check: func(m *Manager, ctx context.Context) ports.HealthResult {
				return m.GetLiveness(ctx)
			},
		},
		{
			name: "liveness with missing checker",
			config: ports.HealthCheckConfig{
				Timeout:        time.Second,
				LivenessChecks: []string{"missing"},
			},
			check: func(m *Manager, ctx context.Context) ports.HealthResult {
				return m.GetLiveness(ctx)
			},
		},
		{
			name: "readiness with no configured checks",
			config: ports.HealthCheckConfig{
				Timeout:         time.Second,
				ReadinessChecks: nil,
			},
			check: func(m *Manager, ctx context.Context) ports.HealthResult {
				return m.GetReadiness(ctx)
			},
		},
		{
			name: "readiness with missing checker",
			config: ports.HealthCheckConfig{
				Timeout:         time.Second,
				ReadinessChecks: []string{"missing"},
			},
			check: func(m *Manager, ctx context.Context) ports.HealthResult {
				return m.GetReadiness(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManagerWithConfig(tt.config)

			result := tt.check(manager, context.Background())
			if result.Status != ports.HealthStatusUnhealthy {
				t.Fatalf("expected unhealthy, got %q", result.Status)
			}
			if result.Message == "" {
				t.Fatal("expected invalid configuration message")
			}
		})
	}
}

func TestRegisterCheckerIgnoresNilChecker(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         time.Second,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})

	manager.RegisterChecker(nil)
	manager.RegisterChecker(NewBasicChecker())

	detailed := manager.GetDetailedHealth(context.Background())
	if detailed.Summary.Total != 1 {
		t.Fatalf("expected 1 registered checker, got %d", detailed.Summary.Total)
	}
}

func TestGetDetailedHealthUsesCacheWhenEnabled(t *testing.T) {
	checker := &countingChecker{name: "counted"}
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:        time.Second,
		CacheDuration:  time.Minute,
		EnableCaching:  true,
		EnableDetailed: true,
	})
	manager.RegisterChecker(checker)

	manager.GetDetailedHealth(context.Background())
	manager.GetDetailedHealth(context.Background())

	if got := checker.calls.Load(); got != 1 {
		t.Fatalf("expected cached checker to run once, got %d", got)
	}
}

func TestGetDetailedHealthBypassesCacheWhenDisabled(t *testing.T) {
	checker := &countingChecker{name: "counted"}
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:        time.Second,
		CacheDuration:  time.Minute,
		EnableCaching:  false,
		EnableDetailed: true,
	})
	manager.RegisterChecker(checker)

	manager.GetDetailedHealth(context.Background())
	manager.GetDetailedHealth(context.Background())

	if got := checker.calls.Load(); got != 2 {
		t.Fatalf("expected uncached checker to run twice, got %d", got)
	}
}

func TestGetDetailedHealthHonorsConfiguredTimeout(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:        20 * time.Millisecond,
		EnableCaching:  false,
		EnableDetailed: true,
	})
	checker := &blockingChecker{name: "slow", started: make(chan struct{})}
	manager.RegisterChecker(checker)

	done := make(chan ports.DetailedHealthResponse, 1)
	go func() {
		done <- manager.GetDetailedHealth(context.Background())
	}()

	select {
	case <-checker.started:
	case <-time.After(time.Second):
		t.Fatal("checker did not start")
	}

	select {
	case resp := <-done:
		result, ok := resp.Checks["slow"]
		if !ok {
			t.Fatalf("expected slow checker result, got %#v", resp.Checks)
		}
		if result.Status != ports.HealthStatusUnhealthy {
			t.Fatalf("expected unhealthy result after timeout, got %q", result.Status)
		}
		if !strings.Contains(result.Message, context.DeadlineExceeded.Error()) {
			t.Fatalf("expected timeout message, got %q", result.Message)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("GetDetailedHealth did not honor configured timeout")
	}
}

func TestCustomCheckerReturnsUnknownWhenFunctionMissing(t *testing.T) {
	checker := NewCustomChecker("custom", nil)

	result := checker.Check(context.Background())

	if result.Status != ports.HealthStatusUnknown {
		t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusUnknown)
	}
	if result.Message != "custom health check not configured" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestCustomCheckerDefaultsNonPositiveTimeouts(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "zero", timeout: 0},
		{name: "negative", timeout: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewCustomCheckerWithTimeout("custom", tt.timeout, func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected deadline on custom checker context")
				}
				remaining := time.Until(deadline)
				if remaining < 4*time.Second || remaining > 6*time.Second {
					t.Fatalf("deadline remaining = %v, want default timeout window", remaining)
				}
				return ports.HealthStatusHealthy, "ok", nil
			})

			result := checker.Check(context.Background())
			if result.Status != ports.HealthStatusHealthy {
				t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusHealthy)
			}
		})
	}
}

type countingChecker struct {
	name  string
	calls atomic.Int32
}

func (c *countingChecker) Name() string {
	return c.name
}

func (c *countingChecker) Check(context.Context) ports.HealthResult {
	call := c.calls.Add(1)
	return ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   fmt.Sprintf("call %d", call),
		Timestamp: time.Now(),
	}
}

type blockingChecker struct {
	name    string
	started chan struct{}
}

func (c *blockingChecker) Name() string {
	return c.name
}

func (c *blockingChecker) Check(ctx context.Context) ports.HealthResult {
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	<-ctx.Done()
	return ports.HealthResult{
		Status:    ports.HealthStatusUnhealthy,
		Message:   ctx.Err().Error(),
		Timestamp: time.Now(),
	}
}
