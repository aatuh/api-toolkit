package oidc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/config"
	"github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/oidc"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestIntegrationAliasesMiddleware(t *testing.T) {
	ctx := WithSubject(context.Background(), Subject{UserID: "user_1", TenantID: "org_1"})
	subject, ok := SubjectFromContext(ctx)
	if !ok || subject.UserID != "user_1" || subject.TenantID != "org_1" {
		t.Fatalf("SubjectFromContext() = %#v %v", subject, ok)
	}

	t.Setenv("OIDC_AUTH_ENABLED", "false")
	cfg := LoadConfig(config.NewLoader())
	mw, err := NewMiddleware(context.Background(), cfg, ports.NopLogger{})
	if err != nil {
		t.Fatalf("NewMiddleware() error = %v", err)
	}
	mw.Close()
}

func TestHealthCheckerDelegatesToMiddleware(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := HealthChecker(oidc.Config{Enabled: true, JWKSURL: server.URL, JWKSRefreshTimeout: time.Second}, server.Client())
	result := checker.Check(context.Background())
	if result.Status != ports.HealthStatusHealthy {
		t.Fatalf("status = %s", result.Status)
	}
}
