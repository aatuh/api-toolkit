package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestProbeChecksReturnUnhealthyForInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config ports.HealthCheckConfig
		check  func(*Manager, context.Context) ports.HealthResult
	}{
		{
			name: "liveness with no configured checks",
			config: ports.HealthCheckConfig{
				Timeout:        time.Second,
				LivenessChecks: nil,
			},
			check: func(m *Manager, ctx context.Context) ports.HealthResult {
				return m.GetLiveness(ctx)
			},
		},
		{
			name: "liveness with missing checker",
			config: ports.HealthCheckConfig{
				Timeout:        time.Second,
				LivenessChecks: []string{"missing"},
			},
			check: func(m *Manager, ctx context.Context) ports.HealthResult {
				return m.GetLiveness(ctx)
			},
		},
		{
			name: "readiness with no configured checks",
			config: ports.HealthCheckConfig{
				Timeout:         time.Second,
				ReadinessChecks: nil,
			},
			check: func(m *Manager, ctx context.Context) ports.HealthResult {
				return m.GetReadiness(ctx)
			},
		},
		{
			name: "readiness with missing checker",
			config: ports.HealthCheckConfig{
				Timeout:         time.Second,
				ReadinessChecks: []string{"missing"},
			},
			check: func(m *Manager, ctx context.Context) ports.HealthResult {
				return m.GetReadiness(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManagerWithConfig(tt.config)

			result := tt.check(manager, context.Background())
			if result.Status != ports.HealthStatusUnhealthy {
				t.Fatalf("expected unhealthy, got %q", result.Status)
			}
			if result.Message == "" {
				t.Fatal("expected invalid configuration message")
			}
		})
	}
}

func TestRegisterCheckerIgnoresNilChecker(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         time.Second,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})

	manager.RegisterChecker(nil)
	manager.RegisterChecker(NewBasicChecker())

	detailed := manager.GetDetailedHealth(context.Background())
	if detailed.Summary.Total != 1 {
		t.Fatalf("expected 1 registered checker, got %d", detailed.Summary.Total)
	}
}

func TestGetDetailedHealthUsesCacheWhenEnabled(t *testing.T) {
	checker := &countingChecker{name: "counted"}
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:        time.Second,
		CacheDuration:  time.Minute,
		EnableCaching:  true,
		EnableDetailed: true,
	})
	manager.RegisterChecker(checker)

	manager.GetDetailedHealth(context.Background())
	manager.GetDetailedHealth(context.Background())

	if got := checker.calls.Load(); got != 1 {
		t.Fatalf("expected cached checker to run once, got %d", got)
	}
}

func TestGetDetailedHealthBypassesCacheWhenDisabled(t *testing.T) {
	checker := &countingChecker{name: "counted"}
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:        time.Second,
		CacheDuration:  time.Minute,
		EnableCaching:  false,
		EnableDetailed: true,
	})
	manager.RegisterChecker(checker)

	manager.GetDetailedHealth(context.Background())
	manager.GetDetailedHealth(context.Background())

	if got := checker.calls.Load(); got != 2 {
		t.Fatalf("expected uncached checker to run twice, got %d", got)
	}
}

func TestGetDetailedHealthHonorsConfiguredTimeout(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:        20 * time.Millisecond,
		EnableCaching:  false,
		EnableDetailed: true,
	})
	checker := &blockingChecker{name: "slow", started: make(chan struct{})}
	manager.RegisterChecker(checker)

	done := make(chan ports.DetailedHealthResponse, 1)
	go func() {
		done <- manager.GetDetailedHealth(context.Background())
	}()

	select {
	case <-checker.started:
	case <-time.After(time.Second):
		t.Fatal("checker did not start")
	}

	select {
	case resp := <-done:
		result, ok := resp.Checks["slow"]
		if !ok {
			t.Fatalf("expected slow checker result, got %#v", resp.Checks)
		}
		if result.Status != ports.HealthStatusUnhealthy {
			t.Fatalf("expected unhealthy result after timeout, got %q", result.Status)
		}
		if !strings.Contains(result.Message, context.DeadlineExceeded.Error()) {
			t.Fatalf("expected timeout message, got %q", result.Message)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("GetDetailedHealth did not honor configured timeout")
	}
}

