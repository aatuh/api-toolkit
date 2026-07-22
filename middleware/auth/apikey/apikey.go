package apikey

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/aatuh/api-toolkit/v4/authorization"
	"github.com/aatuh/api-toolkit/v4/httpx"
)

type contextKey struct{}

var (
	errMissingCredential   = errors.New("api key credential is missing")
	errMalformedCredential = errors.New("api key credential is malformed")
	errMultipleCredentials = errors.New("multiple api key credentials were provided")
	errVerifierMissing     = errors.New("api key verifier is required")
)

// PresentedKey describes a credential extracted from a request.
type PresentedKey struct {
	Value  string
	Source string
}

// Principal describes the authenticated API key owner.
type Principal struct {
	ID       string
	Name     string
	TenantID string
	Scopes   []string
	Metadata map[string]any
}

// Verifier validates a presented API key and returns its principal.
type Verifier interface {
	VerifyAPIKey(ctx context.Context, key PresentedKey) (Principal, error)
}

// VerifierFunc adapts a function to the Verifier interface.
type VerifierFunc func(ctx context.Context, key PresentedKey) (Principal, error)

// VerifyAPIKey validates a presented API key.
func (f VerifierFunc) VerifyAPIKey(ctx context.Context, key PresentedKey) (Principal, error) {
	return f(ctx, key)
}

// Config controls API key authentication.
type Config struct {
	Verifier    Verifier
	HeaderNames []string
}

// Middleware authenticates requests with API keys.
type Middleware struct {
	verifier Verifier
	headers  []string
}

// NewMiddleware constructs API key middleware.
func NewMiddleware(cfg Config) (*Middleware, error) {
	if cfg.Verifier == nil {
		return nil, errVerifierMissing
	}
	headers := normalizeHeaders(cfg.HeaderNames)
	return &Middleware{verifier: cfg.Verifier, headers: headers}, nil
}

// Handler requires a valid API key before calling next.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := m.authenticate(w, r, true)
		if !ok {
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

// OptionalHandler authenticates a valid API key when present and otherwise lets
// anonymous requests continue. Malformed or invalid credentials still fail.
func (m *Middleware) OptionalHandler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := m.authenticate(w, r, false)
		if !ok {
			return
		}
		if principal.ID == "" {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

// RequireScopeMiddleware requires an authenticated API key principal with scope.
func RequireScopeMiddleware(scope string) func(http.Handler) http.Handler {
	required := strings.TrimSpace(scope)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				writeAPIKeyProblem(w, http.StatusUnauthorized, "API key required")
				return
			}
			if required != "" && !principal.HasScope(required) {
				writeAPIKeyProblem(w, http.StatusForbidden, "required API key scope missing")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithPrincipal stores an authenticated API key principal in context.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, contextKey{}, principal)
	if principal.ID != "" {
		ctx = authorization.WithActor(ctx, authorization.Actor{UserID: principal.ID})
	}
	if principal.ID != "" || principal.TenantID != "" {
		ctx = authorization.WithScope(ctx, authorization.Scope{
			UserID:   principal.ID,
			TenantID: principal.TenantID,
		})
	}
	return ctx
}

// PrincipalFromContext returns the authenticated API key principal.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(contextKey{}).(Principal)
	if !ok || principal.ID == "" {
		return Principal{}, false
	}
	return principal, true
}

// HasScope reports whether the principal has scope.
func (p Principal) HasScope(scope string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return true
	}
	for _, candidate := range p.Scopes {
		if strings.EqualFold(strings.TrimSpace(candidate), scope) {
			return true
		}
	}
	return false
}

func (m *Middleware) authenticate(w http.ResponseWriter, r *http.Request, required bool) (Principal, bool) {
	key, err := presentedKeyFromRequest(r, m.headers)
	if err != nil {
		if errors.Is(err, errMissingCredential) && !required {
			return Principal{}, true
		}
		status := http.StatusUnauthorized
		detail := "API key required"
		if errors.Is(err, errMalformedCredential) || errors.Is(err, errMultipleCredentials) {
			status = http.StatusBadRequest
			detail = "invalid API key credentials"
		}
		writeAPIKeyProblem(w, status, detail)
		return Principal{}, false
	}
	principal, err := m.verifier.VerifyAPIKey(r.Context(), key)
	if err != nil || principal.ID == "" {
		writeAPIKeyProblem(w, http.StatusUnauthorized, "invalid API key")
		return Principal{}, false
	}
	principal.Scopes = normalizedScopes(principal.Scopes)
	return principal, true
}

func presentedKeyFromRequest(r *http.Request, headers []string) (PresentedKey, error) {
	if r == nil {
		return PresentedKey{}, errMissingCredential
	}
	var found []PresentedKey
	for _, header := range headers {
		switch strings.ToLower(header) {
		case "authorization":
			key, ok, err := apiKeyFromAuthorization(r.Header.Values("Authorization"))
			if err != nil {
				return PresentedKey{}, err
			}
			if ok {
				found = append(found, PresentedKey{Value: key, Source: "Authorization"})
			}
		default:
			for _, value := range r.Header.Values(header) {
				value = strings.TrimSpace(value)
				if value != "" {
					found = append(found, PresentedKey{Value: value, Source: header})
				}
			}
		}
	}
	if len(found) == 0 {
		return PresentedKey{}, errMissingCredential
	}
	if len(found) > 1 {
		return PresentedKey{}, errMultipleCredentials
	}
	return found[0], nil
}

func apiKeyFromAuthorization(values []string) (string, bool, error) {
	var found string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts := strings.Fields(value)
		if len(parts) == 0 {
			continue
		}
		if !strings.EqualFold(parts[0], "ApiKey") {
			continue
		}
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return "", false, errMalformedCredential
		}
		if found != "" {
			return "", false, errMultipleCredentials
		}
		found = parts[1]
	}
	if found == "" {
		return "", false, nil
	}
	return found, true, nil
}

func normalizeHeaders(headers []string) []string {
	if len(headers) == 0 {
		return []string{"Authorization", "X-API-Key"}
	}
	seen := make(map[string]struct{}, len(headers))
	out := make([]string, 0, len(headers))
	for _, header := range headers {
		header = http.CanonicalHeaderKey(strings.TrimSpace(header))
		if header == "" {
			continue
		}
		key := strings.ToLower(header)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, header)
	}
	if len(out) == 0 {
		return []string{"Authorization", "X-API-Key"}
	}
	return out
}

func normalizedScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	seen := map[string]struct{}{}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		key := strings.ToLower(scope)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, scope)
	}
	sort.Strings(out)
	return out
}

func writeAPIKeyProblem(w http.ResponseWriter, status int, detail string) {
	typeSlug := httpx.TypeUnauthorized
	if status == http.StatusForbidden {
		typeSlug = httpx.TypeForbidden
	}
	if status == http.StatusBadRequest {
		typeSlug = httpx.TypeBadRequest
	}
	httpx.WriteProblemChecked(w, status, httpx.Problem{
		Type:   httpx.DefaultTypeURI(typeSlug),
		Title:  http.StatusText(status),
		Detail: detail,
	})
}
