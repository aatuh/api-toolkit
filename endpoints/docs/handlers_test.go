package docs

import (
	"context"
	htmltemplate "html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/specs"
)

type stubRouteRegistrar struct {
	patterns []string
}

func (s *stubRouteRegistrar) Get(pattern string, _ http.HandlerFunc) {
	s.patterns = append(s.patterns, pattern)
}

func TestNewHandlerDefaultsNilManager(t *testing.T) {
	handler := NewHandler(nil)
	if handler == nil {
		t.Fatal("expected handler")
	}
	if handler.manager == nil {
		t.Fatal("expected default docs manager")
	}
}

func TestOpenAPIHandlerWithNilManagerReturnsNotFoundWithoutSpec(t *testing.T) {
	handler := NewHandler(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs/openapi.json", nil)
	handler.OpenAPIHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
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

func TestMiddlewareWithNilHandlerIsIdentity(t *testing.T) {
	var handler *Handler
	middleware := handler.Middleware()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-API-Title"); got != "" {
		t.Fatalf("expected no docs headers from nil middleware, got %q", got)
	}
	if got := rec.Header().Get("X-API-Version"); got != "" {
		t.Fatalf("expected no docs headers from nil middleware, got %q", got)
	}
}

func TestNewDefaultsToStaticMode(t *testing.T) {
	manager, ok := New().(*Manager)
	if !ok {
		t.Fatal("expected concrete docs manager")
	}
	if got := manager.HTMLMode(); got != ports.DocsHTMLModeStatic {
		t.Fatalf("default html mode = %q, want %q", got, ports.DocsHTMLModeStatic)
	}
}

func TestNewSwaggerUIUsesSwaggerMode(t *testing.T) {
	manager, ok := NewSwaggerUI().(*Manager)
	if !ok {
		t.Fatal("expected concrete docs manager")
	}
	if got := manager.HTMLMode(); got != ports.DocsHTMLModeSwaggerUI {
		t.Fatalf("swagger html mode = %q, want %q", got, ports.DocsHTMLModeSwaggerUI)
	}
}

func TestNewWithConfigDefaultsBlankModeToStatic(t *testing.T) {
	manager, ok := NewWithConfig(ports.DocsConfig{
		Title:       "Docs",
		Description: "Default mode",
		Version:     "1.0.0",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  true,
		EnableJSON:  true,
	}).(*Manager)
	if !ok {
		t.Fatal("expected concrete docs manager")
	}
	if got := manager.HTMLMode(); got != ports.DocsHTMLModeStatic {
		t.Fatalf("blank html mode = %q, want %q", got, ports.DocsHTMLModeStatic)
	}
}

func TestHTMLHandlerUsesStrictCSPByDefault(t *testing.T) {
	handler := NewHandler(nil)

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

func TestHTMLHandlerUsesSwaggerUICSPWhenOptedIn(t *testing.T) {
	handler := NewHandler(NewWithConfig(ports.DocsConfig{
		Title:       "Docs UI",
		Description: "Swagger UI docs",
		Version:     "1.2.3",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  true,
		EnableJSON:  true,
		HTMLMode:    ports.DocsHTMLModeSwaggerUI,
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs", nil)
	handler.HTMLHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != defaultDocsCSP {
		t.Fatalf("unexpected CSP: %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "cdn.jsdelivr.net") {
		t.Fatalf("expected external CDN assets in swagger mode, got %q", body)
	}
	if !strings.Contains(body, "<script") {
		t.Fatalf("expected script tags in swagger mode, got %q", body)
	}
}

func TestHTMLHandlerReturnsNotFoundWhenDisabled(t *testing.T) {
	handler := NewHandler(NewWithConfig(ports.DocsConfig{
		Title:       "Docs",
		Description: "Disabled HTML",
		Version:     "1.0.0",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  false,
		EnableJSON:  true,
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs", nil)
	handler.HTMLHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHTMLHandlerEscapesConfiguredValues(t *testing.T) {
	title := `Docs <script>alert("boom")</script>`
	description := `Desc <b>raw</b>`
	openAPIPath := `/docs/openapi.json";window.pwned=true;//`

	handler := NewHandler(NewWithConfig(ports.DocsConfig{
		Title:       title,
		Description: description,
		Version:     "1.0.0",
		Paths: ports.DocsPaths{
			HTML:    "/docs",
			OpenAPI: openAPIPath,
			Version: "/docs/version",
			Info:    "/docs/info",
		},
		EnableHTML: true,
		EnableJSON: true,
		HTMLMode:   ports.DocsHTMLModeSwaggerUI,
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs", nil)
	handler.HTMLHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, `<script>alert("boom")</script>`) {
		t.Fatalf("expected script tag to be escaped, got %q", body)
	}
	if !strings.Contains(body, htmltemplate.HTMLEscapeString(title)) {
		t.Fatalf("expected escaped title, got %q", body)
	}
	if !strings.Contains(body, htmltemplate.HTMLEscapeString(description)) {
		t.Fatalf("expected escaped description, got %q", body)
	}
	if strings.Contains(body, `url: "`+openAPIPath+`"`) {
		t.Fatalf("expected raw openapi path not to appear in script, got %q", body)
	}
	if !strings.Contains(body, `\u0022;window.pwned=true;\/\/`) {
		t.Fatalf("expected quote-breaking payload to stay escaped inside the js string, got %q", body)
	}
}

func TestOpenAPIHandlerServesJSONFromDiscoveredSpec(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "docs/openapi.json", `{"openapi":"3.0.0","info":{"title":"Docs","version":"1.0.0"}}`)

	handler := NewHandler(NewWithConfig(ports.DocsConfig{
		Title:       "Docs",
		Description: "JSON spec",
		Version:     "1.0.0",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  true,
		EnableJSON:  true,
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs/openapi.json", nil)
	handler.OpenAPIHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"openapi":"3.0.0"`) {
		t.Fatalf("expected json spec body, got %q", body)
	}
}

func TestOpenAPIHandlerReturnsNotFoundWhenJSONDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "docs/openapi.json", `{"openapi":"3.0.0","info":{"title":"Docs","version":"1.0.0"}}`)

	handler := NewHandler(NewWithConfig(ports.DocsConfig{
		Title:       "Docs",
		Description: "JSON disabled",
		Version:     "1.0.0",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  true,
		EnableJSON:  false,
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs/openapi.json", nil)
	handler.OpenAPIHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestOpenAPIHandlerServesYAMLWhenEnabled(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "docs/openapi.yaml", "openapi: 3.0.0\ninfo:\n  title: Docs\n  version: 1.0.0\n")

	handler := NewHandler(NewWithConfig(ports.DocsConfig{
		Title:       "Docs",
		Description: "YAML spec",
		Version:     "1.0.0",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  true,
		EnableJSON:  false,
		EnableYAML:  true,
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs/openapi.yaml", nil)
	handler.OpenAPIHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/yaml" {
		t.Fatalf("expected application/yaml content type, got %q", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "openapi: 3.0.0") {
		t.Fatalf("expected yaml spec body, got %q", body)
	}
}

func TestRegisterRoutesToUsesMinimalRegistrar(t *testing.T) {
	handler := NewHandler(nil)
	router := &stubRouteRegistrar{}

	handler.RegisterRoutesTo(router)

	expected := []string{
		specs.Docs,
		specs.DocsOpenAPI,
		specs.DocsVersion,
		specs.DocsInfo,
	}
	if len(router.patterns) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(router.patterns))
	}
	for i := range expected {
		if router.patterns[i] != expected[i] {
			t.Fatalf("route %d = %q", i, router.patterns[i])
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
