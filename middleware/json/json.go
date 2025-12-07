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

type Middleware struct {
	RequireJSON bool
}

func New(require bool) *Middleware { return &Middleware{RequireJSON: require} }

// Middleware implements ports.Middleware by returning the Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
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
