package objectstore

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const (
	defaultSignedURLExpiry = 15 * time.Minute
	maxSignedURLExpiry     = 7 * 24 * time.Hour
)

var (
	// ErrInvalidRef reports that an object bucket or key is empty or unsafe.
	ErrInvalidRef = errors.New("invalid object reference")
	// ErrInvalidObject reports that an object body or metadata is invalid.
	ErrInvalidObject = errors.New("invalid object")
	// ErrObjectTooLarge reports that an object exceeds the configured size limit.
	ErrObjectTooLarge = errors.New("object too large")
	// ErrContentTypeNotAllowed reports that an object content type is outside policy.
	ErrContentTypeNotAllowed = errors.New("object content type not allowed")
	// ErrUnsafeMetadata reports that object metadata appears to contain a secret.
	ErrUnsafeMetadata = errors.New("unsafe object metadata")
	// ErrObjectNotFound reports that an object does not exist.
	ErrObjectNotFound = errors.New("object not found")
	// ErrInvalidSignedURL reports invalid signed URL options.
	ErrInvalidSignedURL = errors.New("invalid signed url options")
)

// Ref identifies an object in a bucket.
type Ref struct {
	Bucket string
	Key    string
}

// PutOptions describes object write policy inputs.
type PutOptions struct {
	Size        int64
	ContentType string
	Metadata    map[string]string
}

// GetResult describes an object read result. Callers must close Body.
type GetResult struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
	Metadata    map[string]string
	ETag        string
}

// SignedURLOptions configures a time-limited object URL.
type SignedURLOptions struct {
	Method      string
	Expires     time.Duration
	ContentType string
}

// Store is the minimal object storage contract used by contrib adapters and generated services.
type Store interface {
	Put(ctx context.Context, ref Ref, body io.Reader, opts PutOptions) error
	Get(ctx context.Context, ref Ref) (GetResult, error)
	Delete(ctx context.Context, ref Ref) error
}

// SignedURLer is implemented by stores that can issue time-limited object URLs.
type SignedURLer interface {
	SignedURL(ctx context.Context, ref Ref, opts SignedURLOptions) (string, error)
}

// Policy enforces object size and content-type constraints.
type Policy struct {
	MaxObjectSize       int64
	AllowedContentTypes []string
}

// ValidateRef rejects empty buckets, unsafe bucket names, and traversal-shaped keys.
func ValidateRef(ref Ref) error {
	bucket := strings.TrimSpace(ref.Bucket)
	key := strings.TrimSpace(ref.Key)
	if !validBucket(bucket) || !validKey(key) {
		return ErrInvalidRef
	}
	return nil
}

// ValidatePut enforces size and content type limits.
func (p Policy) ValidatePut(opts PutOptions) error {
	if p.MaxObjectSize > 0 && opts.Size > p.MaxObjectSize {
		return ErrObjectTooLarge
	}
	if len(p.AllowedContentTypes) == 0 {
		return nil
	}
	got := normalizeContentType(opts.ContentType)
	if got == "" {
		return ErrContentTypeNotAllowed
	}
	for _, allowed := range p.AllowedContentTypes {
		if got == normalizeContentType(allowed) {
			return nil
		}
	}
	return ErrContentTypeNotAllowed
}

// SafeMetadata validates metadata and returns a defensive copy.
func SafeMetadata(metadata map[string]string) (map[string]string, error) {
	if metadata == nil {
		return map[string]string{}, nil
	}
	clone := make(map[string]string, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || strings.ContainsAny(key, "\r\n:") || containsControl(key) || containsControl(value) {
			return nil, ErrInvalidObject
		}
		if unsafeMetadataToken(key) || unsafeMetadataToken(value) {
			return nil, ErrUnsafeMetadata
		}
		clone[key] = value
	}
	return clone, nil
}

// ReadAtMost reads a body into memory while enforcing a maximum size when maxBytes is positive.
func ReadAtMost(body io.Reader, maxBytes int64) ([]byte, int64, error) {
	if body == nil {
		return nil, 0, ErrInvalidObject
	}
	if maxBytes <= 0 {
		payload, err := io.ReadAll(body)
		return payload, int64(len(payload)), err
	}
	limited := io.LimitReader(body, maxBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, 0, err
	}
	if int64(len(payload)) > maxBytes {
		return nil, 0, ErrObjectTooLarge
	}
	return payload, int64(len(payload)), nil
}

// NormalizeSignedURLOptions applies defaults and validates a signed URL request.
func NormalizeSignedURLOptions(opts SignedURLOptions) (SignedURLOptions, error) {
	method := strings.ToUpper(strings.TrimSpace(opts.Method))
	if method == "" {
		method = http.MethodGet
	}
	switch method {
	case http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodHead:
	default:
		return SignedURLOptions{}, ErrInvalidSignedURL
	}
	expires := opts.Expires
	if expires <= 0 {
		expires = defaultSignedURLExpiry
	}
	if expires > maxSignedURLExpiry {
		return SignedURLOptions{}, ErrInvalidSignedURL
	}
	opts.Method = method
	opts.Expires = expires
	opts.ContentType = strings.TrimSpace(opts.ContentType)
	return opts, nil
}

func validBucket(bucket string) bool {
	if len(bucket) < 3 || len(bucket) > 63 {
		return false
	}
	if bucket[0] == '.' || bucket[0] == '-' || bucket[len(bucket)-1] == '.' || bucket[len(bucket)-1] == '-' {
		return false
	}
	for _, r := range bucket {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '.' || r == '-' {
			continue
		}
		return false
	}
	return !strings.Contains(bucket, "..")
}

func validKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, `\`) || containsControl(key) {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func normalizeContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return strings.ToLower(mediaType)
}

func unsafeMetadataToken(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	for _, token := range []string{
		"authorization",
		"bearer ",
		"cookie",
		"password",
		"private_key",
		"secret",
		"set-cookie",
		"token",
		"api_key",
		"apikey",
		"pepper",
	} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}
