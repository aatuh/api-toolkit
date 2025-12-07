package config

import (
	"github.com/aatuh/api-toolkit/adapters/envvar"
)

type Config struct {
	Addr           string `env:"API_ADDR"`         // host:port|:port
	DatabaseURL    string `env:"DATABASE_URL"`     // required
	LogLevel       string `env:"LOG_LEVEL"`        // debug|info|warn|error
	MigrateOnStart bool   `env:"MIGRATE_ON_START"` // true|false
	MigrationsDir  string `env:"MIGRATIONS_DIR"`   // plain - means use embedded
	Env            string `env:"ENV"`              // development|staging|production

	// Optional rate-limit bypass for test/dev environments.
	RateLimitSkipEnabled bool   `env:"RATE_LIMIT_SKIP_ENABLED"` // true|false
	RateLimitSkipHeader  string `env:"RATE_LIMIT_SKIP_HEADER"`  // header name
}

// MustLoadFromEnv loads config or panics if required values are missing.
func MustLoadFromEnv() Config {
	adapter := envvar.New()
	cfg := Config{
		Addr:                 adapter.GetOr("API_ADDR", ":8000"),
		DatabaseURL:          adapter.MustGet("DATABASE_URL"),
		LogLevel:             adapter.GetOr("LOG_LEVEL", "info"),
		MigrateOnStart:       adapter.GetBoolOr("MIGRATE_ON_START", false),
		MigrationsDir:        adapter.GetOr("MIGRATIONS_DIR", "-"),
		Env:                  adapter.GetOr("ENV", "development"),
		RateLimitSkipEnabled: adapter.GetBoolOr("RATE_LIMIT_SKIP_ENABLED", false),
		RateLimitSkipHeader:  adapter.GetOr("RATE_LIMIT_SKIP_HEADER", ""),
	}
	return cfg
}
