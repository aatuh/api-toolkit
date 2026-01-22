package docs

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aatuh/api-toolkit/httpx"
	"github.com/aatuh/api-toolkit/ports"
)

// Manager implements ports.DocsManager for managing documentation.
type Manager struct {
	config   ports.DocsConfig
	provider ports.DocsProvider
}

// New creates a new docs manager with default configuration.
func New() ports.DocsManager {
	return NewWithConfig(ports.DocsConfig{
		Title:       "API Documentation",
		Description: "REST API Documentation",
		Version:     "1.0.0",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  true,
		EnableJSON:  true,
		EnableYAML:  false,
	})
}

// NewWithConfig creates a new docs manager with custom configuration.
func NewWithConfig(config ports.DocsConfig) ports.DocsManager {
	return &Manager{
		config: config,
	}
}

// RegisterProvider registers a documentation provider.
func (m *Manager) RegisterProvider(provider ports.DocsProvider) {
	m.provider = provider
}

// GetHTML returns the HTML documentation.
func (m *Manager) GetHTML() (string, error) {
	if m.provider != nil {
		return m.provider.GetHTML()
	}
	return m.generateDefaultHTML(), nil
}

// GetOpenAPI returns the OpenAPI specification.
func (m *Manager) GetOpenAPI() ([]byte, error) {
	if m.provider != nil {
		return m.provider.GetOpenAPI()
	}
	return m.loadOpenAPIFile()
}

// GetVersion returns the API version.
func (m *Manager) GetVersion() (string, error) {
	if m.provider != nil {
		return m.provider.GetVersion()
	}
	return m.config.Version, nil
}

// GetInfo returns the documentation info.
func (m *Manager) GetInfo() ports.DocsInfo {
	if m.provider != nil {
		return m.provider.GetInfo()
	}
	return ports.DocsInfo{
		Title:       m.config.Title,
		Description: m.config.Description,
		Version:     m.config.Version,
		Contact:     m.config.Contact,
		License:     m.config.License,
	}
}

// ServeHTML serves the HTML documentation.
func (m *Manager) ServeHTML(w http.ResponseWriter, _ *http.Request) {
	html, err := m.GetHTML()
	if err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: "failed to generate documentation",
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// ServeOpenAPI serves the OpenAPI specification.
func (m *Manager) ServeOpenAPI(w http.ResponseWriter, _ *http.Request) {
	openapi, err := m.GetOpenAPI()
	if err != nil {
		httpx.WriteProblem(w, http.StatusNotFound, httpx.Problem{
			Title:  http.StatusText(http.StatusNotFound),
			Detail: "openapi specification not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openapi)
}

// ServeVersion serves the API version.
func (m *Manager) ServeVersion(w http.ResponseWriter, _ *http.Request) {
	version, err := m.GetVersion()
	if err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: "failed to get version",
		})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"version": version,
	})
}

// ServeInfo serves the documentation info.
func (m *Manager) ServeInfo(w http.ResponseWriter, _ *http.Request) {
	info := m.GetInfo()
	httpx.WriteJSON(w, http.StatusOK, info)
}

// generateDefaultHTML generates a default HTML documentation page.
func (m *Manager) generateDefaultHTML() string {
	openAPIPath := m.config.Paths.OpenAPI
	if openAPIPath == "" {
		openAPIPath = ports.DefaultDocsPaths().OpenAPI
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
  <style>
    html, body { margin: 0; padding: 0; background: #fafafa; }
    #swagger-ui { box-sizing: border-box; }
    .info-panel { padding: 16px; background: #fff; border-bottom: 1px solid #e5e5e5; }
    .info-panel h1 { margin: 0 0 8px 0; font-size: 1.5rem; }
    .info-panel p { margin: 0 0 4px 0; color: #555; }
  </style>
</head>
<body>
  <div class="info-panel">
    <h1>%s</h1>
    <p>%s</p>
    <p><strong>Version:</strong> %s</p>
  </div>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
  window.onload = function() {
    const ui = SwaggerUIBundle({
      url: "%s",
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
      layout: "BaseLayout"
    });
    window.ui = ui;
  };
  </script>
</body>
</html>`,
		m.config.Title,
		m.config.Title,
		m.config.Description,
		m.config.Version,
		openAPIPath,
	)
}

// loadOpenAPIFile attempts to load OpenAPI specification from common locations.
func (m *Manager) loadOpenAPIFile() ([]byte, error) {
	candidates := []string{
		"./swagger/swagger.json",
		"./swagger/doc.json",
		"./swagger/openapi.json",
		"./docs/openapi.json",
		"./api-docs.json",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			content, err := os.ReadFile(filepath.Clean(path))
			if err == nil {
				return content, nil
			}
		}
	}

	// Return a minimal OpenAPI spec if no file is found
	return m.generateMinimalOpenAPI(), nil
}

// generateMinimalOpenAPI generates a minimal OpenAPI specification.
func (m *Manager) generateMinimalOpenAPI() []byte {
	openapi := fmt.Sprintf(`{
  "openapi": "3.0.0",
  "info": {
    "title": "%s",
    "description": "%s",
    "version": "%s"
  },
  "servers": [
    {
      "url": "http://localhost:8000",
      "description": "Development server"
    }
  ],
  "paths": {
    "/docs": {
      "get": {
        "summary": "API Documentation",
        "description": "Returns the API documentation page",
        "responses": {
          "200": {
            "description": "HTML documentation page",
            "content": {
              "text/html": {
                "schema": {
                  "type": "string"
                }
              }
            }
          }
        }
      }
    },
    "/livez": {
      "get": {
        "summary": "Liveness Probe",
        "description": "Returns the liveness status of the application",
        "responses": {
          "200": {
            "description": "Application is alive",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "status": {
                      "type": "string",
                      "enum": ["healthy", "unhealthy", "degraded", "unknown"]
                    },
                    "timestamp": {
                      "type": "string",
                      "format": "date-time"
                    },
                    "message": {
                      "type": "string"
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/readyz": {
      "get": {
        "summary": "Readiness Probe",
        "description": "Returns the readiness status of the application",
        "responses": {
          "200": {
            "description": "Application is ready",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "status": {
                      "type": "string",
                      "enum": ["healthy", "unhealthy", "degraded", "unknown"]
                    },
                    "timestamp": {
                      "type": "string",
                      "format": "date-time"
                    },
                    "message": {
                      "type": "string"
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`, m.config.Title, m.config.Description, m.config.Version)

	return []byte(openapi)
}
