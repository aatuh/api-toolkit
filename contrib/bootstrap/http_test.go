package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
	"github.com/aatuh/api-toolkit/v4/endpoints/docs"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
	"github.com/aatuh/api-toolkit/v4/endpoints/version"
	"github.com/aatuh/api-toolkit/v4/httpx/identity"
	"github.com/aatuh/api-toolkit/v4/ports"
	"github.com/aatuh/api-toolkit/v4/specs"
)

type stubServerRunner struct {
	listenErr   error
	shutdownErr error
	listenDone  chan struct{}
	shutdownHit atomic.Bool
}

type stubRouteRegistrar struct {
	patterns []string
	handlers map[string]http.HandlerFunc
}

func newValidatedHealthManager(t *testing.T, config health.Config) *health.Manager {
	t.Helper()
	manager, err := health.NewManager(config)
	if err != nil {
		t.Fatalf("health.NewManager() error = %v", err)
	}
	return manager
}

func (s *stubRouteRegistrar) Get(pattern string, h http.HandlerFunc) {
	if s.handlers == nil {
		s.handlers = make(map[string]http.HandlerFunc)
	}
	s.patterns = append(s.patterns, pattern)
	s.handlers[pattern] = h
}

func newStubServerRunner() *stubServerRunner {
	return &stubServerRunner{
		listenDone: make(chan struct{}),
	}
}

func (s *stubServerRunner) ListenAndServe() error {
	if s.listenErr != nil {
		return s.listenErr
	}
	<-s.listenDone
	return http.ErrServerClosed
}

func (s *stubServerRunner) Shutdown(context.Context) error {
	s.shutdownHit.Store(true)
	close(s.listenDone)
	return s.shutdownErr
}

func TestNewDefaultRouterCanBeConstructedRepeatedly(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "")
	t.Setenv("TRUSTED_PROXIES", "")

	for i := 0; i < 2; i++ {
		router, err := NewDefaultRouter(ports.NopLogger{})
		if err != nil {
			t.Fatalf("router error on iteration %d: %v", i, err)
		}
		if router == nil {
			t.Fatalf("expected router on iteration %d", i)
		}
	}
}

func TestNewDefaultRouterReturnsErrorForInvalidTrustedProxies(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "")
	t.Setenv("TRUSTED_PROXIES", "definitely-not-a-proxy")

	router, err := NewDefaultRouter(ports.NopLogger{})
	if err == nil {
		t.Fatal("expected invalid trusted proxies error")
	}
	if router != nil {
		t.Fatal("expected nil router on invalid trusted proxies")
	}
}

func TestNewDefaultRouterWithConfigIgnoresAmbientEnvironment(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "x-env-header")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "true")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "true")
	t.Setenv("TRUSTED_PROXIES", "definitely-not-a-proxy")

	prefixes, err := identity.ParseTrustedProxies([]string{"127.0.0.1/32"})
	if err != nil {
		t.Fatalf("trusted proxies: %v", err)
	}

	router, err := NewDefaultRouterWithConfig(ports.NopLogger{}, DefaultRouterConfig{
		TrustedProxies: prefixes,
	})
	if err != nil {
		t.Fatalf("router error: %v", err)
	}
	if router == nil {
		t.Fatal("expected router")
	}
}

func TestNewDefaultRouterAppliesStrictMiddlewareStack(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "")
	t.Setenv("TRUSTED_PROXIES", "")
	prevProvider := otel.GetTracerProvider()
	prevPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(prevProvider)
		otel.SetTextMapPropagator(prevPropagator)
	})

	router, err := NewDefaultRouter(ports.NopLogger{})
	if err != nil {
		t.Fatalf("router error: %v", err)
	}
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("X-Correlation-ID", "corr-123")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "corr-123" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	if got := rec.Header().Get("X-Trace-ID"); got == "" {
		t.Fatal("expected X-Trace-ID response header")
	}
	if got := rec.Header().Get("traceparent"); got == "" {
		t.Fatal("expected traceparent response header")
	}
}