func TestCustomCheckerReturnsUnknownWhenFunctionMissing(t *testing.T) {
	checker := NewCustomChecker("custom", nil)

	result := checker.Check(context.Background())

	if result.Status != ports.HealthStatusUnknown {
		t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusUnknown)
	}
	if result.Message != "custom health check not configured" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestCustomCheckerDefaultsNonPositiveTimeouts(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "zero", timeout: 0},
		{name: "negative", timeout: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewCustomCheckerWithTimeout("custom", tt.timeout, func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected deadline on custom checker context")
				}
				remaining := time.Until(deadline)
				if remaining < 4*time.Second || remaining > 6*time.Second {
					t.Fatalf("deadline remaining = %v, want default timeout window", remaining)
				}
				return ports.HealthStatusHealthy, "ok", nil
			})

			result := checker.Check(context.Background())
			if result.Status != ports.HealthStatusHealthy {
				t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusHealthy)
			}
		})
	}
}

func TestCustomCheckerAcceptsNilContext(t *testing.T) {
	checker := NewCustomChecker("custom", func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
		if ctx == nil {
			t.Fatal("expected normalized context")
		}
		return ports.HealthStatusHealthy, "ok", nil
	})

	assertNoPanic(t, "CustomChecker.Check nil context", func() {
		result := checker.Check(nilContext())
		if result.Status != ports.HealthStatusHealthy {
			t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusHealthy)
		}
	})
}

func TestManagerMethodsAcceptNilContext(t *testing.T) {
	manager := NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         time.Second,
		EnableCaching:   false,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(NewBasicChecker())

	assertNoPanic(t, "GetLiveness", func() {
		if result := manager.GetLiveness(nilContext()); result.Status != ports.HealthStatusHealthy {
			t.Fatalf("liveness status = %q, want %q", result.Status, ports.HealthStatusHealthy)
		}
	})
	assertNoPanic(t, "GetReadiness", func() {
		if result := manager.GetReadiness(nilContext()); result.Status != ports.HealthStatusHealthy {
			t.Fatalf("readiness status = %q, want %q", result.Status, ports.HealthStatusHealthy)
		}
	})
	assertNoPanic(t, "GetDetailedHealth", func() {
		if result := manager.GetDetailedHealth(nilContext()); result.Status != ports.HealthStatusHealthy {
			t.Fatalf("detailed status = %q, want %q", result.Status, ports.HealthStatusHealthy)
		}
	})
	assertNoPanic(t, "RefreshAll", func() {
		if result := manager.RefreshAll(nilContext()); result.Status != ports.HealthStatusHealthy {
			t.Fatalf("refresh status = %q, want %q", result.Status, ports.HealthStatusHealthy)
		}
	})
}

func TestDatabaseCheckerFailsClosedWhenPoolMissing(t *testing.T) {
	checker := NewDatabaseChecker(nil)

	assertNoPanic(t, "DatabaseChecker.Check", func() {
		result := checker.Check(context.Background())
		if result.Status != ports.HealthStatusUnhealthy {
			t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusUnhealthy)
		}
		if result.Message != "database pool not configured" {
			t.Fatalf("message = %q", result.Message)
		}
	})
}

func TestDatabaseCheckerAcceptsNilContext(t *testing.T) {
	pool := &stubDatabasePool{}
	checker := NewDatabaseChecker(pool)

	assertNoPanic(t, "DatabaseChecker.Check nil context", func() {
		result := checker.Check(nilContext())
		if result.Status != ports.HealthStatusHealthy {
			t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusHealthy)
		}
	})

	if pool.pingCalls.Load() != 1 {
		t.Fatalf("expected one ping, got %d", pool.pingCalls.Load())
	}
}

