package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

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

func TestNewPrometheusRecorderReusesRegisteredCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()

	first := NewPrometheusRecorder(reg, nil)
	second := NewPrometheusRecorder(reg, nil)

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
