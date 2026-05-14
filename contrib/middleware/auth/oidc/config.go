package oidc

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/config"
)

// LoadConfig reads OIDC configuration from environment.
func LoadConfig(loader *config.Loader) Config {
	if loader == nil {
		loader = config.NewLoader()
	}
	cfg := Config{
		Enabled:                   loader.Bool("OIDC_AUTH_ENABLED", true),
		Issuer:                    loader.String("OIDC_ISSUER", ""),
		Audience:                  loader.String("OIDC_AUDIENCE", ""),
		DiscoveryURL:              loader.String("OIDC_DISCOVERY_URL", ""),
		JWKSURL:                   loader.String("OIDC_JWKS_URL", ""),
		TenantClaim:               loader.String("OIDC_TENANT_CLAIM", "tenant_id"),
		ScopeClaim:                loader.String("OIDC_SCOPE_CLAIM", "scope"),
		AllowedAlgorithms:         loader.CSV("OIDC_ALLOWED_ALGORITHMS"),
		JWKSRefreshInterval:       loader.Duration("OIDC_JWKS_REFRESH_INTERVAL", 10*time.Minute),
		JWKSRefreshTimeout:        loader.Duration("OIDC_JWKS_REFRESH_TIMEOUT", 5*time.Second),
		AllowedClockSkew:          loader.Duration("OIDC_ALLOWED_CLOCK_SKEW", 30*time.Second),
		AllowDangerousDevBypasses: loader.Bool("OIDC_ALLOW_DANGEROUS_DEV_BYPASSES", false),
		SkipHeaderEnabled:         loader.Bool("OIDC_SKIP_HEADER_ENABLED", false),
		SkipHeaderName:            loader.String("OIDC_SKIP_HEADER_NAME", ""),
		SkipTrustedProxies:        loader.CSV("OIDC_SKIP_TRUSTED_PROXIES"),
	}
	cfg.RequiredClaims = claimRequirementsFromEnv()
	return cfg
}

func claimRequirementsFromEnv() ClaimRequirements {
	req := ClaimRequirements{}
	if v, ok := envBool("OIDC_REQUIRE_SUBJECT"); ok {
		req.RequireSubject = &v
	}
	if v, ok := envBool("OIDC_REQUIRE_EXPIRATION"); ok {
		req.RequireExpiration = &v
	}
	if v, ok := envBool("OIDC_REQUIRE_ISSUED_AT"); ok {
		req.RequireIssuedAt = &v
	}
	if v, ok := envBool("OIDC_REQUIRE_NOT_BEFORE"); ok {
		req.RequireNotBefore = &v
	}
	return req
}

func envBool(key string) (bool, bool) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, false
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}
	return val, true
}
