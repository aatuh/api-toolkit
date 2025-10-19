package docs

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/response_writer"
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
func (m *Manager) ServeHTML(w http.ResponseWriter, r *http.Request) {
	html, err := m.GetHTML()
	if err != nil {
		response_writer.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to generate documentation",
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// ServeOpenAPI serves the OpenAPI specification.
func (m *Manager) ServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	openapi, err := m.GetOpenAPI()
	if err != nil {
		response_writer.WriteJSON(w, http.StatusNotFound, map[string]string{
			"error": "OpenAPI specification not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(openapi)
}

// ServeVersion serves the API version.
func (m *Manager) ServeVersion(w http.ResponseWriter, r *http.Request) {
	version, err := m.GetVersion()
	if err != nil {
		response_writer.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to get version",
		})
		return
	}

	response_writer.WriteJSON(w, http.StatusOK, map[string]string{
		"version": version,
	})
}

// ServeInfo serves the documentation info.
func (m *Manager) ServeInfo(w http.ResponseWriter, r *http.Request) {
	info := m.GetInfo()
	response_writer.WriteJSON(w, http.StatusOK, info)
}

// generateDefaultHTML generates a default HTML documentation page.
func (m *Manager) generateDefaultHTML() string {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { border-bottom: 2px solid #333; padding-bottom: 20px; }
        .content { margin-top: 20px; }
        .endpoint { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .method { font-weight: bold; color: #0066cc; }
        .path { font-family: monospace; }
    </style>
</head>
<body>
    <div class="header">
        <h1>%s</h1>
        <p>%s</p>
        <p><strong>Version:</strong> %s</p>
    </div>
    
    <div class="content">
        <h2>Available Endpoints</h2>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/docs</span>
            <p>This documentation page</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/docs/openapi.json</span>
            <p>OpenAPI specification in JSON format</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/docs/version</span>
            <p>API version information</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/docs/info</span>
            <p>API information and metadata</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/livez</span>
            <p>Liveness probe endpoint</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/readyz</span>
            <p>Readiness probe endpoint</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/healthz</span>
            <p>Health check endpoint</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/health/detailed</span>
            <p>Detailed health information</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/version</span>
            <p>Application version information</p>
        </div>
        
        <h2>API Resources</h2>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/v1/foo</span>
            <p>List foo resources</p>
        </div>
        
        <div class="endpoint">
            <span class="method">POST</span> <span class="path">/api/v1/foo</span>
            <p>Create a new foo resource</p>
        </div>
        
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/v1/foo/{id}</span>
            <p>Get a specific foo resource</p>
        </div>
        
        <div class="endpoint">
            <span class="method">PUT</span> <span class="path">/api/v1/foo/{id}</span>
            <p>Update a foo resource</p>
        </div>
        
        <div class="endpoint">
            <span class="method">DELETE</span> <span class="path">/api/v1/foo/{id}</span>
            <p>Delete a foo resource</p>
        </div>
    </div>
</body>
</html>`, m.config.Title, m.config.Title, m.config.Description, m.config.Version)

	return html
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
