package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery"
	idempotencymw "github.com/aatuh/api-toolkit/v3/middleware/idempotency"
	timeoutmw "github.com/aatuh/api-toolkit/v3/middleware/timeout"
	"github.com/aatuh/api-toolkit/v3/ports"
	"github.com/aatuh/api-toolkit/v3/routepolicy"
)

// Labels is a simple key:value map for metric dimensions.
type Labels map[string]string

// MetricsRecorder captures counters and histograms.
//
//revive:disable-next-line:exported
type MetricsRecorder interface {
	IncCounter(name string, labels Labels)
	ObserveHistogram(name string, value float64, labels Labels)
}

// HealthMetricsRecorder captures health-state transition metrics.
//
// Implementations must keep labels bounded and must not include dependency
// names, error strings, request data, tenant identifiers, or health messages.
type HealthMetricsRecorder interface {
	RecordHealthStatusChange(from, to ports.HealthStatus, result ports.DetailedHealthResponse)
}

// IdempotencyOutcomeMetricsRecorder captures bounded idempotency outcome metrics.
type IdempotencyOutcomeMetricsRecorder interface {
	RecordIdempotencyOutcome(event idempotencymw.OutcomeEvent)
}

// HardTimeoutEventMetricsRecorder captures bounded hard-timeout event metrics.
type HardTimeoutEventMetricsRecorder interface {
	RecordHardTimeoutEvent(event timeoutmw.HardTimeoutEvent)
}

// RoutePolicyMetricsRecorder captures bounded per-request route policy labels.
type RoutePolicyMetricsRecorder interface {
	RecordRoutePolicy(labels Labels)
}

// WebhookDeliveryMetricsRecorder captures bounded outbound webhook delivery outcomes.
type WebhookDeliveryMetricsRecorder interface {
	RecordWebhookDelivery(event webhookdelivery.DeliveryObservation)
}

// PrometheusHandler returns a standard /metrics http.Handler if the
// Prometheus client is linked; otherwise returns http.NotFoundHandler.
// This indirection avoids hard dependency on the Prometheus client.
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

// NoopMetrics is the default. Swap later for Prometheus, etc.
type NoopMetrics struct{}

// IncCounter is a no-op implementation.
func (NoopMetrics) IncCounter(_ string, _ Labels) {}

// ObserveHistogram is a no-op implementation.
func (NoopMetrics) ObserveHistogram(_ string, _ float64, _ Labels) {}

// RecordHealthStatusChange is a no-op implementation.
func (NoopMetrics) RecordHealthStatusChange(_, _ ports.HealthStatus, _ ports.DetailedHealthResponse) {
}

// RecordIdempotencyOutcome is a no-op implementation.
func (NoopMetrics) RecordIdempotencyOutcome(idempotencymw.OutcomeEvent) {}

// RecordHardTimeoutEvent is a no-op implementation.
func (NoopMetrics) RecordHardTimeoutEvent(timeoutmw.HardTimeoutEvent) {}

// RecordRoutePolicy is a no-op implementation.
func (NoopMetrics) RecordRoutePolicy(Labels) {}

// RecordWebhookDelivery is a no-op implementation.
func (NoopMetrics) RecordWebhookDelivery(webhookdelivery.DeliveryObservation) {}

// ObserveWebhookDelivery is a no-op implementation.
func (NoopMetrics) ObserveWebhookDelivery(context.Context, webhookdelivery.DeliveryObservation) {}

// Middleware instruments HTTP traffic using a provided recorder.
type Middleware struct {
	M     MetricsRecorder
	Clock ports.Clock
}

// Options configures the metrics middleware.
type Options struct {
	Recorder MetricsRecorder
	Clock    ports.Clock
}

// New constructs a metrics middleware.
func New(opts Options) (*Middleware, error) {
	if opts.Clock == nil {
		opts.Clock = ports.SystemClock{}
	}
	if opts.Recorder == nil {
		opts.Recorder = NoopMetrics{}
	}
	return &Middleware{M: opts.Recorder, Clock: opts.Clock}, nil
}

