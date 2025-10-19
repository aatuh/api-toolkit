package docs

import (
	"net/http"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/specs"
)

// Handler provides HTTP handlers for documentation endpoints.
type Handler struct {
	manager ports.DocsManager
}

// NewHandler creates a new docs handler.
func NewHandler(manager ports.DocsManager) *Handler {
	return &Handler{manager: manager}
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
	h.manager.ServeHTML(w, r)
}

// OpenAPIHandler handles OpenAPI specification requests.
// @Summary OpenAPI Specification
// @Description Returns the OpenAPI specification in JSON format
// @Tags docs
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "OpenAPI specification"
// @Failure 404 {object} map[string]interface{} "OpenAPI specification not found"
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
	// Standard documentation endpoints
	router.Get(specs.Docs, h.HTMLHandler)
	router.Get(specs.DocsOpenAPI, h.OpenAPIHandler)
	router.Get(specs.DocsVersion, h.VersionHandler)
	router.Get(specs.DocsInfo, h.InfoHandler)
}

// RegisterCustomRoutes registers documentation endpoints with custom paths.
func (h *Handler) RegisterCustomRoutes(router interface {
	Get(pattern string, h http.HandlerFunc)
}, paths ports.DocsPaths) {
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
