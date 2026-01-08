package email

import "context"

// Message is a provider-agnostic email payload.
type Message struct {
	From    string
	To      []string
	ReplyTo string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers outbound email messages.
type Sender interface {
	Send(ctx context.Context, msg Message) (string, error)
}
