package health

import (
	"testing"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestPackageLocalInterfacesPreserveV3PortsIdentity(_ *testing.T) {
	requirePortsCheckerFactory(NewBasicChecker)
	requirePortsManagerFactory(New)
	requirePortsHandlerFactory(NewHandler)
	requirePortsRouteRegistration((*Handler).RegisterRoutesTo)

	var detailed DetailedManager
	requirePortsDetailedManager(detailed)
	var cached CachedManager
	requirePortsCachedManager(cached)
}

func requirePortsCheckerFactory(func() ports.HealthChecker) {}

func requirePortsManagerFactory(func() ports.HealthManager) {}

func requirePortsHandlerFactory(func(ports.HealthManager) *Handler) {}

func requirePortsRouteRegistration(func(*Handler, ports.MethodRouteRegistrar)) {}

func requirePortsDetailedManager(ports.DetailedHealthManager) {}

func requirePortsCachedManager(ports.CachedHealthManager) {}
