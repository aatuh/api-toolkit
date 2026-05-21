// Package entitlements defines supported-adapter contracts for tenant plans and quotas.
//
// Service, Store, Plan, Feature, and HTTP middleware helpers provide a
// provider-neutral boundary for checking feature access and recording usage.
// Generated services use the package to compose app-owned billing providers
// without importing provider SDKs into stable core code.
//
// Keep tenant IDs and quota names bounded in observers and error responses.
// Provider-specific subscription state belongs behind adapters or generated
// application code, not in this contract package.
package entitlements
