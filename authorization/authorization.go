package authorization

import (
	"context"
	"errors"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

// Owner exposes ownership information for BOLA checks.
type Owner interface {
	OwnerID() string
}

// TenantOwned exposes tenant ownership for scoping checks.
type TenantOwned interface {
	TenantID() string
}

// Require calls the provided authorizer or returns an error when missing.
func Require(ctx context.Context, auth Authorizer, subject any, action string, resource any) error {
	if auth == nil {
		return errors.New("authorizer not configured")
	}
	return auth.Can(ctx, subject, action, resource)
}

// RequireOwnerID enforces resource ownership (BOLA).
func RequireOwnerID(subjectID, ownerID string) error {
	if subjectID == "" || ownerID == "" {
		return httpx.ErrForbidden
	}
	if subjectID != ownerID {
		return httpx.ErrForbidden
	}
	return nil
}

// RequireOwner enforces ownership using an Owner interface.
func RequireOwner(subjectID string, resource Owner) error {
	if resource == nil {
		return httpx.ErrForbidden
	}
	return RequireOwnerID(subjectID, resource.OwnerID())
}

// RequireTenant enforces tenant scoping.
func RequireTenant(tenantID string, resource TenantOwned) error {
	if resource == nil {
		return httpx.ErrForbidden
	}
	if tenantID == "" || resource.TenantID() == "" {
		return httpx.ErrForbidden
	}
	if tenantID != resource.TenantID() {
		return httpx.ErrForbidden
	}
	return nil
}

// ProjectFields returns a new map with only the allowed keys.
func ProjectFields(input map[string]any, allowed []string) map[string]any {
	if input == nil {
		return nil
	}
	allowSet := make(map[string]struct{}, len(allowed))
	for _, k := range allowed {
		if k == "" {
			continue
		}
		allowSet[k] = struct{}{}
	}
	out := make(map[string]any, len(allowSet))
	for k, v := range input {
		if _, ok := allowSet[k]; ok {
			out[k] = v
		}
	}
	return out
}

// MaskFields returns a new map without the denied keys.
func MaskFields(input map[string]any, denied []string) map[string]any {
	if input == nil {
		return nil
	}
	denySet := make(map[string]struct{}, len(denied))
	for _, k := range denied {
		if k == "" {
			continue
		}
		denySet[k] = struct{}{}
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		if _, ok := denySet[k]; ok {
			continue
		}
		out[k] = v
	}
	return out
}

// Scope captures tenant/user scoping hints for repositories.
type Scope struct {
	TenantID string
	UserID   string
}

// Filters builds a simple filter map for repository queries.
func (s Scope) Filters() map[string]any {
	out := map[string]any{}
	if s.TenantID != "" {
		out["tenant_id"] = s.TenantID
	}
	if s.UserID != "" {
		out["user_id"] = s.UserID
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ApplyScope merges scope filters into an existing filter map.
func ApplyScope(filters map[string]any, scope Scope) map[string]any {
	out := make(map[string]any, len(filters)+2)
	for k, v := range filters {
		out[k] = v
	}
	for k, v := range scope.Filters() {
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
