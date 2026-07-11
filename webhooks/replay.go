package webhooks

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

const (
	// TimestampHeader is the default webhook timestamp header.
	TimestampHeader = "X-Webhook-Timestamp"
	// EventIDHeader is the default webhook event id header.
	EventIDHeader = "X-Webhook-Event-ID"
)

// ReplayConfig configures timestamp skew and event-id replay checks.
type ReplayConfig struct {
	Tolerance       time.Duration
	Now             func() time.Time
	RequireEventID  bool
	TimestampHeader string
	EventIDHeader   string
}

// ReplayDecision reports the replay-window check outcome.
type ReplayDecision struct {
	Allowed   bool
	Reason    string
	EventID   string
	Timestamp time.Time
}

// CheckReplayWindow verifies timestamp skew and required event ids using headers.
func CheckReplayWindow(r *http.Request, config ReplayConfig) ReplayDecision {
	if r == nil {
		return ReplayDecision{Allowed: false, Reason: "webhook request is required"}
	}
	eventIDHeader := strings.TrimSpace(config.EventIDHeader)
	if eventIDHeader == "" {
		eventIDHeader = EventIDHeader
	}
	eventID, eventIDPresent, err := singleHeaderValue(r, eventIDHeader)
	if err != nil {
		return ReplayDecision{Allowed: false, Reason: "webhook event id header is invalid"}
	}
	if config.RequireEventID && !eventIDPresent {
		return ReplayDecision{Allowed: false, Reason: "webhook event id is required"}
	}
	if config.Tolerance <= 0 {
		return ReplayDecision{Allowed: true, EventID: eventID}
	}
	timestampHeader := strings.TrimSpace(config.TimestampHeader)
	if timestampHeader == "" {
		timestampHeader = TimestampHeader
	}
	rawTimestamp, timestampPresent, err := singleHeaderValue(r, timestampHeader)
	if err != nil {
		return ReplayDecision{Allowed: false, Reason: "webhook timestamp header is invalid", EventID: eventID}
	}
	if !timestampPresent {
		return ReplayDecision{Allowed: false, Reason: "webhook timestamp is required", EventID: eventID}
	}
	timestamp, err := parseWebhookTimestamp(rawTimestamp)
	if err != nil {
		return ReplayDecision{Allowed: false, Reason: "webhook timestamp is invalid", EventID: eventID}
	}
	now := time.Now().UTC()
	if config.Now != nil {
		now = config.Now().UTC()
	}
	if timestamp.Before(now.Add(-config.Tolerance)) || timestamp.After(now.Add(config.Tolerance)) {
		return ReplayDecision{Allowed: false, Reason: "webhook timestamp is outside the replay window", EventID: eventID, Timestamp: timestamp}
	}
	return ReplayDecision{Allowed: true, EventID: eventID, Timestamp: timestamp}
}

// DeliveryAttempt describes an outbound webhook delivery attempt.
type DeliveryAttempt struct {
	EventID   string    `json:"event_id"`
	URL       string    `json:"url"`
	Attempt   int       `json:"attempt"`
	Timestamp time.Time `json:"timestamp"`
}

// DeliveryResult describes an outbound webhook delivery result.
type DeliveryResult struct {
	StatusCode int            `json:"status_code"`
	Accepted   bool           `json:"accepted"`
	Problem    *httpx.Problem `json:"problem,omitempty"`
}

// DeliveryProblem returns the standard webhook delivery Problem Details value.
func DeliveryProblem(status int, detail string) httpx.Problem {
	if status == 0 {
		status = http.StatusBadGateway
	}
	if strings.TrimSpace(detail) == "" {
		detail = "webhook delivery failed"
	}
	return httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeServiceUnavailable), Title: http.StatusText(status), Detail: detail}
}

// DeliveryContract documents accepted delivery status and retry behavior.
type DeliveryContract struct {
	AcceptedStatus int           `json:"accepted_status"`
	MaxAttempts    int           `json:"max_attempts,omitempty"`
	RetryAfter     time.Duration `json:"retry_after,omitempty"`
}

func parseWebhookTimestamp(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := http.ParseTime(value); err == nil {
		return parsed.UTC(), nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(seconds, 0).UTC(), nil
}
