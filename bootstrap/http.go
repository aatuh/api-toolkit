package bootstrap

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/aatuh/api-toolkit/adapters/chi"
	"github.com/aatuh/api-toolkit/endpoints/docs"
	"github.com/aatuh/api-toolkit/endpoints/health"
	pprofx "github.com/aatuh/api-toolkit/endpoints/pprof"
	"github.com/aatuh/api-toolkit/endpoints/version"
	recoverx "github.com/aatuh/api-toolkit/httpx/recover"
	"github.com/aatuh/api-toolkit/middleware/cors"
	jsonmw "github.com/aatuh/api-toolkit/middleware/json"
	maxbody "github.com/aatuh/api-toolkit/middleware/maxbody"
	metricsmw "github.com/aatuh/api-toolkit/middleware/metrics"
	oteltrace "github.com/aatuh/api-toolkit/middleware/oteltrace"
	rateln "github.com/aatuh/api-toolkit/middleware/ratelimit"
	requestlog "github.com/aatuh/api-toolkit/middleware/requestlog"
	securemw "github.com/aatuh/api-toolkit/middleware/secure"
	timeoutmw "github.com/aatuh/api-toolkit/middleware/timeout"
	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/specs"
)

// NewDefaultRouter constructs a router with a sensible default middleware stack.
func NewDefaultRouter(log ports.Logger) ports.HTTPRouter {
	var r ports.HTTPRouter = chi.New()
	var mw ports.HTTPMiddleware = chi.NewMiddleware()

	// Optional rate limit bypass for tests/dev (similar to Clerk skip header).
	skipHeader := os.Getenv("RATE_LIMIT_SKIP_HEADER")
	skipEnabled := os.Getenv("RATE_LIMIT_SKIP_ENABLED") == "true"

	// Core middlewares
	r.Use(mw.RequestID())
	r.Use(mw.RealIP())
	r.Use(oteltrace.New(oteltrace.Options{}).Middleware())
	r.Use(recoverx.Middleware())

	// Standard middlewares
	corsh := cors.New()
	r.Use(corsh.Handler(cors.DefaultOptions()))
	r.Use(securemw.New().Middleware())
	r.Use(rateln.New(rateln.Options{
		Capacity:    30,
		RefillRate:  15,
		SkipEnabled: skipEnabled,
		SkipHeader:  skipHeader,
	}).Middleware())
	r.Use(maxbody.New(1 << 20).Middleware())
	r.Use(jsonmw.New(true).Middleware())
	r.Use(timeoutmw.New(5 * time.Second).Middleware())
	r.Use(requestlog.New(log).Middleware())
	r.Use(metricsmw.New(metricsmw.NewPrometheusRecorder(nil, nil)).Middleware())

	return r
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
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

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
		shctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shctx)
		return nil
	case err := <-errCh:
		return err
	}
}

// StartServerOrExit runs the HTTP server and exits the process when it fails.
func StartServerOrExit(
	ctx context.Context,
	addr string,
	handler http.Handler,
	log ports.Logger,
) {
	if err := StartServer(ctx, addr, handler, log); err != nil {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
}
