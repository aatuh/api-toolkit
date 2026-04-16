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
	val := strings.TrimSpace(l.env.GetOr(key, ""))
	if val == "" {
		l.errs = append(l.errs, fmt.Errorf("missing env %s", key))
	}
	return val
}

// String reads a string env var with a default.
func (l *Loader) String(key, def string) string {
	return l.env.GetOr(key, def)
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
	val := strings.TrimSpace(l.env.GetOr(key, ""))
	if val == "" {
		return def
	}
	dur, err := time.ParseDuration(val)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("invalid duration for %s: %s", key, val))
		return def
	}
	return dur
}

// CSV reads a comma-separated env var into a slice.
func (l *Loader) CSV(key string) []string {
	return SplitCSV(l.env.GetOr(key, ""))
}

func (l *Loader) raw(key string) (string, bool) {
	if l == nil || l.env == nil {
		return "", false
	}
	val, ok := l.env.Get(key)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(val), true
}

// Err returns aggregated errors, if any.
func (l *Loader) Err() error {
	if len(l.errs) == 0 {
		return nil
	}
	return errors.Join(l.errs...)
}
