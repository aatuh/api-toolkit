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

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v3/adapters/envvar"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics"
	"github.com/aatuh/api-toolkit/v3/endpoints/docs"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	pprofx "github.com/aatuh/api-toolkit/v3/endpoints/pprof"
	"github.com/aatuh/api-toolkit/v3/endpoints/version"
	"github.com/aatuh/api-toolkit/v3/httpx/identity"
	rateln "github.com/aatuh/api-toolkit/v3/middleware/ratelimit"
	"github.com/aatuh/api-toolkit/v3/ports"
	"github.com/aatuh/api-toolkit/v3/specs"
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
	if err := ValidateMiddlewareOrder(profile.MiddlewareOrder, StrictAPIMiddlewareOrder()...); err != nil {
		return nil, fmt.Errorf("default router middleware order: %w", err)
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

type SystemEndpointProfile string

const (
	SystemProfileProduction  SystemEndpointProfile = "production"
	SystemProfileStaging     SystemEndpointProfile = "staging"
	SystemProfileDevelopment SystemEndpointProfile = "development"
	SystemProfileTest        SystemEndpointProfile = "test"
)

type SystemEndpointOptions struct {
	EnablePprof bool
}

// SystemEndpointAdminOptions configures safe admin mounting for operator-only
// system endpoints.
type SystemEndpointAdminOptions struct {
	RequireAdmin func(http.Handler) http.Handler
	EnablePprof  bool
}

// MountSystemEndpoints registers health, docs, version, and metrics endpoints.
// It preserves v2 convenience behavior; prefer MountSystemEndpointsToWithAdmin
// when mounting metrics, pprof, or detailed health from new wiring.
func MountSystemEndpoints(r ports.HTTPRouter, se SystemEndpoints) {
	MountSystemEndpointsToWithOptions(r, se, SystemEndpointOptions{})
}

// MountSystemEndpointsTo registers system endpoints on a minimal GET-only
// surface. It preserves v2 convenience behavior; prefer
// MountSystemEndpointsToWithAdmin when mounting metrics, pprof, or detailed
// health from new wiring.
func MountSystemEndpointsTo(r ports.MethodRouteRegistrar, se SystemEndpoints) {
	MountSystemEndpointsToWithProfile(r, se, string(SystemProfileProduction))
}

// MountSystemEndpointsToWithProfile mounts system endpoints only if the profile
// explicitly opts pprof in. This keeps production/staging defaults off by
// default.
func MountSystemEndpointsToWithProfile(r ports.MethodRouteRegistrar, se SystemEndpoints, profile string) {
	MountSystemEndpointsToWithOptions(r, se, SystemEndpointOptions{
		EnablePprof: isProfilePprofEnabled(profile),
	})
}

// MountSystemEndpointsToWithOptions mounts system endpoints with explicit
// runtime options. It preserves v2 convenience behavior; prefer
// MountSystemEndpointsToWithAdmin when mounting metrics, pprof, or detailed
// health from new wiring.
func MountSystemEndpointsToWithOptions(r ports.MethodRouteRegistrar, se SystemEndpoints, opts SystemEndpointOptions) {
	if r == nil {
		return
	}
	if !opts.EnablePprof {
		se.Pprof = nil
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
	if se.Metrics != nil {
		r.Get(specs.Metrics, func(w http.ResponseWriter, req *http.Request) {
			se.Metrics.ServeHTTP(w, req)
		})
	}
}

// MountSystemEndpointsToWithAdmin mounts public health, docs, and version
// routes normally while mounting operator-only detailed health, metrics, and
// optional pprof routes behind an explicit authorization or internal-network
// wrapper.
func MountSystemEndpointsToWithAdmin(r ports.MethodRouteRegistrar, se SystemEndpoints, opts SystemEndpointAdminOptions) error {
	if opts.RequireAdmin == nil {
		return errors.New("system endpoint admin routes require an authorization wrapper")
	}
	if r == nil {
		return nil
	}
	mountPublicSystemEndpointsTo(r, se)
	return mountAdminSystemEndpointsTo(r, se, opts)
}

func mountPublicSystemEndpointsTo(r ports.MethodRouteRegistrar, se SystemEndpoints) {
	if r == nil {
		return
	}
	if se.Health != nil {
		se.Health.RegisterPublicRoutesTo(r)
	}
	if se.Docs != nil {
		se.Docs.RegisterRoutesTo(r)
	}
	if se.Version != nil {
		se.Version.RegisterRoutesTo(r)
	}
}

func mountAdminSystemEndpointsTo(r ports.MethodRouteRegistrar, se SystemEndpoints, opts SystemEndpointAdminOptions) error {
	if opts.RequireAdmin == nil {
		return errors.New("system endpoint admin routes require an authorization wrapper")
	}
	if r == nil {
		return nil
	}
	if se.Health != nil {
		if err := se.Health.RegisterAdminDetailedHealthRoute(r, opts.RequireAdmin); err != nil {
			return err
		}
	}
	if opts.EnablePprof && se.Pprof != nil {
		pprofx.RegisterRoutes(pprofRouter{
			router: r,
			h:      opts.RequireAdmin(se.Pprof),
		})
	}
	if se.Metrics != nil {
		metricsHandler := opts.RequireAdmin(se.Metrics)
		r.Get(specs.Metrics, metricsHandler.ServeHTTP)
	}
	return nil
}

func isProfilePprofEnabled(profile string) bool {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case string(SystemProfileDevelopment), "dev", "localhost", "local":
		return true
	default:
		return false
	}
}

// PrometheusMetricsHandler returns the standard Prometheus metrics handler for
// explicit mounting on specs.Metrics.
func PrometheusMetricsHandler() http.Handler {
	return metricsmw.PrometheusHandler()
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
