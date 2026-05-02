package deprecation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddlewareWritesDeprecationSunsetAndLinks(t *testing.T) {
	deprecatedAt := time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC)
	sunsetAt := time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC)
	mw, err := New(Config{
		DeprecatedAt: deprecatedAt,
		SunsetAt:     sunsetAt,
		Links: []Link{
			{URL: "https://developer.example.test/deprecations/widgets", Type: "text/html", Title: "Migration guide"},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Deprecation") != "@1777708800" {
		t.Fatalf("Deprecation = %q", rec.Header().Get("Deprecation"))
	}
	if rec.Header().Get("Sunset") != "Tue, 02 Jun 2026 08:00:00 GMT" {
		t.Fatalf("Sunset = %q", rec.Header().Get("Sunset"))
	}
	link := rec.Header().Get("Link")
	if !strings.Contains(link, `rel="deprecation"`) || !strings.Contains(link, `type="text/html"`) {
		t.Fatalf("Link = %q", link)
	}
}

func TestMiddlewareDisabledAndInvalidConfig(t *testing.T) {
	mw, err := New(Config{Disabled: true, DeprecatedAt: time.Now()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)
	if rec.Header().Get("Deprecation") != "" {
		t.Fatalf("Deprecation = %q, want empty", rec.Header().Get("Deprecation"))
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	_, err = New(Config{
		DeprecatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		SunsetAt:     time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected invalid config error")
	}
}
