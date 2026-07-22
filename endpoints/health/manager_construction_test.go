package health

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewManagerRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{
			name: "non-positive timeout",
			mutate: func(config *Config) {
				config.Timeout = 0
			},
			want: "timeout",
		},
		{
			name: "non-positive cache duration while caching enabled",
			mutate: func(config *Config) {
				config.CacheDuration = 0
			},
			want: "cache duration",
		},
		{
			name: "negative cache duration",
			mutate: func(config *Config) {
				config.CacheDuration = -time.Second
			},
			want: "cache duration",
		},
		{
			name: "empty checker name",
			mutate: func(config *Config) {
				config.LivenessChecks = []string{""}
			},
			want: "empty",
		},
		{
			name: "duplicate checker name",
			mutate: func(config *Config) {
				config.ReadinessChecks = []string{"basic", "basic"}
			},
			want: "duplicate",
		},
		{
			name: "no configured checks",
			mutate: func(config *Config) {
				config.LivenessChecks = nil
				config.ReadinessChecks = nil
			},
			want: "no liveness or readiness checks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.mutate(&config)
			if _, err := NewManager(config); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewManager() error = %v, want message containing %q", err, tt.want)
			}
		})
	}
}

func TestRegisterCheckerCheckedRejectsInvalidCheckers(t *testing.T) {
	manager, err := NewManager(DefaultConfig())
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}

	if err := manager.RegisterCheckerChecked(nil); err == nil {
		t.Fatal("nil checker registration succeeded")
	}
	var typedNil *BasicChecker
	if err := manager.RegisterCheckerChecked(typedNil); err == nil {
		t.Fatal("typed nil checker registration succeeded")
	}
	if err := manager.RegisterCheckerChecked(managerConstructionChecker{name: ""}); err == nil {
		t.Fatal("empty-name checker registration succeeded")
	}
	if err := manager.RegisterCheckerChecked(managerConstructionChecker{name: "basic"}); err != nil {
		t.Fatalf("first checker registration: %v", err)
	}
	if err := manager.RegisterCheckerChecked(managerConstructionChecker{name: "basic"}); err == nil {
		t.Fatal("duplicate checker registration succeeded")
	}
}

func TestManagerUsesInjectedClockForCache(t *testing.T) {
	clock := &managerConstructionClock{now: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)}
	config := DefaultConfig()
	config.Clock = clock
	config.CacheDuration = time.Minute
	config.LivenessChecks = []string{"counted"}
	config.ReadinessChecks = []string{"counted"}
	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}
	checker := &managerConstructionCountingChecker{name: "counted"}
	if err := manager.RegisterCheckerChecked(checker); err != nil {
		t.Fatalf("RegisterCheckerChecked(): %v", err)
	}

	manager.GetReadiness(context.Background())
	manager.GetReadiness(context.Background())
	if got := checker.calls.Load(); got != 1 {
		t.Fatalf("cached checker calls = %d, want 1", got)
	}

	clock.Advance(2 * time.Minute)
	manager.GetReadiness(context.Background())
	if got := checker.calls.Load(); got != 2 {
		t.Fatalf("expired cached checker calls = %d, want 2", got)
	}
}

func TestManagerReturnsWhenCheckerIgnoresCancellation(t *testing.T) {
	config := DefaultConfig()
	config.Timeout = 20 * time.Millisecond
	config.EnableCaching = false
	config.LivenessChecks = []string{"blocked"}
	config.ReadinessChecks = []string{"blocked"}
	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}
	checker := managerConstructionBlockingChecker{
		name:    "blocked",
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	if err := manager.RegisterCheckerChecked(checker); err != nil {
		t.Fatalf("RegisterCheckerChecked(): %v", err)
	}
	t.Cleanup(func() { close(checker.release) })

	started := time.Now()
	result := manager.GetReadiness(context.Background())
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("GetReadiness() took %v after checker ignored cancellation", elapsed)
	}
	if result.Status != StatusUnhealthy || !strings.Contains(result.Message, "timed out") {
		t.Fatalf("GetReadiness() = %#v, want an unhealthy timeout result", result)
	}
	select {
	case <-checker.started:
	case <-time.After(time.Second):
		t.Fatal("blocking checker did not start")
	}
}

func TestLegacyManagerWithNoCheckersFailsClosed(t *testing.T) {
	manager := NewManagerWithConfig(Config{
		Timeout:        time.Second,
		CacheDuration:  time.Second,
		EnableCaching:  false,
		EnableDetailed: true,
	})

	if result := manager.GetDetailedHealth(context.Background()); result.Status != StatusUnhealthy {
		t.Fatalf("GetDetailedHealth() status = %q, want %q", result.Status, StatusUnhealthy)
	}
	if result := manager.RefreshAll(context.Background()); result.Status != StatusUnhealthy {
		t.Fatalf("RefreshAll() status = %q, want %q", result.Status, StatusUnhealthy)
	}
}

func TestManagerSupportsConcurrentRegistrationAndChecking(t *testing.T) {
	config := DefaultConfig()
	config.EnableCaching = false
	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}
	if err := manager.RegisterCheckerChecked(managerConstructionChecker{name: "basic"}); err != nil {
		t.Fatalf("RegisterCheckerChecked(basic): %v", err)
	}

	var wait sync.WaitGroup
	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			_ = manager.RegisterCheckerChecked(managerConstructionChecker{name: "concurrent-" + string(rune('a'+i))})
		}(i)
		wait.Add(1)
		go func() {
			defer wait.Done()
			for j := 0; j < 10; j++ {
				_ = manager.GetDetailedHealth(context.Background())
				_ = manager.GetReadiness(context.Background())
			}
		}()
	}
	wait.Wait()
}

type managerConstructionClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *managerConstructionClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *managerConstructionClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()
}

type managerConstructionChecker struct {
	name string
}

func (c managerConstructionChecker) Name() string { return c.name }

func (managerConstructionChecker) Check(context.Context) Result {
	return Result{Status: StatusHealthy}
}

type managerConstructionCountingChecker struct {
	name  string
	calls atomic.Int32
}

func (c *managerConstructionCountingChecker) Name() string { return c.name }

func (c *managerConstructionCountingChecker) Check(context.Context) Result {
	c.calls.Add(1)
	return Result{Status: StatusHealthy}
}

type managerConstructionBlockingChecker struct {
	name    string
	started chan struct{}
	release chan struct{}
}

func (c managerConstructionBlockingChecker) Name() string { return c.name }

func (c managerConstructionBlockingChecker) Check(context.Context) Result {
	close(c.started)
	<-c.release
	return Result{Status: StatusHealthy}
}
