package jsonmw

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
	req.Header.Set("Content-Type", "text/application/json")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected problem content type, got %q", got)
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

func TestHandlerSkipsJSONEnforcementForBodylessRequests(t *testing.T) {
	mw, err := New(Options{RequireJSON: true})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name   string
		method string
	}{
		{name: "get", method: http.MethodGet},
		{name: "delete", method: http.MethodDelete},
		{name: "post without body", method: http.MethodPost},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set("Content-Type", "text/plain")
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected 204, got %d", rec.Code)
			}
		})
	}
}
