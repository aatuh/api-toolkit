package docs

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

// Manager implements ManagerContract for managing documentation.
type Manager struct {
	config   Config
	provider Provider
}

type openAPIDocument struct {
	content     []byte
	contentType string
}

type docsPageData struct {
	Title       string
	Description string
	Version     string
	OpenAPIPath string
	InfoPath    string
	VersionPath string
}

var defaultHTMLTemplate = template.Must(template.New("docs-default").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .Title }}</title>
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
    <h1>{{ .Title }}</h1>
    <p>{{ .Description }}</p>
    <p><strong>Version:</strong> {{ .Version }}</p>
  </div>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
  window.onload = function() {
    const ui = SwaggerUIBundle({
      url: "{{ .OpenAPIPath }}",
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
      layout: "BaseLayout"
    });
    window.ui = ui;
  };
  </script>
</body>
</html>`))

var staticHTMLTemplate = template.Must(template.New("docs-static").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .Title }}</title>
</head>
<body>
  <main>
    <h1>{{ .Title }}</h1>
    <p>{{ .Description }}</p>
    <p>Version: {{ .Version }}</p>
    <ul>
      <li><a href="{{ .OpenAPIPath }}">OpenAPI JSON</a></li>
      <li><a href="{{ .VersionPath }}">Version Info</a></li>
      <li><a href="{{ .InfoPath }}">API Info</a></li>
    </ul>
    <p>This strict docs mode serves only first-party content and avoids third-party assets.</p>
  </main>
</body>
</html>`))

// New creates a new docs manager with default configuration.
func New() ManagerContract {
	return NewWithConfig(Config{
		Title:       "API Documentation",
		Description: "REST API Documentation",
		Version:     "1.0.0",
		Paths:       DefaultPaths(),
		EnableHTML:  true,
		EnableJSON:  true,
		EnableYAML:  false,
		HTMLMode:    HTMLModeStatic,
	})
}

// NewStrict creates a docs manager that avoids third-party assets in HTML mode.
func NewStrict() ManagerContract {
	return New()
}

// NewSwaggerUI creates a docs manager that opts into the CDN-backed Swagger UI surface.
func NewSwaggerUI() ManagerContract {
	return NewWithConfig(Config{
		Title:       "API Documentation",
		Description: "REST API Documentation",
		Version:     "1.0.0",
		Paths:       DefaultPaths(),
		EnableHTML:  true,
		EnableJSON:  true,
		EnableYAML:  false,
		HTMLMode:    HTMLModeSwaggerUI,
	})
}

// NewWithConfig creates a new docs manager with custom configuration.
func NewWithConfig(config Config) ManagerContract {
	if config.HTMLMode == "" {
		config.HTMLMode = HTMLModeStatic
	}
	return &Manager{
		config: config,
	}
}

// RegisterProvider registers a documentation provider.
func (m *Manager) RegisterProvider(provider Provider) {
	m.provider = provider
}

// GetHTML returns the HTML documentation.
func (m *Manager) GetHTML() (string, error) {
	if !m.config.EnableHTML {
		return "", fs.ErrNotExist
	}
	if m.provider != nil {
		return m.provider.GetHTML()
	}
	if m.config.HTMLMode == HTMLModeStatic {
		return m.generateStaticHTML(), nil
	}
	return m.generateDefaultHTML(), nil
}

// GetOpenAPI returns the OpenAPI specification.
func (m *Manager) GetOpenAPI() ([]byte, error) {
	doc, err := m.getOpenAPIDocument()
	if err != nil {
		return nil, err
	}
	return doc.content, nil
}

