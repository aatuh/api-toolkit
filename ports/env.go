package ports

import "time"

// EnvVar manages environment variables with typed getters.
type EnvVar interface {
	// LoadEnvFiles loads environment variables from specific files.
	LoadEnvFiles(paths []string) error
	// Get returns the value and if it exists.
	Get(key string) (string, bool)
	// GetOr returns the value or default if not present.
	GetOr(key, def string) string
	// MustGet returns the value or panics if not present.
	MustGet(key string) string
	// GetBoolOr returns the value as boolean or default if not present.
	GetBoolOr(key string, def bool) bool
	// MustGetBool returns the value as a boolean or panics if not present.
	MustGetBool(key string) bool
	// GetIntOr returns the value as an integer or default if not present.
	GetIntOr(key string, def int) int
	// MustGetInt returns the value as an integer or panics if not present.
	MustGetInt(key string) int
	// GetInt64Or returns the value as an int64 or default if not present.
	GetInt64Or(key string, def int64) int64
	// MustGetInt64 returns the value as an int64 or panics if not present.
	MustGetInt64(key string) int64
	// GetUintOr returns the value as a uint or default if not present.
	GetUintOr(key string, def uint) uint
	// MustGetUint returns the value as a uint or panics if not present.
	MustGetUint(key string) uint
	// GetUint64Or returns the value as a uint64 or default if not present.
	GetUint64Or(key string, def uint64) uint64
	// MustGetUint64 returns the value as a uint64 or panics if not present.
	MustGetUint64(key string) uint64
	// GetFloat64Or returns the value as a float64 or default if not present.
	GetFloat64Or(key string, def float64) float64
	// MustGetFloat64 returns the value as a float64 or panics if not present.
	MustGetFloat64(key string) float64
	// MustGetDuration returns the value as a duration or panics if not present.
	MustGetDuration(key string) time.Duration
	// GetDurationOr returns the value as a duration or default if not present.
	GetDurationOr(key string, def time.Duration) time.Duration
	// Bind populates a struct from environment variables.
	Bind(dst any) error
	// MustBind panics on binding errors.
	MustBind(dst any)
	// BindWithPrefix binds with a prefix.
	BindWithPrefix(dst any, prefix string) error
	// MustBindWithPrefix panics on binding errors with prefix.
	MustBindWithPrefix(dst any, prefix string)
	// DumpRedacted returns environment with secrets redacted.
	DumpRedacted() map[string]string
}
