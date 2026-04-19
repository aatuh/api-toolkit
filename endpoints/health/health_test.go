package health

import (
	"context"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestGetLivenessReturnsUnhealthyWhenNoChecksConfigured(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:        time.Second,
		LivenessChecks: nil,
	})

	result := manager.GetLiveness(context.Background())
	if result.Status != ports.HealthStatusUnhealthy {
		t.Fatalf("expected unhealthy, got %q", result.Status)
	}
	if result.Message == "" {
		t.Fatal("expected invalid configuration message")
	}
}

func TestGetLivenessReturnsUnhealthyWhenCheckerMissing(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:        time.Second,
		LivenessChecks: []string{"missing"},
	})

	result := manager.GetLiveness(context.Background())
	if result.Status != ports.HealthStatusUnhealthy {
		t.Fatalf("expected unhealthy, got %q", result.Status)
	}
	if result.Message == "" {
		t.Fatal("expected invalid configuration message")
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
