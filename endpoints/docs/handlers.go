package docs

import (
	"net/http"

	"github.com/aatuh/api-toolkit/v3/ports"
	"github.com/aatuh/api-toolkit/v3/specs"
)

const defaultDocsCSP = "default-src 'self'; img-src 'self' data: https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; font-src 'self' data: https://cdn.jsdelivr.net; media-src 'self' data:; connect-src 'self' https://cdn.jsdelivr.net; frame-ancestors 'self'"
const strictDocsCSP = "default-src 'self'; base-uri 'none'; connect-src 'self'; font-src 'self'; form-action 'self'; frame-ancestors 'none'; img-src 'self' data:; media-src 'self'; object-src 'none'; script-src 'self'; style-src 'self'"

// Handler provides HTTP handlers for documentation endpoints.
type Handler struct {
	manager ports.DocsManager
}

// NewHandler creates a new docs handler.
func NewHandler(manager ports.DocsManager) *Handler {
	if manager == nil {
		manager = New()
	}
	return &Handler{manager: manager}
}

// NewDefaultHandler builds a handler using the default docs manager.
func NewDefaultHandler() *Handler {
	return NewHandler(New())
}

// HTMLHandler handles HTML documentation requests.
// @Summary API Documentation
// @Description Returns the API documentation page
// @Tags docs
// @Accept html
// @Produce html
// @Success 200 {string} string "HTML documentation page"
// @Router /docs [get]
func (h *Handler) HTMLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", docsCSP(h.manager))
	h.manager.ServeHTML(w, r)
}

// OpenAPIHandler handles OpenAPI specification requests.
// @Summary OpenAPI Specification
// @Description Returns the configured OpenAPI specification when the docs surface is enabled
// @Tags docs
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "OpenAPI specification"
// @Failure 404 {object} map[string]interface{} "OpenAPI specification disabled or not found"
// @Router /docs/openapi.json [get]
func (h *Handler) OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
	h.manager.ServeOpenAPI(w, r)
}

// VersionHandler handles version requests.
func (h *Handler) VersionHandler(w http.ResponseWriter, r *http.Request) {
	h.manager.ServeVersion(w, r)
}

// InfoHandler handles info requests.
func (h *Handler) InfoHandler(w http.ResponseWriter, r *http.Request) {
	h.manager.ServeInfo(w, r)
}

// RegisterRoutes registers all documentation endpoints on the given router.
func (h *Handler) RegisterRoutes(router interface {
	Get(pattern string, h http.HandlerFunc)
}) {
	if router == nil {
		return
	}
	h.RegisterRoutesTo(docsRouteRegistrar{register: router.Get})
}

// RegisterRoutesTo registers all documentation endpoints on the given registrar.
func (h *Handler) RegisterRoutesTo(router ports.MethodRouteRegistrar) {
	if h == nil || router == nil {
		return
	}

	router.Get(specs.Docs, h.HTMLHandler)
	router.Get(specs.DocsOpenAPI, h.OpenAPIHandler)
	router.Get(specs.DocsVersion, h.VersionHandler)
	router.Get(specs.DocsInfo, h.InfoHandler)
}

// RegisterCustomRoutes registers documentation endpoints with custom paths.
func (h *Handler) RegisterCustomRoutes(router interface {
	Get(pattern string, h http.HandlerFunc)
}, paths ports.DocsPaths) {
	if router == nil {
		return
	}
	h.RegisterCustomRoutesTo(docsRouteRegistrar{register: router.Get}, paths)
}

// RegisterCustomRoutesTo registers documentation endpoints with custom paths.
func (h *Handler) RegisterCustomRoutesTo(router ports.MethodRouteRegistrar, paths ports.DocsPaths) {
	if h == nil || router == nil {
		return
	}

	if paths.HTML != "" {
		router.Get(paths.HTML, h.HTMLHandler)
	}
	if paths.OpenAPI != "" {
		router.Get(paths.OpenAPI, h.OpenAPIHandler)
	}
	if paths.Version != "" {
		router.Get(paths.Version, h.VersionHandler)
	}
	if paths.Info != "" {
		router.Get(paths.Info, h.InfoHandler)
	}
}

// Middleware creates a middleware that adds documentation information to requests.
func (h *Handler) Middleware() func(http.Handler) http.Handler {
	if h == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add docs info to response headers
			info := h.manager.GetInfo()
			w.Header().Set("X-API-Title", info.Title)
			w.Header().Set("X-API-Version", info.Version)
			if info.Description != "" {
				w.Header().Set("X-API-Description", info.Description)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func docsCSP(manager ports.DocsManager) string {
	if provider, ok := manager.(ports.DocsHTMLModeProvider); ok {
		if provider.HTMLMode() == ports.DocsHTMLModeSwaggerUI {
			return defaultDocsCSP
		}
		return strictDocsCSP
	}
	return strictDocsCSP
}

type docsRouteRegistrar struct {
	register func(string, http.HandlerFunc)
}

func (r docsRouteRegistrar) Get(pattern string, h http.HandlerFunc) {
	r.register(pattern, h)
}
