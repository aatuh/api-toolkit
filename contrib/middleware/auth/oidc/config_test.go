package oidc

import (
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/config"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("OIDC_AUTH_ENABLED", "true")
	t.Setenv("OIDC_ISSUER", "https://issuer.example")
	t.Setenv("OIDC_AUDIENCE", "api")
	t.Setenv("OIDC_DISCOVERY_URL", "https://issuer.example/.well-known/openid-configuration")
	t.Setenv("OIDC_JWKS_URL", "https://issuer.example/jwks.json")
	t.Setenv("OIDC_TENANT_CLAIM", "organization_id")
	t.Setenv("OIDC_SCOPE_CLAIM", "permissions")
	t.Setenv("OIDC_ALLOWED_ALGORITHMS", "RS256, ES256")
	t.Setenv("OIDC_JWKS_REFRESH_INTERVAL", "2m")
	t.Setenv("OIDC_JWKS_REFRESH_TIMEOUT", "3s")
	t.Setenv("OIDC_ALLOWED_CLOCK_SKEW", "4s")
	t.Setenv("OIDC_REQUIRE_ISSUED_AT", "true")

	cfg := LoadConfig(config.NewLoader())
	if !cfg.Enabled || cfg.Issuer != "https://issuer.example" || cfg.Audience != "api" {
		t.Fatalf("config = %#v", cfg)
	}
	if cfg.DiscoveryURL == "" || cfg.JWKSURL == "" || cfg.TenantClaim != "organization_id" || cfg.ScopeClaim != "permissions" {
		t.Fatalf("config = %#v", cfg)
	}
	if len(cfg.AllowedAlgorithms) != 2 || cfg.AllowedAlgorithms[1] != "ES256" {
		t.Fatalf("AllowedAlgorithms = %#v", cfg.AllowedAlgorithms)
	}
	if cfg.JWKSRefreshInterval != 2*time.Minute || cfg.JWKSRefreshTimeout != 3*time.Second || cfg.AllowedClockSkew != 4*time.Second {
		t.Fatalf("durations = %#v", cfg)
	}
	if cfg.RequiredClaims.RequireIssuedAt == nil || !*cfg.RequiredClaims.RequireIssuedAt {
		t.Fatalf("required claims = %#v", cfg.RequiredClaims)
	}
}
