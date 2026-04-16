package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoaderBoolRecordsInvalidValue(t *testing.T) {
	t.Setenv("BOOL_KEY", "not-a-bool")

	loader := NewLoader()
	got := loader.Bool("BOOL_KEY", true)

	if !got {
		t.Fatal("expected default bool value")
	}
	assertErrorContains(t, loader.Err(), "invalid bool for BOOL_KEY: not-a-bool")
}

func TestLoaderIntRecordsInvalidValue(t *testing.T) {
	t.Setenv("INT_KEY", "not-an-int")

	loader := NewLoader()
	got := loader.Int("INT_KEY", 42)

	if got != 42 {
		t.Fatalf("expected default int value, got %d", got)
	}
	assertErrorContains(t, loader.Err(), "invalid int for INT_KEY: not-an-int")
}

func TestLoaderDurationRecordsInvalidValue(t *testing.T) {
	t.Setenv("DURATION_KEY", "tomorrow")

	loader := NewLoader()
	got := loader.Duration("DURATION_KEY", 5*time.Second)

	if got != 5*time.Second {
		t.Fatalf("expected default duration, got %s", got)
	}
	assertErrorContains(t, loader.Err(), "invalid duration for DURATION_KEY: tomorrow")
}

func TestLoaderAggregatesMixedValidAndInvalidValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://db.example/internal")
	t.Setenv("BOOL_KEY", "not-a-bool")
	t.Setenv("INT_KEY", "123")
	t.Setenv("DURATION_KEY", "later")

	loader := NewLoader()
	if got := loader.Require("DATABASE_URL"); got != "postgres://db.example/internal" {
		t.Fatalf("database url = %q", got)
	}
	if got := loader.Bool("BOOL_KEY", false); got {
		t.Fatalf("expected default false for invalid bool")
	}
	if got := loader.Int("INT_KEY", 0); got != 123 {
		t.Fatalf("int = %d", got)
	}
	if got := loader.Duration("DURATION_KEY", time.Second); got != time.Second {
		t.Fatalf("duration = %s", got)
	}

	err := loader.Err()
	assertErrorContains(t, err, "invalid bool for BOOL_KEY: not-a-bool")
	assertErrorContains(t, err, "invalid duration for DURATION_KEY: later")
}

func TestLoadFromEnvReturnsErrorForInvalidStartupBool(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://db.example/internal")
	t.Setenv("MIGRATE_ON_START", "not-a-bool")

	_, err := LoadFromEnv(nil)
	assertErrorContains(t, err, "invalid bool for MIGRATE_ON_START: not-a-bool")
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %v", want, err)
	}
}
