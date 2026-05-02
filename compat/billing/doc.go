// Package billing exposes the provider-shaped v2 billing compatibility surface
// outside the generic ports package.
//
// New code that wants the existing hosted-checkout, webhook, invoicing, and
// billing-portal contract should depend on this package explicitly instead of
// importing those types from ports. All identifiers here are aliases to the
// existing ports exports so migration stays source-compatible for the rest of
// v2.
//
// This package is compatibility-sensitive, not provider-neutral. Applications
// that need a different billing model should define an app-owned port or use a
// dedicated adapter contract instead of widening the stable ports package.
package billing
