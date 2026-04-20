package config

import "testing"

func TestLoadFromEnvNormalizesValidSemanticValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://db.example/internal")
	t.Setenv("LOG_LEVEL", "ERROR")
	t.Setenv("ENV", "PRODUCTION")

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.LogLevel != "error" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "error")
	}
	if cfg.Env != "production" {
		t.Fatalf("Env = %q, want %q", cfg.Env, "production")
	}
}

func TestLoadFromEnvAggregatesSemanticValidationErrors(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://db.example/internal")
	t.Setenv("LOG_LEVEL", "verbose")
	t.Setenv("ENV", "qa")

	_, err := LoadFromEnv(nil)
	assertErrorContains(t, err, "invalid value for LOG_LEVEL: verbose (allowed: debug, info, warn, error)")
	assertErrorContains(t, err, "invalid value for ENV: qa (allowed: development, staging, production)")
}

func TestLoadFromEnvAggregatesSemanticAndParsingErrors(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://db.example/internal")
	t.Setenv("MIGRATE_ON_START", "not-a-bool")
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := LoadFromEnv(nil)
	assertErrorContains(t, err, "invalid bool for MIGRATE_ON_START: not-a-bool")
	assertErrorContains(t, err, "invalid value for LOG_LEVEL: verbose (allowed: debug, info, warn, error)")
}