// HandlerFunc exposes middleware as a plain function for router use.
func (mw *Middleware) HandlerFunc() func(http.Handler) http.Handler {
	if mw == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return mw.Handler
}

// Middleware implements ports.Middleware by returning Handler.
func (mw *Middleware) Middleware() func(http.Handler) http.Handler {
	if mw == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return mw.Handler
}

// PrometheusRecorder implements MetricsRecorder using Prometheus client.
// This is a minimal adapter; applications can supply their own recorder.
type PrometheusRecorder struct {
	requests            *prometheus.CounterVec
	durations           *prometheus.HistogramVec
	healthStatusChanges *prometheus.CounterVec
	idempotencyOutcomes *prometheus.CounterVec
	hardTimeoutEvents   *prometheus.CounterVec
	routePolicyRequests *prometheus.CounterVec
	webhookDeliveries   *prometheus.CounterVec
}

// ErrIncompatibleCollectorRegistration reports that a Prometheus metric name is
// already registered with an incompatible collector type or descriptor shape.
var ErrIncompatibleCollectorRegistration = errors.New("metrics: incompatible collector already registered")

// NewPrometheusRecorder wires counters and histograms with standard names.
// Consumers may pass a custom registerer (e.g. for testing). When nil, the
// default Prometheus registerer is used.
func NewPrometheusRecorder(registerer prometheus.Registerer, buckets []float64) *PrometheusRecorder {
	recorder, err := NewPrometheusRecorderChecked(registerer, buckets)
	if err != nil {
		panic(err)
	}
	return recorder
}

// NewPrometheusRecorderChecked wires counters and histograms with standard names
// and returns registration conflicts instead of panicking.
func NewPrometheusRecorderChecked(registerer prometheus.Registerer, buckets []float64) (*PrometheusRecorder, error) {
	reg := registerer
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	if len(buckets) == 0 {
		buckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	}
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "route", "status"})
	durations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: buckets,
	}, []string{"method", "route", "status"})
	healthStatusChanges := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "health_status_changes_total",
		Help: "Total number of health status transitions",
	}, []string{"from", "to"})
	idempotencyOutcomes := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "idempotency_outcomes_total",
		Help: "Total number of idempotency decisions by bounded outcome labels",
	}, []string{"method", "store_class", "outcome", "status_class", "fail_open"})
	hardTimeoutEvents := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_hard_timeout_events_total",
		Help: "Total number of hard-timeout outcomes by bounded event labels",
	}, []string{"method", "outcome", "status_class", "timed_out", "panicked", "capture_overflow"})
	routePolicyRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_route_policy_requests_total",
		Help: "Total number of HTTP requests by bounded route policy metadata",
	}, []string{"method", "route", "status_class", "auth", "tenant", "idempotency", "admin", "deprecated", "rate_limit"})
	webhookDeliveries := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_delivery_events_total",
		Help: "Total number of outbound webhook delivery outcomes by bounded labels",
	}, []string{"event_type", "outcome", "status_class"})
	registeredRequests, err := registerOrReuseCounterVec(reg, requests)
	if err != nil {
		return nil, fmt.Errorf("register http_requests_total: %w", err)
	}
	registeredDurations, err := registerOrReuseHistogramVec(reg, durations)
	if err != nil {
		return nil, fmt.Errorf("register http_request_duration_seconds: %w", err)
	}
	registeredHealthStatusChanges, err := registerOrReuseCounterVec(reg, healthStatusChanges)
	if err != nil {
		return nil, fmt.Errorf("register health_status_changes_total: %w", err)
	}
	registeredIdempotencyOutcomes, err := registerOrReuseCounterVec(reg, idempotencyOutcomes)
	if err != nil {
		return nil, fmt.Errorf("register idempotency_outcomes_total: %w", err)
	}
	registeredHardTimeoutEvents, err := registerOrReuseCounterVec(reg, hardTimeoutEvents)
	if err != nil {
		return nil, fmt.Errorf("register http_hard_timeout_events_total: %w", err)
	}
	registeredRoutePolicyRequests, err := registerOrReuseCounterVec(reg, routePolicyRequests)
	if err != nil {
		return nil, fmt.Errorf("register http_route_policy_requests_total: %w", err)
	}
	registeredWebhookDeliveries, err := registerOrReuseCounterVec(reg, webhookDeliveries)
	if err != nil {
		return nil, fmt.Errorf("register webhook_delivery_events_total: %w", err)
	}
	return &PrometheusRecorder{
		requests:            registeredRequests,
		durations:           registeredDurations,
		healthStatusChanges: registeredHealthStatusChanges,
		idempotencyOutcomes: registeredIdempotencyOutcomes,
		hardTimeoutEvents:   registeredHardTimeoutEvents,
		routePolicyRequests: registeredRoutePolicyRequests,
		webhookDeliveries:   registeredWebhookDeliveries,
	}, nil
}

