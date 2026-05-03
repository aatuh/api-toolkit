package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

const (
	defaultMaxBodyBytes              int64 = 1 << 20
	defaultVerificationFailureDetail       = "webhook verification failed"
)

var (
	// ErrMissingSignature reports a request without the configured signature header.
	ErrMissingSignature = errors.New("missing webhook signature")
	// ErrInvalidSignature reports a request with a malformed or non-matching signature.
	ErrInvalidSignature = errors.New("invalid webhook signature")
)

// Verifier verifies a raw webhook request body.
type Verifier interface {
	VerifyWebhook(ctx context.Context, r *http.Request, body []byte) error
}

// VerifierFunc adapts a function to Verifier.
type VerifierFunc func(context.Context, *http.Request, []byte) error

// VerifyWebhook verifies a raw webhook request body.
func (f VerifierFunc) VerifyWebhook(ctx context.Context, r *http.Request, body []byte) error {
	return f(ctx, r, body)
}

// SignatureEncoding describes how a signature header is encoded.
type SignatureEncoding int

const (
	SignatureEncodingHex SignatureEncoding = iota
	SignatureEncodingBase64
	SignatureEncodingBase64URL
)

// HMACConfig configures HMAC-SHA256 signature verification.
type HMACConfig struct {
	Secret     []byte
	HeaderName string
	Encoding   SignatureEncoding
	Prefix     string
}

// NewHMACSHA256Verifier constructs an HMAC-SHA256 verifier.
func NewHMACSHA256Verifier(config HMACConfig) (Verifier, error) {
	if len(config.Secret) == 0 {
		return nil, fmt.Errorf("webhook HMAC secret is required")
	}
	if strings.TrimSpace(config.HeaderName) == "" {
		config.HeaderName = "X-Signature"
	}
	return VerifierFunc(func(ctx context.Context, r *http.Request, body []byte) error {
		signature := strings.TrimSpace(r.Header.Get(config.HeaderName))
		if signature == "" {
			return ErrMissingSignature
		}
		if config.Prefix != "" {
			if !strings.HasPrefix(signature, config.Prefix) {
				return ErrInvalidSignature
			}
			signature = strings.TrimSpace(strings.TrimPrefix(signature, config.Prefix))
		}
		got, err := decodeSignature(signature, config.Encoding)
		if err != nil {
			return ErrInvalidSignature
		}
		mac := hmac.New(sha256.New, config.Secret)
		_, _ = mac.Write(body)
		expected := mac.Sum(nil)
		if !hmac.Equal(expected, got) {
			return ErrInvalidSignature
		}
		return nil
	}), nil
}

// ReceiverConfig configures a webhook receiver.
type ReceiverConfig[T any] struct {
	Verifier     Verifier
	MaxBodyBytes int64
	Replay       ReplayConfig
	Decode       func([]byte) (T, error)
	Handle       func(context.Context, Event[T]) error
	ErrorWriter  func(http.ResponseWriter, int, httpx.Problem)
	// VerificationErrorDetail formats a safe client-facing detail for verifier
	// failures. When nil or empty, a generic detail is returned.
	VerificationErrorDetail func(error) string
}

// Receiver is an http.Handler for verified webhook events.
type Receiver[T any] struct {
	Config ReceiverConfig[T]
}

// Event is a verified and decoded webhook event.
type Event[T any] struct {
	Payload T
	RawBody []byte
	Request *http.Request
}

// ServeHTTP verifies, decodes, and accepts a webhook event.
func (receiver Receiver[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	config := receiver.Config
	writeProblem := config.ErrorWriter
	if writeProblem == nil {
		writeProblem = WriteProblem
	}
	if config.Verifier == nil {
		writeProblem(w, http.StatusInternalServerError, httpx.Problem{
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: "webhook receiver verifier is not configured",
		})
		return
	}
	maxBodyBytes := config.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	body, err := readBody(w, r, maxBodyBytes)
	if err != nil {
		writeProblem(w, http.StatusRequestEntityTooLarge, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypePayloadTooLarge),
			Title:  http.StatusText(http.StatusRequestEntityTooLarge),
			Detail: "webhook payload too large",
		})
		return
	}
	if decision := CheckReplayWindow(r, config.Replay); !decision.Allowed {
		writeProblem(w, http.StatusUnauthorized, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
			Title:  http.StatusText(http.StatusUnauthorized),
			Detail: decision.Reason,
		})
		return
	}
	if err := config.Verifier.VerifyWebhook(r.Context(), r, body); err != nil {
		writeProblem(w, http.StatusUnauthorized, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
			Title:  http.StatusText(http.StatusUnauthorized),
			Detail: verificationErrorDetail(config.VerificationErrorDetail, err),
		})
		return
	}
	decode := config.Decode
	if decode == nil {
		decode = DecodeJSON[T]
	}
	payload, err := decode(body)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeBadRequest),
			Title:  http.StatusText(http.StatusBadRequest),
			Detail: "invalid webhook payload",
		})
		return
	}
	event := Event[T]{Payload: payload, RawBody: append([]byte(nil), body...), Request: r}
	if config.Handle != nil {
		if err := config.Handle(r.Context(), event); err != nil {
			writeProblem(w, http.StatusInternalServerError, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
				Title:  http.StatusText(http.StatusInternalServerError),
				Detail: "webhook handler failed",
			})
			return
		}
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func verificationErrorDetail(format func(error) string, err error) string {
	if format == nil {
		return defaultVerificationFailureDetail
	}
	if detail := strings.TrimSpace(format(err)); detail != "" {
		return detail
	}
	return defaultVerificationFailureDetail
}

// DecodeJSON decodes a webhook payload as JSON.
func DecodeJSON[T any](body []byte) (T, error) {
	var out T
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&out); err != nil {
		return out, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return out, fmt.Errorf("webhook payload must contain a single JSON value")
	}
	return out, nil
}

// WriteProblem writes a webhook Problem Details response.
func WriteProblem(w http.ResponseWriter, status int, problem httpx.Problem) {
	if problem.Title == "" {
		problem.Title = http.StatusText(status)
	}
	httpx.WriteProblem(w, status, problem)
}

func readBody(w http.ResponseWriter, r *http.Request, maxBodyBytes int64) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, fmt.Errorf("request body is required")
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func decodeSignature(signature string, encoding SignatureEncoding) ([]byte, error) {
	switch encoding {
	case SignatureEncodingHex:
		return hex.DecodeString(signature)
	case SignatureEncodingBase64:
		return base64.StdEncoding.DecodeString(signature)
	case SignatureEncodingBase64URL:
		return base64.RawURLEncoding.DecodeString(signature)
	default:
		return nil, fmt.Errorf("unsupported signature encoding")
	}
}
