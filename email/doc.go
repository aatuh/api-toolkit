// Package email defines the stable core email port.
//
// Message is the provider-neutral payload shape used by outbound email senders.
// Sender is intentionally narrow so applications can adapt Resend, SMTP, test,
// or no-op delivery without depending on a provider package from the stable
// core module.
package email
