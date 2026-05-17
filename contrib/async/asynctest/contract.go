// Package asynctest contains reusable async store contract tests.
package asynctest

import (
	"context"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v3/async"
)

// StoreFactory constructs a fresh async store for one contract test run.
//
// The returned store must have at least one due job available for Lease.
type StoreFactory func(t testing.TB) async.Store

// AssertStoreContract verifies behavior shared by async store implementations.
func AssertStoreContract(t testing.TB, newStore StoreFactory) {
	t.Helper()
	if newStore == nil {
		t.Fatal("newStore is required")
	}
	ctx := context.Background()
	store := newStore(t)
	jobs, err := store.Lease(ctx, 1)
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("Lease() jobs = %d, want 1", len(jobs))
	}
	if jobs[0].ID == "" || jobs[0].Kind == "" || jobs[0].TenantID == "" {
		t.Fatalf("Lease() returned malformed job: %#v", jobs[0])
	}
	if err := store.Complete(ctx, jobs[0].ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	store = newStore(t)
	jobs, err = store.Lease(ctx, 1)
	if err != nil {
		t.Fatalf("Lease() for failure path error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("Lease() for failure path jobs = %d, want 1", len(jobs))
	}
	if err := store.Fail(ctx, jobs[0].ID, "bounded failure"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
}
