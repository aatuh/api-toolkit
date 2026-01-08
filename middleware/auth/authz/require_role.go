package authz

import (
	"context"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/httpx"
)

// RolesFromContext returns the roles associated with the current request context.
// Callers should return a stable slice (copy if needed) if the backing store is mutable.
type RolesFromContext func(ctx context.Context) []string

type RequireRoleMiddleware struct {
	role         string
	rolesFromCtx RolesFromContext
}

func NewRequireRoleMiddleware(role string, rolesFromCtx RolesFromContext) *RequireRoleMiddleware {
	return &RequireRoleMiddleware{
		role:         strings.ToLower(strings.TrimSpace(role)),
		rolesFromCtx: rolesFromCtx,
	}
}

func (m *RequireRoleMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		required := m.role
		if required == "" || m.rolesFromCtx == nil {
			next.ServeHTTP(w, r)
			return
		}
		roles := m.rolesFromCtx(r.Context())
		if len(roles) == 0 {
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{
				Title:  http.StatusText(http.StatusForbidden),
				Detail: "forbidden",
			})
			return
		}
		for _, role := range roles {
			if strings.EqualFold(strings.TrimSpace(role), required) {
				next.ServeHTTP(w, r)
				return
			}
		}
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{
			Title:  http.StatusText(http.StatusForbidden),
			Detail: "forbidden",
		})
	})
}

