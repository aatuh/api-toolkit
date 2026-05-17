package specs

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/ports"
)

// RegistryProvider adapts an OpenAPI registry to the DocsProvider interface.
type RegistryProvider struct {
	registry    *Registry
	info        ports.DocsInfo
	openAPIPath string
}

// NewRegistryProvider creates a DocsProvider backed by a registry.
func NewRegistryProvider(registry *Registry, info ports.DocsInfo, openAPIPath string) *RegistryProvider {
	if openAPIPath == "" {
		openAPIPath = ports.DefaultDocsPaths().OpenAPI
	}
	return &RegistryProvider{
		registry:    registry,
		info:        info,
		openAPIPath: openAPIPath,
	}
}

// GetHTML returns a basic Swagger UI page pointing at the registry OpenAPI JSON.
func (p *RegistryProvider) GetHTML() (string, error) {
	title := p.info.Title
	if title == "" {
		title = "API Documentation"
	}
	desc := p.info.Description
	version := p.info.Version
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
		title,
		title,
		desc,
		version,
		p.openAPIPath,
	), nil
}

// GetOpenAPI returns the JSON OpenAPI document from the registry.
func (p *RegistryProvider) GetOpenAPI() ([]byte, error) {
	if p == nil || p.registry == nil {
		return nil, fmt.Errorf("registry not configured")
	}
	return p.registry.OpenAPI()
}

// GetVersion returns the registry version.
func (p *RegistryProvider) GetVersion() (string, error) {
	return p.info.Version, nil
}

// GetInfo returns the registry info.
func (p *RegistryProvider) GetInfo() ports.DocsInfo {
	return p.info
}
