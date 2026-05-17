package main

import (
	"context"
	"testing"

	"github.com/aatuh/api-toolkit/v3/middleware/auth/apikey"
)

func TestDemoVerifier(t *testing.T) {
	verifier := newDemoVerifier([]byte("secret"), "demo")
	principal, err := verifier.VerifyAPIKey(context.Background(), apikey.PresentedKey{Value: "demo"})
	if err != nil {
		t.Fatalf("VerifyAPIKey() error = %v", err)
	}
	if principal.ID != "demo-key" {
		t.Fatalf("principal ID = %q, want demo-key", principal.ID)
	}
	if _, err := verifier.VerifyAPIKey(context.Background(), apikey.PresentedKey{Value: "wrong"}); err == nil {
		t.Fatal("expected wrong key to fail")
	}
}
