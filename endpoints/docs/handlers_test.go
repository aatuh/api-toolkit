package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHandlerDefaultsNilManager(t *testing.T) {
	handler := NewHandler(nil)
	if handler == nil {
		t.Fatal("expected handler")
	}
	if handler.manager == nil {
		t.Fatal("expected default docs manager")
	}
}

func TestOpenAPIHandlerWithNilManagerUsesDefaultManager(t *testing.T) {
	handler := NewHandler(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	handler.OpenAPIHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
}

func TestMiddlewareWithNilManagerUsesDefaultInfo(t *testing.T) {
	handler := NewHandler(nil)
	middleware := handler.Middleware()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-API-Title"); got == "" {
		t.Fatal("expected X-API-Title header")
	}
	if got := rec.Header().Get("X-API-Version"); got == "" {
		t.Fatal("expected X-API-Version header")
	}
}
