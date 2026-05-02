package webhooks

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHMACSignerBuildsReceiverCompatibleSignedRequest(t *testing.T) {
	signer, err := NewHMACSHA256Signer(HMACSignerConfig{Secret: []byte("secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Signer() error = %v", err)
	}
	req, err := BuildSignedRequest(context.Background(), OutgoingEvent[testPayload]{ID: "evt_1", Type: "thing.created", Payload: testPayload{ID: "payload_1"}}, SignedRequestConfig{
		URL:       "https://example.com/webhooks",
		Signer:    signer,
		Timestamp: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildSignedRequest() error = %v", err)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if req.Method != http.MethodPost || req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("request method/header = %s %#v", req.Method, req.Header)
	}
	if req.Header.Get("X-Webhook-Event-ID") != "evt_1" || req.Header.Get("X-Webhook-Timestamp") != "2026-05-02T12:00:00Z" {
		t.Fatalf("event headers = %#v", req.Header)
	}
	verifier, err := NewHMACSHA256Verifier(HMACConfig{Secret: []byte("secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
	}
	if err := verifier.VerifyWebhook(context.Background(), req, body); err != nil {
		t.Fatalf("VerifyWebhook() error = %v", err)
	}
}

func TestHMACSignerSupportsBase64AndMalformedConfig(t *testing.T) {
	if _, err := NewHMACSHA256Signer(HMACSignerConfig{}); err == nil {
		t.Fatal("expected missing secret error")
	}
	signer, err := NewHMACSHA256Signer(HMACSignerConfig{Secret: []byte("secret"), Encoding: SignatureEncodingBase64, Prefix: "sha256="})
	if err != nil {
		t.Fatalf("NewHMACSHA256Signer() error = %v", err)
	}
	req, err := BuildSignedRequest(context.Background(), OutgoingEvent[testPayload]{ID: "evt_1", Payload: testPayload{ID: "payload_1"}}, SignedRequestConfig{
		URL:             "https://example.com/webhooks",
		Signer:          signer,
		SignatureHeader: "X-Hook-Signature",
	})
	if err != nil {
		t.Fatalf("BuildSignedRequest() error = %v", err)
	}
	if !strings.HasPrefix(req.Header.Get("X-Hook-Signature"), "sha256=") {
		t.Fatalf("signature = %q", req.Header.Get("X-Hook-Signature"))
	}
	if _, err := BuildSignedRequest(context.Background(), OutgoingEvent[testPayload]{}, SignedRequestConfig{URL: "https://example.com", Signer: signer}); err == nil {
		t.Fatal("expected missing event id error")
	}
	if _, err := BuildSignedRequest(context.Background(), OutgoingEvent[testPayload]{ID: "evt_1"}, SignedRequestConfig{Signer: signer}); err == nil {
		t.Fatal("expected missing URL error")
	}
}
