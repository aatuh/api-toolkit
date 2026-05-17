package cors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestDefaultOptions(t *testing.T) {
	want := ports.CORSOptions{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}

	if got := DefaultOptions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultOptions() = %#v, want %#v", got, want)
	}
}

func TestHandlerAppliesPreflightHeaders(t *testing.T) {
	var called bool
	handler := New().Handler(ports.CORSOptions{
		AllowedOrigins: []string{"https://app.example"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         600,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")

	handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if called {
		t.Fatal("expected preflight request to short-circuit")
	}
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("status = %d, want 2xx", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("allow origin = %q, want https://app.example", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("max age = %q, want 600", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) {
		t.Fatalf("allow methods = %q, want POST included", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
		t.Fatalf("allow headers = %q, want Authorization included", got)
	}
}

func TestHandlerPassesSimpleRequest(t *testing.T) {
	var called bool
	handler := New().Handler(ports.CORSOptions{
		AllowedOrigins: []string{"https://app.example"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Authorization"},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/resource", nil)
	req.Header.Set("Origin", "https://app.example")

	handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected simple request to reach next handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("allow origin = %q, want https://app.example", got)
	}
}
