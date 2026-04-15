package recover

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareWritesProblemWhenNothingCommitted(t *testing.T) {
	handler := Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected problem content type, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), `"detail":"internal server error"`) {
		t.Fatalf("expected internal error problem body, got %q", rec.Body.String())
	}
}

func TestMiddlewareDoesNotAppendProblemAfterPartialWrite(t *testing.T) {
	handler := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial:"))
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected committed status to remain 200, got %d", rec.Code)
	}
	if rec.Body.String() != "partial:" {
		t.Fatalf("expected partial response to remain unchanged, got %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("expected no appended problem body, got %q", rec.Body.String())
	}
}
