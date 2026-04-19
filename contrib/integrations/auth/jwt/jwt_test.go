package jwt

import (
	"context"
	"testing"
)

func TestLoadConfigReadsClaimRequirementsFromEnv(t *testing.T) {
	t.Setenv("JWT_REQUIRE_SUBJECT", "false")
	t.Setenv("JWT_REQUIRE_ISSUED_AT", "true")
	t.Setenv("JWT_REQUIRE_NOT_BEFORE", "false")

	cfg := LoadConfig(nil)

	if cfg.RequiredClaims.RequireSubject == nil || *cfg.RequiredClaims.RequireSubject {
		t.Fatal("expected subject requirement override to be false")
	}
	if cfg.RequiredClaims.RequireIssuedAt == nil || !*cfg.RequiredClaims.RequireIssuedAt {
		t.Fatal("expected issued-at requirement override to be true")
	}
	if cfg.RequiredClaims.RequireNotBefore == nil || *cfg.RequiredClaims.RequireNotBefore {
		t.Fatal("expected not-before requirement override to be false")
	}
}

func TestSubjectRoundTripAndDisabledConstructor(t *testing.T) {
	subj := Subject{UserID: "user-123", Email: "jwt@example.com", Claims: map[string]any{"role": "admin"}}
	ctx := WithSubject(context.Background(), subj)

	got, ok := SubjectFromContext(ctx)
	if !ok {
		t.Fatal("expected subject from context")
	}
	if got.UserID != subj.UserID || got.Email != subj.Email || got.Claims["role"] != "admin" {
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
