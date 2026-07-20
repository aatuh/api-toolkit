package negotiation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParseAcceptSortsByQualityAndSpecificity(t *testing.T) {
	got, err := ParseAccept("application/*;q=0.8, application/json;q=0.8, text/plain;q=0.9")
	if err != nil {
		t.Fatalf("ParseAccept() error = %v", err)
	}
	mediaTypes := []string{got[0].MediaType, got[1].MediaType, got[2].MediaType}
	want := []string{"text/plain", "application/json", "application/*"}
	if !reflect.DeepEqual(mediaTypes, want) {
		t.Fatalf("media types = %#v, want %#v", mediaTypes, want)
	}
	if _, err := ParseAccept("application/json;q=2"); err == nil {
		t.Fatal("expected invalid q error")
	}
	if _, err := ParseAccept("0/0;q=\"\""); err == nil {
		t.Fatal("expected empty q error")
	}
}

func TestNegotiateMatchesWildcardsSuffixesAndMissingAccept(t *testing.T) {
	offered := []MediaType{"application/json", "text/plain"}
	if got, ok := Negotiate("", offered); !ok || got != "application/json" {
		t.Fatalf("missing Accept = %q, %v", got, ok)
	}
	if got, ok := Negotiate("text/*", offered); !ok || got != "text/plain" {
		t.Fatalf("wildcard = %q, %v", got, ok)
	}
	if got, ok := Negotiate("application/vnd.example+json", offered); !ok || got != "application/json" {
		t.Fatalf("suffix = %q, %v", got, ok)
	}
	if _, ok := Negotiate("application/json;q=0", offered); ok {
		t.Fatal("q=0 should not match")
	}
}

func TestContentTypeAllowed(t *testing.T) {
	allowed := []MediaType{"application/json"}
	if !ContentTypeAllowed("application/vnd.example+json; charset=utf-8", allowed) {
		t.Fatal("expected structured JSON content type to be allowed")
	}
	if ContentTypeAllowed("text/plain", allowed) {
		t.Fatal("text/plain should not be allowed")
	}
}

func TestMiddlewareWrites406And415(t *testing.T) {
	mw, err := New(Config{
		Accept:       []MediaType{"application/json"},
		ContentTypes: []MediaType{"application/json"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, want 406", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"acceptable"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestRequireHelpersAllowValidRequests(t *testing.T) {
	handler := RequireAccept("application/json")(RequireContentType("application/json")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
