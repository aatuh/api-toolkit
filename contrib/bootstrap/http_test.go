package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/aatuh/api-toolkit/v2/endpoints/docs"
	"github.com/aatuh/api-toolkit/v2/endpoints/health"
	"github.com/aatuh/api-toolkit/v2/endpoints/version"
	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/specs"
)

type stubServerRunner struct {
	listenErr   error
	shutdownErr error
	listenDone  chan struct{}
	shutdownHit atomic.Bool
}

type stubRouteRegistrar struct {
	patterns []string
}

func (s *stubRouteRegistrar) Get(pattern string, _ http.HandlerFunc) {
	s.patterns = append(s.patterns, pattern)
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

func pprofStub() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}
