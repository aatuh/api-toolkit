package authorization

import "context"

type scopeKey struct{}
type actorKey struct{}

// Actor identifies an authenticated principal.
type Actor struct {
	UserID string
}

// WithScope stores the authorization scope in context.
func WithScope(ctx context.Context, scope Scope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, scopeKey{}, scope)
}

// ScopeFromContext retrieves the authorization scope from context.
func ScopeFromContext(ctx context.Context) (Scope, bool) {
	if ctx == nil {
		return Scope{}, false
	}
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	return scope, ok
}

// TenantIDFromContext returns the tenant identifier from context scope.
func TenantIDFromContext(ctx context.Context) (string, bool) {
	scope, ok := ScopeFromContext(ctx)
	if !ok || scope.TenantID == "" {
		return "", false
	}
	return scope.TenantID, true
}

// WithActor stores an authenticated actor in context.
func WithActor(ctx context.Context, actor Actor) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, actorKey{}, actor)
}

// ActorFromContext retrieves the authenticated actor from context.
func ActorFromContext(ctx context.Context) (Actor, bool) {
	if ctx == nil {
		return Actor{}, false
	}
	actor, ok := ctx.Value(actorKey{}).(Actor)
	if !ok || actor.UserID == "" {
		return Actor{}, false
	}
	return actor, true
}
