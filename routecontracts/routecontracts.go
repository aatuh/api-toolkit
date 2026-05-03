// Package routecontracts keeps runtime route registration and OpenAPI operation
// registration in one place.
package routecontracts

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/aatuh/api-toolkit/v2/specs"
)

// Router is the minimal router contract used by Registry.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

type patchRouter interface {
	Patch(pattern string, h http.HandlerFunc)
}

// Route binds an HTTP route to its OpenAPI operation.
type Route struct {
	Method     string
	Pattern    string
	Handler    http.Handler
	Middleware []func(http.Handler) http.Handler
	Operation  specs.Operation
}

// Policy derives automatic middleware and operation metadata from a route operation.
type Policy interface {
	Apply(specs.Operation) (specs.Operation, []func(http.Handler) http.Handler, error)
}

// PolicyFunc adapts a function to Policy.
type PolicyFunc func(specs.Operation) (specs.Operation, []func(http.Handler) http.Handler, error)

// Apply derives middleware and operation metadata from an operation.
func (f PolicyFunc) Apply(operation specs.Operation) (specs.Operation, []func(http.Handler) http.Handler, error) {
	if f == nil {
		return operation, nil, nil
	}
	return f(operation)
}

// Options configures Registry behavior.
type Options struct {
	Policies []Policy
}

// Registry registers runtime routes and matching OpenAPI operations.
type Registry struct {
	router Router
	specs  *specs.Registry
	opts   Options
	mu     sync.RWMutex
	routes []Route
}

// NewRegistry constructs a registry with default behavior.
func NewRegistry(router Router, specRegistry *specs.Registry) *Registry {
	return NewRegistryWithOptions(router, specRegistry, Options{})
}

// NewRegistryWithOptions constructs a registry with policy options.
func NewRegistryWithOptions(router Router, specRegistry *specs.Registry, opts Options) *Registry {
	copied := Options{Policies: append([]Policy(nil), opts.Policies...)}
	return &Registry{router: router, specs: specRegistry, opts: copied}
}

// Register registers a route and matching operation.
func (registry *Registry) Register(route Route) error {
	if registry == nil || registry.router == nil {
		return fmt.Errorf("route registry router is not configured")
	}
	method := strings.ToUpper(strings.TrimSpace(route.Method))
	pattern := strings.TrimSpace(route.Pattern)
	if method == "" || pattern == "" {
		return fmt.Errorf("route method and pattern are required")
	}
	if route.Handler == nil {
		return fmt.Errorf("route handler is required")
	}
	operation := route.Operation
	if operation.Method == "" {
		operation.Method = method
	} else if strings.ToUpper(strings.TrimSpace(operation.Method)) != method {
		return fmt.Errorf("operation method must match route method")
	}
	if operation.Path == "" {
		operation.Path = pattern
	} else if strings.TrimSpace(operation.Path) != pattern {
		return fmt.Errorf("operation path must match route pattern")
	}
	var policyMiddleware []func(http.Handler) http.Handler
	for _, policy := range registry.opts.Policies {
		if policy == nil {
			continue
		}
		updated, middleware, err := policy.Apply(operation)
		if err != nil {
			return fmt.Errorf("route policy: %w", err)
		}
		operation = updated
		policyMiddleware = append(policyMiddleware, middleware...)
	}
	middleware := append(append([]func(http.Handler) http.Handler{}, route.Middleware...), policyMiddleware...)
	wrapped := applyMiddleware(route.Handler, middleware)
	switch method {
	case http.MethodGet:
		registry.router.Get(pattern, wrapped.ServeHTTP)
	case http.MethodPost:
		registry.router.Post(pattern, wrapped.ServeHTTP)
	case http.MethodPut:
		registry.router.Put(pattern, wrapped.ServeHTTP)
	case http.MethodDelete:
		registry.router.Delete(pattern, wrapped.ServeHTTP)
	case http.MethodPatch:
		patcher, ok := registry.router.(patchRouter)
		if !ok {
			return fmt.Errorf("router does not support PATCH")
		}
		patcher.Patch(pattern, wrapped.ServeHTTP)
	default:
		return fmt.Errorf("unsupported route method %q", method)
	}
	if registry.specs != nil {
		registry.specs.Register(operation)
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	route.Method = method
	route.Pattern = pattern
	route.Operation = operation
	route.Middleware = append([]func(http.Handler) http.Handler(nil), route.Middleware...)
	registry.routes = append(registry.routes, route)
	return nil
}

// Get registers a GET route.
func (registry *Registry) Get(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return registry.Register(Route{Method: http.MethodGet, Pattern: pattern, Handler: handler, Operation: operation, Middleware: middleware})
}

// Post registers a POST route.
func (registry *Registry) Post(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return registry.Register(Route{Method: http.MethodPost, Pattern: pattern, Handler: handler, Operation: operation, Middleware: middleware})
}

// Put registers a PUT route.
func (registry *Registry) Put(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return registry.Register(Route{Method: http.MethodPut, Pattern: pattern, Handler: handler, Operation: operation, Middleware: middleware})
}

// Delete registers a DELETE route.
func (registry *Registry) Delete(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return registry.Register(Route{Method: http.MethodDelete, Pattern: pattern, Handler: handler, Operation: operation, Middleware: middleware})
}

// Patch registers a PATCH route when the router supports Patch.
func (registry *Registry) Patch(pattern string, operation specs.Operation, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
	return registry.Register(Route{Method: http.MethodPatch, Pattern: pattern, Handler: handler, Operation: operation, Middleware: middleware})
}

// Routes returns the registered routes in deterministic order.
func (registry *Registry) Routes() []Route {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]Route, len(registry.routes))
	copy(out, registry.routes)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pattern != out[j].Pattern {
			return out[i].Pattern < out[j].Pattern
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// CoverageError describes a route-contract coverage problem.
type CoverageError struct {
	Problems []string
}

// Error returns a human-readable error.
func (err CoverageError) Error() string {
	return strings.Join(err.Problems, "; ")
}

// Validate verifies registered routes have matching operations.
func (registry *Registry) Validate() error {
	if registry == nil {
		return nil
	}
	routes := registry.Routes()
	var problems []string
	seen := map[string]struct{}{}
	for _, route := range routes {
		key := route.Method + " " + route.Pattern
		if _, ok := seen[key]; ok {
			problems = append(problems, "route registered more than once: "+key)
		}
		seen[key] = struct{}{}
		if strings.ToUpper(strings.TrimSpace(route.Operation.Method)) != route.Method || strings.TrimSpace(route.Operation.Path) != route.Pattern {
			problems = append(problems, "missing matching operation for "+key)
		}
	}
	if len(problems) > 0 {
		return &CoverageError{Problems: problems}
	}
	return nil
}

func applyMiddleware(handler http.Handler, middleware []func(http.Handler) http.Handler) http.Handler {
	var wrapped http.Handler = handler
	for i := len(middleware) - 1; i >= 0; i-- {
		if middleware[i] != nil {
			wrapped = middleware[i](wrapped)
		}
	}
	return wrapped
}
