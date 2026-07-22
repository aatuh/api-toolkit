//go:build postgres

package operationpostgres

import (
	"context"
	"errors"
	"testing"

	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
	"github.com/aatuh/api-toolkit/v4/operations"
)

func TestStorePersistsTenantOperationsAgainstRealPostgres(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, testpostgres.Migration{
		Name: "operations",
		SQL: `CREATE TABLE operations (
			id text NOT NULL,
			organization_id text NOT NULL,
			state text NOT NULL,
			result bytea,
			error bytea,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL,
			PRIMARY KEY (id, organization_id)
		)`,
	}); err != nil {
		t.Fatalf("create operation schema: %v", err)
	}

	store := New[string](&pgxpooladapter.Adapter{Pool: h.Pool}, Options{Clock: h.FixedTime})
	tenantCtx := WithTenantID(ctx, "org-test")
	id := h.NextText("operation")
	if err := store.CreateOperation(tenantCtx, operations.Operation[string]{ID: id}); err != nil {
		t.Fatalf("CreateOperation() error = %v", err)
	}
	if err := store.CreateOperation(tenantCtx, operations.Operation[string]{ID: id}); err == nil {
		t.Fatal("duplicate operation was accepted")
	}
	got, found, err := store.GetOperation(tenantCtx, id)
	if err != nil || !found || got.State != operations.StatePending {
		t.Fatalf("GetOperation() = (%#v, %t, %v)", got, found, err)
	}
	result := "completed"
	if err := store.UpdateOperation(tenantCtx, operations.Operation[string]{ID: id, State: operations.StateSucceeded, Result: &result}); err != nil {
		t.Fatalf("UpdateOperation() error = %v", err)
	}
	got, found, err = store.GetOperation(tenantCtx, id)
	if err != nil || !found || got.Result == nil || *got.Result != result {
		t.Fatalf("updated GetOperation() = (%#v, %t, %v)", got, found, err)
	}
	if _, found, err := store.GetOperation(tenantCtx, "missing"); err != nil || found {
		t.Fatalf("missing GetOperation() = (%t, %v)", found, err)
	}
	if err := store.CreateOperation(h.CanceledContext(t), operations.Operation[string]{ID: h.NextText("operation")}); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("CreateOperation() canceled context error = %v, want tenant rejection", err)
	}
	if err := store.CreateOperation(WithTenantID(h.CanceledContext(t), "org-test"), operations.Operation[string]{ID: h.NextText("operation")}); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateOperation() canceled context error = %v", err)
	}
}
