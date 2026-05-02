// Package resend adapts the Resend API to the core email sender port.
//
// New constructs a client from an API key, LoadConfig reads contrib environment
// configuration, and HealthChecker can be used by services that want readiness
// visibility for outbound email delivery.
//
// Keep API keys in secret storage and use test or no-op senders in local tests.
package resend
