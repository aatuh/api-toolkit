package clerk

import (
	"context"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg := LoadConfig(nil)

	if !cfg.Enabled {
		t.Fatal("expected Clerk auth to default to enabled")
	}
	if cfg.JWKSRefreshInterval != 10*time.Minute {
		t.Fatalf("JWKSRefreshInterval = %v, want %v", cfg.JWKSRefreshInterval, 10*time.Minute)
	}
	if cfg.JWKSRefreshTimeout != 5*time.Second {
		t.Fatalf("JWKSRefreshTimeout = %v, want %v", cfg.JWKSRefreshTimeout, 5*time.Second)
	}
}

func TestSubjectRoundTripAndDisabledConstructor(t *testing.T) {
	subj := Subject{UserID: "user-123", Email: "clerk@example.com"}
	ctx := WithSubject(context.Background(), subj)

	got, ok := SubjectFromContext(ctx)
	if !ok {
		t.Fatal("expected subject from context")
	}
	if got != subj {
		t.Fatalf("SubjectFromContext() = %#v, want %#v", got, subj)
	}

	mw, err := NewMiddleware(context.Background(), Config{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("NewMiddleware() error = %v", err)
	}
	if mw == nil {
		t.Fatal("expected middleware instance")
	}
}
