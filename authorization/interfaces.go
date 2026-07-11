package authorization

import "context"

// Authorizer checks whether a subject can perform an action on a resource.
type Authorizer interface {
	Can(ctx context.Context, subject any, action string, resource any) error
}

// AuthorizerFunc adapts a function to the Authorizer contract.
type AuthorizerFunc func(ctx context.Context, subject any, action string, resource any) error

// Can executes the authorization function.
func (f AuthorizerFunc) Can(ctx context.Context, subject any, action string, resource any) error {
	return f(ctx, subject, action, resource)
}

// PolicyRequest captures policy engine evaluation inputs.
type PolicyRequest struct {
	Subject  any
	Action   string
	Resource any
	Context  any
}

// PolicyDecision represents a policy evaluation outcome.
type PolicyDecision struct {
	Allow  bool
	Reason string
	Data   any
}

// PolicyEngine evaluates policy requests.
type PolicyEngine interface {
	Evaluate(ctx context.Context, req PolicyRequest) (PolicyDecision, error)
}
