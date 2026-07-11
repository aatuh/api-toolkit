// Package healthchecktest provides reusable health checker contract tests for
// contrib adapters.
package healthchecktest

import (
	"context"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v3/endpoints/health"
)

// AssertCheckerContract verifies common readiness-check result semantics for a
// contrib adapter. If allowedStatuses is non-empty, the check result must match
// one of those statuses.
func AssertCheckerContract(t testing.TB, checker health.Checker, wantName string, allowedStatuses ...health.Status) health.Result {
	t.Helper()
	if checker == nil {
		t.Fatal("checker is nil")
	}

	name := checker.Name()
	if name == "" {
		t.Fatal("checker name is empty")
	}
	if wantName != "" && name != wantName {
		t.Fatalf("checker name = %q, want %q", name, wantName)
	}

	start := time.Now().Add(-1 * time.Second)
	result := checker.Check(context.Background())
	end := time.Now().Add(1 * time.Second)

	if !isValidStatus(result.Status) {
		t.Fatalf("checker status = %q, want a valid health status", result.Status)
	}
	if len(allowedStatuses) > 0 && !statusAllowed(result.Status, allowedStatuses) {
		t.Fatalf("checker status = %q, want one of %v", result.Status, allowedStatuses)
	}
	if result.Message == "" {
		t.Fatalf("checker %q returned empty message for status %q", name, result.Status)
	}
	if result.Timestamp.IsZero() {
		t.Fatalf("checker %q returned zero timestamp", name)
	}
	if result.Timestamp.Before(start) || result.Timestamp.After(end) {
		t.Fatalf("checker %q timestamp = %v, want between %v and %v", name, result.Timestamp, start, end)
	}
	if result.Duration < 0 {
		t.Fatalf("checker %q duration = %v, want non-negative", name, result.Duration)
	}

	return result
}

func isValidStatus(status health.Status) bool {
	switch status {
	case health.StatusHealthy,
		health.StatusUnhealthy,
		health.StatusDegraded,
		health.StatusUnknown:
		return true
	default:
		return false
	}
}

func statusAllowed(status health.Status, allowed []health.Status) bool {
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}
