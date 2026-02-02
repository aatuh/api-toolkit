package bootstrap

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit-contrib/adapters/chi"
	metricsmw "github.com/aatuh/api-toolkit-contrib/middleware/metrics"
	"github.com/aatuh/api-toolkit/endpoints/docs"
	"github.com/aatuh/api-toolkit/endpoints/health"
	pprofx "github.com/aatuh/api-toolkit/endpoints/pprof"
	"github.com/aatuh/api-toolkit/endpoints/version"
	"github.com/aatuh/api-toolkit/httpx/identity"
	rateln "github.com/aatuh/api-toolkit/middleware/ratelimit"
	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/specs"
)

// NewDefaultRouter constructs a router with a sensible default middleware stack.
func NewDefaultRouter(log ports.Logger) (ports.HTTPRouter, error) {
	r := chi.New()

	// Optional rate limit bypass for tests/dev (similar to Clerk skip header).
	skipHeader := os.Getenv("RATE_LIMIT_SKIP_HEADER")
	skipEnabled := os.Getenv("RATE_LIMIT_SKIP_ENABLED") == "true"
	allowDevBypass := os.Getenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES") == "true"

	resolver := identity.Resolver{HeaderPolicy: identity.HeaderPolicyBoth}
	if raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")); raw != "" {
		if prefixes, err := identity.ParseTrustedProxies(strings.Split(raw, ",")); err == nil {
			resolver.TrustedProxies = prefixes
		}
	}

	profile, err := ProfileStrictAPI(log,
		WithIdentityResolver(resolver),
		WithRateLimitOptions(rateln.Options{
			Capacity:                  30,
			RefillRate:                15,
			SkipEnabled:               skipEnabled,
			SkipHeader:                skipHeader,
			AllowDangerousDevBypasses: allowDevBypass,
		}),
		WithMetricsRecorder(metricsmw.NewPrometheusRecorder(nil, nil)),
	)
	if err != nil {
		return nil, err
	}
	profile.Apply(r)
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
	if se.Health != nil {
		se.Health.RegisterRoutes(r)
	}
	if se.Docs != nil {
		se.Docs.RegisterRoutes(r)
	}
	if se.Version != nil {
		se.Version.RegisterRoutes(r)
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
	router interface {
		Get(pattern string, h http.HandlerFunc)
	}
	h http.Handler
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
	srv := HardenedServer(addr, handler)

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shctx)
		return nil
	case err := <-errCh:
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
