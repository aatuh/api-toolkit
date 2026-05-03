package oauth2

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v2/authorization"
	"github.com/aatuh/api-toolkit/v2/specs"
)

// TokenClaims captures validated OAuth2 token claims.
type TokenClaims struct {
	Subject   string
	Issuer    string
	Audience  []string
	Scopes    []string
	TenantID  string
	ExpiresAt time.Time
	IssuedAt  time.Time
	NotBefore time.Time
	Raw       map[string]any
}

// Actor maps claims to the toolkit authorization actor.
func (claims TokenClaims) Actor() authorization.Actor {
	return authorization.Actor{UserID: claims.Subject}
}

// AuthorizationScope maps claims to the toolkit authorization scope.
func (claims TokenClaims) AuthorizationScope() authorization.Scope {
	return authorization.Scope{UserID: claims.Subject, TenantID: claims.TenantID}
}

// Validator validates a bearer token and returns provider-neutral claims.
type Validator interface {
	ValidateToken(ctx context.Context, token string) (TokenClaims, error)
}

// ValidatorFunc adapts a function to Validator.
type ValidatorFunc func(context.Context, string) (TokenClaims, error)

// ValidateToken validates a bearer token.
func (f ValidatorFunc) ValidateToken(ctx context.Context, token string) (TokenClaims, error) {
	if f == nil {
		return TokenClaims{}, fmt.Errorf("oauth2 validator function is nil")
	}
	return f(ctx, token)
}

// JWKSConfig describes provider-neutral JWKS validation configuration.
type JWKSConfig struct {
	Issuer    string
	Audience  []string
	JWKSURL   string
	ClockSkew time.Duration
}

// ScopeSet is a normalized set of OAuth2 scopes.
type ScopeSet map[string]struct{}

// Has reports whether scope is present.
func (set ScopeSet) Has(scope string) bool {
	_, ok := set[strings.TrimSpace(scope)]
	return ok
}

// NewScopeSet constructs a normalized scope set from values.
func NewScopeSet(scopes ...string) ScopeSet {
	set := ScopeSet{}
	for _, scope := range scopes {
		for _, part := range strings.Fields(scope) {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				set[trimmed] = struct{}{}
			}
		}
	}
	return set
}

// RequireScopes verifies that claims include all required scopes.
func RequireScopes(claims TokenClaims, required ...string) error {
	set := NewScopeSet(claims.Scopes...)
	for _, scope := range required {
		if scope = strings.TrimSpace(scope); scope != "" && !set.Has(scope) {
			return fmt.Errorf("required OAuth2 scope %q is missing", scope)
		}
	}
	return nil
}

// SecurityScheme returns a bearer OAuth2 security scheme for OpenAPI.
func SecurityScheme(scopes ...string) specs.SecurityScheme {
	sorted := append([]string(nil), scopes...)
	sort.Strings(sorted)
	flows := map[string]any{}
	if len(sorted) > 0 {
		flows["scopes"] = sorted
	}
	return specs.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT", Flows: flows}
}

// RegisterSecurityScheme registers an OAuth2-compatible bearer security scheme.
func RegisterSecurityScheme(registry *specs.Registry, name string, scheme specs.SecurityScheme) {
	if registry == nil {
		return
	}
	if strings.TrimSpace(name) == "" {
		name = "OAuth2"
	}
	registry.RegisterSecurityScheme(name, scheme)
}

// BearerToken extracts a bearer token from Authorization.
func BearerToken(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	prefix := "Bearer "
	if len(value) <= len(prefix) || !strings.EqualFold(value[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(value[len(prefix):]), strings.TrimSpace(value[len(prefix):]) != ""
}
