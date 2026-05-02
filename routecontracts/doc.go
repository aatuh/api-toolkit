// Package routecontracts registers HTTP routes and OpenAPI operations together.
//
// It is intentionally router-neutral and uses only the common method
// registration shape already provided by ports.HTTPRouter. PATCH support is
// detected with a local optional interface so stable router interfaces are not
// widened.
package routecontracts
