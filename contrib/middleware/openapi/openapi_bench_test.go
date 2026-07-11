package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

func BenchmarkOpenAPIRequestValidation(b *testing.B) {
	spec := buildPingSpec()
	mw, err := New(spec)
	if err != nil {
		b.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkOpenAPIResponseValidation(b *testing.B) {
	spec := buildPingSpec()
	mw, err := New(spec, WithResponseValidation(ResponseValidationOptions{
		Enabled:      true,
		MaxBodyBytes: 1 << 20,
	}))
	if err != nil {
		b.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
