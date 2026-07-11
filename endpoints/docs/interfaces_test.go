package docs

import (
	"testing"
)

func TestPackageLocalInterfacesPreserveV3PortsIdentity(_ *testing.T) {
	requirePortsManagerFactory(New)
	requirePortsHandlerFactory(NewHandler)
	requirePortsProviderRegistration((*Manager).RegisterProvider)
	requirePortsRouteRegistration((*Handler).RegisterRoutesTo)

	var htmlModeProvider HTMLModeProvider
	requirePortsHTMLModeProvider(htmlModeProvider)
}

func requirePortsManagerFactory(func() ManagerContract) {}

func requirePortsHandlerFactory(func(ManagerContract) *Handler) {}

func requirePortsProviderRegistration(func(*Manager, Provider)) {}

func requirePortsRouteRegistration(func(*Handler, RouteRegistrar)) {}

func requirePortsHTMLModeProvider(HTMLModeProvider) {}