func registerOrReuseCounterVec(reg prometheus.Registerer, collector *prometheus.CounterVec) (*prometheus.CounterVec, error) {
	if collector == nil || reg == nil {
		return collector, nil
	}
	if err := reg.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			if existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing, nil
			}
			return nil, fmt.Errorf("%w: expected *prometheus.CounterVec, got %T", ErrIncompatibleCollectorRegistration, alreadyRegistered.ExistingCollector)
		}
		return nil, err
	}
	return collector, nil
}

func registerOrReuseHistogramVec(reg prometheus.Registerer, collector *prometheus.HistogramVec) (*prometheus.HistogramVec, error) {
	if collector == nil || reg == nil {
		return collector, nil
	}
	if err := reg.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			if existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.HistogramVec); ok {
				return existing, nil
			}
			return nil, fmt.Errorf("%w: expected *prometheus.HistogramVec, got %T", ErrIncompatibleCollectorRegistration, alreadyRegistered.ExistingCollector)
		}
		return nil, err
	}
	return collector, nil
}

// IncCounter increments the Prometheus counter.
func (p *PrometheusRecorder) IncCounter(_ string, labels Labels) {
	if p == nil || p.requests == nil {
		return
	}
	method, route, status := sanitizeHTTPLabels(labels)
	p.requests.WithLabelValues(method, route, status).Inc()
}

// ObserveHistogram records the Prometheus histogram observation.
func (p *PrometheusRecorder) ObserveHistogram(_ string, value float64, labels Labels) {
	if p == nil || p.durations == nil {
		return
	}
	method, route, status := sanitizeHTTPLabels(labels)
	p.durations.WithLabelValues(method, route, status).Observe(value)
}

// RecordHealthStatusChange records a bounded health-state transition counter.
func (p *PrometheusRecorder) RecordHealthStatusChange(from, to ports.HealthStatus, _ ports.DetailedHealthResponse) {
	if p == nil || p.healthStatusChanges == nil {
		return
	}
	labels := HealthStatusChangeLabels(from, to)
	p.healthStatusChanges.WithLabelValues(labels["from"], labels["to"]).Inc()
}

// RecordIdempotencyOutcome records one bounded idempotency decision.
func (p *PrometheusRecorder) RecordIdempotencyOutcome(event idempotencymw.OutcomeEvent) {
	if p == nil || p.idempotencyOutcomes == nil {
		return
	}
	labels := IdempotencyOutcomeLabels(event)
	p.idempotencyOutcomes.WithLabelValues(
		labels["method"],
		labels["store_class"],
		labels["outcome"],
		labels["status_class"],
		labels["fail_open"],
	).Inc()
}

