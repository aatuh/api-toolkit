package resend

import (
	"net/http"
	"testing"
)

func TestNewAppliesOptions(t *testing.T) {
	client := &http.Client{}
	got := New(" api-key ", WithBaseURL("https://api.example/"), WithHTTPClient(client))

	if got.APIKey != "api-key" {
		t.Fatalf("APIKey = %q, want %q", got.APIKey, "api-key")
	}
	if got.BaseURL != "https://api.example" {
		t.Fatalf("BaseURL = %q, want %q", got.BaseURL, "https://api.example")
	}
	if got.HTTPClient != client {
		t.Fatal("expected custom HTTP client to be used")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg := LoadConfig(nil)
	if cfg.Enabled {
		t.Fatal("expected resend integration to default disabled")
	}
	if cfg.BaseURL != "" {
		t.Fatalf("BaseURL = %q, want empty", cfg.BaseURL)
	}
}
