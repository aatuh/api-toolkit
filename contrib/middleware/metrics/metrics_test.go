package metrics

import "testing"

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
