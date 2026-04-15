package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestHandlerReturnsUnauthorizedWhenTokenMissing(t *testing.T) {
	mw := &Middleware{enabled: true, log: ports.NopLogger{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandlerReturnsUnauthorizedWhenAuthorizationHeaderMalformed(t *testing.T) {
	mw := &Middleware{enabled: true, log: ports.NopLogger{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic abc123")
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
