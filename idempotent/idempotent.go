package idempotent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/operations"
)

const defaultKeyHeader = "Idempotency-Key"

// CreateConfig describes idempotency requirements for create workflows.
type CreateConfig struct {
	HeaderName string
	Required   bool
}

// UpdateConfig describes idempotency requirements for update workflows.
type UpdateConfig struct {
	HeaderName string
	Required   bool
}

// AsyncConfig describes an idempotent async replay response.
type AsyncConfig struct {
	ID         string
	Location   string
	RetryAfter time.Duration
}

// RequireKey extracts a required idempotency key from the request.
func RequireKey(r *http.Request, headerName string) (string, error) {
	if strings.TrimSpace(headerName) == "" {
		headerName = defaultKeyHeader
	}
	if r == nil {
		return "", fmt.Errorf("request is required")
	}
	key := strings.TrimSpace(r.Header.Get(headerName))
	if key == "" {
		return "", fmt.Errorf("%s header is required", headerName)
	}
	return key, nil
}

// RequestHash returns a deterministic SHA-256 hash for method, target, and body bytes.
func RequestHash(r *http.Request, body []byte) string {
	h := sha256.New()
	if r != nil {
		_, _ = h.Write([]byte(strings.ToUpper(r.Method)))
		_, _ = h.Write([]byte("\n"))
		if r.URL != nil {
			_, _ = h.Write([]byte(r.URL.RequestURI()))
		}
	}
	_, _ = h.Write([]byte("\n"))
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// ConflictProblem returns the standard idempotency conflict problem.
func ConflictProblem(detail string) httpx.Problem {
	if strings.TrimSpace(detail) == "" {
		detail = "idempotency key was reused with a different request"
	}
	return httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeConflict), Title: http.StatusText(http.StatusConflict), Detail: detail}
}

// ReplayProblem returns the standard idempotency replay problem.
func ReplayProblem(detail string) httpx.Problem {
	if strings.TrimSpace(detail) == "" {
		detail = "idempotent response was replayed"
	}
	return httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeConflict), Title: "Idempotent replay", Detail: detail}
}

// WriteConflict writes a 409 idempotency conflict Problem Details response.
func WriteConflict(w http.ResponseWriter, detail string) {
	httpx.WriteProblem(w, http.StatusConflict, ConflictProblem(detail))
}

// WriteAcceptedReplay writes a replayed 202 Accepted operation response.
func WriteAcceptedReplay(w http.ResponseWriter, config AsyncConfig) {
	w.Header().Set("Idempotency-Replayed", "true")
	operations.WriteAccepted(w, operations.AcceptedConfig{ID: config.ID, Location: config.Location, RetryAfter: config.RetryAfter})
}

// OperationExtensions returns OpenAPI extensions for idempotent operation contracts.
func OperationExtensions(required bool) map[string]any {
	return map[string]any{
		"x-idempotency-key": map[string]any{
			"required": required,
			"header":   defaultKeyHeader,
		},
	}
}
