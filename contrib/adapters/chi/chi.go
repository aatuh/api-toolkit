package chi

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/aatuh/api-toolkit/v2/middleware/auth/authz"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// ChiRouter wraps chi.Router to implement our interface.
//
//revive:disable-next-line:exported
type ChiRouter struct {
	*chi.Mux
}

var _ ports.HTTPRouter = (*ChiRouter)(nil)

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

var _ ports.HTTPMiddleware = (*Middleware)(nil)

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

var _ ports.URLParamExtractor = (*URLParamExtractor)(nil)

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

// MiddlewareSpecResolver resolves a Chi route handler to an authorization middleware.
//
// Route and method context are provided for actionable diagnostics and stable
// migration checks.
//
// Returning nil means the route is treated as unprotected and skipped unless
// ValidateRequireRoleMiddlewareRoutesStrict is used.
type MiddlewareSpecResolver func(method, route string, handler http.Handler) *authz.RequireRoleMiddleware

// MiddlewareSpecResolution allows explicit classification for route-level validation.
type MiddlewareSpecResolution struct {
	// Middleware is the resolved route middleware.
	Middleware *authz.RequireRoleMiddleware
	// SkipFromValidation skips this route during authz contract validation.
	SkipFromValidation bool
}

// MiddlewareSpecResolverWithCoverage resolves a Chi route handler with explicit
// include/exclude intent.
//
// Return SkipFromValidation=true for public routes and false with a concrete
// middleware for protected routes.
type MiddlewareSpecResolverWithCoverage func(method, route string, handler http.Handler) MiddlewareSpecResolution

const (
	authzBootstrapStrictEnv = "API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT"
)

const (
	envValueStrict  = "strict"
	envValueEnabled = "enabled"
	envValueTrue    = "true"
	envValueOn      = "on"
	envValue1       = "1"

	envValuePermissive = "permissive"
	envValueDisabled   = "disabled"
	envValueFalse      = "false"
	envValueOff        = "off"
	envValue0          = "0"

	envModeCI         = "ci"
	envModeProduction = "production"
	envModeProd       = "prod"
	envModeStaging    = "staging"
	envModeStage      = "stage"
)

// ValidateRequireRoleMiddlewareRoutes validates authz middleware coverage for chi routes.
//
// The helper traverses chi's registered route tree, converts each method+pattern
// pair into a RequireRoleRouteSpec, and validates all entries in one startup pass.
//
// By default, returning nil from the legacy resolver skips validation for that
// route to support mixed public/protected routers.
func ValidateRequireRoleMiddlewareRoutes(router *chi.Mux, resolve MiddlewareSpecResolver) error {
	return validateRequireRoleMiddlewareRoutes(router, newLegacyMiddlewareSpecResolver(resolve, false))
}

// ValidateRequireRoleMiddlewareRoutesAuto validates authz route coverage using the
// default deployment policy.
//
// The default is permissive in local/dev and strict in CI-like environments.
// Set API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT to explicitly override.
func ValidateRequireRoleMiddlewareRoutesAuto(router *chi.Mux, resolve MiddlewareSpecResolver) error {
	return ValidateRequireRoleMiddlewareRoutesAutoWithLogger(router, resolve, nil)
}

// ValidateRequireRoleMiddlewareRoutesAutoWithLogger validates authz route coverage
// using the default deployment policy and emits startup intent logs.
func ValidateRequireRoleMiddlewareRoutesAutoWithLogger(router *chi.Mux, resolve MiddlewareSpecResolver, log ports.Logger) error {
	if log == nil {
		log = ports.NopLogger{}
	}
	strict, source := authzBootstrapStrictMode()
	if log != nil {
		log.Info(
			"authz route bootstrap validation policy",
			"mode", policyModeName(strict),
			"source", source,
		)
	}
	return validateRequireRoleMiddlewareRoutes(router, newLegacyMiddlewareSpecResolver(resolve, strict))
}

