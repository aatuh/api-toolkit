package authorization

import (
	"context"
	"errors"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestAllowlistAuthorizerDefaultDeny(t *testing.T) {
	auth := NewAllowlistAuthorizer()
	if err := auth.Can(context.Background(), "user", "read", nil); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAllowlistAuthorizerAllowAny(t *testing.T) {
	auth := NewAllowlistAuthorizer()
	if err := auth.AllowAny("read"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := auth.Can(context.Background(), "user", "read", nil); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestAllowlistAuthorizerDelegates(t *testing.T) {
	auth := NewAllowlistAuthorizer()
	deny := AuthorizerFunc(func(ctx context.Context, subject any, action string, resource any) error {
		return httpx.ErrForbidden
	})
	if err := auth.Allow("write", deny); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := auth.Can(context.Background(), "user", "write", nil); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAllowlistAuthorizerRejectsEmptyAction(t *testing.T) {
	auth := NewAllowlistAuthorizer()
	if err := auth.AllowAny(" "); err == nil {
		t.Fatal("expected error for empty action")
	}
}
