// Package webhookdelivery defines supported-adapter outbound webhook delivery
// contracts for tenant-scoped API services.
//
// Use Catalog, Dispatcher, EndpointStore, DeliveryStore, SecretResolver, and
// async worker helpers to sign, dispatch, retry, and replay app-owned webhook
// deliveries. The package owns provider-neutral contracts and safe headers; it
// does not choose persistence, tenant mapping, or receiver-specific schemas.
//
// Keep signing secrets and raw payloads out of logs and metrics. Retry
// classification and replay paths must stay tenant-scoped and fail closed on
// mismatches.
package webhookdelivery
