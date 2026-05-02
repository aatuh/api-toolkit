// Package tenant provides stable tenant scoping middleware.
//
// Use the middleware to place a tenant identifier in request context before
// authorization, idempotency, repositories, or audit logging need tenant-aware
// behavior. Invalid or missing tenant data should fail at the application edge
// according to the service's routing policy.
package tenant
