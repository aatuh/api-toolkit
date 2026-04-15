package ports

import "net/http"

// MethodRouteRegistrar defines the minimal route registration surface used by GET-only handlers.
type MethodRouteRegistrar interface {
	Get(pattern string, h http.HandlerFunc)
}

// MiddlewareChain defines the minimal middleware registration surface used by composed profiles.
type MiddlewareChain interface {
	Use(middlewares ...func(http.Handler) http.Handler)
}

// HTTPRouter defines the interface for HTTP routing.
type HTTPRouter interface {
	http.Handler
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
	Mount(pattern string, h http.Handler)
	Use(middlewares ...func(http.Handler) http.Handler)
}

// HTTPMiddleware defines the interface for HTTP middleware.
type HTTPMiddleware interface {
	RequestID() func(http.Handler) http.Handler
	RealIP() func(http.Handler) http.Handler
	Recoverer() func(http.Handler) http.Handler
}

// Middleware defines the interface for middlewares.
type Middleware interface {
	Middleware() func(http.Handler) http.Handler
}

// CORSHandler defines the interface for CORS handling.
type CORSHandler interface {
	Handler(opts CORSOptions) func(http.Handler) http.Handler
}

// CORSOptions defines CORS configuration.
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// URLParamExtractor defines the interface for extracting URL parameters.
type URLParamExtractor interface {
	URLParam(r *http.Request, key string) string
}
