package health

import (
	"testing"
	"time"
)

type stubDurationLoader map[string]time.Duration

func (l stubDurationLoader) Duration(key string, def time.Duration) time.Duration {
	if value, ok := l[key]; ok {
		return value
	}
	return def
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg := LoadConfig(stubDurationLoader{})

	if cfg.RefreshInterval != 30*time.Second {
		t.Fatalf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 30*time.Second)
	}
	if cfg.CacheDuration != 60*time.Second {
		t.Fatalf("CacheDuration = %v, want %v", cfg.CacheDuration, 60*time.Second)
	}
}

func TestLoadConfigPositiveOverrides(t *testing.T) {
	cfg := LoadConfig(stubDurationLoader{
		"HEALTH_REFRESH_INTERVAL": 15 * time.Second,
		"HEALTH_CACHE_DURATION":   45 * time.Second,
	})

	if cfg.RefreshInterval != 15*time.Second {
		t.Fatalf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 15*time.Second)
	}
	if cfg.CacheDuration != 45*time.Second {
		t.Fatalf("CacheDuration = %v, want %v", cfg.CacheDuration, 45*time.Second)
	}
}

func TestLoadConfigZeroRefreshFallsBackToDefault(t *testing.T) {
	cfg := LoadConfig(stubDurationLoader{
		"HEALTH_REFRESH_INTERVAL": 0,
	})

	if cfg.RefreshInterval != 30*time.Second {
		t.Fatalf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 30*time.Second)
	}
	if cfg.CacheDuration != 60*time.Second {
		t.Fatalf("CacheDuration = %v, want %v", cfg.CacheDuration, 60*time.Second)
	}
}

func TestLoadConfigZeroCacheFallsBackToDerivedDefault(t *testing.T) {
	cfg := LoadConfig(stubDurationLoader{
		"HEALTH_REFRESH_INTERVAL": 10 * time.Second,
		"HEALTH_CACHE_DURATION":   0,
	})

	if cfg.RefreshInterval != 10*time.Second {
		t.Fatalf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 10*time.Second)
	}
	if cfg.CacheDuration != 20*time.Second {
		t.Fatalf("CacheDuration = %v, want %v", cfg.CacheDuration, 20*time.Second)
	}
}

func TestLoadConfigNegativeRefreshFallsBackToDefault(t *testing.T) {
	cfg := LoadConfig(stubDurationLoader{
		"HEALTH_REFRESH_INTERVAL": -time.Second,
	})

	if cfg.RefreshInterval != 30*time.Second {
		t.Fatalf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 30*time.Second)
	}
	if cfg.CacheDuration != 60*time.Second {
		t.Fatalf("CacheDuration = %v, want %v", cfg.CacheDuration, 60*time.Second)
	}
}

func TestLoadConfigNegativeCacheFallsBackToDerivedDefault(t *testing.T) {
	cfg := LoadConfig(stubDurationLoader{
		"HEALTH_REFRESH_INTERVAL": 25 * time.Second,
		"HEALTH_CACHE_DURATION":   -time.Second,
	})

	if cfg.RefreshInterval != 25*time.Second {
		t.Fatalf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 25*time.Second)
	}
	if cfg.CacheDuration != 50*time.Second {
		t.Fatalf("CacheDuration = %v, want %v", cfg.CacheDuration, 50*time.Second)
	}
}

func TestLoadConfigInvalidDurationsFallBackToDefaults(t *testing.T) {
	t.Setenv("HEALTH_REFRESH_INTERVAL", "not-a-duration")
	t.Setenv("HEALTH_CACHE_DURATION", "still-not-a-duration")

	cfg := LoadConfig(nil)

	if cfg.RefreshInterval != 30*time.Second {
		t.Fatalf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 30*time.Second)
	}
	if cfg.CacheDuration != 60*time.Second {
		t.Fatalf("CacheDuration = %v, want %v", cfg.CacheDuration, 60*time.Second)
	}
}
