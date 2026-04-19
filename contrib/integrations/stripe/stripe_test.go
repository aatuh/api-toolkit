package stripe

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
)

func TestNewAppliesOptionsAndWebhookHelpers(t *testing.T) {
	provider := New("sk_test", "", WithDevMode(true))
	if provider == nil {
		t.Fatal("expected provider")
	}

	payload, err := json.Marshal(map[string]any{
		"id":      "evt_test",
		"type":    "checkout.session.completed",
		"created": int64(1),
		"data": map[string]any{
			"object": map[string]any{"id": "cs_test"},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	safeCtx := AllowInsecureWebhookContext(context.Background(), netip.MustParseAddr("127.0.0.1"))
	event, err := provider.ParseWebhook(safeCtx, payload, "")
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}
	if event.ID != "evt_test" || event.Type != "checkout.session.completed" {
		t.Fatalf("unexpected webhook event: %#v", event)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg := LoadConfig(nil)
	if cfg.Enabled {
		t.Fatal("expected stripe integration to default disabled")
	}
	if cfg.FrontendBaseURL != "http://localhost:3000" {
		t.Fatalf("FrontendBaseURL = %q, want %q", cfg.FrontendBaseURL, "http://localhost:3000")
	}
}
