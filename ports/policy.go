package ports

import "context"

// PolicyRequest captures inputs for policy engine evaluation.
// Context carries request-scoped attributes in a format understood by the adapter.
type PolicyRequest struct {
	Subject  any
	Action   string
	Resource any
	Context  any
}

// PolicyDecision represents the outcome of a policy evaluation.
// Data may contain adapter-specific details for diagnostics.
type PolicyDecision struct {
	Allow  bool
	Reason string
	Data   any
}

// PolicyEngine evaluates policy requests.
type PolicyEngine interface {
	Evaluate(ctx context.Context, req PolicyRequest) (PolicyDecision, error)
}
