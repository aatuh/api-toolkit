// Package audittest contains reusable audit recorder contract tests.
package audittest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/audit"
)

// RecorderFactory constructs a fresh audit recorder for a contract test.
type RecorderFactory func(testing.TB) audit.Recorder

// AssertRecorderContract verifies common audit recorder behavior.
func AssertRecorderContract(t *testing.T, newRecorder RecorderFactory) {
	t.Helper()

	t.Run("records valid event", func(t *testing.T) {
		t.Parallel()

		recorder := newRecorder(t)
		if err := recorder.Record(context.Background(), validEvent()); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	})

	t.Run("rejects invalid event", func(t *testing.T) {
		t.Parallel()

		event := validEvent()
		event.Action = ""
		recorder := newRecorder(t)
		if err := recorder.Record(context.Background(), event); !errors.Is(err, audit.ErrInvalidEvent) {
			t.Fatalf("Record() error = %v, want %v", err, audit.ErrInvalidEvent)
		}
	})

	t.Run("rejects unsafe metadata", func(t *testing.T) {
		t.Parallel()

		event := validEvent()
		event.Metadata = map[string]string{"api_key": "redacted"}
		recorder := newRecorder(t)
		if err := recorder.Record(context.Background(), event); !errors.Is(err, audit.ErrUnsafeMetadata) {
			t.Fatalf("Record() error = %v, want %v", err, audit.ErrUnsafeMetadata)
		}
	})
}

func validEvent() audit.Event {
	return audit.Event{
		ID:       "audit_01",
		TenantID: "org_01",
		Actor: audit.Actor{
			Type: "user",
			ID:   "usr_01",
		},
		Action: "widget.create",
		Resource: audit.Resource{
			Type: "widget",
			ID:   "wgt_01",
		},
		Result:     audit.ResultSuccess,
		RequestID:  "req_01",
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		Metadata:   map[string]string{"plan": "pro"},
	}
}
