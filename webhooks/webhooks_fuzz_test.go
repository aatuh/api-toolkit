package webhooks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func FuzzHMACVerifierSignatures(f *testing.F) {
	for _, seed := range []struct {
		body      string
		signature string
	}{
		{`{"id":"evt_1"}`, "not-hex"},
		{`{"id":"evt_1"} trailing`, ""},
		{"", "sha256=bad"},
		{"payload", strings.Repeat("0", 64)},
	} {
		f.Add(seed.body, seed.signature)
	}
	f.Fuzz(func(t *testing.T, rawBody, candidateSignature string) {
		body := []byte(limitWebhookFuzzString(rawBody, 4096))
		candidateSignature = limitWebhookFuzzString(candidateSignature, 4096)

		verifier, err := NewHMACSHA256Verifier(HMACConfig{
			Secret:     []byte("secret"),
			HeaderName: "X-Hook-Signature",
			Encoding:   SignatureEncodingHex,
			Prefix:     "sha256=",
		})
		if err != nil {
			t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
		}
		signer, err := NewHMACSHA256Signer(HMACSignerConfig{
			Secret: []byte("secret"),
			Prefix: "sha256=",
		})
		if err != nil {
			t.Fatalf("NewHMACSHA256Signer() error = %v", err)
		}
		validSignature, err := signer.SignWebhook(context.Background(), body)
		if err != nil {
			t.Fatalf("SignWebhook() error = %v", err)
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(string(body)))
		req.Header.Set("X-Hook-Signature", validSignature)
		if err := verifier.VerifyWebhook(context.Background(), req, body); err != nil {
			t.Fatalf("valid signature failed verification: %v", err)
		}

		mutatedBody := append(append([]byte(nil), body...), 0)
		if err := verifier.VerifyWebhook(context.Background(), req, mutatedBody); !errors.Is(err, ErrInvalidSignature) {
			t.Fatalf("signature verified for mutated body; err=%v", err)
		}

		req.Header.Set("X-Hook-Signature", candidateSignature)
		err = verifier.VerifyWebhook(context.Background(), req, body)
		if strings.TrimSpace(candidateSignature) == validSignature {
			if err != nil {
				t.Fatalf("trim-equivalent valid signature failed verification: %v", err)
			}
			return
		}
		if err == nil {
			t.Fatalf("invalid candidate signature verified for body %q", string(body))
		}
	})
}

func limitWebhookFuzzString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
