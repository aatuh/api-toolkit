package webhooks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckReplayWindow(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	request := httptest.NewRequest(http.MethodPost, "/webhooks", nil)
	request.Header.Set(TimestampHeader, now.Format(time.RFC3339))
	request.Header.Set(EventIDHeader, "evt_1")
	decision := CheckReplayWindow(request, ReplayConfig{Tolerance: time.Minute, RequireEventID: true, Now: func() time.Time { return now }})
	if !decision.Allowed || decision.EventID != "evt_1" {
		t.Fatalf("decision = %#v", decision)
	}

	request.Header.Set(TimestampHeader, now.Add(-2*time.Minute).Format(time.RFC3339))
	decision = CheckReplayWindow(request, ReplayConfig{Tolerance: time.Minute, RequireEventID: true, Now: func() time.Time { return now }})
	if decision.Allowed || !strings.Contains(decision.Reason, "outside") {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestReceiverRejectsMissingReplayEventID(t *testing.T) {
	receiver := Receiver[map[string]string]{Config: ReceiverConfig[map[string]string]{
		Verifier: VerifierFunc(func(context.Context, *http.Request, []byte) error { return nil }),
		Replay:   ReplayConfig{RequireEventID: true},
	}}
	recorder := httptest.NewRecorder()
	receiver.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(`{"ok":"yes"}`)))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
