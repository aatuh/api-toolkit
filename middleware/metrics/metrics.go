package metrics

import (
	"net/http"
	"time"
)

// Labels is a simple key:value map for metric dimensions.
type Labels map[string]string

// MetricsRecorder captures counters and histograms.
type MetricsRecorder interface {
	IncCounter(name string, labels Labels)
	ObserveHistogram(name string, value float64, labels Labels)
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
			"path":   r.URL.Path,
			"status": itoa(ww.status),
		}
		mw.M.IncCounter("http_requests_total", labels)
		mw.M.ObserveHistogram(
			"http_request_duration_ms",
			float64(time.Since(start).Milliseconds()),
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
