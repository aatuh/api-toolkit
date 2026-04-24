package email

import (
	"context"
	"reflect"
	"testing"
)

func TestMessageCanBeDeliveredThroughSender(t *testing.T) {
	msg := Message{
		From:    "support@example.test",
		To:      []string{"user@example.test", "admin@example.test"},
		ReplyTo: "reply@example.test",
		Subject: "Welcome",
		Text:    "plain text",
		HTML:    "<p>plain text</p>",
	}
	sender := &recordingSender{id: "email_123"}

	id, err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if id != "email_123" {
		t.Fatalf("id = %q, want email_123", id)
	}
	if !reflect.DeepEqual(sender.message, msg) {
		t.Fatalf("message = %#v, want %#v", sender.message, msg)
	}
}

type recordingSender struct {
	id      string
	message Message
}

func (s *recordingSender) Send(_ context.Context, msg Message) (string, error) {
	s.message = msg
	return s.id, nil
}

var _ Sender = (*recordingSender)(nil)
