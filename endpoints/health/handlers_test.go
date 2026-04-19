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
		if status != ports.HealthStatusHealthy {
			t.Fatalf("expected healthy status, got %q", status)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
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

func TestRegisterRoutesToUsesMinimalRegistrar(t *testing.T) {
	handler := NewHandler(nil)
	router := &stubRouteRegistrar{}

	handler.RegisterRoutesTo(router)

	expected := []string{
		specs.Livez,
		specs.Readyz,
		specs.Healthz,
		specs.Health,
		specs.HealthDetailed,
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
