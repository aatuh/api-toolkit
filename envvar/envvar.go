package envvar

import (
	"os"
	"strconv"
	"strings"

	"github.com/aatuh/envvar"
)

// Adapter provides environment variable access using the envvar library.
type Adapter struct{}

// New creates a new envvar adapter.
func New() *Adapter {
	return &Adapter{}
}

// LoadEnvFiles loads environment variables from files.
// Tries .env then /env/.env by default.
func (a *Adapter) LoadEnvFiles(paths []string) error {
	envvar.MustLoadEnvVars(paths)
	return nil
}

// Get returns the raw value and presence indicator.
func (a *Adapter) Get(key string) (string, bool) {
	v := envvar.Get(key)
	return v, v != ""
}

// GetOr returns the value or default if not present.
func (a *Adapter) GetOr(key, def string) string {
	return envvar.GetOr(key, def)
}

// MustGet returns the value or panics if not present.
func (a *Adapter) MustGet(key string) string {
	return envvar.MustGet(key)
}

// GetBoolOr returns the value as boolean or default if not present.
func (a *Adapter) GetBoolOr(key string, def bool) bool {
	return envvar.GetBoolOr(key, def)
}

// MustGetBool returns the value as boolean or panics if not present.
func (a *Adapter) MustGetBool(key string) bool {
	return envvar.MustGetBool(key)
}

// GetIntOr returns the value as integer or default if not present.
func (a *Adapter) GetIntOr(key string, def int) int {
	v := envvar.Get(key)
	if v == "" {
		return def
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	return def
}

// MustGetInt returns the value as integer or panics if not present.
func (a *Adapter) MustGetInt(key string) int {
	return envvar.MustGetInt(key)
}

// GetInt64Or returns the value as int64 or default if not present.
func (a *Adapter) GetInt64Or(key string, def int64) int64 {
	v := envvar.Get(key)
	if v == "" {
		return def
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	return def
}

// MustGetInt64 returns the value as int64 or panics if not present.
func (a *Adapter) MustGetInt64(key string) int64 {
	v := envvar.MustGet(key)
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic("environment variable " + key + " is not a valid int64: " + v)
	}
	return i
}

// GetUintOr returns the value as uint or default if not present.
func (a *Adapter) GetUintOr(key string, def uint) uint {
	v := envvar.Get(key)
	if v == "" {
		return def
	}
	if i, err := strconv.ParseUint(v, 10, 32); err == nil {
		return uint(i)
	}
	return def
}

// MustGetUint returns the value as uint or panics if not present.
func (a *Adapter) MustGetUint(key string) uint {
	v := envvar.MustGet(key)
	i, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		panic("environment variable " + key + " is not a valid uint: " + v)
	}
	return uint(i)
}

// GetUint64Or returns the value as uint64 or default if not present.
func (a *Adapter) GetUint64Or(key string, def uint64) uint64 {
	v := envvar.Get(key)
	if v == "" {
		return def
	}
	if i, err := strconv.ParseUint(v, 10, 64); err == nil {
		return i
	}
	return def
}

// MustGetUint64 returns the value as uint64 or panics if not present.
func (a *Adapter) MustGetUint64(key string) uint64 {
	v := envvar.MustGet(key)
	i, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		panic("environment variable " + key + " is not a valid uint64: " + v)
	}
	return i
}

// GetFloat64Or returns the value as float64 or default if not present.
func (a *Adapter) GetFloat64Or(key string, def float64) float64 {
	v := envvar.Get(key)
	if v == "" {
		return def
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return def
}

// MustGetFloat64 returns the value as float64 or panics if not present.
func (a *Adapter) MustGetFloat64(key string) float64 {
	v := envvar.MustGet(key)
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		panic("environment variable " + key + " is not a valid float64: " + v)
	}
	return f
}

// GetDurationOr returns the value as duration or default if not present.
func (a *Adapter) GetDurationOr(key string, def int64) int64 {
	return a.GetInt64Or(key, def)
}

// MustGetDuration returns the value as duration or panics if not present.
func (a *Adapter) MustGetDuration(key string) int64 {
	return a.MustGetInt64(key)
}

// Bind populates a struct from environment variables.
// This is a simplified implementation that doesn't use struct tags.
func (a *Adapter) Bind(dst any) error {
	// TODO: implement struct binding if needed
	return nil
}

// MustBind panics on binding errors.
func (a *Adapter) MustBind(dst any) {
	if err := a.Bind(dst); err != nil {
		panic(err)
	}
}

// BindWithPrefix binds with a prefix.
func (a *Adapter) BindWithPrefix(dst any, prefix string) error {
	// TODO: implement prefix binding if needed
	return nil
}

// MustBindWithPrefix panics on binding errors with prefix.
func (a *Adapter) MustBindWithPrefix(dst any, prefix string) {
	if err := a.BindWithPrefix(dst, prefix); err != nil {
		panic(err)
	}
}

// DumpRedacted returns environment with secrets redacted.
func (a *Adapter) DumpRedacted() map[string]string {
	env := os.Environ()
	out := make(map[string]string, len(env))
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(k)
		if strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "TOKEN") ||
			strings.Contains(upper, "PASSWORD") ||
			strings.HasSuffix(upper, "_KEY") {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}
