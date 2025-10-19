package cors

import (
	"net/http"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/go-chi/cors"
)

// Handler provides CORS functionality.
type Handler struct{}

// New creates a new CORS handler that implements ports.CORSHandler.
func New() ports.CORSHandler {
	return &Handler{}
}

// DefaultOptions returns sensible default CORS options.
func DefaultOptions() ports.CORSOptions {
	return ports.CORSOptions{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}
}

// Handler returns a CORS handler with the given options.
func (h *Handler) Handler(opts ports.CORSOptions) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   opts.AllowedOrigins,
		AllowedMethods:   opts.AllowedMethods,
		AllowedHeaders:   opts.AllowedHeaders,
		AllowCredentials: opts.AllowCredentials,
		MaxAge:           opts.MaxAge,
	})
}