// ValidateRequireRoleMiddlewareRoutesStrict validates authz middleware coverage and
// treats nil middleware as a validation failure.
//
// This is useful during migration windows when every checked route must declare a
// middleware explicitly.
func ValidateRequireRoleMiddlewareRoutesStrict(router *chi.Mux, resolve MiddlewareSpecResolver) error {
	return validateRequireRoleMiddlewareRoutes(router, newLegacyMiddlewareSpecResolver(resolve, true))
}

// ValidateRequireRoleMiddlewareRoutesWithCoverage validates routes using an
// explicit resolver that can distinguish protected routes from public routes.
func ValidateRequireRoleMiddlewareRoutesWithCoverage(router *chi.Mux, resolve MiddlewareSpecResolverWithCoverage) error {
	return ValidateRequireRoleMiddlewareRoutesWithCoverageAndLogger(router, resolve, nil)
}

// ValidateRequireRoleMiddlewareRoutesWithCoverageAndLogger validates routes using
// an explicit resolver and emits startup intent logs.
func ValidateRequireRoleMiddlewareRoutesWithCoverageAndLogger(
	router *chi.Mux,
	resolve MiddlewareSpecResolverWithCoverage,
	log ports.Logger,
) error {
	if log == nil {
		log = ports.NopLogger{}
	}
	return validateRequireRoleMiddlewareRoutes(router, coverageMiddlewareSpecResolver{resolver: resolve, log: log})
}

func validateRequireRoleMiddlewareRoutes(router *chi.Mux, resolve middlewareSpecResolver) error {
	return authz.ValidateRequireRoleMiddlewareRoutes(buildRequireRoleRouteSpecs(router, resolve))
}

func buildRequireRoleRouteSpecs(router *chi.Mux, resolve middlewareSpecResolver) []authz.RequireRoleRouteSpec {
	if router == nil {
		return nil
	}
	return flattenChiRoutes(router.Routes(), "", resolve)
}

func flattenChiRoutes(routes []chi.Route, prefix string, resolve middlewareSpecResolver) []authz.RequireRoleRouteSpec {
	out := make([]authz.RequireRoleRouteSpec, 0, len(routes)*2)
	for _, route := range routes {
		pattern := chiRoutePattern(prefix, route.Pattern)
		for method, handler := range route.Handlers {
			spec, shouldValidate := resolve.resolve(method, pattern, handler)
			if !shouldValidate {
				continue
			}
			normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
			if normalizedMethod == "" || normalizedMethod == "*" {
				normalizedMethod = "ANY"
			}
			out = append(out, authz.RequireRoleRouteSpec{
				Method:     normalizedMethod,
				Route:      pattern,
				Middleware: spec,
			})
		}
		if route.SubRoutes != nil {
			childPrefix := strings.TrimSuffix(pattern, "/*")
			out = append(out, flattenChiRoutes(route.SubRoutes.Routes(), childPrefix, resolve)...)
		}
	}
	return out
}

func newLegacyMiddlewareSpecResolver(resolve MiddlewareSpecResolver, strict bool) middlewareSpecResolver {
	return legacyMiddlewareSpecResolver{
		resolver: resolve,
		strict:   strict,
	}
}

type legacyMiddlewareSpecResolver struct {
	resolver MiddlewareSpecResolver
	strict   bool
}

type coverageMiddlewareSpecResolver struct {
	resolver MiddlewareSpecResolverWithCoverage
	log      ports.Logger
}

type middlewareSpecResolver interface {
	resolve(method, route string, handler http.Handler) (*authz.RequireRoleMiddleware, bool)
}

func (r legacyMiddlewareSpecResolver) resolve(method, route string, handler http.Handler) (*authz.RequireRoleMiddleware, bool) {
	if r.resolver == nil {
		return nil, false
	}
	mw := r.resolver(method, route, handler)
	if mw != nil {
		return mw, true
	}
	if r.strict {
		return nil, true
	}
	return nil, false
}

