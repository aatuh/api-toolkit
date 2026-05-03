// Package webhooks provides reusable webhook receiver and signing primitives.
//
// It verifies raw request bodies before decoding JSON payloads, signs outbound
// JSON webhook requests, and leaves provider-specific event schemas, replay
// storage, delivery retries, and idempotency policy to application code.
// Receiver verifier failures return a generic client-facing detail by default;
// use ReceiverConfig.VerificationErrorDetail only for explicitly safe text.
package webhooks
