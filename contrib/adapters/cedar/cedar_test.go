package cedar

import (
	"context"
	"testing"

	cedarcore "github.com/cedar-policy/cedar-go"

	"github.com/aatuh/api-toolkit/v2/ports"
)

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
	decision, err := engine.Evaluate(context.Background(), ports.PolicyRequest{
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
	decision, err := engine.Evaluate(context.Background(), ports.PolicyRequest{
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
