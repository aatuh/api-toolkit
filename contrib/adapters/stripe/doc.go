// Package stripe adapts Stripe Checkout, webhooks, portal, and invoicing to the
// explicit compat/billing contracts.
//
// New constructs a Stripe-backed provider. Webhook verification is required by
// default; skip-verification paths are development-only and must be enabled with
// explicit dev context. This adapter is contrib-maintained and outside the
// stable core API promise.
package stripe
