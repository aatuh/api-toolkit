package health

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type constructionClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *constructionClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *constructionClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()
}

type constructionChecker struct {
	name  string
	check func(context.Context) Result
}

func (c *constructionChecker) Name() string { return c.name }

func (c *constructionChecker) Check(ctx context.Context) Result {
	if c.check == nil {
		return Result{Status: StatusHealthy}
	}
	return c.check(ctx)
}

func TestConfigValidateRejectsInvalidManagerConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "zero timeout",
			mutate: func(config *Config) {
				config.Timeout = 0
			},
		},
		{
			name: "negative timeout",
			mutate: func(config *Config) {
				config.Timeout = -time.Second
			},
		},
		{
			name: "zero cache duration while caching enabled",
			mutate: func(config *Config) {
				config.CacheDuration = 0
			},
		},
		{
			name: "negative cache duration while caching enabled",
			mutate: func(config *Config) {
				config.CacheDuration = -time.Second
			},
		},
		{
			name: "no liveness checks",
			mutate: func(config *Config) {
				config.LivenessChecks = nil
			},
		},
		{
			name: "blank readiness check name",
			mutate: func(config *Config) {
				config.ReadinessChecks = []string{" "}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig()
			test.mutate(&config)
			if err := config.Validate(); err == nil {
				t.Fatal("Config.Validate() error = nil")
			}
			if _, err := NewManager(config); err == nil {
				t.Fatal("NewManager() error = nil")
			}
		})
	}
}

func TestNewManagerAndRegisterCheckerCheckedRejectInvalidRegistration(t *testing.T) {
	manager, err := NewManager(DefaultConfig())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	var typedNil *constructionChecker
	for _, checker := range []Checker{
		nil,
		typedNil,
		&constructionChecker{name: " "},
	} {
		if err := manager.RegisterCheckerChecked(checker); err == nil {
			t.Fatalf("RegisterCheckerChecked(%#v) error = nil", checker)
		}
	}

	first := &constructionChecker{name: "database"}
	if err := manager.RegisterCheckerChecked(first); err != nil {
		t.Fatalf("RegisterCheckerChecked(first) error = %v", err)
	}
	if err := manager.RegisterCheckerChecked(&constructionChecker{name: "database"}); err == nil {
		t.Fatal("RegisterCheckerChecked(duplicate) error = nil")
	}

	manager.mu.RLock()
	registered := manager.checkers["database"]
	manager.mu.RUnlock()
	if registered != first {
		t.Fatalf("registered checker = %#v, want original %#v", registered, first)
	}
}

func TestNewManagerUsesInjectedClockForCache(t *testing.T) {
	clock := &constructionClock{now: time.Date(2026, time.August, 22, 12, 0, 0, 0, time.UTC)}
	config := DefaultConfig()
	config.Clock = clock
	config.CacheDuration = time.Minute
	config.LivenessChecks = []string{"counted"}
	config.ReadinessChecks = []string{"counted"}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	var calls atomic.Int32
	if err := manager.RegisterCheckerChecked(&constructionChecker{
		name: "counted",
		check: func(context.Context) Result {
			calls.Add(1)
			return Result{Status: StatusHealthy}
		},
	}); err != nil {
		t.Fatalf("RegisterCheckerChecked() error = %v", err)
	}

	manager.GetReadiness(context.Background())
	clock.Advance(30 * time.Second)
	manager.GetReadiness(context.Background())
	if got := calls.Load(); got != 1 {
		t.Fatalf("cached check calls = %d, want 1", got)
	}
	clock.Advance(31 * time.Second)
	manager.GetReadiness(context.Background())
	if got := calls.Load(); got != 2 {
		t.Fatalf("expired check calls = %d, want 2", got)
	}
}

func TestManagerReturnsAtTimeoutWhenCheckerIgnoresCancellation(t *testing.T) {
	config := DefaultConfig()
	config.Timeout = 10 * time.Millisecond
	config.EnableCaching = false
	config.LivenessChecks = []string{"blocked"}
	config.ReadinessChecks = []string{"blocked"}
	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	release := make(chan struct{})
	defer close(release)
	if err := manager.RegisterCheckerChecked(&constructionChecker{
		name: "blocked",
		check: func(context.Context) Result {
			<-release
			return Result{Status: StatusHealthy}
		},
	}); err != nil {
		t.Fatalf("RegisterCheckerChecked() error = %v", err)
	}

	started := time.Now()
	result := manager.GetReadiness(context.Background())
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("GetReadiness() blocked for %v after timeout", elapsed)
	}
	if result.Status != StatusUnhealthy {
		t.Fatalf("GetReadiness() status = %q, want %q", result.Status, StatusUnhealthy)
	}
}

func TestManagerConcurrentRegistrationAndChecking(t *testing.T) {
	config := DefaultConfig()
	config.EnableCaching = false
	config.LivenessChecks = []string{"stable"}
	config.ReadinessChecks = []string{"stable"}
	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := manager.RegisterCheckerChecked(&constructionChecker{name: "stable"}); err != nil {
		t.Fatalf("RegisterCheckerChecked(stable) error = %v", err)
	}

	var group sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 20; i++ {
		group.Add(2)
		go func(index int) {
			defer group.Done()
			name := fmt.Sprintf("extra-%d", index)
			if err := manager.RegisterCheckerChecked(&constructionChecker{name: name}); err != nil {
				errs <- err
			}
		}(i)
		go func() {
			defer group.Done()
			manager.GetDetailedHealth(context.Background())
			manager.GetReadiness(context.Background())
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent registration error = %v", err)
	}
}
