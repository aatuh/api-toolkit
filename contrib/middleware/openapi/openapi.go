// Package openapi provides OpenAPI request and response validation middleware.
package openapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
)

// ErrorHandler handles OpenAPI validation failures.
type ErrorHandler func(http.ResponseWriter, *http.Request, int, error)

// Options configures request validation middleware.
type Options struct {
	Spec               *openapi3.T
	FilterOptions      openapi3filter.Options
	ErrorHandler       ErrorHandler
	IgnoreNotFound     bool
	ResponseValidation ResponseValidationOptions
}

// Option customizes middleware behavior.
type Option func(*Options)

// WithSpec sets the OpenAPI spec to validate against.
func WithSpec(spec *openapi3.T) Option {
	return func(o *Options) {
		o.Spec = spec
	}
}

// WithFilterOptions overrides request validation options.
func WithFilterOptions(opts openapi3filter.Options) Option {
	return func(o *Options) {
		o.FilterOptions = opts
	}
}

// WithErrorHandler overrides the error writer.
func WithErrorHandler(fn ErrorHandler) Option {
	return func(o *Options) {
		o.ErrorHandler = fn
	}
}

// WithIgnoreNotFound controls how unmatched routes are handled.
func WithIgnoreNotFound(ignore bool) Option {
	return func(o *Options) {
		o.IgnoreNotFound = ignore
	}
}

// ResponseValidationOptions configures response validation.
type ResponseValidationOptions struct {
	Enabled      bool
	MaxBodyBytes int64
	ErrorHandler ErrorHandler
}

// WithResponseValidation configures response validation.
func WithResponseValidation(opts ResponseValidationOptions) Option {
	return func(o *Options) {
		o.ResponseValidation = opts
	}
}

// Middleware validates requests against an OpenAPI spec.
type Middleware struct {
	router             routers.Router
	filterOpts         *openapi3filter.Options
	errorHandler       ErrorHandler
	ignore404          bool
	responseValidation *responseValidation
}

// New constructs a validation middleware from a spec.
func New(spec *openapi3.T, opts ...Option) (*Middleware, error) {
	cfg := Options{
		Spec: spec,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.Spec == nil {
		return nil, errors.New("openapi spec is required")
	}
	if err := cfg.Spec.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate openapi spec: %w", err)
	}
	router, err := legacyrouter.NewRouter(cfg.Spec)
	if err != nil {
		return nil, fmt.Errorf("build openapi router: %w", err)
	}
	handler := cfg.ErrorHandler
	if handler == nil {
		handler = defaultErrorHandler
	}
	respValidation := newResponseValidation(cfg.ResponseValidation, handler)
	return &Middleware{
		router:             router,
		filterOpts:         &cfg.FilterOptions,
		errorHandler:       handler,
		ignore404:          cfg.IgnoreNotFound,
		responseValidation: respValidation,
	}, nil
}

// NewFromFile loads and validates a spec from disk.
func NewFromFile(path string, opts ...Option) (*Middleware, error) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, err
	}
	return New(spec, opts...)
}

// Middleware implements ports.Middleware by returning the handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.Handler
}

// Handler wraps the next handler with OpenAPI request validation.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.router == nil {
			next.ServeHTTP(w, r)
			return
		}
		route, pathParams, err := m.router.FindRoute(r)
		if err != nil {
			if m.ignore404 {
				next.ServeHTTP(w, r)
				return
			}
			status := http.StatusNotFound
			if errors.Is(err, routers.ErrMethodNotAllowed) {
				status = http.StatusMethodNotAllowed
			}
			m.errorHandler(w, r, status, err)
			return
		}
		input := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
			Options:    m.filterOpts,
		}
		if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
			status := statusFromError(err)
			m.errorHandler(w, r, status, err)
			return
		}
		if m.responseValidation == nil {
			next.ServeHTTP(w, r)
			return
		}
		capture := newResponseCapture(m.responseValidation.maxBodyBytes)
		next.ServeHTTP(capture, r)
		if capture.TooLarge() {
			m.responseValidation.errorHandler(w, r, http.StatusInternalServerError, errResponseTooLarge)
			return
		}
		respInput := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: input,
			Status:                 capture.Status(),
			Header:                 capture.Header(),
			Options:                m.filterOpts,
		}
		respInput.SetBodyBytes(capture.Body())
		if err := openapi3filter.ValidateResponse(r.Context(), respInput); err != nil {
			m.responseValidation.errorHandler(w, r, http.StatusInternalServerError, err)
			return
		}
		capture.WriteTo(w)
	})
}

func statusFromError(err error) int {
	return statusFromOpenAPIError(err)
}
