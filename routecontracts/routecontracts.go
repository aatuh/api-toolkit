package routecontracts

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/aatuh/api-toolkit/v2/specs"
)

// Router is the route registration surface required by this package.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

type patchRouter interface {
	Patch(pattern string, h http.HandlerFunc)
}

// Route describes one runtime route and its OpenAPI operation.
type Route struct {
	Method     string
	Pattern    string
	Handler    http.Handler
	Middleware []func(http.Handler) http.Handler
	Operation  specs.Operation
}

// Registry registers routes and keeps route contract metadata for validation.
type Registry struct {
	router Router
	specs  *specs.Registry
	mu     sync.RWMutex
	routes []Route
}

// CoverageError reports route contract validation failures.
type CoverageError struct {
	Problems []string
}

func (e *CoverageError) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return ""
	}
	return strings.Join(e.Problems, "; ")
}

// NewRegistry creates a route contract registry.
func NewRegistry(router Router, specRegistry *specs.Registry) *Registry {
	return &Registry{router: router, specs: specRegistry}
}

// Register registers a route handler and matching OpenAPI operation.
func (r *Registry) Register(route Route) error {
	if r == nil {
		return fmt.Errorf("route contract registry is nil")
	}
	if r.router == nil {
		return fmt.Errorf("route router is nil")
	}
	route.Method = normalizeMethod(route.Method)
	if route.Method == "" {
		route.Method = normalizeMethod(route.Operation.Method)
	}
	route.Pattern = strings.TrimSpace(route.Pattern)
	if route.Pattern == "" {
		route.Pattern = strings.TrimSpace(route.Operation.Path)
	}
	if route.Method == "" {
		return fmt.Errorf("route method is required")
	}
	if route.Pattern == "" {
		return fmt.Errorf("route pattern is required")
	}
	if route.Handler == nil {
		return fmt.Errorf("route handler is required")
	}
	if route.Operation.Method == "" {
		route.Operation.Method = route.Method
	}
	if route.Operation.Path == "" {
		route.Operation.Path = route.Pattern
	}
	if normalizeMethod(route.Operation.Method) != route.Method {
		return fmt.Errorf("operation method %q does not match route method %q", route.Operation.Method, route.Method)
	}
	if strings.TrimSpace(route.Operation.Path) != route.Pattern {
		return fmt.Errorf("operation path %q does not match route pattern %q", route.Operation.Path, route.Pattern)
	}
	wrapped := applyMiddleware(route.Handler, route.Middleware)
	switch route.Method {
	case http.MethodGet:
		r.router.Get(route.Pattern, wrapped.ServeHTTP)
	case http.MethodPost:
		r.router.Post(route.Pattern, wrapped.ServeHTTP)
	case http.MethodPut:
		r.router.Put(route.Pattern, wrapped.ServeHTTP)
	case http.MethodDelete:
		r.router.Delete(route.Pattern, wrapped.ServeHTTP)
	case http.MethodPatch:
		patcher, ok := r.router.(patchRouter)
		if !ok {
			return fmt.Errorf("router does not support PATCH")
		}
		patcher.Patch(route.Pattern, wrapped.ServeHTTP)
	default:
		return fmt.Errorf("unsupported route method %q", route.Method)
	}
	if r.specs != nil {
		r.specs.Register(route.Operation)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	route.Middleware = append([]func(http.Handler) http.Handler(nil), route.Middleware...)
	r.routes = append(r.routes, route)
	return nil
}

// Get registers a GET route contract.
func (r *Registry) Get(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return r.Register(Route{Method: http.MethodGet, Pattern: pattern, Operation: operation, Handler: handler, Middleware: middleware})
}

// Post registers a POST route contract.
func (r *Registry) Post(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return r.Register(Route{Method: http.MethodPost, Pattern: pattern, Operation: operation, Handler: handler, Middleware: middleware})
}

// Put registers a PUT route contract.
func (r *Registry) Put(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return r.Register(Route{Method: http.MethodPut, Pattern: pattern, Operation: operation, Handler: handler, Middleware: middleware})
}

// Delete registers a DELETE route contract.
func (r *Registry) Delete(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return r.Register(Route{Method: http.MethodDelete, Pattern: pattern, Operation: operation, Handler: handler, Middleware: middleware})
}

// Patch registers a PATCH route contract when the router supports PATCH.
func (r *Registry) Patch(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return r.Register(Route{Method: http.MethodPatch, Pattern: pattern, Operation: operation, Handler: handler, Middleware: middleware})
}

// Routes returns registered route contracts in deterministic method/path order.
func (r *Registry) Routes() []Route {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]Route(nil), r.routes...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pattern == out[j].Pattern {
			return out[i].Method < out[j].Method
		}
		return out[i].Pattern < out[j].Pattern
	})
	return out
}

// Validate checks for duplicate or incomplete route contracts.
func (r *Registry) Validate() error {
	if r == nil {
		return &CoverageError{Problems: []string{"route contract registry is nil"}}
	}
	routes := r.Routes()
	seen := map[string]struct{}{}
	var problems []string
	for _, route := range routes {
		method := normalizeMethod(route.Method)
		pattern := strings.TrimSpace(route.Pattern)
		key := method + " " + pattern
		if method == "" || pattern == "" {
			problems = append(problems, "route method and pattern are required")
			continue
		}
		if route.Handler == nil {
			problems = append(problems, key+" missing handler")
		}
		if normalizeMethod(route.Operation.Method) != method || strings.TrimSpace(route.Operation.Path) != pattern {
			problems = append(problems, key+" missing matching operation")
		}
		if _, ok := seen[key]; ok {
			problems = append(problems, key+" registered more than once")
		}
		seen[key] = struct{}{}
	}
	if len(problems) > 0 {
		return &CoverageError{Problems: problems}
	}
	return nil
}

func applyMiddleware(handler http.Handler, middleware []func(http.Handler) http.Handler) http.Handler {
	wrapped := handler
	for i := len(middleware) - 1; i >= 0; i-- {
		if middleware[i] != nil {
			wrapped = middleware[i](wrapped)
		}
	}
	return wrapped
}

func normalizeMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}
