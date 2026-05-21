// Package opa provides a supported-adapter Open Policy Agent policy engine.
//
// Use New with Config when authorization.NewPolicyAuthorizer should delegate
// provider-neutral policy requests to an OPA endpoint. The adapter maps request
// input to JSON, reads the configured result key, treats malformed or non-2xx
// responses as failures, and keeps client-facing errors generic.
//
// This contrib adapter remains outside the stable core API promise. Keep OPA
// query paths, timeouts, and policy documents application-owned.
package opa
