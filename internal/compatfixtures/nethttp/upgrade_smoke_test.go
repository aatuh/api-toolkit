//go:build downstreamcompat

// Package upgradesmoke is copied into a temporary downstream module by the
// compatibility corpus. It deliberately uses only net/http and stable root APIs.
package upgradesmoke

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestNetHTTPConsumer(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/widgets", nil))
	if rec.Code != http.StatusCreated || rec.Body.String() != "{\"status\":\"created\"}\n" {
		t.Fatalf("unexpected downstream response: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
