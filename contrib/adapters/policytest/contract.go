// Package policytest provides reusable contract checks for contrib policy
// engine adapters.
package policytest

import (
	"context"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/authorization"
)

// EngineFactory constructs a policy engine for a contract scenario.
type EngineFactory func(t *testing.T) authorization.PolicyEngine

// AssertEngineContract verifies provider-neutral policy engine behavior shared
// by adapters such as OPA and Cedar.
func AssertEngineContract(t *testing.T, allow, deny, malformed EngineFactory) {
	t.Helper()
	t.Run("allow decision", func(t *testing.T) {
		decision, err := allow(t).Evaluate(context.Background(), contractRequest())
		if err != nil {
			t.Fatalf("evaluate allow: %v", err)
		}
		if !decision.Allow {
			t.Fatal("expected allow decision")
		}
	})
	t.Run("deny decision", func(t *testing.T) {
		decision, err := deny(t).Evaluate(context.Background(), contractRequest())
		if err != nil {
			t.Fatalf("evaluate deny: %v", err)
		}
		if decision.Allow {
			t.Fatal("expected deny decision")
		}
	})
	t.Run("malformed policy input fails safe", func(t *testing.T) {
		_, err := malformed(t).Evaluate(context.Background(), contractRequest())
		if err == nil {
			t.Fatal("expected malformed policy error")
		}
		for _, secret := range []string{"redacted-token-placeholder", "redacted-key-placeholder"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("policy error leaked sensitive value %q: %v", secret, err)
			}
		}
	})
}

func contractRequest() authorization.PolicyRequest {
	return authorization.PolicyRequest{
		Subject: map[string]any{
			"id": "user_123",
		},
		Action: "read",
		Resource: map[string]any{
			"id": "doc_123",
		},
		Context: map[string]any{
			"tenant":    "tenant_123",
			"markerOne": "redacted-token-placeholder",
			"markerTwo": "redacted-key-placeholder",
		},
	}
}
