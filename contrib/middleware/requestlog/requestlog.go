package requestlog

import (
	"context"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery"
	"github.com/aatuh/api-toolkit/v3/httpx/identity"
	idempotencymw "github.com/aatuh/api-toolkit/v3/middleware/idempotency"
	timeoutmw "github.com/aatuh/api-toolkit/v3/middleware/timeout"
	coretrace "github.com/aatuh/api-toolkit/v3/middleware/trace"
	"github.com/aatuh/api-toolkit/v3/ports"
	"github.com/aatuh/api-toolkit/v3/routepolicy"
)

const (
	FieldRequestID              = "request_id"
	FieldTraceID                = "trace_id"
	FieldSpanID                 = "span_id"
	FieldRoute                  = "route"
	FieldStatus                 = "status"
	FieldCommittedStatus        = "committed_status"
	FieldLatencyMS              = "latency_ms"
	FieldMethod                 = "method"
	FieldPath                   = "path"
	FieldBytes                  = "bytes"
	FieldClientIP               = "client_ip"
	FieldUserAgent              = "user_agent"
	FieldRequestHeaders         = "req_headers"
	FieldResponseHeaders        = "resp_headers"
	FieldStack                  = "stack"
	FieldPanicRecovered         = "panic_recovered"
	FieldPolicyAuth             = "policy_auth"
	FieldPolicyTenant           = "policy_tenant"
	FieldPolicyIdempotency      = "policy_idempotency"
	FieldPolicyRateLimit        = "policy_rate_limit"
	FieldPolicyAdmin            = "policy_admin"
	FieldPolicyDeprecated       = "policy_deprecated"
	FieldIdempotencyMethod      = "idempotency_method"
	FieldIdempotencyStoreClass  = "idempotency_store_class"
	FieldIdempotencyOutcome     = "idempotency_outcome"
	FieldIdempotencyStatusClass = "idempotency_status_class"
	FieldIdempotencyFailOpen    = "idempotency_fail_open"

	FieldHardTimeoutMethod          = "hard_timeout_method"
	FieldHardTimeoutOutcome         = "hard_timeout_outcome"
	FieldHardTimeoutStatusClass     = "hard_timeout_status_class"
	FieldHardTimeoutTimedOut        = "hard_timeout_timed_out"
	FieldHardTimeoutPanicked        = "hard_timeout_panicked"
	FieldHardTimeoutCaptureOverflow = "hard_timeout_capture_overflow"

	FieldWebhookDeliveryEventType   = "webhook_delivery_event_type"
	FieldWebhookDeliveryOutcome     = "webhook_delivery_outcome"
	FieldWebhookDeliveryStatusClass = "webhook_delivery_status_class"
)

const redactedValue = "[redacted]"

var defaultRedactedHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
	"x-access-token":      {},
	"x-auth-token":        {},
	"x-api-key":           {},
	"x-bearer-token":      {},
	"x-id-token":          {},
	"x-refresh-token":     {},
	"x-session-id":        {},
	"x-session-token":     {},
	"x-secret":            {},
	"x-password":          {},
	"bearer":              {},
}

