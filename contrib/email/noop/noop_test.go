package noop

import (
	"context"
	"errors"
	"testing"

	"github.com/aatuh/api-toolkit/v4/email"
)

func TestSenderSendReturnsConfiguredID(t *testing.T) {
	sender := &Sender{ID: "local-only"}

	id, err := sender.Send(context.Background(), email.Message{})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if id != "local-only" {
		t.Fatalf("Send id = %q, want local-only", id)
	}
}

func TestSenderSendDefaultsIDAndAllowsNilReceiver(t *testing.T) {
	sender := &Sender{}
	id, err := sender.Send(context.Background(), email.Message{})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if id != "noop" {
		t.Fatalf("Send id = %q, want noop", id)
	}

	var nilSender *Sender
	id, err = nilSender.Send(context.Background(), email.Message{})
	if err != nil {
		t.Fatalf("nil Send returned error: %v", err)
	}
	if id != "" {
		t.Fatalf("nil Send id = %q, want empty", id)
	}
}

func TestSenderSendHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New().Send(ctx, email.Message{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send error = %v, want context.Canceled", err)
	}
}
