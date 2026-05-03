package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OutgoingEvent is a JSON webhook event envelope for outbound signing helpers.
type OutgoingEvent[T any] struct {
	ID      string `json:"id"`
	Type    string `json:"type,omitempty"`
	Payload T      `json:"payload,omitempty"`
}

// Signer signs a raw webhook request body.
type Signer interface {
	SignWebhook(ctx context.Context, body []byte) (string, error)
}

// SignerFunc adapts a function to Signer.
type SignerFunc func(context.Context, []byte) (string, error)

// SignWebhook signs a raw webhook request body.
func (f SignerFunc) SignWebhook(ctx context.Context, body []byte) (string, error) {
	if f == nil {
		return "", fmt.Errorf("webhook signer function is nil")
	}
	return f(ctx, body)
}

// HMACSignerConfig configures HMAC-SHA256 webhook signing.
type HMACSignerConfig struct {
	Secret   []byte
	Encoding SignatureEncoding
	Prefix   string
}

// NewHMACSHA256Signer constructs an HMAC-SHA256 signer.
func NewHMACSHA256Signer(config HMACSignerConfig) (Signer, error) {
	if len(config.Secret) == 0 {
		return nil, fmt.Errorf("webhook HMAC secret is required")
	}
	return SignerFunc(func(ctx context.Context, body []byte) (string, error) {
		mac := hmac.New(sha256.New, config.Secret)
		_, _ = mac.Write(body)
		signature, err := encodeSignature(mac.Sum(nil), config.Encoding)
		if err != nil {
			return "", err
		}
		return config.Prefix + signature, nil
	}), nil
}

// SignedRequestConfig configures BuildSignedRequest.
type SignedRequestConfig struct {
	Method          string
	URL             string
	Signer          Signer
	EventID         string
	Timestamp       time.Time
	SignatureHeader string
	EventIDHeader   string
	TimestampHeader string
	ContentType     string
	Headers         http.Header
}

// BuildSignedRequest builds a JSON webhook request and signs the exact request body.
func BuildSignedRequest[T any](ctx context.Context, event OutgoingEvent[T], config SignedRequestConfig) (*http.Request, error) {
	if ctx == nil {
		return nil, fmt.Errorf("webhook request context is required")
	}
	method := strings.TrimSpace(config.Method)
	if method == "" {
		method = http.MethodPost
	}
	if strings.TrimSpace(config.URL) == "" {
		return nil, fmt.Errorf("webhook request URL is required")
	}
	if config.Signer == nil {
		return nil, fmt.Errorf("webhook signer is required")
	}
	eventID := strings.TrimSpace(config.EventID)
	if eventID == "" {
		eventID = strings.TrimSpace(event.ID)
	}
	if eventID == "" {
		return nil, fmt.Errorf("webhook event id is required")
	}
	if event.ID == "" {
		event.ID = eventID
	}
	body, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	signature, err := config.Signer.SignWebhook(ctx, body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, config.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for name, values := range config.Headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	contentType := strings.TrimSpace(config.ContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	signatureHeader := strings.TrimSpace(config.SignatureHeader)
	if signatureHeader == "" {
		signatureHeader = "X-Signature"
	}
	eventIDHeader := strings.TrimSpace(config.EventIDHeader)
	if eventIDHeader == "" {
		eventIDHeader = EventIDHeader
	}
	timestampHeader := strings.TrimSpace(config.TimestampHeader)
	if timestampHeader == "" {
		timestampHeader = TimestampHeader
	}
	timestamp := config.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set(signatureHeader, signature)
	req.Header.Set(eventIDHeader, eventID)
	req.Header.Set(timestampHeader, timestamp.UTC().Format(time.RFC3339))
	return req, nil
}

func encodeSignature(signature []byte, encoding SignatureEncoding) (string, error) {
	switch encoding {
	case SignatureEncodingHex:
		return hex.EncodeToString(signature), nil
	case SignatureEncodingBase64:
		return base64.StdEncoding.EncodeToString(signature), nil
	case SignatureEncodingBase64URL:
		return base64.RawURLEncoding.EncodeToString(signature), nil
	default:
		return "", fmt.Errorf("unsupported signature encoding")
	}
}
