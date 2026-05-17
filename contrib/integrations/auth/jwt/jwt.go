package jwt

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/config"
	jwtmw "github.com/aatuh/api-toolkit/v3/middleware/auth/jwt"
	"github.com/aatuh/api-toolkit/v3/ports"
)

// Config aliases the JWT middleware configuration.
type Config = jwtmw.Config

// Middleware aliases the JWT middleware.
type Middleware = jwtmw.Middleware

// Subject aliases the JWT subject payload.
type Subject = jwtmw.Subject

// NewMiddleware creates a JWT middleware.
func NewMiddleware(ctx context.Context, cfg Config, log ports.Logger) (*Middleware, error) {
	return jwtmw.NewMiddleware(ctx, cfg, log)
}

// LoadConfig reads JWT configuration from environment.
func LoadConfig(loader *config.Loader) Config {
	if loader == nil {
		loader = config.NewLoader()
	}
	cfg := Config{
		Enabled:                   loader.Bool("JWT_AUTH_ENABLED", true),
		JWKSURL:                   loader.String("JWT_JWKS_URL", ""),
		Issuer:                    loader.String("JWT_ISSUER", ""),
		Audience:                  loader.String("JWT_AUDIENCE", ""),
		AllowedAlgorithms:         loader.CSV("JWT_ALLOWED_ALGORITHMS"),
		JWKSRefreshInterval:       loader.Duration("JWT_JWKS_REFRESH_INTERVAL", 10*time.Minute),
		JWKSRefreshTimeout:        loader.Duration("JWT_JWKS_REFRESH_TIMEOUT", 5*time.Second),
		AllowedClockSkew:          loader.Duration("JWT_ALLOWED_CLOCK_SKEW", 30*time.Second),
		AllowDangerousDevBypasses: loader.Bool("JWT_ALLOW_DANGEROUS_DEV_BYPASSES", false),
		SkipHeaderEnabled:         loader.Bool("JWT_SKIP_HEADER_ENABLED", false),
		SkipHeaderName:            loader.String("JWT_SKIP_HEADER_NAME", ""),
		SkipTrustedProxies:        loader.CSV("JWT_SKIP_TRUSTED_PROXIES"),
	}

	cfg.RequiredClaims = claimRequirementsFromEnv()
	return cfg
}

// WithSubject stores the authenticated subject in context.
func WithSubject(ctx context.Context, subj Subject) context.Context {
	return jwtmw.WithSubject(ctx, subj)
}

// SubjectFromContext retrieves the subject from context.
func SubjectFromContext(ctx context.Context) (Subject, bool) {
	return jwtmw.SubjectFromContext(ctx)
}

// HealthChecker returns a JWKS health checker.
func HealthChecker(cfg Config, client *http.Client) ports.HealthChecker {
	return jwtmw.HealthChecker(cfg, client)
}

func claimRequirementsFromEnv() jwtmw.ClaimRequirements {
	req := jwtmw.ClaimRequirements{}
	if v, ok := envBool("JWT_REQUIRE_SUBJECT"); ok {
		req.RequireSubject = &v
	}
	if v, ok := envBool("JWT_REQUIRE_EXPIRATION"); ok {
		req.RequireExpiration = &v
	}
	if v, ok := envBool("JWT_REQUIRE_ISSUED_AT"); ok {
		req.RequireIssuedAt = &v
	}
	if v, ok := envBool("JWT_REQUIRE_NOT_BEFORE"); ok {
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
