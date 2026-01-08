package config

import "fmt"

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
	cfg, err := LoadFromEnv(nil)
	if err != nil {
		panic(err)
	}
	return cfg
}

// LoadFromEnv loads config from environment using an optional loader.
func LoadFromEnv(loader *Loader) (Config, error) {
	if loader == nil {
		loader = NewLoader()
	}
	cfg := Config{
		Addr:                 loader.String("API_ADDR", ":8000"),
		DatabaseURL:          loader.Require("DATABASE_URL"),
		LogLevel:             loader.String("LOG_LEVEL", "info"),
		MigrateOnStart:       loader.Bool("MIGRATE_ON_START", false),
		MigrationsDir:        loader.String("MIGRATIONS_DIR", "-"),
		Env:                  loader.String("ENV", "development"),
		RateLimitSkipEnabled: loader.Bool("RATE_LIMIT_SKIP_ENABLED", false),
		RateLimitSkipHeader:  loader.String("RATE_LIMIT_SKIP_HEADER", ""),
	}
	if err := loader.Err(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