// RecordHardTimeoutEvent records one bounded hard-timeout event.
func (p *PrometheusRecorder) RecordHardTimeoutEvent(event timeoutmw.HardTimeoutEvent) {
	if p == nil || p.hardTimeoutEvents == nil {
		return
	}
	labels := HardTimeoutEventLabels(event)
	p.hardTimeoutEvents.WithLabelValues(
		labels["method"],
		labels["outcome"],
		labels["status_class"],
		labels["timed_out"],
		labels["panicked"],
		labels["capture_overflow"],
	).Inc()
}

// RecordRoutePolicy records bounded route policy labels for one request.
func (p *PrometheusRecorder) RecordRoutePolicy(labels Labels) {
	if p == nil || p.routePolicyRequests == nil {
		return
	}
	bounded := RoutePolicyLabels(labels)
	p.routePolicyRequests.WithLabelValues(
		bounded["method"],
		bounded["route"],
		bounded["status_class"],
		bounded["auth"],
		bounded["tenant"],
		bounded["idempotency"],
		bounded["admin"],
		bounded["deprecated"],
		bounded["rate_limit"],
	).Inc()
}

// RecordWebhookDelivery records one bounded outbound webhook delivery outcome.
func (p *PrometheusRecorder) RecordWebhookDelivery(event webhookdelivery.DeliveryObservation) {
	if p == nil || p.webhookDeliveries == nil {
		return
	}
	labels := WebhookDeliveryLabels(event)
	p.webhookDeliveries.WithLabelValues(
		labels["event_type"],
		labels["outcome"],
		labels["status_class"],
	).Inc()
}

// ObserveWebhookDelivery implements webhookdelivery.MetricsRecorder.
func (p *PrometheusRecorder) ObserveWebhookDelivery(ctx context.Context, event webhookdelivery.DeliveryObservation) {
	_ = ctx
	p.RecordWebhookDelivery(event)
}

// HealthStatusChangeHook converts a health scheduler status-change callback
// into a metrics recorder call.
func HealthStatusChangeHook(recorder HealthMetricsRecorder) func(context.Context, ports.HealthStatus, ports.HealthStatus, ports.DetailedHealthResponse) {
	return func(_ context.Context, from, to ports.HealthStatus, result ports.DetailedHealthResponse) {
		if recorder == nil {
			return
		}
		recorder.RecordHealthStatusChange(from, to, result)
	}
}

// IdempotencyOutcomeHook converts an idempotency middleware callback into a
// metrics recorder call.
func IdempotencyOutcomeHook(recorder IdempotencyOutcomeMetricsRecorder) idempotencymw.OutcomeHandler {
	return func(_ context.Context, event idempotencymw.OutcomeEvent) {
		if recorder == nil {
			return
		}
		recorder.RecordIdempotencyOutcome(event)
	}
}

// HardTimeoutEventHook converts a hard-timeout middleware callback into a
// metrics recorder call.
func HardTimeoutEventHook(recorder HardTimeoutEventMetricsRecorder) func(timeoutmw.HardTimeoutEvent) {
	return func(event timeoutmw.HardTimeoutEvent) {
		if recorder == nil {
			return
		}
		recorder.RecordHardTimeoutEvent(event)
	}
}

// WebhookDeliveryHook converts webhook delivery observations into a metrics
// recorder call.
func WebhookDeliveryHook(recorder WebhookDeliveryMetricsRecorder) webhookdelivery.MetricsRecorder {
	if recorder == nil {
		return nil
	}
	return webhookDeliveryHook{recorder: recorder}
}

type webhookDeliveryHook struct {
	recorder WebhookDeliveryMetricsRecorder
}

func (h webhookDeliveryHook) ObserveWebhookDelivery(_ context.Context, event webhookdelivery.DeliveryObservation) {
	if h.recorder != nil {
		h.recorder.RecordWebhookDelivery(event)
	}
}

// HealthStatusChangeLabels returns the bounded label set used for health status
// transition metrics.
func HealthStatusChangeLabels(from, to ports.HealthStatus) Labels {
	return Labels{
		"from": sanitizeHealthStatus(from),
		"to":   sanitizeHealthStatus(to),
	}
}

