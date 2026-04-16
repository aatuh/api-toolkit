package metrics

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/response_writer"
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
	requests  *prometheus.CounterVec
	durations *prometheus.HistogramVec
}

// NewPrometheusRecorder wires counters and histograms with standard names.
// Consumers may pass a custom registerer (e.g. for testing). When nil, the
// default Prometheus registerer is used.
func NewPrometheusRecorder(registerer prometheus.Registerer, buckets []float64) *PrometheusRecorder {
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
	return &PrometheusRecorder{
		requests:  registerOrReuseCounterVec(reg, requests),
		durations: registerOrReuseHistogramVec(reg, durations),
	}
}

func registerOrReuseCounterVec(reg prometheus.Registerer, collector *prometheus.CounterVec) *prometheus.CounterVec {
	if collector == nil || reg == nil {
		return collector
	}
	if err := reg.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			if existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing
			}
		}
	}
	return collector
}

func registerOrReuseHistogramVec(reg prometheus.Registerer, collector *prometheus.HistogramVec) *prometheus.HistogramVec {
	if collector == nil || reg == nil {
		return collector
	}
	if err := reg.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			if existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.HistogramVec); ok {
				return existing
			}
		}
	}
	return collector
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
		start := mw.Clock.Now()
		ww := response_writer.Wrap(w)
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
	route := chiRoutePattern(r)
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
	method = labels["method"]
	if method == "" {
		method = "UNKNOWN"
	}
	route = labels["route"]
	if route == "" {
		route = "unknown"
	}
	status = labels["status"]
	if status == "" {
		status = "0"
	}
	return method, route, status
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
