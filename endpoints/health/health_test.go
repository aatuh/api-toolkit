package health

import (
	"context"
	"fmt"
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