var defaultRedactedPayloadFields = map[string]struct{}{
	"authorization": {},
	"auth":          {},
	"x_auth":        {},
	"api_key":       {},
	"api_token":     {},
	"apikey":        {},
	"api_key_id":    {},
	"apikeyid":      {},
	"id_token":      {},
	"idtoken":       {},
	"password":      {},
	"password_hash": {},
	"refresh_token": {},
	"refreshtoken":  {},
	"secret":        {},
	"session":       {},
	"session_id":    {},
	"session_token": {},
	"sessiontoken":  {},
	"token":         {},
	"access_token":  {},
	"accesstoken":   {},
	"id":            {},
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
		if r != nil {
			r = r.WithContext(routepolicy.SeedObservabilityContext(r.Context()))
		}
		start := m.clock.Now()
		ww := wrapResponseWriter(w)
		defer func() {
			rec := recover()
			status := ww.Status()
			if rec != nil {
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

func (m *Middleware) logRequest(r *http.Request, ww *responseRecorder, start time.Time, status int, panicValue any) {
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
	if labels, ok := routepolicy.ObservabilityLabelsFromRequest(r); ok {
		fields = appendPolicyLabels(fields, labels)
	}
	if panicValue != nil {
		if ww.Committed() && ww.Status() != 0 {
			fields = append(fields, FieldCommittedStatus, ww.Status())
		}
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

func appendPolicyLabels(fields []any, labels routepolicy.ObservabilityLabels) []any {
	values := labels.Map()
	return append(fields,
		FieldPolicyAuth, values["auth"],
		FieldPolicyTenant, values["tenant"],
		FieldPolicyIdempotency, values["idempotency"],
		FieldPolicyRateLimit, values["rate_limit"],
		FieldPolicyAdmin, values["admin"],
		FieldPolicyDeprecated, values["deprecated"],
	)
}

// IdempotencyOutcomeLogHook returns an idempotency middleware outcome hook that
// writes bounded outcome fields to the configured logger.
func IdempotencyOutcomeLogHook(log ports.Logger) idempotencymw.OutcomeHandler {
	if log == nil {
		return nil
	}
	return func(_ context.Context, event idempotencymw.OutcomeEvent) {
		log.Info("idempotency outcome", IdempotencyOutcomeFields(event)...)
	}
}

// IdempotencyOutcomeFields returns bounded log fields for an idempotency
// outcome. It intentionally omits request paths, tenant IDs, idempotency keys,
// request IDs, bodies, and raw error strings.
func IdempotencyOutcomeFields(event idempotencymw.OutcomeEvent) []any {
	labels := event.MetricLabels()
	return []any{
		FieldIdempotencyMethod, labels["method"],
		FieldIdempotencyStoreClass, labels["store_class"],
		FieldIdempotencyOutcome, labels["outcome"],
		FieldIdempotencyStatusClass, labels["status_class"],
		FieldIdempotencyFailOpen, event.FailOpen,
	}
}

// HardTimeoutEventLogHook returns a hard-timeout middleware event hook that
// writes bounded timeout fields to the configured logger.
func HardTimeoutEventLogHook(log ports.Logger) func(timeoutmw.HardTimeoutEvent) {
	if log == nil {
		return nil
	}
	return func(event timeoutmw.HardTimeoutEvent) {
		fields := HardTimeoutEventFields(event)
		switch event.Outcome {
		case timeoutmw.HardTimeoutOutcomeTimeout:
			log.Warn("hard timeout event", fields...)
		case timeoutmw.HardTimeoutOutcomePanic, timeoutmw.HardTimeoutOutcomeCaptureOverflow:
			log.Error("hard timeout event", fields...)
		default:
			log.Error("hard timeout event", fields...)
		}
	}
}

// HardTimeoutEventFields returns bounded log fields for a hard-timeout event.
// It intentionally omits request paths, tenant IDs, request IDs, headers,
// bodies, panic values, and raw error strings.
func HardTimeoutEventFields(event timeoutmw.HardTimeoutEvent) []any {
	return []any{
		FieldHardTimeoutMethod, hardTimeoutMethodLabel(event.Method),
		FieldHardTimeoutOutcome, hardTimeoutOutcomeLabel(event.Outcome),
		FieldHardTimeoutStatusClass, hardTimeoutStatusClass(event.Status),
		FieldHardTimeoutTimedOut, event.TimedOut,
		FieldHardTimeoutPanicked, event.Panicked,
		FieldHardTimeoutCaptureOverflow, event.CaptureOverflow,
	}
}

// WebhookDeliveryLogHook returns a webhook delivery observation hook that
// writes bounded delivery fields to the configured logger.
func WebhookDeliveryLogHook(log ports.Logger) webhookdelivery.MetricsRecorder {
	if log == nil {
		return nil
	}
	return webhookDeliveryLogHook{log: log}
}

type webhookDeliveryLogHook struct {
	log ports.Logger
}

func (h webhookDeliveryLogHook) ObserveWebhookDelivery(_ context.Context, event webhookdelivery.DeliveryObservation) {
	if h.log == nil {
		return
	}
	fields := WebhookDeliveryFields(event)
	switch webhookdelivery.SafeLabel(event.Outcome) {
	case "accepted":
		h.log.Info("webhook delivery event", fields...)
	default:
		h.log.Warn("webhook delivery event", fields...)
	}
}

// WebhookDeliveryFields returns bounded log fields for an outbound webhook
// delivery observation. It intentionally omits tenant IDs, endpoint IDs,
// delivery IDs, URLs, payloads, secrets, and raw error strings.
func WebhookDeliveryFields(event webhookdelivery.DeliveryObservation) []any {
	return []any{
		FieldWebhookDeliveryEventType, webhookdelivery.SafeLabel(event.EventType),
		FieldWebhookDeliveryOutcome, webhookdelivery.SafeLabel(event.Outcome),
		FieldWebhookDeliveryStatusClass, webhookDeliveryStatusClass(event.StatusClass),
	}
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

func hardTimeoutMethodLabel(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace:
		return method
	case "":
		return "UNKNOWN"
	default:
		return "OTHER"
	}
}

func hardTimeoutOutcomeLabel(outcome timeoutmw.HardTimeoutOutcome) string {
	switch outcome {
	case timeoutmw.HardTimeoutOutcomeTimeout,
		timeoutmw.HardTimeoutOutcomePanic,
		timeoutmw.HardTimeoutOutcomeCaptureOverflow:
		return string(outcome)
	default:
		return "unknown"
	}
}

func hardTimeoutStatusClass(status int) string {
	switch {
	case status >= 100 && status < 200:
		return "1xx"
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "none"
	}
}

func webhookDeliveryStatusClass(statusClass string) string {
	switch strings.TrimSpace(statusClass) {
	case "1xx", "2xx", "3xx", "4xx", "5xx":
		return strings.TrimSpace(statusClass)
	}
	switch webhookdelivery.SafeLabel(statusClass) {
	case "transport":
		return "transport"
	default:
		return "unknown"
	}
}

type headerRedactor struct {
	extra map[string]struct{}
}

func newHeaderRedactor(opts Options) headerRedactor {
	extra := make(map[string]struct{}, len(opts.RedactHeaders))
	for _, name := range opts.RedactHeaders {
		if norm := normalizeHeaderName(name); norm != "" {
			extra[normalizeHeaderForMatch(norm)] = struct{}{}
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
	normalized := normalizeHeaderForMatch(norm)
	if _, ok := defaultRedactedHeaders[normalized]; ok {
		return true
	}
	if isSensitiveHeaderFamily(normalized) {
		return true
	}
	if isAPIKeyHeader(normalized) {
		return true
	}
	_, ok := r.extra[normalized]
	return ok
}

// RedactPayloadFields returns a copy of fields with common sensitive payload keys
// replaced by redactedValue.
func RedactPayloadFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		if IsSensitiveFieldName(k) {
			out[k] = redactedValue
			continue
		}
		out[k] = v
	}
	return out
}

// RedactPayloadFieldsDeep returns a copy of fields with sensitive payload keys
// replaced by redactedValue. Nested maps and slices are sanitized
// recursively.
func RedactPayloadFieldsDeep(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = redactPayloadValue(k, v)
	}
	return out
}

func redactPayloadValue(key string, value any) any {
	if IsSensitiveFieldName(key) {
		return redactedValue
	}
	switch typed := value.(type) {
	case map[string]any:
		return redactPayloadStringMap(typed)
	case map[string]string:
		return redactPayloadStringMap(typed)
	case []any:
		return redactPayloadSlice(typed)
	case []map[string]any:
		return redactPayloadSlice(typed)
	case []map[string]string:
		return redactPayloadSlice(typed)
	default:
		return value
	}
}

func redactPayloadStringMap[V any](fields map[string]V) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = redactPayloadValue(k, v)
	}
	return out
}

func redactPayloadSlice[V any](values []V) []any {
	out := make([]any, len(values))
	for i, item := range values {
		out[i] = redactPayloadValue("", item)
	}
	return out
}

// IsSensitiveFieldName reports whether the field name should be treated as
// sensitive for request log payloads.
func IsSensitiveFieldName(name string) bool {
	norm := normalizePayloadFieldName(name)
	if norm == "" {
		return false
	}
	if _, ok := defaultRedactedPayloadFields[norm]; ok {
		return true
	}
	if isSensitivePayloadAPIKey(norm) {
		return true
	}
	if hasSensitivePayloadBoundary(norm, "token") ||
		hasSensitivePayloadBoundary(norm, "secret") ||
		hasSensitivePayloadBoundary(norm, "password") ||
		hasSensitivePayloadBoundary(norm, "auth") ||
		hasSensitivePayloadBoundary(norm, "session") {
		return true
	}
	return false
}

func normalizeHeaderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeHeaderForMatch(name string) string {
	return strings.ReplaceAll(normalizeHeaderName(name), "_", "-")
}

func isSensitiveHeaderFamily(name string) bool {
	if strings.HasPrefix(name, "x-") {
		return strings.Contains(name, "token") ||
			strings.Contains(name, "secret") ||
			strings.Contains(name, "password") ||
			strings.Contains(name, "session") ||
			strings.Contains(name, "auth")
	}
	if strings.HasPrefix(name, "authorization") || strings.Contains(name, "-authorization-") {
		return true
	}
	return false
}

func isAPIKeyHeader(name string) bool {
	return strings.Contains(name, "api-key") ||
		strings.Contains(name, "api_key") ||
		strings.Contains(name, "apikey") ||
		strings.Contains(name, "api-token")
}

func isSensitivePayloadAPIKey(name string) bool {
	return strings.Contains(name, "api_key") ||
		strings.Contains(name, "apikey") ||
		strings.Contains(name, "api_token")
}

func normalizePayloadFieldName(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(name, "-", "_")))
}

func hasSensitivePayloadBoundary(name, segment string) bool {
	if segment == "" {
		return false
	}
	if name == segment {
		return true
	}
	if strings.HasPrefix(name, segment+"_") ||
		strings.HasSuffix(name, "_"+segment) {
		return true
	}
	return strings.Contains(name, "_"+segment+"_")
}