// IdempotencyOutcomeLabels returns the bounded label set used for idempotency
// outcome metrics.
func IdempotencyOutcomeLabels(event idempotencymw.OutcomeEvent) Labels {
	labels := Labels(event.MetricLabels())
	if event.FailOpen {
		labels["fail_open"] = "true"
	} else {
		labels["fail_open"] = "false"
	}
	return labels
}

// HardTimeoutEventLabels returns the bounded label set used for hard-timeout
// metrics and logs. It intentionally excludes paths, panic payloads, headers,
// response bodies, tenants, and request IDs.
func HardTimeoutEventLabels(event timeoutmw.HardTimeoutEvent) Labels {
	return Labels{
		"method":           canonicalHTTPMethodWithFallback(event.Method),
		"outcome":          sanitizeHardTimeoutOutcome(event.Outcome),
		"status_class":     statusClass(canonicalHTTPStatus(itoa(event.Status))),
		"timed_out":        boolLabel(event.TimedOut),
		"panicked":         boolLabel(event.Panicked),
		"capture_overflow": boolLabel(event.CaptureOverflow),
	}
}

// WebhookDeliveryLabels returns the bounded label set used for outbound
// webhook delivery metrics and logs. It intentionally excludes tenant IDs,
// endpoint IDs, delivery IDs, URLs, payloads, secrets, and raw error strings.
func WebhookDeliveryLabels(event webhookdelivery.DeliveryObservation) Labels {
	return Labels{
		"event_type":   webhookdelivery.SafeLabel(event.EventType),
		"outcome":      webhookdelivery.SafeLabel(event.Outcome),
		"status_class": sanitizeWebhookStatusClass(event.StatusClass),
	}
}

// RoutePolicyLabels returns the bounded label set used for route-policy
// request metrics. The route label should be a route pattern, not a raw request
// path; blank routes are normalized to "unknown".
func RoutePolicyLabels(labels Labels) Labels {
	method, route, status := sanitizeHTTPLabels(labels)
	statusClassLabel := sanitizeStatusClass(labels["status_class"])
	if statusClassLabel == "none" {
		statusClassLabel = statusClass(status)
	}
	return Labels{
		"method":       method,
		"route":        route,
		"status_class": statusClassLabel,
		"auth":         sanitizePolicyLabel(labels["auth"], "none"),
		"tenant":       sanitizePolicyLabel(labels["tenant"], "none"),
		"idempotency":  sanitizePolicyLabel(labels["idempotency"], "none"),
		"admin":        sanitizePolicyLabel(labels["admin"], "none"),
		"deprecated":   sanitizePolicyLabel(labels["deprecated"], "false"),
		"rate_limit":   sanitizePolicyLabel(labels["rate_limit"], "none"),
	}
}

func sanitizeHealthStatus(status ports.HealthStatus) string {
	switch status {
	case ports.HealthStatusHealthy:
		return string(ports.HealthStatusHealthy)
	case ports.HealthStatusUnhealthy:
		return string(ports.HealthStatusUnhealthy)
	case ports.HealthStatusDegraded:
		return string(ports.HealthStatusDegraded)
	case ports.HealthStatusUnknown, "":
		return string(ports.HealthStatusUnknown)
	default:
		return "other"
	}
}

func canonicalHTTPMethodWithFallback(method string) string {
	if method := canonicalHTTPMethod(method); method != "" {
		return method
	}
	return "UNKNOWN"
}

