package app

import (
	"context"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v3/audit"
)

func TestAuditServiceRecordsAndRedactsMetadata(t *testing.T) {
	service := NewAuditService()
	err := service.Record(context.Background(), audit.Event{
		TenantID:  "org_1",
		Actor:     audit.Actor{Type: "user", ID: "usr_1"},
		Action:    "widget.create",
		Resource:  audit.Resource{Type: "widget", ID: "wgt_1"},
		Result:    audit.ResultSuccess,
		RequestID: "req_1",
		Metadata: map[string]string{
			"count":   "2",
			"api_key": "atk_secret",
			"note":    "contains secret token",
		},
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	events, err := service.Events(context.Background())
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].ID == "" || events[0].OccurredAt.IsZero() {
		t.Fatalf("event missing generated fields: %#v", events[0])
	}
	if events[0].Metadata["count"] != "2" {
		t.Fatalf("safe metadata missing: %#v", events[0].Metadata)
	}
	for key, value := range events[0].Metadata {
		if strings.Contains(strings.ToLower(key), "key") || strings.Contains(strings.ToLower(value), "secret") {
			t.Fatalf("unsafe audit metadata survived: %#v", events[0].Metadata)
		}
	}
}

func TestAuditServiceWithRecorderRedactsBeforeDelegating(t *testing.T) {
	recorder := &recordingAuditRecorder{}
	service := NewAuditServiceWithRecorder(recorder)
	err := service.Record(context.Background(), audit.Event{
		TenantID: "org_1",
		Actor:    audit.Actor{Type: "user", ID: "usr_1"},
		Action:   "api_key.create",
		Resource: audit.Resource{Type: "api_key", ID: "key_1"},
		Result:   audit.ResultSuccess,
		Metadata: map[string]string{
			"scope_count": "2",
			"token":       "raw-secret-token",
		},
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("delegated events = %#v", recorder.events)
	}
	if recorder.events[0].ID == "" || recorder.events[0].OccurredAt.IsZero() {
		t.Fatalf("delegated event missing generated fields: %#v", recorder.events[0])
	}
	if _, ok := recorder.events[0].Metadata["token"]; ok {
		t.Fatalf("delegated metadata leaked token: %#v", recorder.events[0].Metadata)
	}
	if recorder.events[0].Metadata["scope_count"] != "2" {
		t.Fatalf("delegated metadata = %#v", recorder.events[0].Metadata)
	}
}

type recordingAuditRecorder struct {
	events []audit.Event
}

func (r *recordingAuditRecorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}
