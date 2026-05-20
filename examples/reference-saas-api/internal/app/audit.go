package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/aatuh/api-toolkit/contrib/v3/audit"
)

type AuditService struct {
	mu       sync.Mutex
	next     int
	now      func() time.Time
	recorder audit.Recorder
	events   []audit.Event
}

func NewAuditService() *AuditService {
	return &AuditService{now: time.Now}
}

func NewAuditServiceWithRecorder(recorder audit.Recorder) *AuditService {
	service := NewAuditService()
	service.recorder = recorder
	return service
}

func (s *AuditService) Record(ctx context.Context, event audit.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	event.ID = fmt.Sprintf("aud_%06d", s.next)
	event.TenantID = strings.TrimSpace(event.TenantID)
	event.Actor.Type = cleanAuditLabel(event.Actor.Type)
	event.Actor.ID = strings.TrimSpace(event.Actor.ID)
	event.Action = cleanAuditLabel(event.Action)
	event.Resource.Type = cleanAuditLabel(event.Resource.Type)
	event.Resource.ID = strings.TrimSpace(event.Resource.ID)
	if event.Result == "" {
		event.Result = audit.ResultSuccess
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.now().UTC()
	}
	event.RequestID = strings.TrimSpace(event.RequestID)
	event.Metadata = safeAuditMetadata(event.Metadata)
	if err := audit.ValidateEvent(event); err != nil {
		return err
	}
	if s.recorder != nil {
		return s.recorder.Record(ctx, event)
	}
	s.events = append(s.events, event)
	return nil
}

func (s *AuditService) Events(ctx context.Context) ([]audit.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]audit.Event(nil), s.events...)
	for i := range out {
		out[i].Metadata = audit.CloneMetadata(out[i].Metadata)
	}
	return out, nil
}

func safeAuditMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		cleanKey := cleanAuditMetadataPart(key)
		if cleanKey == "" || unsafeAuditMetadataPart(cleanKey) {
			continue
		}
		value := cleanAuditMetadataPart(metadata[key])
		if value == "" || unsafeAuditMetadataPart(value) {
			continue
		}
		out[cleanKey] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanAuditLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			out.WriteRune(r)
		}
		if out.Len() >= 80 {
			break
		}
	}
	return out.String()
}

func cleanAuditMetadataPart(value string) string {
	value = strings.TrimSpace(value)
	var out strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			continue
		}
		out.WriteRune(r)
		if out.Len() >= 128 {
			break
		}
	}
	return strings.TrimSpace(out.String())
}

func unsafeAuditMetadataPart(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, token := range []string{"authorization", "bearer ", "cookie", "password", "private_key", "secret", "set-cookie", "token", "api_key", "apikey", "pepper", "idempotency"} {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}
