// Package upgradesmoke is copied by scripts/upgrade_smoke_check.sh into a
// temporary module pinned to the previous release before replacing the module
// with the current checkout.
package upgradesmoke

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/binding"
	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/middleware/maxbody"
)

type createWidgetRequest struct {
	Name string `json:"name" required:"true"`
}

func TestStableCoreUpgradeSmoke(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", strings.NewReader(`{"name":"starter"}`))
	body, err := binding.DecodeJSON[createWidgetRequest](req, binding.JSONConfig{
		MaxBytes:      1024,
		RequireObject: true,
	})
	if err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if body.Name != "starter" {
		t.Fatalf("body name = %q", body.Name)
	}

	bodyLimit, err := maxbody.New(maxbody.Options{MaxBytes: 1024})
	if err != nil {
		t.Fatalf("new maxbody: %v", err)
	}
	handler := bodyLimit.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", strings.NewReader(`{"name":"starter"}`)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q", got)
	}
}
