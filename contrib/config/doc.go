// Package config provides fail-fast configuration loading helpers.
//
// Missing required values and invalid present values are reported as errors by
// LoadFromEnv and trigger a panic through MustLoadFromEnv, making startup
// configuration failures explicit instead of silently falling back to defaults.
package config
