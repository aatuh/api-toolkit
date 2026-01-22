package ports

import "context"

// Authorizer checks whether a subject can perform an action on a resource.
type Authorizer interface {
	Can(ctx context.Context, subject any, action string, resource any) error
}

// AuthorizerFunc adapts a function to the Authorizer interface.
type AuthorizerFunc func(ctx context.Context, subject any, action string, resource any) error

// Can executes the authorization function.
func (f AuthorizerFunc) Can(ctx context.Context, subject any, action string, resource any) error {
	return f(ctx, subject, action, resource)
}
