package config

import (
	"github.com/aatuh/api-toolkit/envvar"
)

type Config struct {
	Addr           string `env:"API_ADDR"`         // ":8000"
	DatabaseURL    string `env:"DATABASE_URL"`     // required
	LogLevel       string `env:"LOG_LEVEL"`        // "debug"|"info"|"warn"|"error"
	MigrateOnStart bool   `env:"MIGRATE_ON_START"` // MIGRATE_ON_START
	MigrationsDir  string `env:"MIGRATIONS_DIR"`   // "-" means use embedded
	Env            string `env:"ENV"`              // "development"|"staging"|"production"
}

// MustLoadFromEnv loads config or panics if required values are missing.
func MustLoadFromEnv() Config {
	adapter := envvar.New()
	cfg := Config{
		Addr:           adapter.GetOr("API_ADDR", ":8000"),
		DatabaseURL:    adapter.MustGet("DATABASE_URL"),
		LogLevel:       adapter.GetOr("LOG_LEVEL", "info"),
		MigrateOnStart: adapter.GetBoolOr("MIGRATE_ON_START", false),
		MigrationsDir:  adapter.GetOr("MIGRATIONS_DIR", "-"),
		Env:            adapter.GetOr("ENV", "development"),
	}
	return cfg
}