func TestDefaultRouterConfigFromEnvParsesTrustedProxies(t *testing.T) {
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "x-test-bypass")
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "true")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "true")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32,::1/128")

	cfg, err := DefaultRouterConfigFromEnv(nil)
	if err != nil {
		t.Fatalf("config from env: %v", err)
	}

	if !cfg.RateLimit.SkipEnabled {
		t.Fatal("expected rate-limit skip enabled")
	}
	if got := cfg.RateLimit.SkipHeader; got != "x-test-bypass" {
		t.Fatalf("skip header = %q", got)
	}
	if !cfg.RateLimit.AllowDangerousDevBypasses {
		t.Fatal("expected dangerous dev bypasses enabled")
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("expected 2 trusted proxies, got %d", len(cfg.TrustedProxies))
	}
	if want := netip.MustParsePrefix("127.0.0.1/32"); cfg.TrustedProxies[0] != want {
		t.Fatalf("trusted proxy[0] = %v", cfg.TrustedProxies[0])
	}
}

func TestRunServerReturnsShutdownError(t *testing.T) {
	srv := newStubServerRunner()
	srv.shutdownErr = errors.New("shutdown failed")

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runServer(ctx, srv)
	}()

	cancel()

	select {
	case err := <-errCh:
		if err == nil || err.Error() != "shutdown server: shutdown failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runServer")
	}

	if !srv.shutdownHit.Load() {
		t.Fatal("expected shutdown to be called")
	}
}

func TestRunServerReturnsNilAfterGracefulShutdown(t *testing.T) {
	srv := newStubServerRunner()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runServer(ctx, srv)
	}()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runServer")
	}

	if !srv.shutdownHit.Load() {
		t.Fatal("expected shutdown to be called")
	}
}

func TestRunServerReturnsListenError(t *testing.T) {
	srv := newStubServerRunner()
	srv.listenErr = errors.New("listen failed")

	err := runServer(context.Background(), srv)
	if err == nil || err.Error() != "listen failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv.shutdownHit.Load() {
		t.Fatal("did not expect shutdown to be called")
	}
}

func TestRunShutdownHooksRunsHooksInOrder(t *testing.T) {
	var calls []string
	err := runShutdownHooks(context.Background(), []ShutdownHook{
		{Name: "first", Hook: func(context.Context) error {
			calls = append(calls, "first")
			return nil
		}},
		{Name: "second", Hook: func(context.Context) error {
			calls = append(calls, "second")
			return nil
		}},
	})
	if err != nil {
		t.Fatalf("shutdown hooks: %v", err)
	}
	if strings.Join(calls, ",") != "first,second" {
		t.Fatalf("calls = %v", calls)
	}
}

func TestRunShutdownHooksReportsNamedError(t *testing.T) {
	err := runShutdownHooks(context.Background(), []ShutdownHook{{
		Name: "jwks",
		Hook: func(context.Context) error {
			return errors.New("close failed")
		},
	}})
	if err == nil || err.Error() != "shutdown hook jwks: close failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMountSystemEndpointsToUsesMinimalRegistrar(t *testing.T) {
	router := &stubRouteRegistrar{}

	MountSystemEndpointsTo(router, SystemEndpoints{
		Health:  health.NewHandler(nil),
		Docs:    docs.NewHandler(nil),
		Version: version.NewHandler(version.Config{}),
	})

	expected := []string{
		specs.Livez,
		specs.Readyz,
		specs.Healthz,
		specs.Health,
		specs.Docs,
		specs.DocsOpenAPI,
		specs.DocsVersion,
		specs.DocsInfo,
		specs.Version,
	}
	if len(router.patterns) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(router.patterns))
	}
	for i := range expected {
		if router.patterns[i] != expected[i] {
			t.Fatalf("route %d = %q", i, router.patterns[i])
		}
	}
}

