package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/operationpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/outboxpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/async"
	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/operations"

	"example.com/reference-saas-api/internal/app"
)

type WidgetImportOperationStore struct {
	store *operationpostgres.Store[app.WidgetImportResult]
}

func NewWidgetImportOperationStore(pool contracts.DatabasePool) *WidgetImportOperationStore {
	return &WidgetImportOperationStore{store: operationpostgres.New[app.WidgetImportResult](pool, operationpostgres.Options{})}
}

func (s *WidgetImportOperationStore) CreateWidgetImportOperation(ctx context.Context, tenantID string, operation operations.Operation[app.WidgetImportResult]) error {
	if s == nil || s.store == nil {
		return operationpostgres.ErrStoreNotConfigured
	}
	return s.store.CreateOperation(operationpostgres.WithTenantID(ctx, tenantID), operation)
}

func (s *WidgetImportOperationStore) GetWidgetImportOperation(ctx context.Context, tenantID, id string) (operations.Operation[app.WidgetImportResult], bool, error) {
	if s == nil || s.store == nil {
		return operations.Operation[app.WidgetImportResult]{}, false, operationpostgres.ErrStoreNotConfigured
	}
	return s.store.GetOperation(operationpostgres.WithTenantID(ctx, tenantID), id)
}

func (s *WidgetImportOperationStore) UpdateWidgetImportOperation(ctx context.Context, tenantID string, operation operations.Operation[app.WidgetImportResult]) error {
	if s == nil || s.store == nil {
		return operationpostgres.ErrStoreNotConfigured
	}
	return s.store.UpdateOperation(operationpostgres.WithTenantID(ctx, tenantID), operation)
}

type WidgetImportOutbox struct {
	store      *outboxpostgres.Store
	operations *WidgetImportOperationStore
	mu         sync.Mutex
	leased     map[string]leasedWidgetImport
}

type leasedWidgetImport struct {
	TenantID    string
	OperationID string
}

func NewWidgetImportOutbox(pool contracts.DatabasePool, operations *WidgetImportOperationStore) *WidgetImportOutbox {
	return &WidgetImportOutbox{
		store:      outboxpostgres.New(pool, outboxpostgres.Options{}),
		operations: operations,
		leased:     map[string]leasedWidgetImport{},
	}
}

func (s *WidgetImportOutbox) EnqueueWidgetImport(ctx context.Context, event app.WidgetImportEvent) error {
	if s == nil || s.store == nil {
		return outboxpostgres.ErrStoreNotConfigured
	}
	return s.store.Enqueue(ctx, outboxpostgres.Event{
		ID:       strings.TrimSpace(event.ID),
		TenantID: strings.TrimSpace(event.TenantID),
		Type:     strings.TrimSpace(event.Kind),
		Payload:  append([]byte(nil), event.Payload...),
	})
}

func (s *WidgetImportOutbox) Lease(ctx context.Context, limit int) ([]async.Job, error) {
	if s == nil || s.store == nil {
		return nil, outboxpostgres.ErrStoreNotConfigured
	}
	jobs, err := s.store.Lease(ctx, limit)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		operationID := widgetImportOperationID(job.Payload)
		if operationID == "" {
			continue
		}
		s.remember(job.ID, leasedWidgetImport{TenantID: job.TenantID, OperationID: operationID})
		if s.operations == nil {
			continue
		}
		operation, ok, err := s.operations.GetWidgetImportOperation(ctx, job.TenantID, operationID)
		if err != nil || !ok || operation.State != operations.StatePending {
			if err != nil {
				return nil, err
			}
			continue
		}
		running, err := operations.TransitionOperation(operation, operations.TransitionConfig[app.WidgetImportResult]{To: operations.StateRunning})
		if err != nil {
			return nil, err
		}
		if err := s.operations.UpdateWidgetImportOperation(ctx, job.TenantID, running); err != nil {
			return nil, err
		}
	}
	return jobs, nil
}

func (s *WidgetImportOutbox) Complete(ctx context.Context, id string) error {
	if s == nil || s.store == nil {
		return outboxpostgres.ErrStoreNotConfigured
	}
	err := s.store.Complete(ctx, id)
	s.forget(id)
	return err
}

func (s *WidgetImportOutbox) Fail(ctx context.Context, id string, message string) error {
	if s == nil || s.store == nil {
		return outboxpostgres.ErrStoreNotConfigured
	}
	if err := s.store.Fail(ctx, id, message); err != nil {
		return err
	}
	leased := s.forget(id)
	if s.operations == nil || leased.OperationID == "" || leased.TenantID == "" {
		return nil
	}
	operation, ok, err := s.operations.GetWidgetImportOperation(ctx, leased.TenantID, leased.OperationID)
	if err != nil || !ok || operations.IsTerminal(operation.State) {
		return err
	}
	failed, err := operations.TransitionOperation(operation, operations.TransitionConfig[app.WidgetImportResult]{
		To:      operations.StateFailed,
		Problem: &httpx.Problem{Title: "Async work failed", Detail: "worker failed"},
	})
	if err != nil {
		return err
	}
	return s.operations.UpdateWidgetImportOperation(ctx, leased.TenantID, failed)
}

func (s *WidgetImportOutbox) remember(id string, leased leasedWidgetImport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leased[strings.TrimSpace(id)] = leased
}

func (s *WidgetImportOutbox) forget(id string) leasedWidgetImport {
	s.mu.Lock()
	defer s.mu.Unlock()
	leased := s.leased[strings.TrimSpace(id)]
	delete(s.leased, strings.TrimSpace(id))
	return leased
}

func widgetImportOperationID(payload []byte) string {
	var body struct {
		OperationID string `json:"operation_id"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.OperationID)
}