func (m *Manager) getOpenAPIDocument() (openAPIDocument, error) {
	if !m.config.EnableJSON && !m.config.EnableYAML {
		return openAPIDocument{}, fs.ErrNotExist
	}
	if m.provider != nil {
		content, err := m.provider.GetOpenAPI()
		if err != nil {
			return openAPIDocument{}, err
		}
		format, contentType := detectOpenAPIFormat(content)
		if !m.isFormatEnabled(format) {
			return openAPIDocument{}, fs.ErrNotExist
		}
		return openAPIDocument{
			content:     content,
			contentType: contentType,
		}, nil
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
func (m *Manager) GetInfo() Info {
	if m.provider != nil {
		return m.provider.GetInfo()
	}
	return Info{
		Title:       m.config.Title,
		Description: m.config.Description,
		Version:     m.config.Version,
		Contact:     m.config.Contact,
		License:     m.config.License,
	}
}

// HTMLMode reports the configured docs HTML rendering mode.
func (m *Manager) HTMLMode() HTMLMode {
	if m == nil {
		return HTMLModeStatic
	}
	if m.config.HTMLMode == "" {
		return HTMLModeStatic
	}
	return m.config.HTMLMode
}

// ServeHTML serves the HTML documentation.
func (m *Manager) ServeHTML(w http.ResponseWriter, _ *http.Request) {
	html, err := m.GetHTML()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			httpx.WriteProblemChecked(w, http.StatusNotFound, httpx.Problem{
				Title:  http.StatusText(http.StatusNotFound),
				Detail: "documentation not found",
			})
			return
		}
		httpx.WriteProblemChecked(w, http.StatusInternalServerError, httpx.Problem{
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
	doc, err := m.getOpenAPIDocument()
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			httpx.WriteProblemChecked(w, http.StatusInternalServerError, httpx.Problem{
				Title:  http.StatusText(http.StatusInternalServerError),
				Detail: "failed to load openapi specification",
			})
			return
		}
		httpx.WriteProblemChecked(w, http.StatusNotFound, httpx.Problem{
			Title:  http.StatusText(http.StatusNotFound),
			Detail: "openapi specification disabled or not found",
		})
		return
	}

	w.Header().Set("Content-Type", doc.contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc.content)
}

// ServeVersion serves the API version.
func (m *Manager) ServeVersion(w http.ResponseWriter, _ *http.Request) {
	version, err := m.GetVersion()
	if err != nil {
		httpx.WriteProblemChecked(w, http.StatusInternalServerError, httpx.Problem{
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: "failed to get version",
		})
		return
	}

	httpx.WriteJSONChecked(w, http.StatusOK, map[string]string{
		"version": version,
	})
}

// ServeInfo serves the documentation info.
func (m *Manager) ServeInfo(w http.ResponseWriter, _ *http.Request) {
	info := m.GetInfo()
	httpx.WriteJSONChecked(w, http.StatusOK, info)
}

// generateDefaultHTML generates a default HTML documentation page.
func (m *Manager) generateDefaultHTML() string {
	return renderDocsTemplate(defaultHTMLTemplate, docsPageData{
		Title:       m.config.Title,
		Description: m.config.Description,
		Version:     m.config.Version,
		OpenAPIPath: defaultPath(m.config.Paths.OpenAPI, DefaultPaths().OpenAPI),
	})
}

func (m *Manager) generateStaticHTML() string {
	defaultPaths := DefaultPaths()
	return renderDocsTemplate(staticHTMLTemplate, docsPageData{
		Title:       m.config.Title,
		Description: m.config.Description,
		Version:     m.config.Version,
		OpenAPIPath: defaultPath(m.config.Paths.OpenAPI, defaultPaths.OpenAPI),
		InfoPath:    defaultPath(m.config.Paths.Info, defaultPaths.Info),
		VersionPath: defaultPath(m.config.Paths.Version, defaultPaths.Version),
	})
}

// loadOpenAPIFile attempts to load OpenAPI specification from common locations.
func (m *Manager) loadOpenAPIFile() (openAPIDocument, error) {
	candidates := make([]string, 0, 7)
	if m.config.EnableJSON {
		candidates = append(candidates,
			"./swagger/swagger.json",
			"./swagger/doc.json",
			"./swagger/openapi.json",
			"./docs/openapi.json",
			"./api-docs.json",
		)
	}
	if m.config.EnableYAML {
		candidates = append(candidates,
			"./swagger/swagger.yaml",
			"./swagger/swagger.yml",
			"./swagger/openapi.yaml",
			"./swagger/openapi.yml",
			"./docs/openapi.yaml",
			"./docs/openapi.yml",
			"./api-docs.yaml",
			"./api-docs.yml",
		)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			content, err := os.ReadFile(filepath.Clean(path))
			if err == nil {
				_, contentType := detectOpenAPIFormat(content)
				return openAPIDocument{
					content:     content,
					contentType: contentType,
				}, nil
			}
		}
	}

	return openAPIDocument{}, fs.ErrNotExist
}

func (m *Manager) isFormatEnabled(format string) bool {
	switch format {
	case "json":
		return m.config.EnableJSON
	case "yaml":
		return m.config.EnableYAML
	default:
		return false
	}
}

func detectOpenAPIFormat(content []byte) (string, string) {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return "json", "application/json"
	}
	if strings.HasPrefix(string(trimmed[:1]), "{") || strings.HasPrefix(string(trimmed[:1]), "[") {
		return "json", "application/json"
	}
	return "yaml", "application/yaml"
}

func renderDocsTemplate(tmpl *template.Template, data docsPageData) string {
	if tmpl == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return ""
	}
	return buf.String()
}

func defaultPath(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