func TestMountSystemEndpointsToRegistersMetricsWhenProvided(t *testing.T) {
	router := &stubRouteRegistrar{}

	MountSystemEndpointsTo(router, SystemEndpoints{
		Health:  health.NewHandler(nil),
		Docs:    docs.NewHandler(nil),
		Version: version.NewHandler(version.Config{}),
		Metrics: PrometheusMetricsHandler(),
	})

	expected := []string{
		specs.Livez,
		specs.Readyz,
		specs.Healthz,
		specs.Health,
		specs.Docs,
		specs.DocsOpenAPI,
		specs.DocsVersion,
		specs.DocsInfo,
		specs.Version,
		specs.Metrics,
	}
	if len(router.patterns) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(router.patterns))
	}
	for i := range expected {
		if router.patterns[i] != expected[i] {
			t.Fatalf("route %d = %q", i, router.patterns[i])
		}
	}
}

func TestMountSystemEndpointsToDisablesPprofByDefault(t *testing.T) {
	router := &stubRouteRegistrar{}

	MountSystemEndpointsTo(router, SystemEndpoints{
		Health:  health.NewHandler(nil),
		Docs:    docs.NewHandler(nil),
		Version: version.NewHandler(version.Config{}),
		Pprof:   pprofStub(),
	})

	for _, route := range []string{
		specs.PprofIndex,
		specs.PprofCmdline,
		specs.PprofProfile,
		specs.PprofSymbol,
		specs.PprofTrace,
	} {
		for _, got := range router.patterns {
			if got == route {
				t.Fatalf("unexpected pprof route on default mount: %s", route)
			}
		}
	}
}

func TestMountSystemEndpointsToWithProfileEnablesPprofInDevelopment(t *testing.T) {
	router := &stubRouteRegistrar{}

	MountSystemEndpointsToWithProfile(router, SystemEndpoints{
		Health:  health.NewHandler(nil),
		Docs:    docs.NewHandler(nil),
		Version: version.NewHandler(version.Config{}),
		Pprof:   pprofStub(),
	}, string(SystemProfileDevelopment))

	expected := map[string]bool{
		specs.PprofIndex:   false,
		specs.PprofCmdline: false,
		specs.PprofProfile: false,
		specs.PprofSymbol:  false,
		specs.PprofTrace:   false,
	}
	for _, route := range router.patterns {
		if _, ok := expected[route]; ok {
			expected[route] = true
		}
	}
	for route, seen := range expected {
		if !seen {
			t.Fatalf("expected development profile to include pprof route: %s", route)
		}
	}
}

func TestMountSystemEndpointsToWithAdminRequiresWrapper(t *testing.T) {
	err := MountSystemEndpointsToWithAdmin(&stubRouteRegistrar{}, SystemEndpoints{}, SystemEndpointAdminOptions{})
	if err == nil {
		t.Fatal("expected missing admin wrapper error")
	}
}

func TestMountSystemEndpointsToWithAdminMountsOperatorRoutesBehindWrapper(t *testing.T) {
	manager := newValidatedHealthManager(t, health.Config{
		Timeout:         time.Second,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(health.NewBasicChecker())
	router := &stubRouteRegistrar{}
	wrapped := 0
	requireAdmin := func(next http.Handler) http.Handler {
		wrapped++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Admin-Wrapper", "true")
			next.ServeHTTP(w, r)
		})
	}

	err := MountSystemEndpointsToWithAdmin(router, SystemEndpoints{
		Health:  health.NewHandler(manager),
		Docs:    docs.NewHandler(nil),
		Version: version.NewHandler(version.Config{}),
		Metrics: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		Pprof: pprofStub(),
	}, SystemEndpointAdminOptions{
		RequireAdmin: requireAdmin,
		EnablePprof:  true,
	})
	if err != nil {
		t.Fatalf("mount admin system endpoints: %v", err)
	}

	for _, pattern := range []string{
		specs.Livez,
		specs.Readyz,
		specs.Healthz,
		specs.Health,
		specs.Docs,
		specs.DocsOpenAPI,
		specs.DocsVersion,
		specs.DocsInfo,
		specs.Version,
		specs.HealthDetailed,
		specs.Metrics,
		specs.PprofIndex,
		specs.PprofCmdline,
		specs.PprofProfile,
		specs.PprofSymbol,
		specs.PprofTrace,
	} {
		if router.handlers[pattern] == nil {
			t.Fatalf("expected mounted route %s; got %v", pattern, router.patterns)
		}
	}
	for _, pattern := range []string{specs.HealthDetailed, specs.Metrics, specs.PprofIndex} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, pattern, nil)
		router.handlers[pattern](rec, req)
		if got := rec.Header().Get("X-Admin-Wrapper"); got != "true" {
			t.Fatalf("%s admin wrapper header = %q", pattern, got)
		}
	}
	if wrapped < 3 {
		t.Fatalf("wrapped = %d, want at least 3", wrapped)
	}
}

