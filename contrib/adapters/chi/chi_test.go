package chi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestNewRoutesAndExtractors(t *testing.T) {
	router := New()
	extractor := NewURLParamExtractor()

	router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got := extractor.URLParam(r, "id"); got != "42" {
			t.Fatalf("extractor URLParam() = %q, want 42", got)
		}
		if got := URLParam(r, "id"); got != "42" {
			t.Fatalf("package URLParam() = %q, want 42", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/users/42", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestNewMiddlewareAppliesRequestIDAndRealIP(t *testing.T) {
	mw := NewMiddleware()
	var (
		seenRequestID string
		seenRemoteIP  string
	)

	handler := mw.RequestID()(mw.RealIP()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRequestID = chimiddleware.GetReqID(r.Context())
		seenRemoteIP = r.RemoteAddr
		w.WriteHeader(http.StatusNoContent)
	})))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.50:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if seenRequestID == "" {
		t.Fatal("expected request ID to be populated")
	}
	if seenRemoteIP != "203.0.113.10" {
		t.Fatalf("remote IP = %q, want 203.0.113.10", seenRemoteIP)
	}
}

func TestNewMiddlewareRecoverer(t *testing.T) {
	mw := NewMiddleware()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)

	mw.Recoverer()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
