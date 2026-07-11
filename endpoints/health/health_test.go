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

	compatbilling "github.com/aatuh/api-toolkit/v3/compat/billing"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestProbeChecksReturnUnhealthyForInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		check  func(*Manager, context.Context) Result
	}{
		{
			name: "liveness with no configured checks",
			config: Config{
				Timeout:        time.Second,
				LivenessChecks: nil,
			},
			check: func(m *Manager, ctx context.Context) Result {
				return m.GetLiveness(ctx)
			},
		},
		{
			name: "liveness with missing checker",
			config: Config{
				Timeout:        time.Second,
				LivenessChecks: []string{"missing"},
			},
			check: func(m *Manager, ctx context.Context) Result {
				return m.GetLiveness(ctx)
			},
		},
		{
			name: "readiness with no configured checks",
			config: Config{
				Timeout:         time.Second,
				ReadinessChecks: nil,
			},
			check: func(m *Manager, ctx context.Context) Result {
				return m.GetReadiness(ctx)
			},
		},
		{
			name: "readiness with missing checker",
			config: Config{
				Timeout:         time.Second,
				ReadinessChecks: []string{"missing"},
			},
			check: func(m *Manager, ctx context.Context) Result {
				return m.GetReadiness(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManagerWithConfig(tt.config)

			result := tt.check(manager, context.Background())
			if result.Status != StatusUnhealthy {
				t.Fatalf("expected unhealthy, got %q", result.Status)
			}
			if result.Message == "" {
				t.Fatal("expected invalid configuration message")
			}
		})
	}
}

