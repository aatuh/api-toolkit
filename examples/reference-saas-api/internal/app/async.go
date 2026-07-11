package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/async"
	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/operations"
)

const WidgetImportJobKind = "widgets.import"

type WidgetImportItem struct {
	Name string `json:"name"`
}

type WidgetImportResult struct {
	Created   int      `json:"created"`
	WidgetIDs []string `json:"widget_ids"`
}

type AsyncService struct {
	mu               sync.Mutex
	nextOperation    int
	nextEvent        int
	widgets          *WidgetService
	operationStore   WidgetImportOperationStore
	outbox           WidgetImportOutbox
	operations       map[string]operations.Operation[WidgetImportResult]
	operationTenants map[string]string
	replays          map[string]string
	events           map[string]outboxEvent
	queue            []string
}

type WidgetImportOperationStore interface {
	CreateWidgetImportOperation(ctx context.Context, tenantID string, operation operations.Operation[WidgetImportResult]) error
	GetWidgetImportOperation(ctx context.Context, tenantID, id string) (operations.Operation[WidgetImportResult], bool, error)
	UpdateWidgetImportOperation(ctx context.Context, tenantID string, operation operations.Operation[WidgetImportResult]) error
}

type WidgetImportOutbox interface {
	EnqueueWidgetImport(ctx context.Context, event WidgetImportEvent) error
}

type WidgetImportEvent struct {
	ID          string
	TenantID    string
	Kind        string
	OperationID string
	Payload     []byte
}

type outboxEvent struct {
	ID          string
	OperationID string
	TenantID    string
	Kind        string
	Payload     []byte
	State       string
	Attempts    int
}

type widgetImportPayload struct {
	OperationID string             `json:"operation_id"`
	Items       []WidgetImportItem `json:"items"`
}

func NewAsyncService(widgets *WidgetService) *AsyncService {
	if widgets == nil {
		widgets = NewWidgetService()
	}
	return &AsyncService{
		widgets:          widgets,
		operations:       map[string]operations.Operation[WidgetImportResult]{},
		operationTenants: map[string]string{},
		replays:          map[string]string{},
		events:           map[string]outboxEvent{},
	}
}

func NewAsyncServiceWithStores(widgets *WidgetService, operationStore WidgetImportOperationStore, outbox WidgetImportOutbox) *AsyncService {
	service := NewAsyncService(widgets)
	service.operationStore = operationStore
	service.outbox = outbox
	return service
}

func (s *AsyncService) StartWidgetImport(ctx context.Context, tenantID, idempotencyKey string, items []WidgetImportItem) (operations.Operation[WidgetImportResult], bool, error) {
	if err := ctx.Err(); err != nil {
		return operations.Operation[WidgetImportResult]{}, false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	cleaned, err := cleanWidgetImportItems(items)
	if err != nil {
		return operations.Operation[WidgetImportResult]{}, false, err
	}
	if tenantID == "" || idempotencyKey == "" {
		return operations.Operation[WidgetImportResult]{}, false, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	replayKey := tenantID + "\x00widgets.import\x00" + idempotencyKey
	if operationID, ok := s.replays[replayKey]; ok {
		return s.operations[operationID], true, nil
	}
	s.nextOperation++
	operationID := formatGeneratedID("op", s.nextOperation)
	operation := operations.Operation[WidgetImportResult]{ID: operationID, State: operations.StatePending}
	payload, err := json.Marshal(widgetImportPayload{OperationID: operationID, Items: cleaned})
	if err != nil {
		return operations.Operation[WidgetImportResult]{}, false, err
	}
	s.nextEvent++
	eventID := formatGeneratedID("out", s.nextEvent)
	s.operations[operationID] = operation
	s.operationTenants[operationID] = tenantID
	s.replays[replayKey] = operationID
	if s.operationStore != nil {
		if err := s.operationStore.CreateWidgetImportOperation(ctx, tenantID, operation); err != nil {
			return operations.Operation[WidgetImportResult]{}, false, err
		}
	}
	if s.outbox != nil {
		if err := s.outbox.EnqueueWidgetImport(ctx, WidgetImportEvent{ID: eventID, TenantID: tenantID, Kind: WidgetImportJobKind, OperationID: operationID, Payload: payload}); err != nil {
			return operations.Operation[WidgetImportResult]{}, false, err
		}
		return operation, false, nil
	}
	s.events[eventID] = outboxEvent{
		ID:          eventID,
		OperationID: operationID,
		TenantID:    tenantID,
		Kind:        WidgetImportJobKind,
		Payload:     payload,
		State:       "pending",
	}
	s.queue = append(s.queue, eventID)
	return operation, false, nil
}

func (s *AsyncService) GetOperation(ctx context.Context, tenantID, id string) (operations.Operation[WidgetImportResult], bool, error) {
	if err := ctx.Err(); err != nil {
		return operations.Operation[WidgetImportResult]{}, false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	if tenantID == "" || id == "" {
		return operations.Operation[WidgetImportResult]{}, false, ErrValidation
	}
	if s.operationStore != nil {
		return s.operationStore.GetWidgetImportOperation(ctx, tenantID, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.operationTenants[id] != tenantID {
		return operations.Operation[WidgetImportResult]{}, false, nil
	}
	operation, ok := s.operations[id]
	return operation, ok, nil
}

func (s *AsyncService) Lease(ctx context.Context, limit int) ([]async.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]async.Job, 0, limit)
	for _, eventID := range s.queue {
		if len(jobs) >= limit {
			break
		}
		event := s.events[eventID]
		if event.State != "pending" {
			continue
		}
		event.State = "running"
		event.Attempts++
		s.events[eventID] = event
		if operation, ok := s.operations[event.OperationID]; ok && operation.State == operations.StatePending {
			running, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateRunning})
			if err != nil {
				return nil, err
			}
			s.operations[event.OperationID] = running
		}
		jobs = append(jobs, async.Job{
			ID:       event.ID,
			Kind:     event.Kind,
			TenantID: event.TenantID,
			Payload:  append([]byte(nil), event.Payload...),
			Attempts: event.Attempts,
		})
	}
	return jobs, nil
}

func (s *AsyncService) Complete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[id]
	if !ok {
		return ErrNotFound
	}
	event.State = "succeeded"
	s.events[id] = event
	return nil
}