func (r coverageMiddlewareSpecResolver) resolve(method, route string, handler http.Handler) (*authz.RequireRoleMiddleware, bool) {
	if r.resolver == nil {
		return nil, false
	}
	res := r.resolver(method, route, handler)
	logger := r.log
	if logger == nil {
		logger = ports.NopLogger{}
	}
	logger.Debug(
		"authz route validation intent",
		"method", method,
		"route", route,
		"intent", intentFromCoverageResolution(res),
	)
	if res.SkipFromValidation {
		return nil, false
	}
	return res.Middleware, true
}

func intentFromCoverageResolution(res MiddlewareSpecResolution) string {
	if res.SkipFromValidation {
		return "skip"
	}
	if res.Middleware == nil {
		return "validate:missing"
	}
	return "validate"
}

func policyModeName(strict bool) string {
	if strict {
		return "strict"
	}
	return "permissive"
}

func authzBootstrapStrictMode() (bool, string) {
	if explicit, source, ok := authzBootstrapStrictModeFromEnv(); ok {
		return explicit, source
	}
	for _, source := range []struct {
		name string
		val  string
	}{
		{name: "CI", val: os.Getenv("CI")},
		{name: "GITHUB_ACTIONS", val: os.Getenv("GITHUB_ACTIONS")},
		{name: "ENV", val: os.Getenv("ENV")},
		{name: "APP_ENV", val: os.Getenv("APP_ENV")},
		{name: "GO_ENV", val: os.Getenv("GO_ENV")},
		{name: "API_TOOLKIT_ENV", val: os.Getenv("API_TOOLKIT_ENV")},
	} {
		if isEnvValueStrict(source.val) {
			return true, source.name
		}
	}
	return false, "default"
}

func authzBootstrapStrictModeFromEnv() (bool, string, bool) {
	return parseBooleanStrictOverride(os.Getenv(authzBootstrapStrictEnv), authzBootstrapStrictEnv)
}

func isEnvValueStrict(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case envModeCI, envModeProduction, envModeProd, envModeStaging, envModeStage, envValueStrict, envValueEnabled, envValueTrue, envValueOn, envValue1:
		return true
	default:
		return false
	}
}

func parseBooleanStrictOverride(raw, source string) (bool, string, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return false, "", false
	}
	switch trimmed {
	case envValueStrict, envValueEnabled, envValueTrue, envValueOn, envValue1:
		return true, source, true
	case envValuePermissive, envValueDisabled, envValueFalse, envValueOff, envValue0:
		return false, source, true
	default:
		return false, source, true
	}
}

func chiRoutePattern(prefix, pattern string) string {
	cleanPrefix := normalizeRoutePattern(strings.TrimSpace(prefix))
	cleanPattern := normalizeRoutePattern(strings.TrimSpace(pattern))

	if cleanPrefix == "/" || cleanPrefix == "" {
		return cleanPattern
	}
	if cleanPattern == "" {
		return cleanPrefix
	}
	if cleanPattern == "/" {
		if cleanPrefix != "" && !strings.HasSuffix(cleanPrefix, "/") {
			return cleanPrefix + "/"
		}
		return cleanPrefix
	}
	if strings.HasSuffix(cleanPrefix, "/") && strings.HasPrefix(cleanPattern, "/") {
		return cleanPrefix + strings.TrimPrefix(cleanPattern, "/")
	}
	if !strings.HasSuffix(cleanPrefix, "/") && !strings.HasPrefix(cleanPattern, "/") {
		return cleanPrefix + "/" + cleanPattern
	}
	return cleanPrefix + cleanPattern
}

func normalizeRoutePattern(pattern string) string {
	if pattern == "" {
		return ""
	}
	if pattern[0] != '/' {
		return "/" + pattern
	}
	return pattern
}
