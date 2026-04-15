package jsonmw

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNilMiddlewareHandler(t *testing.T) {
	var mw *Middleware
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandlerRejectsInvalidJSONContentType(t *testing.T) {
	mw, err := New(Options{RequireJSON: true})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "text/application/json")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
}

func TestHandlerAcceptsValidJSONContentTypes(t *testing.T) {
	mw, err := New(Options{RequireJSON: true})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []string{
		"application/json; charset=utf-8",
		"application/problem+json",
	}
	for _, contentType := range tests {
		t.Run(contentType, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Content-Type", contentType)
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected 204, got %d", rec.Code)
			}
		})
	}
}