func (s *AsyncService) Fail(ctx context.Context, id string, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[id]
	if !ok {
		return ErrNotFound
	}
	event.State = "failed"
	s.events[id] = event
	if operation, ok := s.operations[event.OperationID]; ok && !operations.IsTerminal(operation.State) {
		failed, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{
			To:      operations.StateFailed,
			Problem: &httpx.Problem{Title: "Async work failed", Detail: "worker failed"},
		})
		if err != nil {
			return err
		}
		s.operations[event.OperationID] = failed
	}
	_ = message
	return nil
}

func (s *AsyncService) Handle(ctx context.Context, job async.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if async.SafeLabel(job.Kind) != WidgetImportJobKind {
		return ErrValidation
	}
	var payload widgetImportPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return ErrValidation
	}
	tenantID := strings.TrimSpace(job.TenantID)
	operationID := strings.TrimSpace(payload.OperationID)
	if tenantID == "" || operationID == "" {
		return ErrValidation
	}
	createdIDs := make([]string, 0, len(payload.Items))
	for i, item := range payload.Items {
		widget, _, err := s.widgets.Create(ctx, tenantID, item.Name, operationID+"-"+formatGeneratedID("item", i+1))
		if err != nil {
			return err
		}
		createdIDs = append(createdIDs, widget.ID)
	}
	result := WidgetImportResult{Created: len(createdIDs), WidgetIDs: createdIDs}
	if s.operationStore != nil {
		return s.completeStoredOperation(ctx, tenantID, operationID, result)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.operations[operationID]
	if !ok {
		return ErrNotFound
	}
	if operations.IsTerminal(operation.State) {
		return nil
	}
	if operation.State == operations.StatePending {
		running, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateRunning})
		if err != nil {
			return err
		}
		operation = running
	}
	succeeded, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateSucceeded, Result: &result})
	if err != nil {
		return err
	}
	s.operations[operationID] = succeeded
	return nil
}

func (s *AsyncService) completeStoredOperation(ctx context.Context, tenantID, operationID string, result WidgetImportResult) error {
	operation, ok, err := s.operationStore.GetWidgetImportOperation(ctx, tenantID, operationID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if operations.IsTerminal(operation.State) {
		return nil
	}
	if operation.State == operations.StatePending {
		running, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateRunning})
		if err != nil {
			return err
		}
		operation = running
	}
	succeeded, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateSucceeded, Result: &result})
	if err != nil {
		return err
	}
	return s.operationStore.UpdateWidgetImportOperation(ctx, tenantID, succeeded)
}

func (s *AsyncService) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.runOnce(ctx); err != nil && ctx.Err() == nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *AsyncService) runOnce(ctx context.Context) error {
	jobs, err := s.Lease(ctx, 1)
	if err != nil || len(jobs) == 0 {
		return err
	}
	job := jobs[0]
	if err := s.Handle(ctx, job); err != nil {
		return s.Fail(ctx, job.ID, async.SafeFailureMessage(err))
	}
	return s.Complete(ctx, job.ID)
}

func cleanWidgetImportItems(items []WidgetImportItem) ([]WidgetImportItem, error) {
	if len(items) == 0 || len(items) > 100 {
		return nil, ErrValidation
	}
	out := make([]WidgetImportItem, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" || len(name) > 120 {
			return nil, ErrValidation
		}
		out = append(out, WidgetImportItem{Name: name})
	}
	return out, nil
}

func formatGeneratedID(prefix string, n int) string {
	return fmt.Sprintf("%s_%06d", prefix, n)
}
