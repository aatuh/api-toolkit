package jsonmw

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/v2/httpx"
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
		if !shouldRequireJSONContentType(r) {
			next.ServeHTTP(w, r)
			return
		}
		ct := r.Header.Get(contentType)
		if ct == "" {
			ct = applicationJSON
		}
		if !isJSON(ct) {
			httpx.WriteProblem(w, http.StatusUnsupportedMediaType, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeUnsupportedMedia),
				Title:  http.StatusText(http.StatusUnsupportedMediaType),
				Detail: contentType + " must be " + applicationJSON,
			})
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
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	return mediaType == applicationJSON || strings.HasSuffix(mediaType, "+json")
}

func shouldRequireJSONContentType(r *http.Request) bool {
	if r == nil {
		return false
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return false
	}
	return r.ContentLength > 0 || len(r.TransferEncoding) > 0
}
