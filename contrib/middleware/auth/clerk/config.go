package clerk

import (
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/config"
)

// LoadConfig reads Clerk config from environment.
func LoadConfig(loader *config.Loader) Config {
	if loader == nil {
		loader = config.NewLoader()
	}
	return Config{
		Enabled:                   loader.Bool("CLERK_AUTH_ENABLED", true),
		JWKSURL:                   loader.String("CLERK_JWKS_URL", ""),
		Issuer:                    loader.String("CLERK_ISSUER", ""),
		Audience:                  loader.String("CLERK_AUDIENCE", ""),
		AllowedAlgorithms:         loader.CSV("CLERK_ALLOWED_ALGORITHMS"),
		JWKSRefreshInterval:       loader.Duration("CLERK_JWKS_REFRESH_INTERVAL", 10*time.Minute),
		JWKSRefreshTimeout:        loader.Duration("CLERK_JWKS_REFRESH_TIMEOUT", 5*time.Second),
		AllowedClockSkew:          loader.Duration("CLERK_ALLOWED_CLOCK_SKEW", 30*time.Second),
		AllowDangerousDevBypasses: loader.Bool("CLERK_ALLOW_DANGEROUS_DEV_BYPASSES", false),
		SkipHeaderEnabled:         loader.Bool("CLERK_SKIP_HEADER_ENABLED", false),
		SkipHeaderName:            loader.String("CLERK_SKIP_HEADER_NAME", ""),
		SkipTrustedProxies:        loader.CSV("CLERK_SKIP_TRUSTED_PROXIES"),
	}
}
