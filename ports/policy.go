package ports

import "context"

// PolicyRequest captures inputs for policy engine evaluation.
// Context carries request-scoped attributes in a format understood by the adapter.
//
// Deprecated: Use authorization.PolicyRequest. This value type remains
// available for v3 compatibility and may be removed in v4.
type PolicyRequest struct {
	Subject  any
	Action   string
	Resource any
	Context  any
}

// PolicyDecision represents the outcome of a policy evaluation.
// Data may contain adapter-specific details for diagnostics.
//
// Deprecated: Use authorization.PolicyDecision. This value type remains
// available for v3 compatibility and may be removed in v4.
type PolicyDecision struct {
	Allow  bool
	Reason string
	Data   any
}

// PolicyEngine evaluates policy requests.
//
// Deprecated: Use authorization.PolicyEngine. This interface remains available
// for v3 compatibility and may be removed in v4.
type PolicyEngine interface {
	Evaluate(ctx context.Context, req PolicyRequest) (PolicyDecision, error)
}
