//go:build downstreamcompat

// Package upgradesmoke is copied into a temporary downstream module by the
// compatibility corpus. It represents a small chi service using a root helper.
package upgradesmoke

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/go-chi/chi/v5"
)

func TestChiConsumerWritesAPublicResponse(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got, want := response.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
