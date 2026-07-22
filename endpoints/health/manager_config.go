package health

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultManagerTimeout       = 5 * time.Second
	defaultManagerCacheDuration = 5 * time.Second
)

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

// DefaultConfig returns the explicit production defaults for a health manager.
func DefaultConfig() Config {
	return Config{
		Timeout:         defaultManagerTimeout,
		CacheDuration:   defaultManagerCacheDuration,
		EnableCaching:   true,
		EnableDetailed:  false,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
		Clock:           wallClock{},
	}
}

// Validate checks that a manager configuration will fail closed rather than
// silently disabling probe or cache behavior.
func (c Config) Validate() error {
	if c.Timeout <= 0 {
		return errors.New("health manager timeout must be greater than zero")
	}
	if c.CacheDuration <= 0 {
		return errors.New("health manager cache duration must be greater than zero")
	}
	if len(c.LivenessChecks) == 0 && len(c.ReadinessChecks) == 0 {
		return errors.New("health manager has no liveness or readiness checks configured")
	}
	if err := validateCheckNames("liveness", c.LivenessChecks); err != nil {
		return err
	}
	if err := validateCheckNames("readiness", c.ReadinessChecks); err != nil {
		return err
	}
	return nil
}

func validateCheckNames(kind string, names []string) error {
	seen := make(map[string]struct{}, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			return fmt.Errorf("health manager %s checker name is empty", kind)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("health manager %s checker %q is duplicated", kind, name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func normalizeManagerConfig(config Config) Config {
	if config.Clock == nil {
		config.Clock = wallClock{}
	}
	config.LivenessChecks = normalizeCheckNames(config.LivenessChecks)
	config.ReadinessChecks = normalizeCheckNames(config.ReadinessChecks)
	return config
}

func normalizeCheckNames(names []string) []string {
	normalized := make([]string, len(names))
	for i, name := range names {
		normalized[i] = strings.TrimSpace(name)
	}
	return normalized
}

func normalizeLegacyConfig(config Config) Config {
	if config.Timeout <= 0 {
		config.Timeout = defaultManagerTimeout
	}
	return normalizeManagerConfig(config)
}