func TestNewAPIServiceBuildsRouterAndMountsSafeSystemEndpoints(t *testing.T) {
	manager := newValidatedHealthManager(t, health.Config{
		Timeout:         time.Second,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(health.NewBasicChecker())

	service, err := NewAPIService(APIServiceConfig{
		Addr: ":0",
		Log:  ports.NopLogger{},
		RegisterRoutes: func(r contracts.HTTPRouter) error {
			r.Get("/hello", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			return nil
		},
		SystemEndpoints: SystemEndpoints{
			Health:  health.NewHandler(manager),
			Docs:    docs.NewHandler(nil),
			Version: version.NewHandler(version.Config{}),
			Metrics: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
			Pprof: pprofStub(),
		},
		Admin: SystemEndpointAdminOptions{
			RequireAdmin: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Admin-Wrapper", "true")
					next.ServeHTTP(w, r)
				})
			},
			EnablePprof: true,
		},
	})
	if err != nil {
		t.Fatalf("new API service: %v", err)
	}

	rec := httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/hello", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("/hello status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.Metrics, nil))
	if got := rec.Header().Get("X-Admin-Wrapper"); got != "true" {
		t.Fatalf("metrics admin wrapper header = %q", got)
	}
}

func TestNewAPIServiceSeparatesAdminListenerRoutes(t *testing.T) {
	manager := newValidatedHealthManager(t, health.Config{
		Timeout:         time.Second,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	manager.RegisterChecker(health.NewBasicChecker())

	service, err := NewAPIService(APIServiceConfig{
		Addr:      ":0",
		AdminAddr: "127.0.0.1:0",
		Log:       ports.NopLogger{},
		SystemEndpoints: SystemEndpoints{
			Health: health.NewHandler(manager),
			Metrics: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
			Pprof: pprofStub(),
		},
		Admin: SystemEndpointAdminOptions{
			RequireAdmin: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Admin-Wrapper", "true")
					next.ServeHTTP(w, r)
				})
			},
			EnablePprof: true,
		},
	})
	if err != nil {
		t.Fatalf("new API service: %v", err)
	}

	rec := httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.Readyz, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("public readiness status = %d", rec.Code)
	}

	for _, path := range []string{specs.HealthDetailed, specs.Metrics, specs.PprofIndex} {
		rec = httptest.NewRecorder()
		service.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("public %s status = %d, want 404", path, rec.Code)
		}

		rec = httptest.NewRecorder()
		service.AdminHandler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))
		if got := rec.Header().Get("X-Admin-Wrapper"); got != "true" {
			t.Fatalf("admin %s wrapper header = %q", path, got)
		}
	}

	rec = httptest.NewRecorder()
	service.AdminHandler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.Readyz, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin readiness status = %d, want 404", rec.Code)
	}
}

func TestNewAPIServicePublicLivenessDoesNotDependOnReadiness(t *testing.T) {
	manager := newValidatedHealthManager(t, health.Config{
		Timeout:         time.Second,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"database"},
	})
	manager.RegisterChecker(health.NewBasicChecker())
	manager.RegisterChecker(health.NewCustomChecker("database", func(context.Context) (health.Status, string, interface{}) {
		return health.StatusUnhealthy, "database unavailable", nil
	}))

	service, err := NewAPIService(APIServiceConfig{
		Addr:      ":0",
		AdminAddr: "127.0.0.1:0",
		Log:       ports.NopLogger{},
		SystemEndpoints: SystemEndpoints{
			Health: health.NewHandler(manager),
		},
		Admin: SystemEndpointAdminOptions{
			RequireAdmin: func(next http.Handler) http.Handler { return next },
		},
	})
	if err != nil {
		t.Fatalf("new API service: %v", err)
	}

	livez := httptest.NewRecorder()
	service.Handler().ServeHTTP(livez, httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.Livez, nil))
	if livez.Code != http.StatusOK {
		t.Fatalf("liveness status = %d, want 200", livez.Code)
	}

	readyz := httptest.NewRecorder()
	service.Handler().ServeHTTP(readyz, httptest.NewRequestWithContext(context.Background(), http.MethodGet, specs.Readyz, nil))
	if readyz.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d, want 503", readyz.Code)
	}
}

