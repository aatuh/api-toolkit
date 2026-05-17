package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestLoadConfigReadsClaimRequirementsFromEnv(t *testing.T) {
	t.Setenv("JWT_REQUIRE_SUBJECT", "false")
	t.Setenv("JWT_REQUIRE_ISSUED_AT", "true")
	t.Setenv("JWT_REQUIRE_NOT_BEFORE", "false")

	cfg := LoadConfig(nil)

	if cfg.RequiredClaims.RequireSubject == nil || *cfg.RequiredClaims.RequireSubject {
		t.Fatal("expected subject requirement override to be false")
	}
	if cfg.RequiredClaims.RequireIssuedAt == nil || !*cfg.RequiredClaims.RequireIssuedAt {
		t.Fatal("expected issued-at requirement override to be true")
	}
	if cfg.RequiredClaims.RequireNotBefore == nil || *cfg.RequiredClaims.RequireNotBefore {
		t.Fatal("expected not-before requirement override to be false")
	}
}

func TestSubjectRoundTripAndDisabledConstructor(t *testing.T) {
	subj := Subject{UserID: "user-123", Email: "jwt@example.com", Claims: map[string]any{"role": "admin"}}
	ctx := WithSubject(context.Background(), subj)

	got, ok := SubjectFromContext(ctx)
	if !ok {
		t.Fatal("expected subject from context")
	}
	if got.UserID != subj.UserID || got.Email != subj.Email || got.Claims["role"] != "admin" {
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

func TestLoadConfigReadsSkipHeaderSettingsFromEnv(t *testing.T) {
	t.Setenv("JWT_SKIP_HEADER_ENABLED", "true")
	t.Setenv("JWT_SKIP_HEADER_NAME", "X-Skip-JWT")
	t.Setenv("JWT_SKIP_TRUSTED_PROXIES", "10.0.0.0/8,127.0.0.1/32")

	cfg := LoadConfig(nil)
	if !cfg.SkipHeaderEnabled {
		t.Fatal("expected skip header to be enabled")
	}
	if cfg.SkipHeaderName != "X-Skip-JWT" {
		t.Fatalf("SkipHeaderName = %q, want X-Skip-JWT", cfg.SkipHeaderName)
	}
	if len(cfg.SkipTrustedProxies) != 2 {
		t.Fatalf("SkipTrustedProxies len = %d, want 2", len(cfg.SkipTrustedProxies))
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
