package version

import (
	"encoding/json"
	"net/http"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/specs"
)

// Config controls how the version endpoint is registered.
type Config struct {
	Path string
	Info ports.VersionInfo
}

// Handler wires the version endpoint.
type Handler struct {
	path string
	info ports.VersionInfo
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
	if h == nil {
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
