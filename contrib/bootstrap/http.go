package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/envvar"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	"github.com/aatuh/api-toolkit/v2/endpoints/docs"
	"github.com/aatuh/api-toolkit/v2/endpoints/health"
	pprofx "github.com/aatuh/api-toolkit/v2/endpoints/pprof"
	"github.com/aatuh/api-toolkit/v2/endpoints/version"
	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	rateln "github.com/aatuh/api-toolkit/v2/middleware/ratelimit"
	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/specs"
)

// DefaultRouterConfig defines the inputs used by NewDefaultRouterWithConfig.
type DefaultRouterConfig struct {
	RateLimit      rateln.Options
	TrustedProxies []netip.Prefix
	Metrics        metricsmw.MetricsRecorder
}

// DefaultRouterConfigFromEnv loads router defaults from environment variables.
func DefaultRouterConfigFromEnv(env ports.EnvVar) (DefaultRouterConfig, error) {
	if env == nil {
		env = envvar.New()
	}

	cfg := DefaultRouterConfig{
		RateLimit: rateln.Options{
			Capacity:                  30,
			RefillRate:                15,
			SkipHeader:                env.GetOr("RATE_LIMIT_SKIP_HEADER", ""),
			SkipEnabled:               env.GetBoolOr("RATE_LIMIT_SKIP_ENABLED", false),
			AllowDangerousDevBypasses: env.GetBoolOr("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", false),
		},
	}

	if raw := strings.TrimSpace(env.GetOr("TRUSTED_PROXIES", "")); raw != "" {
		prefixes, err := identity.ParseTrustedProxies(strings.Split(raw, ","))
		if err != nil {
			return DefaultRouterConfig{}, fmt.Errorf("parse trusted proxies: %w", err)
		}
		cfg.TrustedProxies = prefixes
	}

	return cfg, nil
}

// NewDefaultRouter constructs a router with a sensible default middleware stack.
func NewDefaultRouter(log ports.Logger) (ports.HTTPRouter, error) {
	cfg, err := DefaultRouterConfigFromEnv(nil)
	if err != nil {
		return nil, err
	}
	return NewDefaultRouterWithConfig(log, cfg)
}

// NewDefaultRouterWithConfig constructs a router from explicit configuration.
func NewDefaultRouterWithConfig(log ports.Logger, cfg DefaultRouterConfig) (ports.HTTPRouter, error) {
	r := chi.New()

	resolver := identity.Resolver{HeaderPolicy: identity.HeaderPolicyBoth}
	resolver.TrustedProxies = append(resolver.TrustedProxies, cfg.TrustedProxies...)

	rateLimit := cfg.RateLimit
	if rateLimit.Capacity == 0 {
		rateLimit.Capacity = 30
	}
	if rateLimit.RefillRate == 0 {
		rateLimit.RefillRate = 15
	}

	opts := []ProfileOption{
		WithIdentityResolver(resolver),
		WithRateLimitOptions(rateLimit),
	}
	if cfg.Metrics != nil {
		opts = append(opts, WithMetricsRecorder(cfg.Metrics))
	}

	profile, err := ProfileStrictAPI(log, opts...)
	if err != nil {
		return nil, err
	}
	profile.ApplyTo(r)
	return r, nil
}

// SystemEndpoints bundles handlers/config for mounting.
type SystemEndpoints struct {
	Health  *health.Handler
	Docs    *docs.Handler
	Version *version.Handler
	Pprof   http.Handler
	Metrics http.Handler
}

// MountSystemEndpoints registers health, docs, version, and metrics endpoints.
func MountSystemEndpoints(r ports.HTTPRouter, se SystemEndpoints) {
	MountSystemEndpointsTo(r, se)
}

// MountSystemEndpointsTo registers system endpoints on a minimal GET-only surface.
func MountSystemEndpointsTo(r ports.MethodRouteRegistrar, se SystemEndpoints) {
	if r == nil {
		return
	}
	if se.Health != nil {
		se.Health.RegisterRoutesTo(r)
	}
	if se.Docs != nil {
		se.Docs.RegisterRoutesTo(r)
	}
	if se.Version != nil {
		se.Version.RegisterRoutesTo(r)
	}
	if se.Pprof != nil {
		pprofx.RegisterRoutes(pprofRouter{
			router: r,
			h:      se.Pprof,
		})
	}
	metricsHandler := se.Metrics
	if metricsHandler == nil {
		metricsHandler = metricsmw.PrometheusHandler()
	}
	if metricsHandler != nil {
		r.Get(specs.Metrics, func(w http.ResponseWriter, req *http.Request) {
			metricsHandler.ServeHTTP(w, req)
		})
	}
}

type pprofRouter struct {
	router ports.MethodRouteRegistrar
	h      http.Handler
}

type serverRunner interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

func (p pprofRouter) Get(pattern string, _ http.HandlerFunc) {
	p.router.Get(pattern, func(w http.ResponseWriter, r *http.Request) {
		p.h.ServeHTTP(w, r)
	})
}

// StartServer runs an HTTP server and performs graceful shutdown when the
// context is canceled.
func StartServer(
	ctx context.Context,
	addr string,
	handler http.Handler,
	log ports.Logger,
) error {
	if log == nil {
		log = ports.NopLogger{}
	}
	log.Info("http server starting", "addr", addr)
	return runServer(ctx, HardenedServer(addr, handler))
}

func runServer(ctx context.Context, srv serverRunner) error {
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shctx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		if err, ok := <-errCh; ok {
			return err
		}
		return nil
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		return err
	}
}

// ServerOption configures an http.Server instance.
type ServerOption func(*http.Server)

// HardenedServer builds an http.Server with safe defaults and optional overrides.
func HardenedServer(addr string, handler http.Handler, opts ...ServerOption) *http.Server {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(srv)
		}
	}
	return srv
}

// WithReadHeaderTimeout overrides ReadHeaderTimeout.
func WithReadHeaderTimeout(d time.Duration) ServerOption {
	return func(s *http.Server) {
		s.ReadHeaderTimeout = d
	}
}

// WithReadTimeout overrides ReadTimeout.
func WithReadTimeout(d time.Duration) ServerOption {
	return func(s *http.Server) {
		s.ReadTimeout = d
	}
}

// WithWriteTimeout overrides WriteTimeout.
func WithWriteTimeout(d time.Duration) ServerOption {
	return func(s *http.Server) {
		s.WriteTimeout = d
	}
}

// WithIdleTimeout overrides IdleTimeout.
func WithIdleTimeout(d time.Duration) ServerOption {
	return func(s *http.Server) {
		s.IdleTimeout = d
	}
}

// WithMaxHeaderBytes overrides MaxHeaderBytes.
func WithMaxHeaderBytes(n int) ServerOption {
	return func(s *http.Server) {
		s.MaxHeaderBytes = n
	}
}

// StartServerOrExit runs the HTTP server and exits the process when it fails.
func StartServerOrExit(
	ctx context.Context,
	addr string,
	handler http.Handler,
	log ports.Logger,
) {
	if log == nil {
		log = ports.NopLogger{}
	}
	if err := StartServer(ctx, addr, handler, log); err != nil {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
}
