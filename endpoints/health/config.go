package health

import (
	"os"
	"strings"
	"time"
)

const defaultRefreshInterval = 30 * time.Second

// Config describes periodic health refresh settings.
//
// Contract:
//   - RefreshInterval defaults to 30 seconds when the configured value is
//     missing, invalid, zero, or negative.
//   - CacheDuration defaults to twice the effective RefreshInterval when the
//     configured value is missing, invalid, zero, or negative.
type Config struct {
	RefreshInterval time.Duration
	CacheDuration   time.Duration
}

// DurationLoader supplies duration values for configuration.
type DurationLoader interface {
	Duration(key string, def time.Duration) time.Duration
}

// LoadConfig reads health cache config from environment.
//
// HEALTH_REFRESH_INTERVAL falls back to 30 seconds when unset, invalid, zero,
// or negative. HEALTH_CACHE_DURATION falls back to twice the effective refresh
// interval when unset, invalid, zero, or negative.
func LoadConfig(loader DurationLoader) Config {
	if loader == nil {
		loader = envDurationLoader{}
	}
	refresh := normalizeConfigDuration(
		loader.Duration("HEALTH_REFRESH_INTERVAL", defaultRefreshInterval),
		defaultRefreshInterval,
	)
	cache := loader.Duration("HEALTH_CACHE_DURATION", 2*refresh)
	cache = normalizeConfigDuration(cache, 2*refresh)
	return Config{RefreshInterval: refresh, CacheDuration: cache}
}

func normalizeConfigDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

type envDurationLoader struct{}

func (envDurationLoader) Duration(key string, def time.Duration) time.Duration {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return def
	}
	dur, err := time.ParseDuration(val)
	if err != nil {
		return def
	}
	return dur
}
