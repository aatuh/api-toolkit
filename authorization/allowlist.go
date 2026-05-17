package authorization

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/ports"
)

// AllowlistAuthorizer enforces explicit allow rules for actions.
// Missing rules default to forbidden.
type AllowlistAuthorizer struct {
	mu    sync.RWMutex
	rules map[string]ports.Authorizer
}

// NewAllowlistAuthorizer creates an allowlist-based authorizer.
func NewAllowlistAuthorizer() *AllowlistAuthorizer {
	return &AllowlistAuthorizer{rules: make(map[string]ports.Authorizer)}
}

// Allow registers an authorizer for a specific action.
func (a *AllowlistAuthorizer) Allow(action string, auth ports.Authorizer) error {
	if a == nil {
		return errors.New("authorizer is nil")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return errors.New("action is required")
	}
	if auth == nil {
		return errors.New("authorizer is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rules[action] = auth
	return nil
}

// AllowFunc registers an authorizer function for a specific action.
func (a *AllowlistAuthorizer) AllowFunc(action string, fn ports.AuthorizerFunc) error {
	if fn == nil {
		return errors.New("authorizer function is required")
	}
	return a.Allow(action, fn)
}

// AllowAny permits the specified action without additional checks.
func (a *AllowlistAuthorizer) AllowAny(action string) error {
	return a.Allow(action, ports.AuthorizerFunc(func(context.Context, any, string, any) error {
		return nil
	}))
}

// Can evaluates the allowlist entry for the given action.
func (a *AllowlistAuthorizer) Can(ctx context.Context, subject any, action string, resource any) error {
	if a == nil {
		return errors.New("authorizer not configured")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return httpx.ErrForbidden
	}
	a.mu.RLock()
	rule, ok := a.rules[action]
	a.mu.RUnlock()
	if !ok || rule == nil {
		return httpx.ErrForbidden
	}
	return rule.Can(ctx, subject, action, resource)
}
