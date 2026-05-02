// Package webhooks provides reusable webhook receiver primitives.
//
// It verifies raw request bodies before decoding JSON payloads and leaves
// provider-specific event schemas, replay storage, and idempotency policy to
// application code.
package webhooks