func sanitizeHardTimeoutOutcome(outcome timeoutmw.HardTimeoutOutcome) string {
	switch outcome {
	case timeoutmw.HardTimeoutOutcomeTimeout,
		timeoutmw.HardTimeoutOutcomePanic,
		timeoutmw.HardTimeoutOutcomeCaptureOverflow:
		return string(outcome)
	default:
		return "unknown"
	}
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// Handler wraps the next handler to record counters and duration.
// It is intended to emit exactly one observation on both normal and panic
// paths. For panic paths, it should infer the final visible status before
// re-panicking so outer recovery can still produce the response contract.
func (mw *Middleware) Handler(next http.Handler) http.Handler {
	if mw == nil {
		return next
	}
	if mw.M == nil {
		mw.M = NoopMetrics{}
	}
	if mw.Clock == nil {
		mw.Clock = ports.SystemClock{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r != nil {
			r = r.WithContext(routepolicy.SeedObservabilityContext(r.Context()))
		}
		start := mw.Clock.Now()
		ww := wrapResponseWriter(w)
		defer func() {
			rec := recover()
			status := ww.Status()
			if rec != nil && !ww.Committed() {
				status = http.StatusInternalServerError
			}
			mw.observeRequest(r, start, status)
			if rec != nil {
				panic(rec)
			}
		}()
		next.ServeHTTP(ww, r)
	})
}

func (mw *Middleware) observeRequest(r *http.Request, start time.Time, status int) {
	route := routePattern(r)
	if route == "" {
		route = "unknown"
	}
	labels := Labels{
		"method": r.Method,
		"route":  route,
		"status": itoa(status),
	}
	mw.M.IncCounter("http_requests_total", labels)
	mw.M.ObserveHistogram(
		"http_request_duration_seconds",
		mw.Clock.Now().Sub(start).Seconds(),
		labels,
	)
	if recorder, ok := mw.M.(RoutePolicyMetricsRecorder); ok {
		if policyLabels, ok := routepolicy.ObservabilityLabelsFromRequest(r); ok {
			routeLabels := Labels{
				"method":       r.Method,
				"route":        route,
				"status_class": statusClass(canonicalHTTPStatus(itoa(status))),
			}
			for key, value := range policyLabels.Map() {
				routeLabels[key] = value
			}
			recorder.RecordRoutePolicy(routeLabels)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var a [12]byte
	i := len(a)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		a[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		a[i] = '-'
	}
	return string(a[i:])
}

func sanitizeHTTPLabels(labels Labels) (method, route, status string) {
	method = canonicalHTTPMethod(labels["method"])
	if method == "" {
		method = "UNKNOWN"
	}
	route = strings.TrimSpace(labels["route"])
	if route == "" {
		route = "unknown"
	}
	status = canonicalHTTPStatus(labels["status"])
	if status == "" {
		status = "0"
	}
	return method, route, status
}

func canonicalHTTPMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return ""
	}
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
	default:
		return "OTHER"
	}
}

func canonicalHTTPStatus(status string) string {
	status = strings.TrimSpace(status)
	if len(status) != 3 {
		return ""
	}
	code := 0
	for _, ch := range status {
		if ch < '0' || ch > '9' {
			return ""
		}
		code = code*10 + int(ch-'0')
	}
	if code < 100 || code > 599 {
		return ""
	}
	return status
}

func statusClass(status string) string {
	if len(status) != 3 {
		return "none"
	}
	switch status[0] {
	case '1', '2', '3', '4', '5':
		return string([]byte{status[0], 'x', 'x'})
	default:
		return "none"
	}
}

func sanitizeStatusClass(value string) string {
	switch strings.TrimSpace(value) {
	case "1xx", "2xx", "3xx", "4xx", "5xx":
		return strings.TrimSpace(value)
	default:
		return "none"
	}
}

func sanitizeWebhookStatusClass(value string) string {
	if statusClass := sanitizeStatusClass(value); statusClass != "none" {
		return statusClass
	}
	switch webhookdelivery.SafeLabel(value) {
	case "transport":
		return "transport"
	default:
		return "unknown"
	}
}

func sanitizePolicyLabel(value, fallback string) string {
	switch value {
	case "required", "configured", "none", "true", "false":
		return value
	default:
		return fallback
	}
}

func routePattern(r *http.Request) string {
	if r == nil {
		return ""
	}
	ctx := chi.RouteContext(r.Context())
	if ctx != nil {
		if pattern := ctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return strings.TrimSpace(r.Pattern)
}
