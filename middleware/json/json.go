package jsonmw

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Middleware struct {
	RequireJSON bool
}

func New(require bool) *Middleware { return &Middleware{RequireJSON: require} }

func (m *Middleware) Handler(next http.Handler) http.Handler {
	if !m.RequireJSON {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			http.Error(w, "missing content-type", http.StatusUnsupportedMediaType)
			return
		}
		if !isJSON(ct) {
			http.Error(w, "content-type must be application/json",
				http.StatusUnsupportedMediaType)
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
	return strings.Contains(ct, "application/json") ||
		strings.HasSuffix(ct, "+json")
}
