// Package billing contains the v3 compatibility model for hosted checkout,
// billing portal, invoicing, and generic payment-provider webhooks.
//
// These contracts are defined here directly so provider-shaped billing
// boundaries stay out of the generic ports package. Use this package when the
// hosted-checkout, webhook, invoicing, and billing-portal shape fits the
// application; otherwise define an app-owned port or use a dedicated adapter
// contract.
//
// This package is compatibility-sensitive, not provider-neutral. Applications
// that need a different billing model should define an app-owned port or use a
// dedicated adapter contract instead of widening the stable ports package.
package billing
