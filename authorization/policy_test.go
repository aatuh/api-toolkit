package authorization

import (
	"context"
	"errors"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

type stubPolicyEngine struct {
	lastReq  PolicyRequest
	decision PolicyDecision
	err      error
}

func (s *stubPolicyEngine) Evaluate(_ context.Context, req PolicyRequest) (PolicyDecision, error) {
	s.lastReq = req
	return s.decision, s.err
}

func TestPolicyAuthorizerAllows(t *testing.T) {
	engine := &stubPolicyEngine{
		decision: PolicyDecision{Allow: true},
	}
	auth := NewPolicyAuthorizer(engine, PolicyAuthorizerOptions{
		ContextProvider: func(_ context.Context) any {
			return map[string]any{"tenant": "acme"}
		},
	})
	if err := auth.Can(context.Background(), "user", "read", "resource"); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	if engine.lastReq.Action != "read" {
		t.Fatalf("expected action captured, got %q", engine.lastReq.Action)
	}
	if engine.lastReq.Context == nil {
		t.Fatal("expected context to be passed")
	}
}

func TestPolicyAuthorizerDenies(t *testing.T) {
	engine := &stubPolicyEngine{
		decision: PolicyDecision{Allow: false},
	}
	auth := NewPolicyAuthorizer(engine, PolicyAuthorizerOptions{})
	if err := auth.Can(context.Background(), "user", "read", "resource"); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestPolicyAuthorizerDenyOnError(t *testing.T) {
	engine := &stubPolicyEngine{
		err: errors.New("boom"),
	}
	auth := NewPolicyAuthorizer(engine, PolicyAuthorizerOptions{
		DenyOnError: true,
	})
	if err := auth.Can(context.Background(), "user", "read", "resource"); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
