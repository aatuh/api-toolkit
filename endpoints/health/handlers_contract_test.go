package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	health "github.com/aatuh/api-toolkit/v3/endpoints/health"
	"github.com/aatuh/api-toolkit/v3/specs"
)

type Response = health.Response
type Result = health.Result
type DetailedResponse = health.DetailedResponse
type Summary = health.Summary
type Checker = health.Checker
type ManagerContract = health.ManagerContract
type DetailedManager = health.DetailedManager
type CachedManager = health.CachedManager

const (
	StatusHealthy   = health.StatusHealthy
	StatusUnhealthy = health.StatusUnhealthy
)

type contractRouteRegistrar struct {
	patterns []string
}

func (r *contractRouteRegistrar) Get(pattern string, _ http.HandlerFunc) {
	r.patterns = append(r.patterns, pattern)
}

type externalCapabilityHealthManager struct {
	detailed       bool
	cached         Response
	cachedOK       bool
	getHealthCalls int
}

var _ ManagerContract = (*externalCapabilityHealthManager)(nil)
var _ DetailedManager = (*externalCapabilityHealthManager)(nil)
var _ CachedManager = (*externalCapabilityHealthManager)(nil)

func (*externalCapabilityHealthManager) RegisterChecker(Checker) {}

func (*externalCapabilityHealthManager) RegisterCheckers(...Checker) {}

func (*externalCapabilityHealthManager) GetLiveness(context.Context) Result {
	return Result{Status: StatusHealthy, Timestamp: time.Now()}
}

func (*externalCapabilityHealthManager) GetReadiness(context.Context) Result {
	return Result{Status: StatusHealthy, Timestamp: time.Now()}
}

func (m *externalCapabilityHealthManager) GetHealth(context.Context) Response {
	m.getHealthCalls++
	return Response{Status: StatusUnhealthy, Timestamp: time.Now()}
}

func (*externalCapabilityHealthManager) GetDetailedHealth(context.Context) DetailedResponse {
	now := time.Now()
	return DetailedResponse{
		Status:    StatusHealthy,
		Timestamp: now,
		Checks: map[string]Result{
			"basic": {Status: StatusHealthy, Timestamp: now},
		},
		Summary: Summary{Total: 1, Healthy: 1},
	}
}

func (m *externalCapabilityHealthManager) DetailedHealthEnabled() bool {
	return m.detailed
}

func (m *externalCapabilityHealthManager) CachedHealth() (Response, bool) {
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
	t.Fatalf("expected %q route when manager opts in through DetailedManager", specs.HealthDetailed)
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
		cached: Response{
			Status:    StatusHealthy,
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
		if status != StatusHealthy {
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
