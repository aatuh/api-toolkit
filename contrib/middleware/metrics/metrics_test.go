package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

type captureRecorder struct {
	counterName string
	counter     Labels
	histName    string
	histValue   float64
	hist        Labels
}

func (r *captureRecorder) IncCounter(name string, labels Labels) {
	r.counterName = name
	r.counter = cloneLabels(labels)
}

func (r *captureRecorder) ObserveHistogram(name string, value float64, labels Labels) {
	r.histName = name
	r.histValue = value
	r.hist = cloneLabels(labels)
}

func TestNewDefaults(t *testing.T) {
	mw, err := New(Options{})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw.Clock == nil {
		t.Fatal("expected default clock")
	}
	if _, ok := mw.M.(NoopMetrics); !ok {
		t.Fatalf("expected NoopMetrics, got %T", mw.M)
	}
}

func TestNewPrometheusRecorderCheckedReusesRegisteredCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()

	first, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}
	second, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	if first.requests != second.requests {
		t.Fatal("expected duplicate counter registration to reuse existing collector")
	}
	if first.durations != second.durations {
		t.Fatal("expected duplicate histogram registration to reuse existing collector")
	}

	first.IncCounter("", Labels{
		"method": "GET",
		"route":  "/widgets",
		"status": "200",
	})
	second.ObserveHistogram("", 0.25, Labels{
		"method": "GET",
		"route":  "/widgets",
		"status": "200",
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(families) != 2 {
		t.Fatalf("expected 2 metric families, got %d", len(families))
	}
}

func TestNewPrometheusRecorderCheckedReusesRegisteredCollectorsWithDefaultRegisterer(t *testing.T) {
	reg := withDefaultPrometheusRegistry(t)

	first, err := NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}
	second, err := NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	if first.requests != second.requests {
		t.Fatal("expected duplicate counter registration to reuse existing collector")
	}
	if first.durations != second.durations {
		t.Fatal("expected duplicate histogram registration to reuse existing collector")
	}
	first.IncCounter("", Labels{
		"method": "GET",
		"route":  "/widgets",
		"status": "200",
	})
	second.ObserveHistogram("", 0.25, Labels{
		"method": "GET",
		"route":  "/widgets",
		"status": "200",
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(families) != 2 {
		t.Fatalf("expected 2 metric families, got %d", len(families))
	}
}

func TestNewPrometheusRecorderCheckedReturnsConflictErrorWithCustomRegisterer(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "route", "status"}))

	recorder, err := NewPrometheusRecorderChecked(reg, nil)
	if err == nil {
		t.Fatal("expected registration conflict error")
	}
	if recorder != nil {
		t.Fatal("expected recorder to be nil on registration conflict")
	}
	if !errors.Is(err, ErrIncompatibleCollectorRegistration) {
		t.Fatalf("expected ErrIncompatibleCollectorRegistration, got %v", err)
	}
	if !strings.Contains(err.Error(), "http_requests_total") {
		t.Fatalf("expected metric name in error, got %q", err)
	}
}

func TestNewPrometheusRecorderCheckedReturnsConflictErrorWithDefaultRegisterer(t *testing.T) {
	withDefaultPrometheusRegistry(t)
	prometheus.MustRegister(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_request_duration_seconds",
		Help: "HTTP request duration in seconds",
	}, []string{"method", "route", "status"}))

	recorder, err := NewPrometheusRecorderChecked(nil, nil)
	if err == nil {
		t.Fatal("expected registration conflict error")
	}
	if recorder != nil {
		t.Fatal("expected recorder to be nil on registration conflict")
	}
	if !errors.Is(err, ErrIncompatibleCollectorRegistration) {
		t.Fatalf("expected ErrIncompatibleCollectorRegistration, got %v", err)
	}
	if !strings.Contains(err.Error(), "http_request_duration_seconds") {
		t.Fatalf("expected metric name in error, got %q", err)
	}
}

func TestSanitizeHTTPLabelsBoundsMethodAndStatus(t *testing.T) {
	method, route, status := sanitizeHTTPLabels(Labels{
		"method": "post",
		"route":  " /widgets/{id} ",
		"status": "201",
	})
	if method != "POST" || route != "/widgets/{id}" || status != "201" {
		t.Fatalf("labels = %q %q %q, want POST /widgets/{id} 201", method, route, status)
	}

	method, route, status = sanitizeHTTPLabels(Labels{
		"method": "BREW",
		"status": "599x",
	})
	if method != "OTHER" || route != "unknown" || status != "0" {
		t.Fatalf("labels = %q %q %q, want OTHER unknown 0", method, route, status)
	}
}

func TestHandlerRecordsRecoveredPanicAs500BeforeCommit(t *testing.T) {
	recorder := &captureRecorder{}
	mw, err := New(Options{Recorder: recorder})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic", nil)
	defer func() {
		if got := recover(); got != "boom" {
			t.Fatalf("expected panic boom, got %v", got)
		}
		if recorder.counterName != "http_requests_total" {
			t.Fatalf("counter name = %q", recorder.counterName)
		}
		if recorder.counter["status"] != "500" {
			t.Fatalf("counter status = %q", recorder.counter["status"])
		}
		if recorder.histName != "http_request_duration_seconds" {
			t.Fatalf("histogram name = %q", recorder.histName)
		}
		if recorder.hist["status"] != "500" {
			t.Fatalf("histogram status = %q", recorder.hist["status"])
		}
	}()
	handler.ServeHTTP(rec, req)
}

func cloneLabels(labels Labels) Labels {
	out := make(Labels, len(labels))
	for key, value := range labels {
		out[key] = value
	}
	return out
}

func withDefaultPrometheusRegistry(t *testing.T) *prometheus.Registry {
	t.Helper()

	reg := prometheus.NewRegistry()
	prevRegisterer := prometheus.DefaultRegisterer
	prevGatherer := prometheus.DefaultGatherer
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = prevRegisterer
		prometheus.DefaultGatherer = prevGatherer
	})
	return reg
}
