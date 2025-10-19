package specs

// System endpoints for health checks and documentation
const (
	// Health check endpoints
	Livez          = "/livez"
	Readyz         = "/readyz"
	Healthz        = "/healthz"
	Health         = "/health"
	HealthDetailed = "/health/detailed"

	// Documentation endpoints
	Docs        = "/docs"
	DocsOpenAPI = "/docs/openapi.json"
	DocsVersion = "/docs/version"
	DocsInfo    = "/docs/info"

	// System endpoints
	Version = "/version"
)

// HealthEndpoints groups all health-related endpoints
var HealthEndpoints = struct {
	Livez          string
	Readyz         string
	Healthz        string
	Health         string
	HealthDetailed string
}{
	Livez:          Livez,
	Readyz:         Readyz,
	Healthz:        Healthz,
	Health:         Health,
	HealthDetailed: HealthDetailed,
}

// DocsEndpoints groups all documentation-related endpoints
var DocsEndpoints = struct {
	Docs    string
	OpenAPI string
	Version string
	Info    string
}{
	Docs:    Docs,
	OpenAPI: DocsOpenAPI,
	Version: DocsVersion,
	Info:    DocsInfo,
}

// SystemEndpoints groups all system-related endpoints
var SystemEndpoints = struct {
	Version string
}{
	Version: Version,
}

// AllEndpoints groups all available endpoints
var AllEndpoints = struct {
	Health interface{}
	Docs   interface{}
	System interface{}
}{
	Health: HealthEndpoints,
	Docs:   DocsEndpoints,
	System: SystemEndpoints,
}
