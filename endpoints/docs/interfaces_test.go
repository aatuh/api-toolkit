package docs

import (
	"testing"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestPackageLocalInterfacesPreserveV3PortsIdentity(_ *testing.T) {
	requirePortsManagerFactory(New)
	requirePortsHandlerFactory(NewHandler)
	requirePortsProviderRegistration((*Manager).RegisterProvider)
	requirePortsRouteRegistration((*Handler).RegisterRoutesTo)

	var htmlModeProvider HTMLModeProvider
	requirePortsHTMLModeProvider(htmlModeProvider)
}

func requirePortsManagerFactory(func() ports.DocsManager) {}

func requirePortsHandlerFactory(func(ports.DocsManager) *Handler) {}

func requirePortsProviderRegistration(func(*Manager, ports.DocsProvider)) {}

func requirePortsRouteRegistration(func(*Handler, ports.MethodRouteRegistrar)) {}

func requirePortsHTMLModeProvider(ports.DocsHTMLModeProvider) {}
