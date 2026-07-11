package authorization

import "github.com/aatuh/api-toolkit/v3/ports"

// Authorizer is the package-local authorization decision contract.
//
// It is an alias for ports.Authorizer during v3 compatibility. New
// authorization integrations should import this package instead of adding to
// root ports.
type Authorizer = ports.Authorizer

// AuthorizerFunc adapts a function to the package-local Authorizer contract.
//
// It is an alias for ports.AuthorizerFunc during v3 compatibility.
type AuthorizerFunc = ports.AuthorizerFunc

// PolicyEngine is the package-local policy evaluation contract.
//
// It is an alias for ports.PolicyEngine during v3 compatibility.
type PolicyEngine = ports.PolicyEngine

// PolicyRequest is the package-local policy evaluation input.
//
// It is an alias for ports.PolicyRequest during v3 compatibility.
type PolicyRequest = ports.PolicyRequest

// PolicyDecision is the package-local policy evaluation result.
//
// It is an alias for ports.PolicyDecision during v3 compatibility.
type PolicyDecision = ports.PolicyDecision