func TestNewAPIServiceReturnsRouteRegistrationError(t *testing.T) {
	_, err := NewAPIService(APIServiceConfig{
		Addr: ":0",
		Log:  ports.NopLogger{},
		RegisterRoutes: func(contracts.HTTPRouter) error {
			return errors.New("route table invalid")
		},
	})
	if err == nil || err.Error() != "register routes: route table invalid" {
		t.Fatalf("unexpected route registration error: %v", err)
	}
}

func TestNewAPIServiceRequiresAdminWrapperForSystemEndpoints(t *testing.T) {
	_, err := NewAPIService(APIServiceConfig{
		Addr: ":0",
		Log:  ports.NopLogger{},
		SystemEndpoints: SystemEndpoints{
			Health: health.NewHandler(nil),
		},
	})
	if err == nil {
		t.Fatal("expected admin wrapper error")
	}
}

func TestNewAPIServiceRunsStartupChecks(t *testing.T) {
	_, err := NewAPIService(APIServiceConfig{
		Addr: ":0",
		Log:  ports.NopLogger{},
		StartupChecks: []StartupCheck{{
			Name: "policy",
			Check: func(context.Context) error {
				return errors.New("invalid policy")
			},
		}},
	})
	if err == nil || err.Error() != "startup check policy: invalid policy" {
		t.Fatalf("unexpected startup check error: %v", err)
	}
}

func TestNewAPIServiceValidatesDeclaredMiddlewareOrder(t *testing.T) {
	router := chi.New()
	_, err := NewAPIService(APIServiceConfig{
		Addr:   ":0",
		Log:    ports.NopLogger{},
		Router: router,
		MiddlewareOrder: []MiddlewareStage{
			MiddlewareRequestID,
			MiddlewareTracing,
			MiddlewareRecovery,
		},
		RequiredMiddlewareOrder: []MiddlewareStage{
			MiddlewareRequestID,
			MiddlewareRecovery,
			MiddlewareTracing,
		},
	})
	if err == nil {
		t.Fatal("expected middleware order validation error")
	}
	if got := err.Error(); !strings.Contains(got, "middleware order") || !strings.Contains(got, "tracing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIServiceStopsBackgroundTasksOnShutdown(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	service, err := NewAPIService(APIServiceConfig{
		Addr: "127.0.0.1:0",
		Log:  ports.NopLogger{},
		BackgroundTasks: []BackgroundTask{{
			Name: "health-scheduler",
			Run: func(ctx context.Context) error {
				close(started)
				<-ctx.Done()
				close(stopped)
				return ctx.Err()
			},
		}},
	})
	if err != nil {
		t.Fatalf("new API service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.Start(ctx)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("background task did not start")
	}
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("start returned error after shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("service did not stop after context cancellation")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("background task did not observe shutdown")
	}
}

func TestAPIServiceReturnsBackgroundTaskFailure(t *testing.T) {
	service, err := NewAPIService(APIServiceConfig{
		Addr: "127.0.0.1:0",
		Log:  ports.NopLogger{},
		BackgroundTasks: []BackgroundTask{{
			Name: "health-scheduler",
			Run: func(context.Context) error {
				return errors.New("refresh failed")
			},
		}},
	})
	if err != nil {
		t.Fatalf("new API service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = service.Start(ctx)
	if err == nil {
		t.Fatal("expected background task failure")
	}
	if got := err.Error(); !strings.Contains(got, "background task health-scheduler") || !strings.Contains(got, "refresh failed") {
		t.Fatalf("unexpected background task error: %v", err)
	}
}

func pprofStub() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}
