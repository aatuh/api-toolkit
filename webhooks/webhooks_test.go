package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testPayload struct {
	ID string `json:"id"`
}

func TestHMACVerifierSupportsHexAndBase64(t *testing.T) {
	body := []byte(`{"id":"evt_1"}`)
	hexVerifier, err := NewHMACSHA256Verifier(HMACConfig{Secret: []byte("secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(string(body)))
	req.Header.Set("X-Signature", signHex(body, "secret"))
	if err := hexVerifier.VerifyWebhook(context.Background(), req, body); err != nil {
		t.Fatalf("VerifyWebhook hex error = %v", err)
	}

	base64Verifier, err := NewHMACSHA256Verifier(HMACConfig{
		Secret:     []byte("secret"),
		HeaderName: "X-Hook-Signature",
		Encoding:   SignatureEncodingBase64,
		Prefix:     "sha256=",
	})
	if err != nil {
		t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
	}
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(string(body)))
	req.Header.Set("X-Hook-Signature", "sha256="+signBase64(body, "secret"))
	if err := base64Verifier.VerifyWebhook(context.Background(), req, body); err != nil {
		t.Fatalf("VerifyWebhook base64 error = %v", err)
	}
}

func TestHMACVerifierRejectsMissingMalformedAndWrongSignatures(t *testing.T) {
	body := []byte(`{"id":"evt_1"}`)
	verifier, err := NewHMACSHA256Verifier(HMACConfig{Secret: []byte("secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	if err := verifier.VerifyWebhook(context.Background(), req, body); !errors.Is(err, ErrMissingSignature) {
		t.Fatalf("missing error = %v", err)
	}
	req.Header.Set("X-Signature", "not-hex")
	if err := verifier.VerifyWebhook(context.Background(), req, body); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("malformed error = %v", err)
	}
	req.Header.Set("X-Signature", signHex(body, "wrong"))
	if err := verifier.VerifyWebhook(context.Background(), req, body); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("wrong signature error = %v", err)
	}
	if _, err := NewHMACSHA256Verifier(HMACConfig{}); err == nil {
		t.Fatal("expected missing secret error")
	}
}

func TestReceiverAcceptsVerifiedJSONEvent(t *testing.T) {
	body := []byte(`{"id":"evt_1"}`)
	verifier, err := NewHMACSHA256Verifier(HMACConfig{Secret: []byte("secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
	}
	var handled Event[testPayload]
	receiver := Receiver[testPayload]{Config: ReceiverConfig[testPayload]{
		Verifier: verifier,
		Handle: func(ctx context.Context, event Event[testPayload]) error {
			handled = event
			return nil
		},
	}}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(string(body)))
	req.Header.Set("X-Signature", signHex(body, "secret"))
	rec := httptest.NewRecorder()
	receiver.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
	}
	if handled.Payload.ID != "evt_1" || string(handled.RawBody) != string(body) {
		t.Fatalf("handled = %#v", handled)
	}
}

func TestReceiverWritesProblemDetailsForFailureCases(t *testing.T) {
	verifier, err := NewHMACSHA256Verifier(HMACConfig{Secret: []byte("secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
	}
	tests := []struct {
		name       string
		body       string
		signature  string
		maxBytes   int64
		wantStatus int
	}{
		{name: "oversized", body: `{"id":"evt_1"}`, signature: signHex([]byte(`{"id":"evt_1"}`), "secret"), maxBytes: 4, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "missing signature", body: `{"id":"evt_1"}`, wantStatus: http.StatusUnauthorized},
		{name: "bad json", body: `{`, signature: signHex([]byte(`{`), "secret"), wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := Receiver[testPayload]{Config: ReceiverConfig[testPayload]{
				Verifier:     verifier,
				MaxBodyBytes: tt.maxBytes,
			}}
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(tt.body))
			if tt.signature != "" {
				req.Header.Set("X-Signature", tt.signature)
			}
			rec := httptest.NewRecorder()
			receiver.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("content type = %q", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestReceiverDoesNotExposeVerifierErrorDetailByDefault(t *testing.T) {
	receiver := Receiver[testPayload]{Config: ReceiverConfig[testPayload]{
		Verifier: VerifierFunc(func(context.Context, *http.Request, []byte) error {
			return errors.New("provider token leaked in verifier detail")
		}),
	}}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"id":"evt_1"}`))
	rec := httptest.NewRecorder()

	receiver.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "provider token leaked") {
		t.Fatalf("verifier detail leaked in response body: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "webhook verification failed") {
		t.Fatalf("response body missing safe detail: %s", rec.Body.String())
	}
}

func TestReceiverAllowsSafeVerifierErrorDetailOverride(t *testing.T) {
	receiver := Receiver[testPayload]{Config: ReceiverConfig[testPayload]{
		Verifier: VerifierFunc(func(context.Context, *http.Request, []byte) error {
			return ErrMissingSignature
		}),
		VerificationErrorDetail: func(error) string {
			return "missing webhook signature"
		},
	}}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"id":"evt_1"}`))
	rec := httptest.NewRecorder()

	receiver.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "missing webhook signature") {
		t.Fatalf("response body missing override detail: %s", rec.Body.String())
	}
}

func signHex(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func signBase64(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
