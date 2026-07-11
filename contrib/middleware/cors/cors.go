package cors

import (
	"net/http"

	"github.com/go-chi/cors"

	"github.com/aatuh/api-toolkit/contrib/v3/contracts"
)

// Handler provides CORS functionality.
type Handler struct{}

// New creates a new CORS handler that implements contracts.CORSHandler.
func New() contracts.CORSHandler {
	return &Handler{}
}

// DefaultOptions returns sensible default CORS options.
func DefaultOptions() contracts.CORSOptions {
	return contracts.CORSOptions{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}
}

// Handler returns a CORS handler with the given options.
func (h *Handler) Handler(opts contracts.CORSOptions) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   opts.AllowedOrigins,
		AllowedMethods:   opts.AllowedMethods,
		AllowedHeaders:   opts.AllowedHeaders,
		AllowCredentials: opts.AllowCredentials,
		MaxAge:           opts.MaxAge,
	})
}
