package health

import "github.com/aatuh/api-toolkit/v3/ports"

// Checker is the package-local health check contract.
//
// It is an alias for ports.HealthChecker during v3 compatibility. New health
// integrations should import this package instead of adding to root ports.
type Checker = ports.HealthChecker

// ManagerContract is the package-local health manager contract.
//
// It is an alias for ports.HealthManager during v3 compatibility. The name
// avoids colliding with this package's concrete Manager implementation.
type ManagerContract = ports.HealthManager

// DetailedManager is the optional detailed health capability contract.
//
// It is an alias for ports.DetailedHealthManager during v3 compatibility.
type DetailedManager = ports.DetailedHealthManager

// CachedManager is the optional cached health capability contract.
//
// It is an alias for ports.CachedHealthManager during v3 compatibility.
type CachedManager = ports.CachedHealthManager

// RouteRegistrar is the minimal health route registration contract.
//
// It is an alias for ports.MethodRouteRegistrar during v3 compatibility.
type RouteRegistrar = ports.MethodRouteRegistrar
