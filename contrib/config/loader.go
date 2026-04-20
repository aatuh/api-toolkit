package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/envvar"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// Loader reads env vars with defaults and aggregates errors.
type Loader struct {
	env  ports.EnvVar
	errs []error
}

// NewLoader creates a loader backed by the default env adapter.
func NewLoader() *Loader {
	return &Loader{env: envvar.New()}
}

// Require returns the env var or records an error if missing.
func (l *Loader) Require(key string) string {
	val := strings.TrimSpace(l.ensureEnv().GetOr(key, ""))
	if val == "" {
		if l == nil {
			return val
		}
		l.errs = append(l.errs, fmt.Errorf("missing env %s", key))
	}
	return val
}

// String reads a string env var with a default.
func (l *Loader) String(key, def string) string {
	return l.ensureEnv().GetOr(key, def)
}

// Bool reads a bool env var with a default.
func (l *Loader) Bool(key string, def bool) bool {
	val, ok := l.raw(key)
	if !ok {
		return def
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("invalid bool for %s: %s", key, val))
		return def
	}
	return parsed
}

// Int reads an int env var with a default.
func (l *Loader) Int(key string, def int) int {
	val, ok := l.raw(key)
	if !ok {
		return def
	}
	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("invalid int for %s: %s", key, val))
		return def
	}
	return int(parsed)
}

// Duration reads a duration env var with a default.
func (l *Loader) Duration(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(l.ensureEnv().GetOr(key, ""))
	if val == "" {
		return def
	}
	dur, err := time.ParseDuration(val)
	if err != nil {
		if l == nil {
			return def
		}
		l.errs = append(l.errs, fmt.Errorf("invalid duration for %s: %s", key, val))
		return def
	}
	return dur
}

// CSV reads a comma-separated env var into a slice.
func (l *Loader) CSV(key string) []string {
	return SplitCSV(l.ensureEnv().GetOr(key, ""))
}

// OneOf validates an enum-like string value and records an error when the
// normalized value is not in the allowed set.
func (l *Loader) OneOf(key, value string, allowed ...string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if normalized == candidate {
			return normalized
		}
	}
	if l != nil {
		l.errs = append(l.errs, fmt.Errorf(
			"invalid value for %s: %s (allowed: %s)",
			key,
			strings.TrimSpace(value),
			strings.Join(allowed, ", "),
		))
	}
	return normalized
}

func (l *Loader) raw(key string) (string, bool) {
	val, ok := l.ensureEnv().Get(key)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(val), true
}

// Err returns aggregated errors, if any.
func (l *Loader) Err() error {
	if l == nil {
		return nil
	}
	if len(l.errs) == 0 {
		return nil
	}
	return errors.Join(l.errs...)
}

func (l *Loader) ensureEnv() ports.EnvVar {
	if l == nil {
		return envvar.New()
	}
	if l.env == nil {
		l.env = envvar.New()
	}
	return l.env
}
