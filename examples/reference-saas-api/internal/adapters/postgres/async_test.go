package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/operationpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/outboxpostgres"
	"github.com/aatuh/api-toolkit/v4/operations"

	"example.com/reference-saas-api/internal/app"
)

func TestWidgetImportOperationIDParsesPayloadSafely(t *testing.T) {
	if got := widgetImportOperationID([]byte(`{"operation_id":" op_1 "}`)); got != "op_1" {
		t.Fatalf("widgetImportOperationID() = %q", got)
	}
	if got := widgetImportOperationID([]byte(`{"operation_id":""}`)); got != "" {
		t.Fatalf("empty widgetImportOperationID() = %q", got)
	}
	if got := widgetImportOperationID([]byte(`not-json`)); got != "" {
		t.Fatalf("invalid widgetImportOperationID() = %q", got)
	}
}

func TestWidgetImportOperationStoreRequiresPool(t *testing.T) {
	store := NewWidgetImportOperationStore(nil)
	if err := store.CreateWidgetImportOperation(context.Background(), "org_1", operationsOperationForTest()); !errors.Is(err, operationpostgres.ErrStoreNotConfigured) {
		t.Fatalf("CreateWidgetImportOperation() error = %v, want %v", err, operationpostgres.ErrStoreNotConfigured)
	}
}

func TestWidgetImportOutboxRequiresPool(t *testing.T) {
	outbox := NewWidgetImportOutbox(nil, nil)
	if _, err := outbox.Lease(context.Background(), 1); !errors.Is(err, outboxpostgres.ErrStoreNotConfigured) {
		t.Fatalf("Lease() error = %v, want %v", err, outboxpostgres.ErrStoreNotConfigured)
	}
}

func operationsOperationForTest() operations.Operation[app.WidgetImportResult] {
	return operations.Operation[app.WidgetImportResult]{ID: "op_1", State: operations.StatePending}
}
