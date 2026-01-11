package oteltrace

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	attrHTTPMethod     = "http.method"
	attrHTTPRoute      = "http.route"
	attrHTTPStatusCode = "http.status_code"
	attrHTTPDurationMS = "http.server.duration_ms"
)

// Options configures tracing middleware.
type Options struct {
	TracerName   string
	RoutePattern func(*http.Request) string
}

// Middleware instruments inbound HTTP requests with OpenTelemetry spans.
type Middleware struct {
	tracer       trace.Tracer
	routePattern func(*http.Request) string
}

// New constructs a new tracing middleware.
func New(opts Options) *Middleware {
	name := strings.TrimSpace(opts.TracerName)
	if name == "" {
		name = "api-toolkit/http"
	}
	patternFn := opts.RoutePattern
	if patternFn == nil {
		patternFn = chiRoutePattern
	}
	return &Middleware{
		tracer:       otel.Tracer(name),
		routePattern: patternFn,
	}
}

// Middleware implements ports.Middleware by returning Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.Handler
}

// Handler wraps the next handler with a tracing span.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		propagator := otel.GetTextMapPropagator()
		ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		start := time.Now()
		ww := &respWriter{ResponseWriter: w, status: http.StatusOK}
		ctx, span := m.tracer.Start(ctx, spanName(r.Method, ""), trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		r = r.WithContext(ctx)
		next.ServeHTTP(ww, r)

		route := strings.TrimSpace(m.routePattern(r))
		if route == "" {
			route = "unknown"
		}
		duration := time.Since(start)

		span.SetName(spanName(r.Method, route))
		span.SetAttributes(
			attribute.String(attrHTTPMethod, r.Method),
			attribute.String(attrHTTPRoute, route),
			attribute.Int(attrHTTPStatusCode, ww.status),
			attribute.Int64(attrHTTPDurationMS, duration.Milliseconds()),
		)
		if ww.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(ww.status))
		}
	})
}

func spanName(method, route string) string {
	method = strings.TrimSpace(method)
	route = strings.TrimSpace(route)
	if route == "" {
		route = "unknown"
	}
	if method == "" {
		return route
	}
	return method + " " + route
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

type respWriter struct {
	http.ResponseWriter
	status int
}

func (w *respWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
