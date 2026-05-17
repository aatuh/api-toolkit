package httpcache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v3/httpx"
)

// ETag is an HTTP entity tag, including quotes and optional weak prefix.
type ETag string

// String returns the wire representation of the ETag.
func (e ETag) String() string {
	return string(e)
}

// StrongETag builds a strong ETag from an opaque value.
func StrongETag(value string) ETag {
	if parsed, err := ParseETag(value); err == nil {
		if parsed.IsWeak() {
			return ETag(strings.TrimPrefix(parsed.String(), "W/"))
		}
		return parsed
	}
	return ETag(`"` + escapeETagValue(value) + `"`)
}

// WeakETag builds a weak ETag from an opaque value.
func WeakETag(value string) ETag {
	if parsed, err := ParseETag(value); err == nil {
		if parsed.IsWeak() {
			return parsed
		}
		return ETag("W/" + parsed.String())
	}
	return ETag(`W/"` + escapeETagValue(value) + `"`)
}

// HashETag builds a strong ETag from a SHA-256 hash of data.
func HashETag(data []byte) ETag {
	sum := sha256.Sum256(data)
	return StrongETag(hex.EncodeToString(sum[:]))
}

// ParseETag validates and normalizes a single ETag.
func ParseETag(value string) (ETag, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("etag is required")
	}
	if strings.HasPrefix(value, "W/") {
		inner, err := parseQuotedETag(strings.TrimPrefix(value, "W/"))
		if err != nil {
			return "", err
		}
		return ETag("W/" + inner), nil
	}
	inner, err := parseQuotedETag(value)
	if err != nil {
		return "", err
	}
	return ETag(inner), nil
}

// IsWeak reports whether the ETag is weak.
func (e ETag) IsWeak() bool {
	return strings.HasPrefix(e.String(), "W/")
}

// Validators are the current validators for a representation.
type Validators struct {
	ETag         ETag
	LastModified time.Time
}

// Decision describes the result of evaluating conditional request headers.
type Decision struct {
	Status             int
	NotModified        bool
	PreconditionFailed bool
}

// EvaluateRead evaluates conditional read headers for GET or HEAD handlers.
func EvaluateRead(r *http.Request, validators Validators) Decision {
	if r == nil {
		return Decision{}
	}
	if header := r.Header.Get("If-None-Match"); strings.TrimSpace(header) != "" {
		if etagMatches(header, validators.ETag, false) {
			return Decision{Status: http.StatusNotModified, NotModified: true}
		}
		return Decision{}
	}
	if validators.LastModified.IsZero() {
		return Decision{}
	}
	if header := r.Header.Get("If-Modified-Since"); strings.TrimSpace(header) != "" {
		since, err := http.ParseTime(header)
		if err == nil && !truncateHTTPTime(validators.LastModified).After(since) {
			return Decision{Status: http.StatusNotModified, NotModified: true}
		}
	}
	return Decision{}
}

// EvaluateWrite evaluates write preconditions for unsafe handlers.
func EvaluateWrite(r *http.Request, validators Validators) Decision {
	if r == nil {
		return Decision{}
	}
	if header := r.Header.Get("If-Match"); strings.TrimSpace(header) != "" {
		if !etagMatches(header, validators.ETag, true) {
			return Decision{Status: http.StatusPreconditionFailed, PreconditionFailed: true}
		}
		return Decision{}
	}
	if validators.LastModified.IsZero() {
		return Decision{}
	}
	if header := r.Header.Get("If-Unmodified-Since"); strings.TrimSpace(header) != "" {
		since, err := http.ParseTime(header)
		if err == nil && truncateHTTPTime(validators.LastModified).After(since) {
			return Decision{Status: http.StatusPreconditionFailed, PreconditionFailed: true}
		}
	}
	return Decision{}
}

// SetValidators writes ETag and Last-Modified response headers when present.
func SetValidators(w http.ResponseWriter, validators Validators) {
	if w == nil {
		return
	}
	if validators.ETag != "" {
		w.Header().Set("ETag", validators.ETag.String())
	}
	if !validators.LastModified.IsZero() {
		w.Header().Set("Last-Modified", truncateHTTPTime(validators.LastModified).UTC().Format(http.TimeFormat))
	}
}

// WriteNotModified writes a 304 response with validators and no body.
func WriteNotModified(w http.ResponseWriter, validators Validators) {
	SetValidators(w, validators)
	w.WriteHeader(http.StatusNotModified)
}

// WritePreconditionFailed writes a 412 Problem Details response.
func WritePreconditionFailed(w http.ResponseWriter) {
	httpx.WriteProblem(w, http.StatusPreconditionFailed, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeConflict),
		Title:  http.StatusText(http.StatusPreconditionFailed),
		Detail: "conditional request precondition failed",
	})
}

func parseQuotedETag(value string) (string, error) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", fmt.Errorf("etag must be quoted")
	}
	for _, r := range value[1 : len(value)-1] {
		if r == '"' || r < 0x21 || r == 0x7f {
			return "", fmt.Errorf("etag contains invalid characters")
		}
	}
	return value, nil
}

func escapeETagValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.ReplaceAll(value, `\`, "")
	value = strings.ReplaceAll(value, `"`, "")
	if value == "" {
		value = "0"
	}
	return value
}

func etagMatches(header string, current ETag, strong bool) bool {
	current = normalizeCurrentETag(current)
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" {
			return current != ""
		}
		candidate, err := ParseETag(part)
		if err != nil {
			continue
		}
		if strong {
			if !candidate.IsWeak() && !current.IsWeak() && candidate == current {
				return true
			}
			continue
		}
		if opaqueETag(candidate) == opaqueETag(current) {
			return true
		}
	}
	return false
}

func normalizeCurrentETag(current ETag) ETag {
	if current == "" {
		return ""
	}
	if parsed, err := ParseETag(current.String()); err == nil {
		return parsed
	}
	return ""
}

func opaqueETag(value ETag) string {
	return strings.TrimPrefix(value.String(), "W/")
}

func truncateHTTPTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Second)
}
