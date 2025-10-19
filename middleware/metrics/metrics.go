package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Labels is a simple key:value map for metric dimensions.
type Labels map[string]string

// MetricsRecorder captures counters and histograms.
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

func (NoopMetrics) IncCounter(_ string, _ Labels)                  {}
func (NoopMetrics) ObserveHistogram(_ string, _ float64, _ Labels) {}

// Middleware instruments HTTP traffic using a provided recorder.
type Middleware struct {
	M MetricsRecorder
}

// New constructs a metrics middleware.
func New(m MetricsRecorder) *Middleware { return &Middleware{M: m} }

// HandlerFunc exposes middleware as a plain function for router use.
func (mw *Middleware) HandlerFunc() func(http.Handler) http.Handler {
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
	paused := promauto.With(reg)
	return &PrometheusRecorder{
		requests: paused.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "route", "status"}),
		durations: paused.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: buckets,
		}, []string{"method", "route", "status"}),
	}
}

func (p *PrometheusRecorder) IncCounter(_ string, labels Labels) {
	if p == nil || p.requests == nil {
		return
	}
	method, route, status := sanitizeHTTPLabels(labels)
	p.requests.WithLabelValues(method, route, status).Inc()
}

func (p *PrometheusRecorder) ObserveHistogram(_ string, value float64, labels Labels) {
	if p == nil || p.durations == nil {
		return
	}
	method, route, status := sanitizeHTTPLabels(labels)
	p.durations.WithLabelValues(method, route, status).Observe(value)
}

// Handler wraps the next handler to record counters and duration.
func (mw *Middleware) Handler(next http.Handler) http.Handler {
	if mw.M == nil {
		mw.M = NoopMetrics{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &respWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r)

		labels := Labels{
			"method": r.Method,
			"route":  r.URL.Path,
			"path":   r.URL.Path,
			"status": itoa(ww.status),
		}
		mw.M.IncCounter("http_requests_total", labels)
		mw.M.ObserveHistogram(
			"http_request_duration_seconds",
			time.Since(start).Seconds(),
			labels,
		)
	})
}

type respWriter struct {
	http.ResponseWriter
	status int
}

func (w *respWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
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
		route = labels["path"]
	}
	if route == "" {
		route = "unknown"
	}
	status = labels["status"]
	if status == "" {
		status = "0"
	}
	return method, route, status
}
