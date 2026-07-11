package tenant

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/v3/authorization"
	"github.com/aatuh/api-toolkit/v3/httpx"
)

var (
	errNoSourcesConfigured = errors.New("tenant sources not configured")
	errTenantMismatch      = errors.New("tenant scope mismatch")
	errTenantMissing       = errors.New("tenant scope missing")
	errTenantHeaderInvalid = errors.New("tenant header is invalid")
)

// TenantFromContext extracts a tenant identifier from context.
type TenantFromContext func(ctx context.Context) (string, bool)

// ErrorHandler handles authorization errors from the tenant middleware.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// URLParamExtractor retrieves a path parameter from the current request.
type URLParamExtractor interface {
	URLParam(r *http.Request, key string) string
}

// Options configures tenant scoping.
type Options struct {
	Optional          bool
	HeaderName        string
	URLParam          string
	URLParamExtractor URLParamExtractor
	TenantFromContext TenantFromContext
	// RequireAllSources requires every configured tenant source to be present
	// and to agree. Use it when a route must prove the request tenant matches
	// the authenticated tenant scope.
	RequireAllSources bool
	ErrorHandler      ErrorHandler
}

// Middleware enforces tenant scoping and stores the tenant in context.
type Middleware struct {
	opts Options
}

// New creates a tenant scoping middleware.
func New(opts Options) (*Middleware, error) {
	opts.HeaderName = strings.TrimSpace(opts.HeaderName)
	opts.URLParam = strings.TrimSpace(opts.URLParam)
	sources := 0
	if opts.TenantFromContext != nil {
		sources++
	}
	if opts.HeaderName != "" {
		sources++
	}
	if opts.URLParam != "" {
		sources++
		if opts.URLParamExtractor == nil {
			return nil, errors.New("url param extractor not configured")
		}
	}
	if sources == 0 {
		return nil, errNoSourcesConfigured
	}
	return &Middleware{opts: opts}, nil
}

// Handler wraps the next handler with tenant checks.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, err := m.tenantFromRequest(r)
		if err != nil {
			m.onError(w, r, err)
			return
		}
		if tenant == "" {
			if m.opts.Optional {
				next.ServeHTTP(w, r)
				return
			}
			m.onError(w, r, errTenantMissing)
			return
		}
		scope, _ := authorization.ScopeFromContext(r.Context())
		scope.TenantID = tenant
		ctx := authorization.WithScope(r.Context(), scope)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) tenantFromRequest(r *http.Request) (string, error) {
	if r == nil {
		return "", errors.New("request is nil")
	}
	if m == nil {
		return "", errors.New("tenant middleware is nil")
	}
	sources := 0
	var values []string

	if m.opts.TenantFromContext != nil {
		sources++
		if tenant, ok := m.opts.TenantFromContext(r.Context()); ok {
			tenant = strings.TrimSpace(tenant)
			if tenant != "" {
				values = append(values, tenant)
			}
		}
	}
	if m.opts.HeaderName != "" {
		sources++
		tenant, err := tenantFromHeader(r, m.opts.HeaderName)
		if err != nil {
			return "", err
		}
		if tenant != "" {
			values = append(values, tenant)
		}
	}
	if m.opts.URLParam != "" {
		sources++
		if m.opts.URLParamExtractor == nil {
			return "", errors.New("url param extractor not configured")
		}
		tenant := strings.TrimSpace(m.opts.URLParamExtractor.URLParam(r, m.opts.URLParam))
		if tenant != "" {
			if strings.ContainsAny(tenant, " \t") {
				return "", errTenantHeaderInvalid
			}
			values = append(values, tenant)
		}
	}
	if sources == 0 {
		return "", errNoSourcesConfigured
	}
	if len(values) == 0 {
		return "", nil
	}
	if m.opts.RequireAllSources && len(values) != sources {
		return "", errTenantMissing
	}
	expected := values[0]
	for _, value := range values[1:] {
		if value != expected {
			return "", errTenantMismatch
		}
	}
	return expected, nil
}

func (m *Middleware) onError(w http.ResponseWriter, r *http.Request, err error) {
	if m == nil {
		return
	}
	if m.opts.ErrorHandler != nil {
		m.opts.ErrorHandler(w, r, err)
		return
	}
	detail := "tenant scope invalid"
	switch {
	case errors.Is(err, errTenantMissing):
		if isAuthenticatedRequest(r) {
			detail = "tenant scope required"
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeForbidden),
				Title:  http.StatusText(http.StatusForbidden),
				Detail: detail,
			})
			return
		}
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
			Title:  http.StatusText(http.StatusUnauthorized),
			Detail: "authentication required",
		})
		return
	case errors.Is(err, errTenantMismatch):
		detail = "tenant scope mismatch"
	case errors.Is(err, errTenantHeaderInvalid):
		detail = "tenant scope invalid"
	case errors.Is(err, errNoSourcesConfigured):
		detail = "tenant scope misconfigured"
	}
	httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeForbidden),
		Title:  http.StatusText(http.StatusForbidden),
		Detail: detail,
	})
}

func isAuthenticatedRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	_, ok := authorization.ActorFromContext(r.Context())
	return ok
}

func tenantFromHeader(r *http.Request, headerName string) (string, error) {
	if r == nil {
		return "", errors.New("request is nil")
	}
	values := r.Header.Values(headerName)
	if len(values) > 1 {
		return "", errTenantHeaderInvalid
	}
	if len(values) == 0 {
		return "", nil
	}
	val := strings.TrimSpace(values[0])
	if val == "" {
		return "", nil
	}
	if strings.Contains(val, ",") {
		return "", errTenantHeaderInvalid
	}
	if strings.ContainsAny(val, " \t") {
		return "", errTenantHeaderInvalid
	}
	return val, nil
}
