package list

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
)

// CursorCodec encodes and decodes opaque cursor values.
type CursorCodec interface {
	Encode(values map[string]string, expiresAt time.Time) (string, error)
	Decode(cursor string) (map[string]string, error)
}

// CursorQuery captures cursor pagination input.
type CursorQuery struct {
	Limit  int
	Cursor string
	Values map[string]string
	Raw    map[string][]string
}

// CursorQueryConfig configures cursor query parsing.
type CursorQueryConfig struct {
	DefaultLimit int
	MaxLimit     int
	CursorParam  string
	LimitParam   string
	Codec        CursorCodec
}

// CursorMeta captures cursor pagination metadata.
type CursorMeta struct {
	Count      int    `json:"count"`
	Limit      int    `json:"limit"`
	Cursor     string `json:"cursor,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// CursorResponse wraps list results with cursor pagination metadata.
type CursorResponse[T any] struct {
	Data []T        `json:"data"`
	Meta CursorMeta `json:"meta"`
}

// ParseCursorQueryChecked parses cursor pagination parameters.
func ParseCursorQueryChecked(r *http.Request, cfg CursorQueryConfig) (CursorQuery, error) {
	values := r.URL.Query()
	limitParam := strings.TrimSpace(cfg.LimitParam)
	if limitParam == "" {
		limitParam = "limit"
	}
	cursorParam := strings.TrimSpace(cfg.CursorParam)
	if cursorParam == "" {
		cursorParam = "cursor"
	}
	limit, limitErrs := parseLimit(values.Get(limitParam), cfg.DefaultLimit, cfg.MaxLimit)
	cursor := strings.TrimSpace(values.Get(cursorParam))
	var decoded map[string]string
	var errs fielderrors.FieldErrors
	errs = append(errs, limitErrs...)
	if cursor != "" && cfg.Codec != nil {
		var err error
		decoded, err = cfg.Codec.Decode(cursor)
		if err != nil {
			errs = append(errs, fielderrors.FieldError{
				Field:   cursorParam,
				Code:    "invalid",
				Message: "cursor is invalid",
			})
		}
	}
	query := CursorQuery{
		Limit:  limit,
		Cursor: cursor,
		Values: decoded,
		Raw:    map[string][]string(values),
	}
	if len(errs) > 0 {
		return query, errs
	}
	return query, nil
}

// NewCursorResponse constructs a CursorResponse with paging metadata.
func NewCursorResponse[T any](items []T, nextCursor string, query CursorQuery) CursorResponse[T] {
	return CursorResponse[T]{
		Data: items,
		Meta: CursorMeta{
			Count:      len(items),
			Limit:      query.Limit,
			Cursor:     query.Cursor,
			NextCursor: nextCursor,
		},
	}
}

// NewHMACCursorCodec returns a URL-safe JSON+HMAC cursor codec.
func NewHMACCursorCodec(secret []byte) CursorCodec {
	cp := append([]byte(nil), secret...)
	return hmacCursorCodec{secret: cp}
}

type hmacCursorCodec struct {
	secret []byte
}

type cursorPayload struct {
	Version   int               `json:"v"`
	Values    map[string]string `json:"values,omitempty"`
	ExpiresAt int64             `json:"exp,omitempty"`
}

func (c hmacCursorCodec) Encode(values map[string]string, expiresAt time.Time) (string, error) {
	if len(c.secret) == 0 {
		return "", errors.New("cursor secret is required")
	}
	payload := cursorPayload{
		Version: 1,
		Values:  cloneStringMap(values),
	}
	if !expiresAt.IsZero() {
		payload.ExpiresAt = expiresAt.Unix()
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sig := c.sign(data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (c hmacCursorCodec) Decode(cursor string) (map[string]string, error) {
	if len(c.secret) == 0 {
		return nil, errors.New("cursor secret is required")
	}
	left, right, ok := strings.Cut(strings.TrimSpace(cursor), ".")
	if !ok || left == "" || right == "" {
		return nil, errors.New("cursor must contain payload and signature")
	}
	data, err := base64.RawURLEncoding.DecodeString(left)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(right)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(sig, c.sign(data)) {
		return nil, errors.New("cursor signature mismatch")
	}
	var payload cursorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.Version != 1 {
		return nil, errors.New("unsupported cursor version")
	}
	if payload.ExpiresAt > 0 && time.Now().Unix() > payload.ExpiresAt {
		return nil, errors.New("cursor expired")
	}
	return cloneStringMap(payload.Values), nil
}

func (c hmacCursorCodec) sign(data []byte) []byte {
	mac := hmac.New(sha256.New, c.secret)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
