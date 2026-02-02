package jsonmw

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const (
	contentType     = "Content-Type"
	applicationJSON = "application/json"
)

// Middleware enforces JSON content-type requirements.
type Middleware struct {
	RequireJSON bool
}

// Options configures the JSON middleware.
type Options struct {
	RequireJSON bool
}

// New constructs a JSON middleware with the given requirement.
func New(opts Options) (*Middleware, error) {
	return &Middleware{RequireJSON: opts.RequireJSON}, nil
}

// Middleware implements ports.Middleware by returning the Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with JSON content checks.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	if !m.RequireJSON {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get(contentType)
		if ct == "" {
			ct = applicationJSON
		}
		if !isJSON(ct) {
			http.Error(
				w,
				contentType+" must be "+applicationJSON,
				http.StatusUnsupportedMediaType,
			)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// StrictDecoder creates a JSON decoder that disallows unknown fields.
func StrictDecoder(r *http.Request) (*json.Decoder, error) {
	if r.Body == nil {
		return nil, errors.New("empty body")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec, nil
}

func isJSON(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, applicationJSON) ||
		strings.HasSuffix(ct, "+json")
}
