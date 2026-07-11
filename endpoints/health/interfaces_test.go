package health

import (
	"testing"
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

func requirePortsCheckerFactory(func() Checker) {}

func requirePortsManagerFactory(func() ManagerContract) {}

func requirePortsHandlerFactory(func(ManagerContract) *Handler) {}

func requirePortsRouteRegistration(func(*Handler, RouteRegistrar)) {}

func requirePortsDetailedManager(DetailedManager) {}

func requirePortsCachedManager(CachedManager) {}
