// Package routecontracts registers HTTP routes and OpenAPI operations together.
//
// It is intentionally router-neutral and uses only the common method
// registration shape already provided by ports.HTTPRouter. PATCH support is
// detected with a local optional interface so stable router interfaces are not
// widened.
//
// Registered routes automatically attach bounded routepolicy observability
// labels to the request context so outer metrics and request logging middleware
// can record policy shape without seeing raw scopes, tenants, or policy names.
package routecontracts
