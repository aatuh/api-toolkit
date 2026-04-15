package authz

import (
	"context"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/v2/authorization"
	"github.com/aatuh/api-toolkit/v2/httpx"
)

// RolesFromContext returns the roles associated with the current request context.
// Callers should return a stable slice (copy if needed) if the backing store is mutable.
type RolesFromContext func(ctx context.Context) []string

// RequireRoleMiddleware enforces a required role for a request.
type RequireRoleMiddleware struct {
	role         string
	rolesFromCtx RolesFromContext
}

// NewRequireRoleMiddleware constructs a role enforcement middleware.
func NewRequireRoleMiddleware(role string, rolesFromCtx RolesFromContext) *RequireRoleMiddleware {
	return &RequireRoleMiddleware{
		role:         strings.ToLower(strings.TrimSpace(role)),
		rolesFromCtx: rolesFromCtx,
	}
}

// Handler wraps the next handler with role checks.
func (m *RequireRoleMiddleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		required := m.role
		if required == "" || m.rolesFromCtx == nil {
			next.ServeHTTP(w, r)
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

func authStatus(ctx context.Context) int {
	if _, ok := authorization.ActorFromContext(ctx); ok {
		return http.StatusForbidden
	}
	return http.StatusUnauthorized
}

func writeRoleProblem(w http.ResponseWriter, status int) {
	switch status {
	case http.StatusUnauthorized:
		httpx.WriteProblem(w, status, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
			Title:  http.StatusText(status),
			Detail: "authentication required",
		})
	default:
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeForbidden),
			Title:  http.StatusText(http.StatusForbidden),
			Detail: "forbidden",
		})
	}
}
