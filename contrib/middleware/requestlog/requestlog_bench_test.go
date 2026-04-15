package requestlog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkRequestLog(b *testing.B) {
	mw, err := New(nil, WithRoutePattern(func(*http.Request) string { return "/bench" }))
	if err != nil {
		b.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/bench", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	req.Header.Set("User-Agent", "bench")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkRequestLogWithHeaders(b *testing.B) {
	mw, err := New(nil,
		WithRoutePattern(func(*http.Request) string { return "/bench" }),
		WithRequestHeaders(),
		WithResponseHeaders(),
	)
	if err != nil {
		b.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "sid=secret")
		w.Header().Set("X-Resp", "ok")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/bench", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	req.Header.Set("User-Agent", "bench")
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Cookie", "a=b")
	req.Header.Set("X-Api-Key", "abc123")
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-Extra", "ok")
	req.Header.Add("X-Multi", "a")
	req.Header.Add("X-Multi", "b")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
