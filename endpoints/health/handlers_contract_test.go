package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	health "github.com/aatuh/api-toolkit/v2/endpoints/health"
	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/specs"
)

type contractRouteRegistrar struct {
	patterns []string
}

func (r *contractRouteRegistrar) Get(pattern string, _ http.HandlerFunc) {
	r.patterns = append(r.patterns, pattern)
}

type externalCapabilityHealthManager struct {
	detailed       bool
	cached         ports.HealthResponse
	cachedOK       bool
	getHealthCalls int
}

var _ ports.HealthManager = (*externalCapabilityHealthManager)(nil)
var _ ports.DetailedHealthManager = (*externalCapabilityHealthManager)(nil)
var _ ports.CachedHealthManager = (*externalCapabilityHealthManager)(nil)

func (*externalCapabilityHealthManager) RegisterChecker(ports.HealthChecker) {}

func (*externalCapabilityHealthManager) RegisterCheckers(...ports.HealthChecker) {}

func (*externalCapabilityHealthManager) GetLiveness(context.Context) ports.HealthResult {
	return ports.HealthResult{Status: ports.HealthStatusHealthy, Timestamp: time.Now()}
}

func (*externalCapabilityHealthManager) GetReadiness(context.Context) ports.HealthResult {
	return ports.HealthResult{Status: ports.HealthStatusHealthy, Timestamp: time.Now()}
}

func (m *externalCapabilityHealthManager) GetHealth(context.Context) ports.HealthResponse {
	m.getHealthCalls++
	return ports.HealthResponse{Status: ports.HealthStatusUnhealthy, Timestamp: time.Now()}
}

func (*externalCapabilityHealthManager) GetDetailedHealth(context.Context) ports.DetailedHealthResponse {
	now := time.Now()
	return ports.DetailedHealthResponse{
		Status:    ports.HealthStatusHealthy,
		Timestamp: now,
		Checks: map[string]ports.HealthResult{
			"basic": {Status: ports.HealthStatusHealthy, Timestamp: now},
		},
		Summary: ports.HealthSummary{Total: 1, Healthy: 1},
	}
}

func (m *externalCapabilityHealthManager) DetailedHealthEnabled() bool {
	return m.detailed
}

func (m *externalCapabilityHealthManager) CachedHealth() (ports.HealthResponse, bool) {
	return m.cached, m.cachedOK
}

func TestRegisterRoutesToIncludesDetailedHealthForExportedCapability(t *testing.T) {
	handler := health.NewHandler(&externalCapabilityHealthManager{detailed: true})
	router := &contractRouteRegistrar{}

	handler.RegisterRoutesTo(router)

	for _, pattern := range router.patterns {
		if pattern == specs.HealthDetailed {
			return
		}
	}
	t.Fatalf("expected %q route when manager opts in through ports.DetailedHealthManager", specs.HealthDetailed)
}

func TestDetailedHealthHandlerServesExternalManagerThatOptsIn(t *testing.T) {
	handler := health.NewHandler(&externalCapabilityHealthManager{detailed: true})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.HealthDetailed, nil)
	handler.DetailedHealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddlewareUsesCachedHealthFromExportedCapability(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()
	manager := &externalCapabilityHealthManager{
		cachedOK: true,
		cached: ports.HealthResponse{
			Status:    ports.HealthStatusHealthy,
			Timestamp: timestamp,
			Message:   "cached",
		},
	}
	handler := health.NewHandler(manager)
	middleware := handler.Middleware()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, ok := health.HealthStatusFromContext(r.Context())
		if !ok {
			t.Fatal("expected health status in context")
		}
		if status != ports.HealthStatusHealthy {
			t.Fatalf("expected cached healthy status, got %q", status)
		}
		gotTimestamp, ok := health.HealthTimestampFromContext(r.Context())
		if !ok {
			t.Fatal("expected health timestamp in context")
		}
		if !gotTimestamp.Equal(timestamp) {
			t.Fatalf("expected cached timestamp %s, got %s", timestamp, gotTimestamp)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if manager.getHealthCalls != 0 {
		t.Fatalf("expected middleware to avoid GetHealth probes, got %d calls", manager.getHealthCalls)
	}
}
