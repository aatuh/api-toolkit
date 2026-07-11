package authorization

import (
	"context"
	"testing"

	"github.com/aatuh/api-toolkit/v3/ports"
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

func requirePortsRequire(func(context.Context, ports.Authorizer, any, string, any) error) {}

func requirePortsAllow(func(*AllowlistAuthorizer, string, ports.Authorizer) error) {}

func requirePortsPolicyFactory(func(ports.PolicyEngine, PolicyAuthorizerOptions) *PolicyAuthorizer) {}

func requirePortsAuthorizerFunc(ports.AuthorizerFunc) {}

func requirePortsPolicyRequest(ports.PolicyRequest) {}

func requirePortsPolicyDecision(ports.PolicyDecision) {}
