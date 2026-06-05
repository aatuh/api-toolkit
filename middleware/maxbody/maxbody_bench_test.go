package maxbody

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkMaxBodyWithinLimit(b *testing.B) {
	mw, err := New(Options{MaxBytes: 1 << 20})
	if err != nil {
		b.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	body := bytes.Repeat([]byte("a"), 4096)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	}
}
