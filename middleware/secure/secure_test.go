package secure

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAPIOnlyProfileHeaders(t *testing.T) {
	headers := runSecureHeaders(t, APIOnly())

	if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := headers.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	if got := headers.Get("Referrer-Policy"); got != referrerPolicyStrictOriginWhenCrossOrigin {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	if got := headers.Get("Content-Security-Policy"); got != CSPPolicy(CSPProfileAPI) {
		t.Fatalf("Content-Security-Policy = %q", got)
	}
	if got := headers.Get("Permissions-Policy"); got != "" {
		t.Fatalf("Permissions-Policy = %q", got)
	}
	expectedHSTS := buildHSTSValue(Options{
		HSTSMaxAge:            hstsMaxAgeRecommended,
		HSTSIncludeSubdomains: true,
	})
	if got := headers.Get("Strict-Transport-Security"); got != expectedHSTS {
		t.Fatalf("Strict-Transport-Security = %q", got)
	}
	if got := headers.Get("Cross-Origin-Opener-Policy"); got != "" {
		t.Fatalf("Cross-Origin-Opener-Policy = %q", got)
	}
	if got := headers.Get("Cross-Origin-Embedder-Policy"); got != "" {
		t.Fatalf("Cross-Origin-Embedder-Policy = %q", got)
	}
	if got := headers.Get("Cross-Origin-Resource-Policy"); got != "" {
		t.Fatalf("Cross-Origin-Resource-Policy = %q", got)
	}
}

func TestDocsUIProfileHeaders(t *testing.T) {
	headers := runSecureHeaders(t, DocsUI())

	if got := headers.Get("Content-Security-Policy"); got != CSPPolicy(CSPProfileAPIDocs) {
		t.Fatalf("Content-Security-Policy = %q", got)
	}
	if got := headers.Get("Permissions-Policy"); got != permissionsPolicyMinimal {
		t.Fatalf("Permissions-Policy = %q", got)
	}
}

func TestWebAppProfileHeaders(t *testing.T) {
	headers := runSecureHeaders(t, WebApp())

	expectedCSP := RenderCSPTemplate(CSPTemplateWebApp, CSPTemplateValues{})
	if got := headers.Get("Content-Security-Policy"); got != expectedCSP {
		t.Fatalf("Content-Security-Policy = %q", got)
	}
	if got := headers.Get("Permissions-Policy"); got != permissionsPolicyMinimal {
		t.Fatalf("Permissions-Policy = %q", got)
	}
}

func TestCrossOriginIsolationOptIn(t *testing.T) {
	headers := runSecureHeaders(t, APIOnly(), WithCrossOriginIsolation())

	if got := headers.Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Opener-Policy = %q", got)
	}
	if got := headers.Get("Cross-Origin-Embedder-Policy"); got != "require-corp" {
		t.Fatalf("Cross-Origin-Embedder-Policy = %q", got)
	}
	if got := headers.Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Resource-Policy = %q", got)
	}
}

func TestCrossOriginPolicies(t *testing.T) {
	headers := runSecureHeaders(t, APIOnly(),
		WithCOOP("same-origin"),
		WithCOEP("require-corp"),
		WithCORP("same-site"),
	)

	if got := headers.Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Opener-Policy = %q", got)
	}
	if got := headers.Get("Cross-Origin-Embedder-Policy"); got != "require-corp" {
		t.Fatalf("Cross-Origin-Embedder-Policy = %q", got)
	}
	if got := headers.Get("Cross-Origin-Resource-Policy"); got != "same-site" {
		t.Fatalf("Cross-Origin-Resource-Policy = %q", got)
	}
}

func TestRenderCSPTemplate(t *testing.T) {
	rendered := RenderCSPTemplate(CSPTemplateWebApp, CSPTemplateValues{
		Nonce:      "abc123",
		ScriptSrc:  []string{"https://cdn.example.com"},
		StyleSrc:   []string{"https://fonts.example.com"},
		ImgSrc:     []string{"https://img.example.com"},
		ConnectSrc: []string{"https://api.example.com"},
		FontSrc:    []string{"https://fonts-static.example.com"},
	})

	expected := "default-src 'self'; base-uri 'self'; object-src 'none'; " +
		"frame-ancestors 'none'; form-action 'self'; script-src 'self' 'nonce-abc123' " +
		"https://cdn.example.com; style-src 'self' 'nonce-abc123' https://fonts.example.com; " +
		"img-src 'self' data: https://img.example.com; connect-src 'self' https://api.example.com; " +
		"font-src 'self' https://fonts-static.example.com"
	if rendered != expected {
		t.Fatalf("rendered CSP = %q", rendered)
	}

	empty := RenderCSPTemplate(CSPTemplateWebApp, CSPTemplateValues{})
	if strings.Contains(empty, "{{") {
		t.Fatalf("template placeholders not removed: %q", empty)
	}
	if strings.Contains(empty, "nonce-") {
		t.Fatalf("unexpected nonce in CSP: %q", empty)
	}
}

func TestSecureRejectsNegativeHSTS(t *testing.T) {
	if _, err := New(WithHSTS(-1*time.Second, false, false)); err == nil {
		t.Fatal("expected error for negative hsts max age")
	}
}

func runSecureHeaders(t *testing.T, opts ...Option) http.Header {
	t.Helper()

	mw, err := New(opts...)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Header()
}
