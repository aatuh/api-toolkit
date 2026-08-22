package health

import (
	"fmt"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v4/ports"
)

const (
	defaultManagerTimeout       = 5 * time.Second
	defaultManagerCacheDuration = 5 * time.Second
)

// DefaultConfig returns the explicit configuration used by New.
func DefaultConfig() Config {
	return Config{
		Timeout:         defaultManagerTimeout,
		CacheDuration:   defaultManagerCacheDuration,
		EnableCaching:   true,
		EnableDetailed:  false,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
		Clock:           ports.SystemClock{},
	}
}

// Validate verifies that a Config can create a fail-closed health manager.
func (config Config) Validate() error {
	if config.Timeout <= 0 {
		return fmt.Errorf("health manager timeout must be greater than zero")
	}
	if config.EnableCaching && config.CacheDuration <= 0 {
		return fmt.Errorf("health manager cache duration must be greater than zero when caching is enabled")
	}
	if err := validateCheckNames("liveness", config.LivenessChecks); err != nil {
		return err
	}
	if err := validateCheckNames("readiness", config.ReadinessChecks); err != nil {
		return err
	}
	return nil
}

func normalizeManagerConfig(config Config) Config {
	if config.Clock == nil {
		config.Clock = ports.SystemClock{}
	}
	return config
}

func validateCheckNames(probe string, names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("health manager %s checks must not be empty", probe)
	}
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("health manager %s check name must not be empty", probe)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("health manager %s check %q is duplicated", probe, name)
		}
		seen[name] = struct{}{}
	}
	return nil
}
