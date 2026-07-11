package cedar

import (
	"context"
	"testing"

	cedarcore "github.com/cedar-policy/cedar-go"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/policytest"
	"github.com/aatuh/api-toolkit/v4/authorization"
)

func TestPolicyEngineContract(t *testing.T) {
	policytest.AssertEngineContract(t,
		func(t *testing.T) authorization.PolicyEngine {
			return newContractEngine(t, `permit (
				principal == User::"user_123",
				action == Action::"read",
				resource == Document::"doc_123"
			) when { context.tenant == "tenant_123" };`, false)
		},
		func(t *testing.T) authorization.PolicyEngine {
			return newContractEngine(t, `forbid (
				principal == User::"user_123",
				action == Action::"read",
				resource == Document::"doc_123"
			);`, false)
		},
		func(t *testing.T) authorization.PolicyEngine {
			return newContractEngine(t, `permit (principal, action, resource);`, true)
		},
	)
}

func TestEvaluateAllows(t *testing.T) {
	const policyText = `permit (
		principal == User::"alice",
		action == Action::"view",
		resource == Photo::"pic1"
	);`
	var policy cedarcore.Policy
	if err := policy.UnmarshalCedar([]byte(policyText)); err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	policies := cedarcore.NewPolicySet()
	policies.Add("policy0", &policy)

	engine, err := New(Config{Policies: policies})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	decision, err := engine.Evaluate(context.Background(), authorization.PolicyRequest{
		Subject:  cedarcore.NewEntityUID("User", "alice"),
		Action:   "view",
		Resource: cedarcore.NewEntityUID("Photo", "pic1"),
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !decision.Allow {
		t.Fatal("expected allow")
	}
}

func TestEvaluateWithContext(t *testing.T) {
	const policyText = `permit (
		principal,
		action == Action::"view",
		resource
	) when { context.tenant == "acme" };`
	var policy cedarcore.Policy
	if err := policy.UnmarshalCedar([]byte(policyText)); err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	policies := cedarcore.NewPolicySet()
	policies.Add("policy0", &policy)

	engine, err := New(Config{Policies: policies})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	decision, err := engine.Evaluate(context.Background(), authorization.PolicyRequest{
		Subject:  cedarcore.NewEntityUID("User", "alice"),
		Action:   "view",
		Resource: cedarcore.NewEntityUID("Photo", "pic1"),
		Context:  map[string]any{"tenant": "acme"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !decision.Allow {
		t.Fatal("expected allow")
	}
}

func TestRecordFromMapValidatesKeys(t *testing.T) {
	if _, err := RecordFromMap(map[string]any{"": "bad"}); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func newContractEngine(t *testing.T, policyText string, malformed bool) authorization.PolicyEngine {
	t.Helper()
	var policy cedarcore.Policy
	if err := policy.UnmarshalCedar([]byte(policyText)); err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	policies := cedarcore.NewPolicySet()
	policies.Add("policy0", &policy)
	engine, err := New(Config{
		Policies: policies,
		RequestBuilder: func(req authorization.PolicyRequest) (cedarcore.Request, error) {
			if malformed {
				return cedarcore.Request{}, context.Canceled
			}
			ctxRecord, err := recordFromContext(req.Context)
			if err != nil {
				return cedarcore.Request{}, err
			}
			return cedarcore.Request{
				Principal: cedarcore.NewEntityUID("User", "user_123"),
				Action:    cedarcore.NewEntityUID("Action", cedarcore.String(req.Action)),
				Resource:  cedarcore.NewEntityUID("Document", "doc_123"),
				Context:   ctxRecord,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return engine
}
