package authorization

import (
	"context"
	"errors"
	"fmt"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// PolicyContextProvider supplies contextual attributes for policy evaluation.
type PolicyContextProvider func(ctx context.Context) any

// PolicyAuthorizerOptions configures a policy-backed authorizer.
type PolicyAuthorizerOptions struct {
	ContextProvider PolicyContextProvider
	DenyOnError     bool
}

// PolicyAuthorizer adapts a policy engine to the Authorizer interface.
type PolicyAuthorizer struct {
	engine          ports.PolicyEngine
	contextProvider PolicyContextProvider
	denyOnError     bool
}

// NewPolicyAuthorizer creates an authorizer backed by a policy engine.
func NewPolicyAuthorizer(engine ports.PolicyEngine, opts PolicyAuthorizerOptions) *PolicyAuthorizer {
	return &PolicyAuthorizer{
		engine:          engine,
		contextProvider: opts.ContextProvider,
		denyOnError:     opts.DenyOnError,
	}
}

// Can checks whether the subject can perform the action on the resource.
func (p *PolicyAuthorizer) Can(ctx context.Context, subject any, action string, resource any) error {
	if p == nil || p.engine == nil {
		return errors.New("policy engine not configured")
	}
	req := ports.PolicyRequest{
		Subject:  subject,
		Action:   action,
		Resource: resource,
	}
	if p.contextProvider != nil {
		req.Context = p.contextProvider(ctx)
	}
	decision, err := p.engine.Evaluate(ctx, req)
	if err != nil {
		if p.denyOnError {
			return httpx.ErrForbidden
		}
		return err
	}
	if decision.Allow {
		return nil
	}
	if decision.Reason != "" {
		return fmt.Errorf("policy denied: %s: %w", decision.Reason, httpx.ErrForbidden)
	}
	return httpx.ErrForbidden
}
