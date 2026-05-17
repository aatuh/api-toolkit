package resend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v3/ports"
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

func TestHealthCheckerNilWhenDisabledOrMissingKey(t *testing.T) {
	if checker := HealthChecker(Config{Enabled: false, APIKey: "key"}, nil); checker != nil {
		t.Fatal("expected disabled checker to be nil")
	}
	if checker := HealthChecker(Config{Enabled: true}, nil); checker != nil {
		t.Fatal("expected checker with missing API key to be nil")
	}
}

func TestHealthCheckerPropagatesBaseURLAndAuthHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/domains" {
			t.Fatalf("path = %q, want /domains", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := HealthChecker(Config{Enabled: true, APIKey: "resend-key", BaseURL: server.URL}, server.Client())
	if checker == nil {
		t.Fatal("expected health checker")
	}
	result := checker.Check(context.Background())
	if result.Status != ports.HealthStatusHealthy {
		t.Fatalf("status = %s, want healthy: %s", result.Status, result.Message)
	}
	if gotAuth != "Bearer resend-key" {
		t.Fatalf("Authorization = %q, want bearer key", gotAuth)
	}
}
