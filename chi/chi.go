package chi

import (
	"net/http"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// ChiRouter wraps chi.Router to implement our interface.
type ChiRouter struct {
	*chi.Mux
}

// New creates a new chi router that implements ports.HTTPRouter.
func New() ports.HTTPRouter {
	return &ChiRouter{Mux: chi.NewRouter()}
}

// NewMux creates a new chi.Mux directly.
func NewMux() *chi.Mux {
	return chi.NewRouter()
}

// Middleware provides common middleware functions.
type Middleware struct{}

// NewMiddleware creates a new middleware instance that implements ports.HTTPMiddleware.
func NewMiddleware() ports.HTTPMiddleware {
	return &Middleware{}
}

// RequestID returns the request ID middleware.
func (m *Middleware) RequestID() func(http.Handler) http.Handler {
	return middleware.RequestID
}

// RealIP returns the real IP middleware.
func (m *Middleware) RealIP() func(http.Handler) http.Handler {
	return middleware.RealIP
}

// Recoverer returns the recoverer middleware.
func (m *Middleware) Recoverer() func(http.Handler) http.Handler {
	return middleware.Recoverer
}

// URLParamExtractor implements ports.URLParamExtractor.
type URLParamExtractor struct{}

// NewURLParamExtractor creates a new URL parameter extractor.
func NewURLParamExtractor() ports.URLParamExtractor {
	return &URLParamExtractor{}
}

// URLParam extracts URL parameters from the request context.
func (u *URLParamExtractor) URLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// URLParam is a convenience function for direct usage.
func URLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}
