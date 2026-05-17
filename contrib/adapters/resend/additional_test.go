package resend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/healthchecktest"
	"github.com/aatuh/api-toolkit/v3/email"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestSendSuccessAndRequestContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("headers = %#v", r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		var payload sendRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.From != "sender@example.com" || payload.To[0] != "receiver@example.com" || payload.ReplyTo != "reply@example.com" {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
	}))
	defer server.Close()

	client := New(" test-key ", WithBaseURL(server.URL+"/"), WithHTTPClient(server.Client()))
	id, err := client.Send(context.Background(), email.Message{From: "sender@example.com", To: []string{"receiver@example.com"}, Subject: "hello", HTML: "<p>hi</p>", ReplyTo: "reply@example.com"})
	if err != nil || id != "email_123" {
		t.Fatalf("Send() = %q, %v", id, err)
	}
}

func TestSendValidationAndClientFallbacks(t *testing.T) {
	if _, err := (*Client)(nil).Send(context.Background(), testMessage()); err == nil {
		t.Fatal("expected nil client api key error")
	}
	if _, err := New(" ").Send(context.Background(), testMessage()); err == nil {
		t.Fatal("expected missing api key error")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"email_456"}`))
	}))
	defer server.Close()
	client := New("test-key", WithBaseURL(server.URL), WithHTTPClient(nil))
	client.HTTPClient = server.Client()
	id, err := client.Send(context.Background(), testMessage())
	if err != nil || id != "email_456" {
		t.Fatalf("Send() fallback = %q, %v", id, err)
	}
}

func TestHealthCheckerDisabledAndProviderResponses(t *testing.T) {
	if HealthChecker(Config{Enabled: false, APIKey: "key"}, nil) != nil {
		t.Fatal("disabled health checker should be nil")
	}
	if HealthChecker(Config{Enabled: true}, nil) != nil {
		t.Fatal("missing api key health checker should be nil")
	}
	mode := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains" || r.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("health request = %s %#v", r.URL.Path, r.Header)
		}
		switch mode {
		case "restricted":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"name":"restricted_api_key","message":"restricted"}`))
		case "degraded":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"name":"bad_gateway","message":"downstream down"}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	checker := HealthChecker(Config{Enabled: true, APIKey: "key", BaseURL: server.URL}, server.Client())
	if result := checker.Check(context.Background()); result.Status != ports.HealthStatusHealthy {
		t.Fatalf("healthy status = %q", result.Status)
	}
	healthchecktest.AssertCheckerContract(t, checker, "resend", ports.HealthStatusHealthy)
	mode = "restricted"
	checker = HealthChecker(Config{Enabled: true, APIKey: "key", BaseURL: server.URL}, server.Client())
	if result := checker.Check(context.Background()); result.Status != ports.HealthStatusHealthy || !strings.Contains(result.Message, "send-only") {
		t.Fatalf("restricted status = %#v", result)
	}
	mode = "degraded"
	checker = HealthChecker(Config{Enabled: true, APIKey: "key", BaseURL: server.URL}, server.Client())
	if result := checker.Check(context.Background()); result.Status != ports.HealthStatusDegraded {
		t.Fatalf("degraded status = %#v", result)
	}
}
