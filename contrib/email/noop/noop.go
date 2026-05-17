package noop

import (
	"context"

	"github.com/aatuh/api-toolkit/v3/email"
)

// Sender is a no-op email sender useful for tests or local development.
type Sender struct {
	ID string
}

// New returns a Sender that always succeeds.
func New() *Sender {
	return &Sender{ID: "noop"}
}

// Send returns a static ID without sending anything.
func (s *Sender) Send(ctx context.Context, _ email.Message) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s == nil {
		return "", nil
	}
	if s.ID == "" {
		return "noop", nil
	}
	return s.ID, nil
}

var _ email.Sender = (*Sender)(nil)
