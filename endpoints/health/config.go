package health

import (
	"os"
	"strings"
	"time"
)

// Config describes periodic health refresh settings.
type Config struct {
	RefreshInterval time.Duration
	CacheDuration   time.Duration
}

// DurationLoader supplies duration values for configuration.
type DurationLoader interface {
	Duration(key string, def time.Duration) time.Duration
}

// LoadConfig reads health cache config from environment.
func LoadConfig(loader DurationLoader) Config {
	if loader == nil {
		loader = envDurationLoader{}
	}
	refresh := loader.Duration("HEALTH_REFRESH_INTERVAL", 30*time.Second)
	cache := loader.Duration("HEALTH_CACHE_DURATION", 2*refresh)
	if cache <= 0 {
		cache = 2 * refresh
	}
	return Config{RefreshInterval: refresh, CacheDuration: cache}
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
