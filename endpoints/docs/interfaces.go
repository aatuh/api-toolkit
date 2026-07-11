package docs

import "github.com/aatuh/api-toolkit/v3/ports"

// Provider is the package-local documentation source contract.
//
// It is an alias for ports.DocsProvider during v3 compatibility. New docs
// integrations should import this package instead of adding to root ports.
type Provider = ports.DocsProvider

// ManagerContract is the package-local documentation manager contract.
//
// It is an alias for ports.DocsManager during v3 compatibility. The name avoids
// colliding with this package's concrete Manager implementation.
type ManagerContract = ports.DocsManager

// HTMLModeProvider is the optional documentation HTML mode capability contract.
//
// It is an alias for ports.DocsHTMLModeProvider during v3 compatibility.
type HTMLModeProvider = ports.DocsHTMLModeProvider

// RouteRegistrar is the minimal documentation route registration contract.
//
// It is an alias for ports.MethodRouteRegistrar during v3 compatibility.
type RouteRegistrar = ports.MethodRouteRegistrar
