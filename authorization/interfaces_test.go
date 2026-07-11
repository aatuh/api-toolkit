package authorization

import (
	"context"
	"testing"
)

func TestPackageLocalInterfacesPreserveV3PortsIdentity(_ *testing.T) {
	requirePortsRequire(Require)
	requirePortsAllow((*AllowlistAuthorizer).Allow)
	requirePortsPolicyFactory(NewPolicyAuthorizer)

	var authorizerFunc AuthorizerFunc
	requirePortsAuthorizerFunc(authorizerFunc)

	var request PolicyRequest
	requirePortsPolicyRequest(request)

	var decision PolicyDecision
	requirePortsPolicyDecision(decision)
}

func requirePortsRequire(func(context.Context, Authorizer, any, string, any) error) {}

func requirePortsAllow(func(*AllowlistAuthorizer, string, Authorizer) error) {}

func requirePortsPolicyFactory(func(PolicyEngine, PolicyAuthorizerOptions) *PolicyAuthorizer) {}

func requirePortsAuthorizerFunc(AuthorizerFunc) {}

func requirePortsPolicyRequest(PolicyRequest) {}

func requirePortsPolicyDecision(PolicyDecision) {}
