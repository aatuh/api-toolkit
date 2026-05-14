package audit

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Result describes the outcome recorded by an audit event.
type Result string

const (
	// ResultSuccess records an allowed action that completed successfully.
	ResultSuccess Result = "success"
	// ResultDenied records an authorization or policy denial.
	ResultDenied Result = "denied"
	// ResultFailure records an attempted action that failed.
	ResultFailure Result = "failure"
)

var (
	// ErrInvalidEvent reports that an audit event is missing required fields.
	ErrInvalidEvent = errors.New("invalid audit event")
	// ErrUnsafeMetadata reports that audit metadata appears to contain a secret.
	ErrUnsafeMetadata = errors.New("unsafe audit metadata")
)

// Actor identifies the principal responsible for an action.
type Actor struct {
	Type string
	ID   string
}

// Resource identifies the object affected by an action.
type Resource struct {
	Type string
	ID   string
}

// Event records a security-relevant action for tenant-scoped services.
type Event struct {
	ID         string
	TenantID   string
	Actor      Actor
	Action     string
	Resource   Resource
	Result     Result
	RequestID  string
	Metadata   map[string]string
	OccurredAt time.Time
}

// Recorder stores audit events.
type Recorder interface {
	Record(ctx context.Context, event Event) error
}

// ValidateEvent verifies required fields and metadata safety.
func ValidateEvent(event Event) error {
	if strings.TrimSpace(event.ID) == "" ||
		strings.TrimSpace(event.TenantID) == "" ||
		strings.TrimSpace(event.Actor.Type) == "" ||
		strings.TrimSpace(event.Actor.ID) == "" ||
		strings.TrimSpace(event.Action) == "" ||
		strings.TrimSpace(event.Resource.Type) == "" ||
		strings.TrimSpace(string(event.Result)) == "" {
		return ErrInvalidEvent
	}
	switch event.Result {
	case ResultSuccess, ResultDenied, ResultFailure:
	default:
		return ErrInvalidEvent
	}
	if err := ValidateMetadata(event.Metadata); err != nil {
		return err
	}
	return nil
}

// ValidateMetadata rejects metadata keys that commonly carry raw secrets.
func ValidateMetadata(metadata map[string]string) error {
	for key, value := range metadata {
		if unsafeMetadataToken(key) || unsafeMetadataToken(value) {
			return ErrUnsafeMetadata
		}
	}
	return nil
}

// CloneMetadata returns a defensive copy of metadata.
func CloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	clone := make(map[string]string, len(metadata))
	for key, value := range metadata {
		clone[key] = value
	}
	return clone
}

func unsafeMetadataToken(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	for _, token := range []string{
		"authorization",
		"bearer ",
		"cookie",
		"password",
		"private_key",
		"secret",
		"set-cookie",
		"token",
		"api_key",
		"apikey",
		"pepper",
	} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}
