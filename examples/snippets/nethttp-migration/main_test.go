package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestToolkitNetHTTPCreatesWidget(t *testing.T) {
	rec := serveToolkit(t, "POST", "/widgets", `{"name":"  demo  ","quantity":2}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	assertContentTypePrefix(t, rec, "application/json")

	var got widgetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := widgetResponse{ID: "widget-123", Name: "demo", Quantity: 2}
	if got != want {
		t.Fatalf("response = %#v, want %#v", got, want)
	}
}

func TestToolkitNetHTTPRejectsMalformedJSONAsProblemDetails(t *testing.T) {
	rec := serveToolkit(t, "POST", "/widgets", `{"name":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertContentTypePrefix(t, rec, "application/problem+json")
	assertBodyContains(t, rec, `"detail":"validation failed"`)
	assertBodyContains(t, rec, `"field":"body"`)
}

func TestToolkitNetHTTPRejectsValidationErrorsAsProblemDetails(t *testing.T) {
	rec := serveToolkit(t, "POST", "/widgets", `{"name":"demo","quantity":0}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertContentTypePrefix(t, rec, "application/problem+json")
	assertBodyContains(t, rec, `"field":"quantity"`)
}

func TestToolkitNetHTTPRejectsOversizedBodies(t *testing.T) {
	body := `{"name":"` + strings.Repeat("a", int(maxRequestBodyBytes)) + `","quantity":1}`
	rec := serveToolkit(t, "POST", "/widgets", body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertContentTypePrefix(t, rec, "application/problem+json")
	assertBodyContains(t, rec, `"field":"body"`)
}

func TestToolkitNetHTTPRegistersHealthRoutes(t *testing.T) {
	for _, path := range []string{"/livez", "/readyz", "/healthz", "/health"} {
		rec := serveToolkit(t, "GET", path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d; body: %s", path, rec.Code, http.StatusOK, rec.Body.String())
		}
		assertContentTypePrefix(t, rec, "application/json")
		assertBodyContains(t, rec, `"status":"healthy"`)
	}
}

func TestToolkitMiddlewarePropagatesSlowHandlerDeadline(t *testing.T) {
	mux := http.NewServeMux()
	deadline := make(chan error, 1)
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		deadline <- r.Context().Err()
		w.WriteHeader(http.StatusNoContent)
	})
	handler := toolkitMiddleware(mux, time.Nanosecond)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/slow", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if err := <-deadline; err != context.DeadlineExceeded {
		t.Fatalf("request context error = %v, want %v", err, context.DeadlineExceeded)
	}
}

func serveToolkit(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	toolkitNetHTTP().ServeHTTP(rec, req)
	return rec
}

func assertContentTypePrefix(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, want) {
		t.Fatalf("content type = %q, want prefix %q", got, want)
	}
}

func assertBodyContains(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()

	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body missing %q: %s", want, rec.Body.String())
	}
}
