package docs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/ports"
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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs/openapi.json", nil)
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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
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

func TestHTMLHandlerUsesStrictCSPForStaticMode(t *testing.T) {
	handler := NewHandler(NewWithConfig(ports.DocsConfig{
		Title:       "Strict Docs",
		Description: "First-party docs",
		Version:     "1.2.3",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  true,
		EnableJSON:  true,
		HTMLMode:    ports.DocsHTMLModeStatic,
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs", nil)
	handler.HTMLHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != strictDocsCSP {
		t.Fatalf("unexpected CSP: %q", got)
	}
	body := rec.Body.String()
	if strings.Contains(body, "cdn.jsdelivr.net") {
		t.Fatalf("expected no external CDN assets, got %q", body)
	}
	if strings.Contains(body, "<script") {
		t.Fatalf("expected no script tags, got %q", body)
	}
}
