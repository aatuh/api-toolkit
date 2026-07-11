// Package operationpostgres provides a supported-adapter Postgres operation repository.
//
// Use New with a contracts.DatabasePool and Options to persist operations.Operation
// lifecycle state for async HTTP workflows. Store implements
// operations.Repository and uses WithTenantID to bind repository calls to the
// tenant selected by earlier middleware.
//
// The adapter validates table names, stores result/problem payloads as JSON, and
// exposes HealthChecker for readiness. Missing tenant context fails closed
// instead of writing cross-tenant operation rows.
package operationpostgres
