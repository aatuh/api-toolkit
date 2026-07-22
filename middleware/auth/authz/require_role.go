package authz

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/aatuh/api-toolkit/v4/authorization"
	"github.com/aatuh/api-toolkit/v4/httpx"
)

// RolesFromContext returns the roles associated with the current request context.
// Callers should return a stable slice (copy if needed) if the backing store is mutable.
type RolesFromContext func(ctx context.Context) []string

// RequireRoleRouteSpec provides route-level metadata for bootstrap validation.
type RequireRoleRouteSpec struct {
	Method     string
	Route      string
	Middleware *RequireRoleMiddleware
}

// RequireRoleMiddleware enforces a required role for a request.
type RequireRoleMiddleware struct {
	role         string
	rolesFromCtx RolesFromContext
}

// ErrRequireRoleMissingRole indicates required role was not provided.
var ErrRequireRoleMissingRole = errors.New("required role is missing")

// ErrRequireRoleMissingResolver indicates RolesFromContext was not configured.
var ErrRequireRoleMissingResolver = errors.New("rolesFromContext resolver is missing")

// NewRequireRoleMiddleware constructs a role enforcement middleware and
// returns configuration errors for startup validation.
func NewRequireRoleMiddleware(role string, rolesFromCtx RolesFromContext) (*RequireRoleMiddleware, error) {
	if err := validateRequireRoleConfig(role, rolesFromCtx); err != nil {
		return nil, err
	}
	return &RequireRoleMiddleware{
		role:         strings.ToLower(strings.TrimSpace(role)),
		rolesFromCtx: rolesFromCtx,
	}, nil
}

// NewRequireRoleMiddlewareChecked is retained as an explicit checked alias for
// v2 migration code.
func NewRequireRoleMiddlewareChecked(role string, rolesFromCtx RolesFromContext) (*RequireRoleMiddleware, error) {
	return NewRequireRoleMiddleware(role, rolesFromCtx)
}

// Handler wraps the next handler with role checks.
func (m *RequireRoleMiddleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		required := strings.TrimSpace(m.role)
		if required == "" || m.rolesFromCtx == nil {
			writeRoleProblem(w, authStatus(r.Context()))
			return
		}
		roles := m.rolesFromCtx(r.Context())
		if len(roles) == 0 {
			writeRoleProblem(w, authStatus(r.Context()))
			return
		}
		for _, role := range roles {
			if strings.EqualFold(strings.TrimSpace(role), required) {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeRoleProblem(w, http.StatusForbidden)
	})
}

// ValidateRequireRoleMiddleware validates a configured role middleware against
// route and method context. Use this during bootstrap to fail fast on missing
// role wiring before serving traffic.
func ValidateRequireRoleMiddleware(method, route string, middleware *RequireRoleMiddleware) error {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if normalizedMethod == "" {
		normalizedMethod = "ANY"
	}
	normalizedRoute := strings.TrimSpace(route)
	if normalizedRoute == "" {
		normalizedRoute = "(unknown route)"
	}
	if middleware == nil {
		return fmt.Errorf("invalid role middleware for route %s %s: middleware is nil", normalizedMethod, normalizedRoute)
	}
	if err := validateRequireRoleConfig(middleware.role, middleware.rolesFromCtx); err != nil {
		return fmt.Errorf("invalid role middleware for route %s %s: %w", normalizedMethod, normalizedRoute, err)
	}
	return nil
}

// ValidateRequireRoleMiddlewareRoutes validates a route registry in one pass.
//
// Useful at startup for CI and server bootstrap to fail fast when a route
// middleware is missing required role wiring.
func ValidateRequireRoleMiddlewareRoutes(routes []RequireRoleRouteSpec) error {
	if len(routes) == 0 {
		return nil
	}
	errors := make([]string, 0, len(routes))
	for _, route := range routes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		if method == "" {
			method = "ANY"
		}
		path := strings.TrimSpace(route.Route)
		if path == "" {
			path = "(unknown route)"
		}
		if err := ValidateRequireRoleMiddleware(method, path, route.Middleware); err != nil {
			errors = append(errors, fmt.Sprintf("%s %s: %v", method, path, err))
		}
	}
	if len(errors) == 0 {
		return nil
	}
	sort.Strings(errors)
	return fmt.Errorf("invalid role middleware registrations: %d issue(s): %s", len(errors), strings.Join(errors, "; "))
}

func validateRequireRoleConfig(role string, rolesFromCtx RolesFromContext) error {
	if strings.TrimSpace(role) == "" {
		return ErrRequireRoleMissingRole
	}
	if rolesFromCtx == nil {
		return ErrRequireRoleMissingResolver
	}
	return nil
}

func authStatus(ctx context.Context) int {
	if _, ok := authorization.ActorFromContext(ctx); ok {
		return http.StatusForbidden
	}
	return http.StatusUnauthorized
}

func writeRoleProblem(w http.ResponseWriter, status int) {
	switch status {
	case http.StatusUnauthorized:
		httpx.WriteProblemChecked(w, status, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
			Title:  http.StatusText(status),
			Detail: "authentication required",
		})
	default:
		httpx.WriteProblemChecked(w, http.StatusForbidden, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeForbidden),
			Title:  http.StatusText(http.StatusForbidden),
			Detail: "forbidden",
		})
	}
}
