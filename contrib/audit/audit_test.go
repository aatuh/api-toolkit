package audit

import (
	"errors"
	"testing"
	"time"
)

func TestValidateEventAcceptsCompleteEvent(t *testing.T) {
	t.Parallel()

	event := validEvent()
	event.Metadata = map[string]string{"plan": "pro", "source": "api"}

	if err := ValidateEvent(event); err != nil {
		t.Fatalf("ValidateEvent() error = %v", err)
	}
}

func TestValidateEventRequiresStableFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event Event
	}{
		{name: "id", event: func() Event { event := validEvent(); event.ID = ""; return event }()},
		{name: "tenant", event: func() Event { event := validEvent(); event.TenantID = ""; return event }()},
		{name: "actor type", event: func() Event { event := validEvent(); event.Actor.Type = ""; return event }()},
		{name: "actor id", event: func() Event { event := validEvent(); event.Actor.ID = ""; return event }()},
		{name: "action", event: func() Event { event := validEvent(); event.Action = ""; return event }()},
		{name: "resource type", event: func() Event { event := validEvent(); event.Resource.Type = ""; return event }()},
		{name: "result", event: func() Event { event := validEvent(); event.Result = "maybe"; return event }()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateEvent(tt.event); !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("ValidateEvent() error = %v, want %v", err, ErrInvalidEvent)
			}
		})
	}
}

func TestValidateMetadataRejectsSecretShapedData(t *testing.T) {
	t.Parallel()

	tests := []map[string]string{
		{"authorization": "redacted"},
		{"api_key": "redacted"},
		{"safe": "Bearer abc"},
		{"safe": "private_key material"},
		{"pepper": "configured"},
	}

	for _, metadata := range tests {
		if err := ValidateMetadata(metadata); !errors.Is(err, ErrUnsafeMetadata) {
			t.Fatalf("ValidateMetadata(%v) error = %v, want %v", metadata, err, ErrUnsafeMetadata)
		}
	}
}

func TestCloneMetadataDefensiveCopy(t *testing.T) {
	t.Parallel()

	source := map[string]string{"plan": "pro"}
	clone := CloneMetadata(source)
	source["plan"] = "enterprise"

	if got := clone["plan"]; got != "pro" {
		t.Fatalf("CloneMetadata()[plan] = %q, want pro", got)
	}
}

func validEvent() Event {
	return Event{
		ID:       "audit_01",
		TenantID: "org_01",
		Actor: Actor{
			Type: "user",
			ID:   "usr_01",
		},
		Action: "widget.create",
		Resource: Resource{
			Type: "widget",
			ID:   "wgt_01",
		},
		Result:     ResultSuccess,
		RequestID:  "req_01",
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
	}
}
