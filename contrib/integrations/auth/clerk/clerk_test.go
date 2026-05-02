package clerk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg := LoadConfig(nil)

	if !cfg.Enabled {
		t.Fatal("expected Clerk auth to default to enabled")
	}
	if cfg.JWKSRefreshInterval != 10*time.Minute {
		t.Fatalf("JWKSRefreshInterval = %v, want %v", cfg.JWKSRefreshInterval, 10*time.Minute)
	}
	if cfg.JWKSRefreshTimeout != 5*time.Second {
		t.Fatalf("JWKSRefreshTimeout = %v, want %v", cfg.JWKSRefreshTimeout, 5*time.Second)
	}
}

func TestSubjectRoundTripAndDisabledConstructor(t *testing.T) {
	subj := Subject{UserID: "user-123", Email: "clerk@example.com"}
	ctx := WithSubject(context.Background(), subj)

	got, ok := SubjectFromContext(ctx)
	if !ok {
		t.Fatal("expected subject from context")
	}
	if got != subj {
		t.Fatalf("SubjectFromContext() = %#v, want %#v", got, subj)
	}

	mw, err := NewMiddleware(context.Background(), Config{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("NewMiddleware() error = %v", err)
	}
	if mw == nil {
		t.Fatal("expected middleware instance")
	}
}

func TestHealthCheckerNilWhenDisabledOrMissingURL(t *testing.T) {
	if checker := HealthChecker(Config{Enabled: false, JWKSURL: "https://example.com/jwks"}, nil); checker != nil {
		t.Fatal("expected disabled checker to be nil")
	}
	if checker := HealthChecker(Config{Enabled: true}, nil); checker != nil {
		t.Fatal("expected checker with missing JWKS URL to be nil")
	}
}

func TestHealthCheckerUsesConfiguredClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := HealthChecker(Config{Enabled: true, JWKSURL: server.URL, JWKSRefreshTimeout: time.Second}, server.Client())
	if checker == nil {
		t.Fatal("expected health checker")
	}
	result := checker.Check(context.Background())
	if result.Status != ports.HealthStatusHealthy {
		t.Fatalf("status = %s, want healthy: %s", result.Status, result.Message)
	}
}
