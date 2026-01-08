package health

import (
	"time"

	"github.com/aatuh/api-toolkit/config"
)

// Config describes periodic health refresh settings.
type Config struct {
	RefreshInterval time.Duration
	CacheDuration   time.Duration
}

// LoadConfig reads health cache config from environment.
func LoadConfig(loader *config.Loader) Config {
	if loader == nil {
		loader = config.NewLoader()
	}
	refresh := loader.Duration("HEALTH_REFRESH_INTERVAL", 30*time.Second)
	cache := loader.Duration("HEALTH_CACHE_DURATION", 2*refresh)
	if cache <= 0 {
		cache = 2 * refresh
	}
	return Config{RefreshInterval: refresh, CacheDuration: cache}
}
