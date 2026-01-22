package requestlog

import (
	"net"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/httpx/identity"
	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/response_writer"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Middleware logs structured request summaries.
type Middleware struct {
	Log  ports.Logger
	opts Options
}

// Options configures request logging behavior.
type Options struct {
	Resolver     identity.Resolver
	RoutePattern func(*http.Request) string
}

// Option mutates request log options.
type Option func(*Options)

// WithResolver sets the trusted proxy resolver.
func WithResolver(resolver identity.Resolver) Option {
	return func(o *Options) {
		o.Resolver = resolver
	}
}

// WithRoutePattern sets the route pattern function for logging.
func WithRoutePattern(fn func(*http.Request) string) Option {
	return func(o *Options) {
		o.RoutePattern = fn
	}
}

// New constructs a request logging middleware.
func New(log ports.Logger, opts ...Option) *Middleware {
	cfg := Options{
		Resolver: identity.Resolver{
			HeaderPolicy: identity.HeaderPolicyBoth,
		},
		RoutePattern: chiRoutePattern,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return &Middleware{Log: log, opts: cfg}
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with request logging.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := response_writer.Wrap(w)
		next.ServeHTTP(ww, r)

		route := ""
		if m.opts.RoutePattern != nil {
			route = m.opts.RoutePattern(r)
		}
		if route == "" {
			route = "unknown"
		}
		ip := m.opts.Resolver.ClientIPString(r)
		if ip == "" {
			ip = remoteIP(r.RemoteAddr)
		}
		m.Log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"route", route,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"dur_ms", time.Since(start).Milliseconds(),
			"ip", ip,
			"ua", r.UserAgent(),
			"rid", requestID(r),
		)
	})
}

func requestID(r *http.Request) string {
	if v := r.Header.Get("X-Request-ID"); v != "" {
		return v
	}
	if v := middleware.GetReqID(r.Context()); v != "" {
		return v
	}
	return ""
}

func chiRoutePattern(r *http.Request) string {
	if r == nil {
		return ""
	}
	ctx := chi.RouteContext(r.Context())
	if ctx == nil {
		return ""
	}
	return ctx.RoutePattern()
}

func remoteIP(remote string) string {
	if remote == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}
