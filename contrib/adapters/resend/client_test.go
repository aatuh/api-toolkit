package resend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/email"
)

func TestNewSetsDefaultHTTPTimeout(t *testing.T) {
	client := New("test-key")
	if client.HTTPClient == nil {
		t.Fatal("expected default http client")
	}
	if client.HTTPClient.Timeout <= 0 {
		t.Fatalf("expected bounded default timeout, got %s", client.HTTPClient.Timeout)
	}
}

func TestSendReturnsErrorOnMalformedSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer server.Close()

	client := New("test-key", WithBaseURL(server.URL))
	_, err := client.Send(context.Background(), testMessage())
	if err == nil {
		t.Fatal("expected malformed success response to return an error")
	}
}

func TestSendReturnsErrorOnEmptySuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := New("test-key", WithBaseURL(server.URL))
	_, err := client.Send(context.Background(), testMessage())
	if err == nil {
		t.Fatal("expected empty success response to return an error")
	}
}

func TestSendReturnsProviderErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "provider exploded", http.StatusBadGateway)
	}))
	defer server.Close()

	client := New("test-key", WithBaseURL(server.URL))
	_, err := client.Send(context.Background(), testMessage())
	if err == nil {
		t.Fatal("expected provider error")
	}
	if !strings.Contains(err.Error(), "provider exploded") {
		t.Fatalf("expected provider body in error, got %v", err)
	}
}

func testMessage() email.Message {
	return email.Message{
		From:    "sender@example.com",
		To:      []string{"receiver@example.com"},
		Subject: "hello",
		Text:    "plain text",
	}
}
