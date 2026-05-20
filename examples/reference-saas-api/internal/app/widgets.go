package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"example.com/reference-saas-api/internal/domain"
)

var (
	ErrValidation         = errors.New("validation failed")
	ErrNotFound           = errors.New("not found")
	ErrPreconditionFailed = errors.New("precondition failed")
)

type WidgetService struct {
	mu            sync.Mutex
	next          int
	widgets       map[string]domain.Widget
	createReplays map[string]domain.Widget
	updateReplays map[string]domain.Widget
	deleteReplays map[string]struct{}
	now           func() time.Time
	store         WidgetStore
}

type WidgetStore interface {
	List(ctx context.Context, tenantID string) ([]domain.Widget, error)
	Get(ctx context.Context, tenantID, id string) (domain.Widget, bool, error)
	Save(ctx context.Context, widget domain.Widget) error
}

func NewWidgetService() *WidgetService {
	return &WidgetService{
		widgets:       map[string]domain.Widget{},
		createReplays: map[string]domain.Widget{},
		updateReplays: map[string]domain.Widget{},
		deleteReplays: map[string]struct{}{},
		now:           time.Now,
	}
}

func NewWidgetServiceWithStore(store WidgetStore) *WidgetService {
	service := NewWidgetService()
	service.store = store
	return service
}

func (s *WidgetService) List(ctx context.Context, tenantID string) ([]domain.Widget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrValidation
	}
	if s.store != nil {
		return s.store.List(ctx, tenantID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Widget, 0, len(s.widgets))
	for _, widget := range s.widgets {
		if widget.TenantID == tenantID && !widget.Deleted {
			out = append(out, widget)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *WidgetService) Create(ctx context.Context, tenantID, name, idempotencyKey string) (domain.Widget, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Widget{}, false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	name = strings.TrimSpace(name)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if tenantID == "" || name == "" || idempotencyKey == "" {
		return domain.Widget{}, false, ErrValidation
	}
	replayKey := tenantID + "\x00create\x00" + idempotencyKey
	s.mu.Lock()
	defer s.mu.Unlock()
	if widget, ok := s.createReplays[replayKey]; ok {
		return widget, true, nil
	}
	s.next++
	now := s.now().UTC()
	widget := domain.Widget{
		ID:        fmt.Sprintf("wgt_%06d", s.next),
		TenantID:  tenantID,
		Name:      name,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if s.store != nil {
		if err := s.store.Save(ctx, widget); err != nil {
			return domain.Widget{}, false, err
		}
	} else {
		s.widgets[widget.ID] = widget
	}
	s.createReplays[replayKey] = widget
	return widget, false, nil
}

func (s *WidgetService) Update(ctx context.Context, tenantID, id, name, ifMatch, idempotencyKey string) (domain.Widget, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Widget{}, false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	ifMatch = strings.TrimSpace(ifMatch)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if tenantID == "" || id == "" || name == "" || ifMatch == "" || idempotencyKey == "" {
		return domain.Widget{}, false, ErrValidation
	}
	replayKey := tenantID + "\x00update\x00" + id + "\x00" + idempotencyKey
	s.mu.Lock()
	defer s.mu.Unlock()
	if widget, ok := s.updateReplays[replayKey]; ok {
		return widget, true, nil
	}
	var (
		widget domain.Widget
		ok     bool
		err    error
	)
	if s.store != nil {
		widget, ok, err = s.store.Get(ctx, tenantID, id)
		if err != nil {
			return domain.Widget{}, false, err
		}
	} else {
		widget, ok = s.widgets[id]
	}
	if !ok || widget.Deleted || widget.TenantID != tenantID {
		return domain.Widget{}, false, ErrNotFound
	}
	if widget.ETag() != ifMatch {
		return domain.Widget{}, false, ErrPreconditionFailed
	}
	widget.Name = name
	widget.Version++
	widget.UpdatedAt = s.now().UTC()
	if s.store != nil {
		if err := s.store.Save(ctx, widget); err != nil {
			return domain.Widget{}, false, err
		}
	} else {
		s.widgets[id] = widget
	}
	s.updateReplays[replayKey] = widget
	return widget, false, nil
}

func (s *WidgetService) Delete(ctx context.Context, tenantID, id, idempotencyKey string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if tenantID == "" || id == "" || idempotencyKey == "" {
		return false, ErrValidation
	}
	replayKey := tenantID + "\x00delete\x00" + id + "\x00" + idempotencyKey
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.deleteReplays[replayKey]; ok {
		return true, nil
	}
	var (
		widget domain.Widget
		ok     bool
		err    error
	)
	if s.store != nil {
		widget, ok, err = s.store.Get(ctx, tenantID, id)
		if err != nil {
			return false, err
		}
	} else {
		widget, ok = s.widgets[id]
	}
	if !ok || widget.Deleted || widget.TenantID != tenantID {
		return false, ErrNotFound
	}
	widget.Deleted = true
	widget.Version++
	widget.UpdatedAt = s.now().UTC()
	if s.store != nil {
		if err := s.store.Save(ctx, widget); err != nil {
			return false, err
		}
	} else {
		s.widgets[id] = widget
	}
	s.deleteReplays[replayKey] = struct{}{}
	return false, nil
}
