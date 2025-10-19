package bootstrap

import (
	"context"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/chi"
	"github.com/aatuh/api-toolkit/docs"
	"github.com/aatuh/api-toolkit/health"
	recoverx "github.com/aatuh/api-toolkit/httpx/recover"
	"github.com/aatuh/api-toolkit/middleware/cors"
	jsonmw "github.com/aatuh/api-toolkit/middleware/json"
	maxbody "github.com/aatuh/api-toolkit/middleware/maxbody"
	metricsmw "github.com/aatuh/api-toolkit/middleware/metrics"
	rateln "github.com/aatuh/api-toolkit/middleware/ratelimit"
	requestlog "github.com/aatuh/api-toolkit/middleware/requestlog"
	securemw "github.com/aatuh/api-toolkit/middleware/secure"
	timeoutmw "github.com/aatuh/api-toolkit/middleware/timeout"
	tracemw "github.com/aatuh/api-toolkit/middleware/trace"
	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/specs"
)

// NewDefaultRouter constructs a router with a sensible default middleware stack.
func NewDefaultRouter(log ports.Logger) ports.HTTPRouter {
	var r ports.HTTPRouter = chi.New()
	var mw ports.HTTPMiddleware = chi.NewMiddleware()

	// Core middlewares
	r.Use(mw.RequestID())
	r.Use(mw.RealIP())
	r.Use(recoverx.Middleware())

	// Standard middlewares
	corsh := cors.New()
	r.Use(corsh.Handler(cors.DefaultOptions()))
	r.Use(securemw.New().Middleware())
	r.Use(rateln.New(rateln.Options{Capacity: 30, RefillRate: 15}).Handler)
	r.Use(maxbody.New(1 << 20).Handler)
	r.Use(jsonmw.New(true).Handler)
	r.Use(timeoutmw.New(5 * time.Second).Handler)
	r.Use(requestlog.New(log).Handler)
	r.Use(metricsmw.New(metricsmw.NewPrometheusRecorder(nil, nil)).Handler)
	r.Use(tracemw.Middleware(tracemw.Options{TrustIncoming: false}))

	return r
}

// MountSystemEndpoints registers health, docs, and metrics endpoints.
func MountSystemEndpoints(r ports.HTTPRouter, hm *health.Handler, dm *docs.Handler) {
	hm.RegisterRoutes(r)
	dm.RegisterRoutes(r)
	r.Get(specs.Metrics, func(w http.ResponseWriter, r *http.Request) {
		h := metricsmw.PrometheusHandler()
		h.ServeHTTP(w, r)
	})
}

// StartServer runs an HTTP server and performs graceful shutdown when the
// context is canceled.
func StartServer(ctx context.Context, addr string, handler http.Handler, log ports.Logger) error {
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
