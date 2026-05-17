// Package entitlementstest provides reusable contract tests for entitlements stores.
package entitlementstest

import (
	"context"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v2/entitlements"
)

// StoreFactory creates an isolated store for a test.
type StoreFactory func(t *testing.T, plan entitlements.Plan) entitlements.Store

// RunStoreContract verifies the behavior expected from an entitlements.Store.
func RunStoreContract(t *testing.T, factory StoreFactory) {
	t.Helper()
	store := factory(t, entitlements.Plan{
		Features: map[entitlements.Feature]bool{"widgets": true},
		Quotas:   map[entitlements.Feature]entitlements.Quota{"widgets": {Limit: 2}},
	})
	service := entitlements.Service{Store: store}
	decision, err := service.Consume(context.Background(), "tenant-1", "widgets", 1)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if !decision.Allowed || decision.Used != 1 {
		t.Fatalf("Consume() decision = %#v", decision)
	}
	decision, err = service.Consume(context.Background(), "tenant-1", "widgets", 2)
	if err != nil {
		t.Fatalf("Consume() second error = %v", err)
	}
	if decision.Allowed || decision.Reason != entitlements.ReasonQuotaExceeded {
		t.Fatalf("Consume() second decision = %#v", decision)
	}
}
