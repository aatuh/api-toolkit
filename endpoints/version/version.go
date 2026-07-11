package version

import (
	"encoding/json"
	"net/http"

	"github.com/aatuh/api-toolkit/v4/specs"
)

// Info describes the build metadata exposed by the version endpoint.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// RouteRegistrar is the minimal route registration capability used by Handler.
type RouteRegistrar interface {
	Get(pattern string, h http.HandlerFunc)
}

// Config controls how the version endpoint is registered.
type Config struct {
	Path string
	Info Info
}

// Handler wires the version endpoint.
type Handler struct {
	path string
	info Info
}

// NewHandler constructs a Handler from the provided config.
func NewHandler(cfg Config) *Handler {
	path := cfg.Path
	if path == "" {
		path = specs.Version
	}
	return &Handler{
		path: path,
		info: cfg.Info,
	}
}

// RegisterRoutes mounts the handler on the router.
func (h *Handler) RegisterRoutes(router interface {
	Get(pattern string, h http.HandlerFunc)
}) {
	if router == nil {
		return
	}
	h.RegisterRoutesTo(versionRouteRegistrar{register: router.Get})
}

// RegisterRoutesTo mounts the handler on a minimal route registrar.
func (h *Handler) RegisterRoutesTo(router RouteRegistrar) {
	if h == nil {
		return
	}
	if router == nil {
		return
	}
	router.Get(h.path, h.Handler())
}

// Handler returns an HTTP handler that writes the version info JSON.
func (h *Handler) Handler() http.HandlerFunc {
	info := h.info
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(info)
	}
}

type versionRouteRegistrar struct {
	register func(string, http.HandlerFunc)
}

func (r versionRouteRegistrar) Get(pattern string, h http.HandlerFunc) {
	r.register(pattern, h)
}
