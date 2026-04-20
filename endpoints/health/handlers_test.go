package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/specs"
)

type stubRouteRegistrar struct {
	patterns []string
}

func (s *stubRouteRegistrar) Get(pattern string, _ http.HandlerFunc) {
	s.patterns = append(s.patterns, pattern)
}

func TestNewHandlerDefaultsNilManager(t *testing.T) {
	handler := NewHandler(nil)
	if handler == nil {
		t.Fatal("expected handler")
	}
	if handler.manager == nil {
		t.Fatal("expected default health manager")
	}
}

func TestHealthHandlerWithNilManagerUsesDefaultManager(t *testing.T) {
	handler := NewHandler(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	handler.HealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if _, ok := body["status"]; !ok {
		t.Fatal("expected health response status field")
	}
}

func TestMiddlewareWithNilManagerAddsHealthContext(t *testing.T) {
	handler := NewHandler(nil)
	middleware := handler.Middleware()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, ok := HealthStatusFromContext(r.Context())
		if !ok {
			t.Fatal("expected health status in context")
		}
		if status != ports.HealthStatusUnknown {
			t.Fatalf("expected unknown status without cached snapshot, got %q", status)
		}
		if _, ok := HealthTimestampFromContext(r.Context()); !ok {
			t.Fatal("expected health timestamp in context")
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestMiddlewareUsesCachedHealthSnapshotWithoutProbing(t *testing.T) {
	manager := &probeCountingHealthManager{
		cached: ports.HealthResponse{
			Status:    ports.HealthStatusHealthy,
			Timestamp: time.Unix(123, 0),
			Message:   "cached",
		},
	}
	handler := NewHandler(manager)
	middleware := handler.Middleware()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, ok := HealthStatusFromContext(r.Context())
		if !ok {
			t.Fatal("expected health status in context")
		}
		if status != ports.HealthStatusHealthy {
			t.Fatalf("expected cached healthy status, got %q", status)
		}
		timestamp, ok := HealthTimestampFromContext(r.Context())
		if !ok {
			t.Fatal("expected health timestamp in context")
		}
		if !timestamp.Equal(time.Unix(123, 0)) {
			t.Fatalf("expected cached timestamp, got %s", timestamp)
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

func TestNewDefaultHandlerWithNilPoolSkipsDatabaseCheck(t *testing.T) {
	handler := NewDefaultHandler(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	handler.ReadinessHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Status ports.HealthStatus `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if body.Status != ports.HealthStatusHealthy {
		t.Fatalf("expected healthy readiness status, got %q", body.Status)
	}
}

func TestNewDefaultHandlerDefaultProbesCoverRegisteredChecks(t *testing.T) {
	handler := NewDefaultHandler(nil)

	manager, ok := handler.manager.(*Manager)
	if !ok {
		t.Fatalf("expected concrete manager, got %T", handler.manager)
	}

	for _, checker := range []string{"basic", "memory"} {
		if _, ok := manager.checkers[checker]; !ok {
			t.Fatalf("expected checker %q to be registered", checker)
		}
		if !containsCheck(manager.config.LivenessChecks, checker) {
			t.Fatalf("expected checker %q in liveness checks", checker)
		}
		if !containsCheck(manager.config.ReadinessChecks, checker) {
			t.Fatalf("expected checker %q in readiness checks", checker)
		}
	}
}

func TestRegisterRoutesToUsesMinimalRegistrar(t *testing.T) {
	handler := NewHandler(nil)
	router := &stubRouteRegistrar{}

	handler.RegisterRoutesTo(router)

	expected := []string{
		specs.Livez,
		specs.Readyz,
		specs.Healthz,
		specs.Health,
	}
	if len(router.patterns) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(router.patterns))
	}
	for i := range expected {
		if router.patterns[i] != expected[i] {
			t.Fatalf("route %d = %q", i, router.patterns[i])
		}
	}
}

func TestNewHandlerDisablesDetailedHealthByDefault(t *testing.T) {
	handler := NewHandler(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.HealthDetailed, nil)
	handler.DetailedHealthHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRegisterRoutesToSkipsDetailedHealthWhenDisabled(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         time.Second,
		EnableDetailed:  false,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(NewBasicChecker())

	handler := NewHandler(manager)
	router := &stubRouteRegistrar{}

	handler.RegisterRoutesTo(router)

	for _, pattern := range router.patterns {
		if pattern == specs.HealthDetailed {
			t.Fatalf("did not expect %q route when detailed health is disabled", specs.HealthDetailed)
		}
	}
}

func TestDetailedHealthHandlerReturnsNotFoundWhenDisabled(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         time.Second,
		EnableDetailed:  false,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(NewBasicChecker())

	handler := NewHandler(manager)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.HealthDetailed, nil)
	handler.DetailedHealthHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected problem content type, got %q", got)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode detailed health problem: %v", err)
	}
	if got := body["detail"]; got != "detailed health is disabled" {
		t.Fatalf("expected disabled detail, got %#v", got)
	}
	if got := body["status"]; got != float64(http.StatusNotFound) {
		t.Fatalf("expected problem status 404, got %#v", got)
	}
}

func TestRegisterCustomRoutesToSkipsDetailedHealthWhenDisabled(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         time.Second,
		EnableDetailed:  false,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(NewBasicChecker())

	handler := NewHandler(manager)
	router := &stubRouteRegistrar{}

	handler.RegisterCustomRoutesTo(router, HealthPaths{
		Liveness:       "/live",
		Readiness:      "/ready",
		Health:         "/health",
		DetailedHealth: "/health/detailed",
	})

	for _, pattern := range router.patterns {
		if pattern == "/health/detailed" {
			t.Fatal("did not expect custom detailed route when detailed health is disabled")
		}
	}
}

type externalHealthManager struct{}

func (externalHealthManager) RegisterChecker(ports.HealthChecker) {}

func (externalHealthManager) RegisterCheckers(...ports.HealthChecker) {}

func (externalHealthManager) GetLiveness(context.Context) ports.HealthResult {
	return ports.HealthResult{Status: ports.HealthStatusHealthy, Timestamp: time.Now()}
}

func (externalHealthManager) GetReadiness(context.Context) ports.HealthResult {
	return ports.HealthResult{Status: ports.HealthStatusHealthy, Timestamp: time.Now()}
}

func (externalHealthManager) GetHealth(context.Context) ports.HealthResponse {
	return ports.HealthResponse{Status: ports.HealthStatusHealthy, Timestamp: time.Now()}
}

func (externalHealthManager) GetDetailedHealth(context.Context) ports.DetailedHealthResponse {
	return ports.DetailedHealthResponse{
		Status:    ports.HealthStatusHealthy,
		Timestamp: time.Now(),
		Checks: map[string]ports.HealthResult{
			"db": {Status: ports.HealthStatusHealthy, Timestamp: time.Now()},
		},
		Summary: ports.HealthSummary{Total: 1, Healthy: 1},
	}
}

type probeCountingHealthManager struct {
	cached         ports.HealthResponse
	getHealthCalls int
}

func (*probeCountingHealthManager) RegisterChecker(ports.HealthChecker) {}

func (*probeCountingHealthManager) RegisterCheckers(...ports.HealthChecker) {}

func (*probeCountingHealthManager) GetLiveness(context.Context) ports.HealthResult {
	return ports.HealthResult{Status: ports.HealthStatusHealthy, Timestamp: time.Now()}
}

func (*probeCountingHealthManager) GetReadiness(context.Context) ports.HealthResult {
	return ports.HealthResult{Status: ports.HealthStatusHealthy, Timestamp: time.Now()}
}

func (m *probeCountingHealthManager) GetHealth(context.Context) ports.HealthResponse {
	m.getHealthCalls++
	return ports.HealthResponse{Status: ports.HealthStatusUnhealthy, Timestamp: time.Now()}
}

func (*probeCountingHealthManager) GetDetailedHealth(context.Context) ports.DetailedHealthResponse {
	return ports.DetailedHealthResponse{Status: ports.HealthStatusHealthy, Timestamp: time.Now()}
}

func (m *probeCountingHealthManager) CachedHealth() (ports.HealthResponse, bool) {
	return m.cached, true
}

func TestRegisterRoutesToSkipsDetailedHealthForManagersWithoutOptIn(t *testing.T) {
	handler := NewHandler(externalHealthManager{})
	router := &stubRouteRegistrar{}

	handler.RegisterRoutesTo(router)

	for _, pattern := range router.patterns {
		if pattern == specs.HealthDetailed {
			t.Fatalf("did not expect %q route without explicit detailed-health opt-in", specs.HealthDetailed)
		}
	}
}

func TestDetailedHealthHandlerReturnsNotFoundForManagersWithoutOptIn(t *testing.T) {
	handler := NewHandler(externalHealthManager{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.HealthDetailed, nil)
	handler.DetailedHealthHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDefaultHealthPathsUseCanonicalSpecsEndpoints(t *testing.T) {
	paths := DefaultHealthPaths()

	if paths.Liveness != specs.Livez {
		t.Fatalf("expected liveness path %q, got %q", specs.Livez, paths.Liveness)
	}
	if paths.Readiness != specs.Readyz {
		t.Fatalf("expected readiness path %q, got %q", specs.Readyz, paths.Readiness)
	}
	if paths.Health != specs.Health {
		t.Fatalf("expected health path %q, got %q", specs.Health, paths.Health)
	}
	if paths.DetailedHealth != specs.HealthDetailed {
		t.Fatalf("expected detailed health path %q, got %q", specs.HealthDetailed, paths.DetailedHealth)
	}
}

func containsCheck(checks []string, want string) bool {
	for _, check := range checks {
		if check == want {
			return true
		}
	}
	return false
}