func TestRegisterCheckerIgnoresNilChecker(t *testing.T) {
	manager := NewManagerWithConfig(Config{
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
	manager := NewManagerWithConfig(Config{
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
	manager := NewManagerWithConfig(Config{
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

func TestGetDetailedHealthFailsClosedForNonPositiveCacheDuration(t *testing.T) {
	tests := []struct {
		name          string
		cacheDuration time.Duration
	}{
		{name: "zero", cacheDuration: 0},
		{name: "negative", cacheDuration: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &countingChecker{name: "counted"}
			manager := NewManagerWithConfig(Config{
				Timeout:        time.Second,
				CacheDuration:  tt.cacheDuration,
				EnableCaching:  true,
				EnableDetailed: true,
			})
			manager.RegisterChecker(checker)

			manager.GetDetailedHealth(context.Background())
			manager.GetDetailedHealth(context.Background())

			if got := checker.calls.Load(); got != 2 {
				t.Fatalf("expected caching to fail closed, got %d checker calls", got)
			}
		})
	}
}

func TestGetDetailedHealthHonorsConfiguredTimeout(t *testing.T) {
	manager := NewManagerWithConfig(Config{
		Timeout:        20 * time.Millisecond,
		EnableCaching:  false,
		EnableDetailed: true,
	})
	checker := &blockingChecker{name: "slow", started: make(chan struct{})}
	manager.RegisterChecker(checker)

	done := make(chan DetailedResponse, 1)
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
		if result.Status != StatusUnhealthy {
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

	if result.Status != StatusUnknown {
		t.Fatalf("status = %q, want %q", result.Status, StatusUnknown)
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
			checker := NewCustomCheckerWithTimeout("custom", tt.timeout, func(ctx context.Context) (Status, string, interface{}) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected deadline on custom checker context")
				}
				remaining := time.Until(deadline)
				if remaining < 4*time.Second || remaining > 6*time.Second {
					t.Fatalf("deadline remaining = %v, want default timeout window", remaining)
				}
				return StatusHealthy, "ok", nil
			})

			result := checker.Check(context.Background())
			if result.Status != StatusHealthy {
				t.Fatalf("status = %q, want %q", result.Status, StatusHealthy)
			}
		})
	}
}

func TestCustomCheckerAcceptsNilContext(t *testing.T) {
	checker := NewCustomChecker("custom", func(ctx context.Context) (Status, string, interface{}) {
		if ctx == nil {
			t.Fatal("expected normalized context")
		}
		return StatusHealthy, "ok", nil
	})

	assertNoPanic(t, "CustomChecker.Check nil context", func() {
		result := checker.Check(nilContext())
		if result.Status != StatusHealthy {
			t.Fatalf("status = %q, want %q", result.Status, StatusHealthy)
		}
	})
}

func TestManagerMethodsAcceptNilContext(t *testing.T) {
	manager := NewManagerWithConfig(Config{
		Timeout:         time.Second,
		EnableCaching:   false,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(NewBasicChecker())

	assertNoPanic(t, "GetLiveness", func() {
		if result := manager.GetLiveness(nilContext()); result.Status != StatusHealthy {
			t.Fatalf("liveness status = %q, want %q", result.Status, StatusHealthy)
		}
	})
	assertNoPanic(t, "GetReadiness", func() {
		if result := manager.GetReadiness(nilContext()); result.Status != StatusHealthy {
			t.Fatalf("readiness status = %q, want %q", result.Status, StatusHealthy)
		}
	})
	assertNoPanic(t, "GetDetailedHealth", func() {
		if result := manager.GetDetailedHealth(nilContext()); result.Status != StatusHealthy {
			t.Fatalf("detailed status = %q, want %q", result.Status, StatusHealthy)
		}
	})
	assertNoPanic(t, "RefreshAll", func() {
		if result := manager.RefreshAll(nilContext()); result.Status != StatusHealthy {
			t.Fatalf("refresh status = %q, want %q", result.Status, StatusHealthy)
		}
	})
}

func TestDatabaseCheckerFailsClosedWhenPoolMissing(t *testing.T) {
	checker := NewDatabaseChecker(nil)

	assertNoPanic(t, "DatabaseChecker.Check", func() {
		result := checker.Check(context.Background())
		if result.Status != StatusUnhealthy {
			t.Fatalf("status = %q, want %q", result.Status, StatusUnhealthy)
		}
		if result.Message != "database pool not configured" {
			t.Fatalf("message = %q", result.Message)
		}
	})
}

func TestDatabaseCheckerAcceptsNilContext(t *testing.T) {
	pool := &stubDatabasePool{}
	checker := NewDatabaseChecker(pool)
	if checker.Name() != "database" {
		t.Fatalf("checker name = %q, want database", checker.Name())
	}

	assertNoPanic(t, "DatabaseChecker.Check nil context", func() {
		result := checker.Check(nilContext())
		if result.Status != StatusHealthy {
			t.Fatalf("status = %q, want %q", result.Status, StatusHealthy)
		}
	})

	if pool.pingCalls.Load() != 1 {
		t.Fatalf("expected one ping, got %d", pool.pingCalls.Load())
	}
}

func TestDatabaseCheckerPrefersSnapshotCapability(t *testing.T) {
	pool := &stubDatabasePool{
		snapshot: &DatabasePoolSnapshot{
			AcquireCount:  7,
			AcquiredConns: 2,
			IdleConns:     3,
			MaxConns:      10,
			TotalConns:    5,
		},
	}
	checker := NewDatabaseChecker(pool)

	result := checker.Check(context.Background())

	if result.Status != StatusHealthy {
		t.Fatalf("status = %q, want %q", result.Status, StatusHealthy)
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
}

func TestHTTPCheckerAcceptsNilContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewHTTPChecker("dependency", server.URL, WithHTTPClient(server.Client()))

	assertNoPanic(t, "HTTPChecker.Check nil context", func() {
		result := checker.Check(nilContext())
		if result.Status != StatusHealthy {
			t.Fatalf("status = %q, want %q", result.Status, StatusHealthy)
		}
	})
}

func TestHTTPCheckerOptionsAndFailureStatus(t *testing.T) {
	var gotMethod string
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	checker := NewHTTPChecker(
		"dependency",
		server.URL,
		WithHTTPMethod("post"),
		WithHTTPHeader("X-One", "1"),
		WithHTTPHeaders(map[string]string{"X-Two": "2"}),
		WithHTTPTimeout(time.Second),
		WithHTTPSuccessStatuses(http.StatusAccepted),
		WithHTTPFailureStatus(StatusDegraded),
	)
	if checker.Name() != "dependency" {
		t.Fatalf("checker name = %q, want dependency", checker.Name())
	}
	result := checker.Check(context.Background())
	if result.Status != StatusHealthy {
		t.Fatalf("status = %q, want healthy; message=%q", result.Status, result.Message)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotHeaders.Get("X-One") != "1" || gotHeaders.Get("X-Two") != "2" {
		t.Fatalf("headers = %#v, want configured headers", gotHeaders)
	}

	failing := NewHTTPChecker(
		"dependency",
		server.URL,
		WithHTTPSuccessStatuses(http.StatusNoContent),
		WithHTTPFailureStatus(StatusDegraded),
	)
	result = failing.Check(context.Background())
	if result.Status != StatusDegraded {
		t.Fatalf("failure status = %q, want degraded", result.Status)
	}
	if !strings.Contains(result.Message, "unexpected status 202") {
		t.Fatalf("failure message = %q, want unexpected status", result.Message)
	}
}

func TestHTTPCheckerReportsUnknownWhenURLMissing(t *testing.T) {
	checker := NewHTTPChecker("dependency", "")

	result := checker.Check(context.Background())

	if result.Status != StatusUnknown {
		t.Fatalf("status = %q, want unknown", result.Status)
	}
}

func TestCompositeCheckerAggregatesStatuses(t *testing.T) {
	empty := NewCompositeChecker("dependencies")
	if empty.Name() != "dependencies" {
		t.Fatalf("checker name = %q, want dependencies", empty.Name())
	}
	result := empty.Check(context.Background())
	if result.Status != StatusUnknown {
		t.Fatalf("empty status = %q, want unknown", result.Status)
	}

	composite := NewCompositeChecker(
		"dependencies",
		fixedChecker{name: "cache", status: StatusHealthy, message: "cache ok"},
		fixedChecker{name: "queue", status: StatusDegraded, message: "queue slow"},
	)
	result = composite.Check(context.Background())
	if result.Status != StatusDegraded {
		t.Fatalf("degraded status = %q, want degraded", result.Status)
	}
	if !strings.Contains(result.Message, "queue slow") {
		t.Fatalf("degraded message = %q, want queue detail", result.Message)
	}

	composite = NewCompositeChecker(
		"dependencies",
		fixedChecker{name: "cache", status: StatusHealthy, message: "cache ok"},
		fixedChecker{name: "database", status: StatusUnhealthy, message: "database down"},
	)
	result = composite.Check(context.Background())
	if result.Status != StatusUnhealthy {
		t.Fatalf("unhealthy status = %q, want unhealthy", result.Status)
	}
	if !strings.Contains(result.Message, "database down") {
		t.Fatalf("unhealthy message = %q, want database detail", result.Message)
	}
}

func TestPaymentProviderCheckerAcceptsNilContext(t *testing.T) {
	provider := &stubPaymentProvider{}
	checker := NewPaymentProviderChecker(
		provider,
		WithPaymentProviderName("billing"),
		WithPaymentProviderFailureStatus(StatusDegraded),
	)
	if checker.Name() != "billing" {
		t.Fatalf("checker name = %q, want billing", checker.Name())
	}

	assertNoPanic(t, "PaymentProviderChecker.Check nil context", func() {
		result := checker.Check(nilContext())
		if result.Status != StatusHealthy {
			t.Fatalf("status = %q, want %q", result.Status, StatusHealthy)
		}
	})

	if provider.calls.Load() != 1 {
		t.Fatalf("expected one provider call, got %d", provider.calls.Load())
	}
}

func TestPaymentProviderCheckerFailureStatusAndMissingProvider(t *testing.T) {
	missing := NewPaymentProviderChecker(nil)
	result := missing.Check(context.Background())
	if result.Status != StatusUnknown {
		t.Fatalf("missing provider status = %q, want unknown", result.Status)
	}

	provider := &stubPaymentProvider{err: errors.New("provider unavailable")}
	checker := NewPaymentProviderChecker(provider, WithPaymentProviderFailureStatus(StatusDegraded))
	result = checker.Check(context.Background())
	if result.Status != StatusDegraded {
		t.Fatalf("provider failure status = %q, want degraded", result.Status)
	}
	if !strings.Contains(result.Message, "provider unavailable") {
		t.Fatalf("provider failure message = %q", result.Message)
	}
}

func TestHealthSchedulerRunsUpdatesAndStatusChanges(t *testing.T) {
	manager := &stubRefreshManager{
		results: []DetailedResponse{
			{Status: StatusHealthy, Timestamp: time.Now()},
			{Status: StatusUnhealthy, Timestamp: time.Now()},
		},
	}
	var updates atomic.Int32
	var changes atomic.Int32
	scheduler := NewScheduler(manager, SchedulerConfig{
		Interval: 10 * time.Millisecond,
		Logger:   ports.NopLogger{},
		OnUpdate: func(context.Context, DetailedResponse) {
			updates.Add(1)
		},
		OnStatusChange: func(context.Context, Status, Status, DetailedResponse) {
			changes.Add(1)
		},
	})

	scheduler.runOnce(context.Background())
	scheduler.runOnce(context.Background())

	if updates.Load() != 2 {
		t.Fatalf("updates = %d, want 2", updates.Load())
	}
	if changes.Load() != 1 {
		t.Fatalf("changes = %d, want 1", changes.Load())
	}
}

func TestHealthSchedulerStartHonorsCancellationAndNilInputs(t *testing.T) {
	var nilScheduler *Scheduler
	nilScheduler.Start(context.Background())
	NewScheduler(nil, SchedulerConfig{}).Start(context.Background())

	manager := &stubRefreshManager{results: []DetailedResponse{{Status: StatusHealthy, Timestamp: time.Now()}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	NewScheduler(manager, SchedulerConfig{Interval: time.Millisecond}).Start(ctx)
	if manager.calls.Load() != 1 {
		t.Fatalf("refresh calls = %d, want initial run before cancellation", manager.calls.Load())
	}
}

type countingChecker struct {
	name  string
	calls atomic.Int32
}

func (c *countingChecker) Name() string {
	return c.name
}

func (c *countingChecker) Check(context.Context) Result {
	call := c.calls.Add(1)
	return Result{
		Status:    StatusHealthy,
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

func (c *blockingChecker) Check(ctx context.Context) Result {
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	<-ctx.Done()
	return Result{
		Status:    StatusUnhealthy,
		Message:   ctx.Err().Error(),
		Timestamp: time.Now(),
	}
}

type stubDatabasePool struct {
	pingCalls atomic.Int32
	pingErr   error
	snapshot  *DatabasePoolSnapshot
}

type stubPaymentProvider struct {
	calls atomic.Int32
	err   error
}

type stubRefreshManager struct {
	calls   atomic.Int32
	results []DetailedResponse
}

func (s *stubDatabasePool) Ping(context.Context) error {
	s.pingCalls.Add(1)
	return s.pingErr
}

func (*stubDatabasePool) Close() {}

func (*stubDatabasePool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	return nil, errors.New("not implemented")
}

func (s *stubDatabasePool) StatSnapshot() DatabasePoolSnapshot {
	if s.snapshot == nil {
		return DatabasePoolSnapshot{}
	}
	return *s.snapshot
}

func (*stubPaymentProvider) CreateCheckoutSession(context.Context, compatbilling.CheckoutSessionRequest) (compatbilling.CheckoutSession, error) {
	return compatbilling.CheckoutSession{}, errors.New("not implemented")
}

func (*stubPaymentProvider) ParseWebhook(context.Context, []byte, string) (compatbilling.WebhookEvent, error) {
	return compatbilling.WebhookEvent{}, errors.New("not implemented")
}

func (s *stubPaymentProvider) ListPrices(ctx context.Context) ([]compatbilling.Price, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	s.calls.Add(1)
	if s.err != nil {
		return nil, s.err
	}
	return []compatbilling.Price{{ID: "price_1"}}, nil
}

func (s *stubRefreshManager) RefreshAll(context.Context) DetailedResponse {
	call := int(s.calls.Add(1)) - 1
	if call < len(s.results) {
		return s.results[call]
	}
	return s.results[len(s.results)-1]
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
