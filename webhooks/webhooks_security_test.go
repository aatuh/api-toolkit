package webhooks

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookSecurityConformance(t *testing.T) {
	now := time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)
	verifier, err := NewHMACSHA256Verifier(HMACConfig{Secret: []byte("secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
	}
	signer, err := NewHMACSHA256Signer(HMACSignerConfig{Secret: []byte("secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Signer() error = %v", err)
	}
	wrongSigner, err := NewHMACSHA256Signer(HMACSignerConfig{Secret: []byte("wrong-secret")})
	if err != nil {
		t.Fatalf("NewHMACSHA256Signer() wrong secret error = %v", err)
	}

	t.Run("signature mismatch fails closed", func(t *testing.T) {
		receiver, handledCount := securityConformanceReceiver(t, verifier, now)
		req := signedSecurityConformanceRequest(t, wrongSigner, "evt_mismatch", now)
		rec := httptest.NewRecorder()

		receiver.ServeHTTP(rec, req)

		assertWebhookSecurityFailure(t, rec, http.StatusUnauthorized, "webhook verification failed")
		if *handledCount != 0 {
			t.Fatalf("handler ran after signature mismatch: %d", *handledCount)
		}
	})

	t.Run("body tamper fails closed", func(t *testing.T) {
		receiver, handledCount := securityConformanceReceiver(t, verifier, now)
		req := signedSecurityConformanceRequest(t, signer, "evt_tamper", now)
		tampered := cloneWebhookRequestWithBody(t, req, []byte(`{"id":"evt_tamper","payload":{"id":"changed"}}`))
		rec := httptest.NewRecorder()

		receiver.ServeHTTP(rec, tampered)

		assertWebhookSecurityFailure(t, rec, http.StatusUnauthorized, "webhook verification failed")
		if *handledCount != 0 {
			t.Fatalf("handler ran after body tamper: %d", *handledCount)
		}
	})

	t.Run("replay window rejects stale timestamp", func(t *testing.T) {
		receiver, handledCount := securityConformanceReceiver(t, verifier, now)
		req := signedSecurityConformanceRequest(t, signer, "evt_stale", now.Add(-2*time.Minute))
		rec := httptest.NewRecorder()

		receiver.ServeHTTP(rec, req)

		assertWebhookSecurityFailure(t, rec, http.StatusUnauthorized, "outside the replay window")
		if *handledCount != 0 {
			t.Fatalf("handler ran for stale replay timestamp: %d", *handledCount)
		}
	})

	t.Run("clock skew rejects future timestamp", func(t *testing.T) {
		receiver, handledCount := securityConformanceReceiver(t, verifier, now)
		req := signedSecurityConformanceRequest(t, signer, "evt_future", now.Add(2*time.Minute))
		rec := httptest.NewRecorder()

		receiver.ServeHTTP(rec, req)

		assertWebhookSecurityFailure(t, rec, http.StatusUnauthorized, "outside the replay window")
		if *handledCount != 0 {
			t.Fatalf("handler ran for future clock skew: %d", *handledCount)
		}
	})

	t.Run("duplicate delivery can be idempotent by event id", func(t *testing.T) {
		seen := map[string]bool{}
		processed := 0
		receiver := Receiver[OutgoingEvent[testPayload]]{Config: ReceiverConfig[OutgoingEvent[testPayload]]{
			Verifier: verifier,
			Replay: ReplayConfig{
				Tolerance:      time.Minute,
				RequireEventID: true,
				Now:            func() time.Time { return now },
			},
			Handle: func(ctx context.Context, event Event[OutgoingEvent[testPayload]]) error {
				eventID := event.Request.Header.Get(EventIDHeader)
				if seen[eventID] {
					return nil
				}
				seen[eventID] = true
				processed++
				return nil
			},
		}}

		first := signedSecurityConformanceRequest(t, signer, "evt_duplicate", now)
		second := signedSecurityConformanceRequest(t, signer, "evt_duplicate", now)
		firstRec := httptest.NewRecorder()
		secondRec := httptest.NewRecorder()

		receiver.ServeHTTP(firstRec, first)
		receiver.ServeHTTP(secondRec, second)

		if firstRec.Code != http.StatusAccepted {
			t.Fatalf("first duplicate delivery status = %d body=%s", firstRec.Code, firstRec.Body.String())
		}
		if secondRec.Code != http.StatusAccepted {
			t.Fatalf("second duplicate delivery status = %d body=%s", secondRec.Code, secondRec.Body.String())
		}
		if processed != 1 {
			t.Fatalf("processed duplicate deliveries %d times, want 1", processed)
		}
	})
}

func securityConformanceReceiver(t *testing.T, verifier Verifier, now time.Time) (Receiver[OutgoingEvent[testPayload]], *int) {
	t.Helper()
	handled := 0
	receiver := Receiver[OutgoingEvent[testPayload]]{Config: ReceiverConfig[OutgoingEvent[testPayload]]{
		Verifier: verifier,
		Replay: ReplayConfig{
			Tolerance:      time.Minute,
			RequireEventID: true,
			Now:            func() time.Time { return now },
		},
		Handle: func(context.Context, Event[OutgoingEvent[testPayload]]) error {
			handled++
			return nil
		},
	}}
	return receiver, &handled
}

func signedSecurityConformanceRequest(t *testing.T, signer Signer, eventID string, timestamp time.Time) *http.Request {
	t.Helper()
	req, err := BuildSignedRequest(context.Background(), OutgoingEvent[testPayload]{
		ID:      eventID,
		Type:    "thing.created",
		Payload: testPayload{ID: eventID + "_payload"},
	}, SignedRequestConfig{
		URL:       "https://example.com/webhooks",
		Signer:    signer,
		Timestamp: timestamp,
	})
	if err != nil {
		t.Fatalf("BuildSignedRequest() error = %v", err)
	}
	return req
}

func cloneWebhookRequestWithBody(t *testing.T, req *http.Request, body []byte) *http.Request {
	t.Helper()
	cloned := httptest.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), bytes.NewReader(body))
	cloned.Header = req.Header.Clone()
	return cloned
}

func assertWebhookSecurityFailure(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantDetail string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("content type = %q", rec.Header().Get("Content-Type"))
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, wantDetail) {
		t.Fatalf("response body missing %q: %s", wantDetail, text)
	}
	for _, forbidden := range []string{"secret", "wrong-secret", "changed"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("security failure response leaked %q: %s", forbidden, text)
		}
	}
}