func TestDatabaseCheckerPrefersSnapshotCapability(t *testing.T) {
	pool := &stubDatabasePool{
		snapshot: &ports.DatabasePoolSnapshot{
			AcquireCount:  7,
			AcquiredConns: 2,
			IdleConns:     3,
			MaxConns:      10,
			TotalConns:    5,
		},
	}
	checker := NewDatabaseChecker(pool)

	result := checker.Check(context.Background())

	if result.Status != ports.HealthStatusHealthy {
		t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusHealthy)
	}
	details, ok := result.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map details, got %T", result.Details)
	}
	if got := details["acquire_count"]; got != int64(7) {
		t.Fatalf("acquire_count = %#v", got)
	}
	if got := details["acquired_conns"]; got != int32(2) {
		t.Fatalf("acquired_conns = %#v", got)
	}
	if got := details["idle_conns"]; got != int32(3) {
		t.Fatalf("idle_conns = %#v", got)
	}
	if got := details["max_conns"]; got != int32(10) {
		t.Fatalf("max_conns = %#v", got)
	}
	if got := details["total_conns"]; got != int32(5) {
		t.Fatalf("total_conns = %#v", got)
	}
	if pool.statCalls.Load() != 0 {
		t.Fatalf("expected no legacy Stat calls, got %d", pool.statCalls.Load())
	}
}

func TestHTTPCheckerAcceptsNilContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewHTTPChecker("dependency", server.URL, WithHTTPClient(server.Client()))

	assertNoPanic(t, "HTTPChecker.Check nil context", func() {
		result := checker.Check(nilContext())
		if result.Status != ports.HealthStatusHealthy {
			t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusHealthy)
		}
	})
}

func TestPaymentProviderCheckerAcceptsNilContext(t *testing.T) {
	provider := &stubPaymentProvider{}
	checker := NewPaymentProviderChecker(provider)

	assertNoPanic(t, "PaymentProviderChecker.Check nil context", func() {
		result := checker.Check(nilContext())
		if result.Status != ports.HealthStatusHealthy {
			t.Fatalf("status = %q, want %q", result.Status, ports.HealthStatusHealthy)
		}
	})

	if provider.calls.Load() != 1 {
		t.Fatalf("expected one provider call, got %d", provider.calls.Load())
	}
}

type countingChecker struct {
	name  string
	calls atomic.Int32
}

func (c *countingChecker) Name() string {
	return c.name
}

func (c *countingChecker) Check(context.Context) ports.HealthResult {
	call := c.calls.Add(1)
	return ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   fmt.Sprintf("call %d", call),
		Timestamp: time.Now(),
	}
}

type blockingChecker struct {
	name    string
	started chan struct{}
}

func (c *blockingChecker) Name() string {
	return c.name
}

func (c *blockingChecker) Check(ctx context.Context) ports.HealthResult {
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	<-ctx.Done()
	return ports.HealthResult{
		Status:    ports.HealthStatusUnhealthy,
		Message:   ctx.Err().Error(),
		Timestamp: time.Now(),
	}
}

type stubDatabasePool struct {
	pingCalls atomic.Int32
	statCalls atomic.Int32
	pingErr   error
	snapshot  *ports.DatabasePoolSnapshot
}

type stubPaymentProvider struct {
	calls atomic.Int32
}

func (s *stubDatabasePool) Ping(context.Context) error {
	s.pingCalls.Add(1)
	return s.pingErr
}

func (*stubDatabasePool) Close() {}

func (*stubDatabasePool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	return nil, errors.New("not implemented")
}

func (s *stubDatabasePool) Stat() ports.DatabaseStats {
	s.statCalls.Add(1)
	return nil
}

func (s *stubDatabasePool) StatSnapshot() ports.DatabasePoolSnapshot {
	if s.snapshot == nil {
		return ports.DatabasePoolSnapshot{}
	}
	return *s.snapshot
}

func (*stubPaymentProvider) CreateCheckoutSession(context.Context, ports.CheckoutSessionRequest) (ports.CheckoutSession, error) {
	return ports.CheckoutSession{}, errors.New("not implemented")
}

func (*stubPaymentProvider) ParseWebhook(context.Context, []byte, string) (ports.WebhookEvent, error) {
	return ports.WebhookEvent{}, errors.New("not implemented")
}

func (s *stubPaymentProvider) ListPrices(ctx context.Context) ([]ports.Price, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	s.calls.Add(1)
	return []ports.Price{{ID: "price_1"}}, nil
}

func assertNoPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("%s panicked: %v", name, recovered)
		}
	}()
	fn()
}

func nilContext() context.Context {
	return nil
}
