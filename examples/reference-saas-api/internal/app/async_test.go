package app

import (
	"context"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v3/async"
	"github.com/aatuh/api-toolkit/v3/operations"
)

func TestAsyncServiceCompletesWidgetImport(t *testing.T) {
	ctx := context.Background()
	widgets := NewWidgetService()
	service := NewAsyncService(widgets)
	operation, replayed, err := service.StartWidgetImport(ctx, "org_1", "idem_1", []WidgetImportItem{{Name: " alpha "}, {Name: "beta"}})
	if err != nil {
		t.Fatalf("StartWidgetImport() error = %v", err)
	}
	if replayed || operation.State != operations.StatePending {
		t.Fatalf("operation = %#v replayed=%v", operation, replayed)
	}
	replay, replayed, err := service.StartWidgetImport(ctx, "org_1", "idem_1", []WidgetImportItem{{Name: "alpha"}})
	if err != nil {
		t.Fatalf("StartWidgetImport() replay error = %v", err)
	}
	if !replayed || replay.ID != operation.ID {
		t.Fatalf("replay = %#v replayed=%v, want same operation", replay, replayed)
	}

	jobs, err := service.Lease(ctx, 1)
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].Kind != WidgetImportJobKind || jobs[0].TenantID != "org_1" {
		t.Fatalf("leased jobs = %#v", jobs)
	}
	if err := service.Handle(ctx, jobs[0]); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if err := service.Complete(ctx, jobs[0].ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	got, ok, err := service.GetOperation(ctx, "org_1", operation.ID)
	if err != nil {
		t.Fatalf("GetOperation() error = %v", err)
	}
	if !ok || got.State != operations.StateSucceeded || got.Result == nil || got.Result.Created != 2 {
		t.Fatalf("operation after completion = %#v ok=%v", got, ok)
	}
	if _, ok, err := service.GetOperation(ctx, "org_2", operation.ID); err != nil || ok {
		t.Fatalf("cross-tenant GetOperation() ok=%v err=%v", ok, err)
	}
	list, err := widgets.List(ctx, "org_1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 || list[0].Name != "alpha" || list[1].Name != "beta" {
		t.Fatalf("widgets = %#v", list)
	}
}

func TestAsyncServiceFailureDoesNotExposePayloadOrRawError(t *testing.T) {
	ctx := context.Background()
	service := NewAsyncService(NewWidgetService())
	operation, _, err := service.StartWidgetImport(ctx, "org_1", "idem_1", []WidgetImportItem{{Name: "secret-widget-name"}})
	if err != nil {
		t.Fatalf("StartWidgetImport() error = %v", err)
	}
	jobs, err := service.Lease(ctx, 1)
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("leased jobs = %#v", jobs)
	}
	if err := service.Fail(ctx, jobs[0].ID, "provider failed with secret-widget-name"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	got, ok, err := service.GetOperation(ctx, "org_1", operation.ID)
	if err != nil {
		t.Fatalf("GetOperation() error = %v", err)
	}
	if !ok || got.State != operations.StateFailed || got.Problem == nil {
		t.Fatalf("failed operation = %#v ok=%v", got, ok)
	}
	if strings.Contains(got.Problem.Detail, "secret-widget-name") {
		t.Fatalf("operation problem leaked payload: %#v", got.Problem)
	}
}

func TestAsyncServiceWithStoresPersistsOperationAndOutbox(t *testing.T) {
	ctx := context.Background()
	operationStore := newRecordingWidgetImportOperationStore()
	outbox := &recordingWidgetImportOutbox{}
	service := NewAsyncServiceWithStores(NewWidgetService(), operationStore, outbox)

	operation, replayed, err := service.StartWidgetImport(ctx, "org_1", "idem_1", []WidgetImportItem{WidgetImportItem{Name: "alpha"}})
	if err != nil {
		t.Fatalf("StartWidgetImport() error = %v", err)
	}
	if replayed || operationStore.createdTenant != "org_1" || operationStore.operations[operation.ID].State != operations.StatePending {
		t.Fatalf("operation=%#v replayed=%v store=%#v", operation, replayed, operationStore)
	}
	if outbox.event.ID == "" || outbox.event.OperationID != operation.ID || outbox.event.TenantID != "org_1" || string(outbox.event.Payload) == "" {
		t.Fatalf("outbox event = %#v", outbox.event)
	}
	got, ok, err := service.GetOperation(ctx, "org_1", operation.ID)
	if err != nil || !ok || got.ID != operation.ID {
		t.Fatalf("GetOperation() operation=%#v ok=%v err=%v", got, ok, err)
	}
	if err := service.Handle(ctx, async.Job{ID: outbox.event.ID, Kind: WidgetImportJobKind, TenantID: "org_1", Payload: outbox.event.Payload}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	updated := operationStore.operations[operation.ID]
	if updated.State != operations.StateSucceeded || updated.Result == nil || updated.Result.Created != 1 {
		t.Fatalf("updated operation = %#v", updated)
	}
}

type recordingWidgetImportOperationStore struct {
	createdTenant string
	updatedTenant string
	operations    map[string]operations.Operation[WidgetImportResult]
}

func newRecordingWidgetImportOperationStore() *recordingWidgetImportOperationStore {
	return &recordingWidgetImportOperationStore{operations: map[string]operations.Operation[WidgetImportResult]{}}
}

func (s *recordingWidgetImportOperationStore) CreateWidgetImportOperation(_ context.Context, tenantID string, operation operations.Operation[WidgetImportResult]) error {
	s.createdTenant = tenantID
	s.operations[operation.ID] = operation
	return nil
}

func (s *recordingWidgetImportOperationStore) GetWidgetImportOperation(_ context.Context, tenantID, id string) (operations.Operation[WidgetImportResult], bool, error) {
	operation, ok := s.operations[id]
	if !ok || tenantID != s.createdTenant {
		return operations.Operation[WidgetImportResult]{}, false, nil
	}
	return operation, true, nil
}

func (s *recordingWidgetImportOperationStore) UpdateWidgetImportOperation(_ context.Context, tenantID string, operation operations.Operation[WidgetImportResult]) error {
	s.updatedTenant = tenantID
	s.operations[operation.ID] = operation
	return nil
}

type recordingWidgetImportOutbox struct {
	event WidgetImportEvent
}

func (s *recordingWidgetImportOutbox) EnqueueWidgetImport(_ context.Context, event WidgetImportEvent) error {
	s.event = event
	return nil
}
