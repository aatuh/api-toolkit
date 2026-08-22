package jsonmw

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var errJSONResponseWrite = errors.New("response write failed")

type failingJSONResponseWriter struct {
	header     http.Header
	status     int
	writeCalls int
}

func (w *failingJSONResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingJSONResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *failingJSONResponseWriter) Write([]byte) (int, error) {
	w.writeCalls++
	return 0, errJSONResponseWrite
}

func TestNilMiddlewareHandler(t *testing.T) {
	var mw *Middleware
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("payload"))
	req.Header.Set("Content-Type", "text/application/json")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected problem content type, got %q", got)
	}
}

func TestHandlerStopsAfterProblemWriteFailure(t *testing.T) {
	mw, err := New(Options{RequireJSON: true})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	writer := &failingJSONResponseWriter{}
	nextCalled := false
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("payload"))
	req.Header.Set("Content-Type", "text/plain")
	mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	})).ServeHTTP(writer, req)

	if writer.status != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", writer.status, http.StatusUnsupportedMediaType)
	}
	if writer.writeCalls != 1 {
		t.Fatalf("write calls = %d, want 1", writer.writeCalls)
	}
	if nextCalled {
		t.Fatal("next handler was called after content-type failure")
	}
}

func TestHandlerRejectsMissingContentTypeForBodyBearingWriteRequests(t *testing.T) {
	mw, err := New(Options{RequireJSON: true})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("payload"))
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
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
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
			req := httptest.NewRequestWithContext(context.Background(), tc.method, "/", nil)
			req.Header.Set("Content-Type", "text/plain")
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected 204, got %d", rec.Code)
			}
		})
	}
}

func TestStrictDecoderRejectsNilBody(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	req.Body = nil

	dec, err := StrictDecoder(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if dec != nil {
		t.Fatalf("expected nil decoder, got %#v", dec)
	}
	if err.Error() != "empty body" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStrictDecoderRejectsUnknownFields(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"name":"api","extra":true}`))

	dec, err := StrictDecoder(req)
	if err != nil {
		t.Fatalf("strict decoder: %v", err)
	}
	var payload struct {
		Name string `json:"name"`
	}
	err = dec.Decode(&payload)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), `unknown field "extra"`) {
		t.Fatalf("expected unknown field error, got %v", err)
	}
	if payload.Name != "api" {
		t.Fatalf("expected known field to decode before rejection, got %q", payload.Name)
	}
}

func TestStrictDecoderDecodesKnownFields(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"name":"api"}`))

	dec, err := StrictDecoder(req)
	if err != nil {
		t.Fatalf("strict decoder: %v", err)
	}
	var payload struct {
		Name string `json:"name"`
	}
	if err := dec.Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Name != "api" {
		t.Fatalf("name = %q, want api", payload.Name)
	}
	if err := dec.Decode(&payload); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after single object, got %v", err)
	}
}
