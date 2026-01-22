package ports

import "net/http"

// DocsProvider defines the interface for providing documentation content.
type DocsProvider interface {
	GetHTML() (string, error)
	GetOpenAPI() ([]byte, error)
	GetVersion() (string, error)
	GetInfo() DocsInfo
}

// DocsInfo provides information about the API documentation.
type DocsInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Contact     string `json:"contact,omitempty"`
	License     string `json:"license,omitempty"`
}

// DocsManager defines the interface for managing documentation.
type DocsManager interface {
	RegisterProvider(provider DocsProvider)
	GetHTML() (string, error)
	GetOpenAPI() ([]byte, error)
	GetVersion() (string, error)
	GetInfo() DocsInfo
	ServeHTML(w http.ResponseWriter, r *http.Request)
	ServeOpenAPI(w http.ResponseWriter, r *http.Request)
	ServeVersion(w http.ResponseWriter, r *http.Request)
	ServeInfo(w http.ResponseWriter, r *http.Request)
}

// DocsConfig defines configuration for documentation.
type DocsConfig struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	Contact     string    `json:"contact,omitempty"`
	License     string    `json:"license,omitempty"`
	Paths       DocsPaths `json:"paths"`
	EnableHTML  bool      `json:"enable_html"`
	EnableJSON  bool      `json:"enable_json"`
	EnableYAML  bool      `json:"enable_yaml"`
}

// DocsPaths defines the paths for documentation endpoints.
type DocsPaths struct {
	HTML    string `json:"html"`
	OpenAPI string `json:"openapi"`
	Version string `json:"version"`
	Info    string `json:"info"`
}

// DefaultDocsPaths returns the default documentation endpoint paths.
func DefaultDocsPaths() DocsPaths {
	return DocsPaths{
		HTML:    "/docs",
		OpenAPI: "/docs/openapi.json",
		Version: "/docs/version",
		Info:    "/docs/info",
	}
}
