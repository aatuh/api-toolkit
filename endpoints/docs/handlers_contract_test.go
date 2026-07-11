package docs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	docs "github.com/aatuh/api-toolkit/v3/endpoints/docs"
)

type externalHTMLModeDocsManager struct {
	mode docs.HTMLMode
}

var _ docs.ManagerContract = (*externalHTMLModeDocsManager)(nil)
var _ docs.HTMLModeProvider = (*externalHTMLModeDocsManager)(nil)

func (*externalHTMLModeDocsManager) RegisterProvider(docs.Provider) {}

func (*externalHTMLModeDocsManager) GetHTML() (string, error) {
	return "<html><body>docs</body></html>", nil
}

func (*externalHTMLModeDocsManager) GetOpenAPI() ([]byte, error) {
	return []byte(`{"openapi":"3.0.0"}`), nil
}

func (*externalHTMLModeDocsManager) GetVersion() (string, error) {
	return "1.0.0", nil
}

func (*externalHTMLModeDocsManager) GetInfo() docs.Info {
	return docs.Info{Title: "Docs", Version: "1.0.0"}
}

func (*externalHTMLModeDocsManager) ServeHTML(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<html><body>docs</body></html>"))
}

func (*externalHTMLModeDocsManager) ServeOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"openapi":"3.0.0"}`))
}

func (*externalHTMLModeDocsManager) ServeVersion(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("1.0.0"))
}

func (*externalHTMLModeDocsManager) ServeInfo(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"title":"Docs","version":"1.0.0"}`))
}

func (m *externalHTMLModeDocsManager) HTMLMode() docs.HTMLMode {
	return m.mode
}

func TestHTMLHandlerUsesSwaggerCSPForExportedHTMLModeCapability(t *testing.T) {
	handler := docs.NewHandler(&externalHTMLModeDocsManager{mode: docs.HTMLModeSwaggerUI})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs", nil)
	handler.HTMLHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" || !strings.Contains(got, "https://cdn.jsdelivr.net") {
		t.Fatalf("expected swagger-ui CSP allowing CDN assets, got %q", got)
	}
}

func TestHTMLHandlerUsesStrictCSPForExportedHTMLModeCapability(t *testing.T) {
	handler := docs.NewHandler(&externalHTMLModeDocsManager{mode: docs.HTMLModeStatic})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs", nil)
	handler.HTMLHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	got := rec.Header().Get("Content-Security-Policy")
	if got == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	if strings.Contains(got, "https://cdn.jsdelivr.net") {
		t.Fatalf("expected strict CSP without CDN assets, got %q", got)
	}
	if !strings.Contains(got, "object-src 'none'") {
		t.Fatalf("expected strict CSP contract, got %q", got)
	}
}
