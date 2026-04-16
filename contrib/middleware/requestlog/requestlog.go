package requestlog

import (
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	coretrace "github.com/aatuh/api-toolkit/v2/middleware/trace"
	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/response_writer"
)

const (
	FieldRequestID       = "request_id"
	FieldTraceID         = "trace_id"
	FieldSpanID          = "span_id"
	FieldRoute           = "route"
	FieldStatus          = "status"
	FieldLatencyMS       = "latency_ms"
	FieldMethod          = "method"
	FieldPath            = "path"
	FieldBytes           = "bytes"
	FieldClientIP        = "client_ip"
	FieldUserAgent       = "user_agent"
	FieldRequestHeaders  = "req_headers"
	FieldResponseHeaders = "resp_headers"
	FieldStack           = "stack"
	FieldPanicRecovered  = "panic_recovered"
)

const redactedValue = "[redacted]"

var defaultRedactedHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
}

// Middleware logs structured request summaries.
type Middleware struct {
	Log      ports.Logger
	opts     Options
	redactor headerRedactor
	clock    ports.Clock
}

// Options configures request logging behavior.
type Options struct {
	Resolver           identity.Resolver
	RoutePattern       func(*http.Request) string
	LogRequestHeaders  bool
	LogResponseHeaders bool
	Log5xxStacks       bool
	RedactHeaders      []string
	Clock              ports.Clock
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

// WithRequestHeaders enables logging sanitized request headers.
func WithRequestHeaders() Option {
	return func(o *Options) {
		o.LogRequestHeaders = true
	}
}

// WithResponseHeaders enables logging sanitized response headers.
func WithResponseHeaders() Option {
	return func(o *Options) {
		o.LogResponseHeaders = true
	}
}

// WithRedactedHeaders appends additional header names to redact.
func WithRedactedHeaders(headers ...string) Option {
	return func(o *Options) {
		o.RedactHeaders = append(o.RedactHeaders, headers...)
	}
}

// WithClock overrides the time source used for latency measurement.
func WithClock(clock ports.Clock) Option {
	return func(o *Options) {
		o.Clock = clock
	}
}

// With5xxStackLogging controls whether handled 5xx responses include a stack trace.
func With5xxStackLogging(enabled bool) Option {
	return func(o *Options) {
		o.Log5xxStacks = enabled
	}
}

// New constructs a request logging middleware.
func New(log ports.Logger, opts ...Option) (*Middleware, error) {
	if log == nil {
		log = ports.NopLogger{}
	}
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
	if cfg.Clock == nil {
		cfg.Clock = ports.SystemClock{}
	}
	return &Middleware{
		Log:      log,
		opts:     cfg,
		redactor: newHeaderRedactor(cfg),
		clock:    cfg.Clock,
	}, nil
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with request logging.
// It is intended to emit exactly one request log entry on both normal and panic
// paths. For panic paths, it should infer the final visible status before
// re-panicking so outer recovery can still produce the response contract.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := m.clock.Now()
		ww := response_writer.Wrap(w)
		defer func() {
			rec := recover()
			status := ww.Status()
			if rec != nil && !ww.Committed() {
				status = http.StatusInternalServerError
			}
			m.logRequest(r, ww, start, status, rec)
			if rec != nil {
				panic(rec)
			}
		}()
		next.ServeHTTP(ww, r)
	})
}

func (m *Middleware) logRequest(r *http.Request, ww *response_writer.Writer, start time.Time, status int, panicValue any) {
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
	traceID, spanID := traceIDs(r)
	fields := make([]any, 0, 30)
	fields = append(fields,
		FieldMethod, r.Method,
		FieldPath, r.URL.Path,
		FieldRoute, route,
		FieldStatus, status,
		FieldBytes, ww.BytesWritten(),
		FieldLatencyMS, m.clock.Now().Sub(start).Milliseconds(),
		FieldClientIP, ip,
		FieldUserAgent, r.UserAgent(),
		FieldRequestID, requestID(r),
		FieldTraceID, traceID,
		FieldSpanID, spanID,
	)
	if panicValue != nil {
		fields = append(fields, FieldPanicRecovered, true)
	}
	if m.opts.LogRequestHeaders {
		fields = append(fields, FieldRequestHeaders, m.redactor.Redact(r.Header))
	}
	if m.opts.LogResponseHeaders {
		fields = append(fields, FieldResponseHeaders, m.redactor.Redact(ww.Header()))
	}
	if panicValue != nil {
		if status >= http.StatusInternalServerError && m.opts.Log5xxStacks {
			fields = append(fields, FieldStack, string(debug.Stack()))
		}
		m.Log.Error("http", fields...)
		return
	}
	if status >= http.StatusInternalServerError {
		if m.opts.Log5xxStacks {
			fields = append(fields, FieldStack, string(debug.Stack()))
		}
		m.Log.Error("http", fields...)
		return
	}
	if status >= http.StatusBadRequest {
		m.Log.Warn("http", fields...)
		return
	}
	m.Log.Info("http", fields...)
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

func traceIDs(r *http.Request) (string, string) {
	if r == nil {
		return "", ""
	}
	sc := oteltrace.SpanContextFromContext(r.Context())
	if sc.IsValid() {
		return sc.TraceID().String(), sc.SpanID().String()
	}
	return coretrace.GetTraceID(r), coretrace.GetSpanID(r)
}

type headerRedactor struct {
	extra map[string]struct{}
}

func newHeaderRedactor(opts Options) headerRedactor {
	extra := make(map[string]struct{}, len(opts.RedactHeaders))
	for _, name := range opts.RedactHeaders {
		if norm := normalizeHeaderName(name); norm != "" {
			extra[norm] = struct{}{}
		}
	}
	return headerRedactor{extra: extra}
}

func (r headerRedactor) Redact(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for name, values := range headers {
		key := http.CanonicalHeaderKey(name)
		if r.isSensitive(name) {
			out[key] = redactedValue
			continue
		}
		switch len(values) {
		case 0:
			out[key] = ""
		case 1:
			out[key] = values[0]
		default:
			out[key] = strings.Join(values, ",")
		}
	}
	return out
}

func (r headerRedactor) isSensitive(name string) bool {
	norm := normalizeHeaderName(name)
	if norm == "" {
		return false
	}
	if _, ok := defaultRedactedHeaders[norm]; ok {
		return true
	}
	if isAPIKeyHeader(norm) {
		return true
	}
	_, ok := r.extra[norm]
	return ok
}

func normalizeHeaderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isAPIKeyHeader(name string) bool {
	return strings.Contains(name, "api-key") ||
		strings.Contains(name, "api_key") ||
		strings.Contains(name, "apikey") ||
		strings.Contains(name, "api-token")
}
