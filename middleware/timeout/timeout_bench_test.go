package timeout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkPropagatorSuccess(b *testing.B) {
	mw, err := NewPropagator(Options{Timeout: time.Second})
	if err != nil {
		b.Fatalf("new propagator: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	}
}

func BenchmarkHardTimeoutSuccess(b *testing.B) {
	mw, err := NewHard(Options{Timeout: time.Second, MaxCaptureBytes: 4096})
	if err != nil {
		b.Fatalf("new hard timeout: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	}
}
