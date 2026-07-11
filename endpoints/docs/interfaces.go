package docs

import (
	"net/http"

	"github.com/aatuh/api-toolkit/v4/specs"
)

// HTMLMode controls the HTML presentation mode for the docs surface.
type HTMLMode string

const (
	// HTMLModeSwaggerUI renders the Swagger UI convenience page.
	HTMLModeSwaggerUI HTMLMode = "swagger-ui"
	// HTMLModeStatic renders a first-party static landing page without third-party assets.
	HTMLModeStatic HTMLMode = "static"
)

// Provider defines the package-local documentation content contract.
type Provider interface {
	GetHTML() (string, error)
	GetOpenAPI() ([]byte, error)
	GetVersion() (string, error)
	GetInfo() Info
}

// Info provides documentation metadata.
type Info = specs.DocumentationInfo

// ManagerContract defines the package-local documentation manager contract.
type ManagerContract interface {
	RegisterProvider(provider Provider)
	GetHTML() (string, error)
	GetOpenAPI() ([]byte, error)
	GetVersion() (string, error)
	GetInfo() Info
	ServeHTML(w http.ResponseWriter, r *http.Request)
	ServeOpenAPI(w http.ResponseWriter, r *http.Request)
	ServeVersion(w http.ResponseWriter, r *http.Request)
	ServeInfo(w http.ResponseWriter, r *http.Request)
}

// HTMLModeProvider exposes a manager's configured HTML rendering mode.
type HTMLModeProvider interface {
	HTMLMode() HTMLMode
}

// Config defines documentation endpoint configuration.
type Config struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Contact     string   `json:"contact,omitempty"`
	License     string   `json:"license,omitempty"`
	Paths       Paths    `json:"paths"`
	EnableHTML  bool     `json:"enable_html"`
	EnableJSON  bool     `json:"enable_json"`
	EnableYAML  bool     `json:"enable_yaml"`
	HTMLMode    HTMLMode `json:"html_mode,omitempty"`
}

// Paths defines the paths for documentation endpoints.
type Paths struct {
	HTML    string `json:"html"`
	OpenAPI string `json:"openapi"`
	Version string `json:"version"`
	Info    string `json:"info"`
}

// DefaultPaths returns the default documentation endpoint paths.
func DefaultPaths() Paths {
	return Paths{
		HTML:    "/docs",
		OpenAPI: "/docs/openapi.json",
		Version: "/docs/version",
		Info:    "/docs/info",
	}
}

// RouteRegistrar is the minimal documentation route registration contract.
type RouteRegistrar interface {
	Get(pattern string, h http.HandlerFunc)
}
