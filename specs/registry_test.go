package specs

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestRegistryOpenAPIIncludesOperationMetadata(t *testing.T) {
	registry := NewRegistry(Info{
		Title:       "Widget API",
		Description: "Widget operations",
		Version:     "1.2.3",
	})
	registry.SetServers([]Server{{URL: "https://api.example.test", Description: "production"}})
	registry.Register(Operation{
		Method:      http.MethodPost,
		Path:        "/widgets",
		Summary:     "Create widget",
		Description: "Creates a widget.",
		Tags:        []string{"widgets", "write"},
		Deprecated:  true,
		RequestBody: &RequestBody{
			Description:  "Widget payload",
			Required:     true,
			ContentTypes: []string{"application/json", "application/vnd.widgets+json"},
		},
		Responses: map[int]Response{
			http.StatusCreated: {
				Description:  "Widget created",
				ContentTypes: []string{"application/json"},
			},
			http.StatusAccepted: {
				ContentTypes: []string{"application/problem+json"},
			},
		},
	})

	doc := decodeOpenAPI(t, registry)

	if doc["openapi"] != "3.0.0" {
		t.Fatalf("openapi = %v, want 3.0.0", doc["openapi"])
	}
	info := asMap(t, doc["info"])
	if info["title"] != "Widget API" || info["description"] != "Widget operations" || info["version"] != "1.2.3" {
		t.Fatalf("info = %#v", info)
	}
	servers := asSlice(t, doc["servers"])
	if len(servers) != 1 {
		t.Fatalf("servers length = %d, want 1", len(servers))
	}
	server := asMap(t, servers[0])
	if server["url"] != "https://api.example.test" || server["description"] != "production" {
		t.Fatalf("server = %#v", server)
	}

	operation := operationAt(t, doc, "/widgets", "post")
	if operation["summary"] != "Create widget" {
		t.Fatalf("summary = %v", operation["summary"])
	}
	if operation["description"] != "Creates a widget." {
		t.Fatalf("description = %v", operation["description"])
	}
	if operation["deprecated"] != true {
		t.Fatalf("deprecated = %v", operation["deprecated"])
	}
	if tags := asStringSlice(t, operation["tags"]); !reflect.DeepEqual(tags, []string{"widgets", "write"}) {
		t.Fatalf("tags = %#v", tags)
	}

	requestBody := asMap(t, operation["requestBody"])
	if requestBody["description"] != "Widget payload" || requestBody["required"] != true {
		t.Fatalf("requestBody = %#v", requestBody)
	}
	requestContent := asMap(t, requestBody["content"])
	assertContentType(t, requestContent, "application/json")
	assertContentType(t, requestContent, "application/vnd.widgets+json")

	responses := asMap(t, operation["responses"])
	created := asMap(t, responses["201"])
	if created["description"] != "Widget created" {
		t.Fatalf("201 description = %v", created["description"])
	}
	assertContentType(t, asMap(t, created["content"]), "application/json")
	accepted := asMap(t, responses["202"])
	if accepted["description"] != http.StatusText(http.StatusAccepted) {
		t.Fatalf("202 description = %v", accepted["description"])
	}
	assertContentType(t, asMap(t, accepted["content"]), "application/problem+json")
}

func TestRegistryOpenAPIDefaults(t *testing.T) {
	registry := NewRegistry(Info{})
	registry.Register(Operation{
		Method: http.MethodGet,
		Path:   "/ping",
	})
	registry.Register(Operation{Method: http.MethodGet})
	registry.Register(Operation{Path: "/missing-method"})

	doc := decodeOpenAPI(t, registry)

	info := asMap(t, doc["info"])
	if info["title"] != "API" || info["version"] != "0.0.0" {
		t.Fatalf("default info = %#v", info)
	}
	operation := operationAt(t, doc, "/ping", "get")
	responses := asMap(t, operation["responses"])
	okResponse := asMap(t, responses["200"])
	if okResponse["description"] != http.StatusText(http.StatusOK) {
		t.Fatalf("default response = %#v", okResponse)
	}
	paths := asMap(t, doc["paths"])
	if _, ok := paths["/missing-method"]; ok {
		t.Fatal("operation without method should not be registered")
	}
}

func TestNilRegistryOpenAPIReturnsDefaultDocument(t *testing.T) {
	var registry *Registry
	doc := decodeOpenAPI(t, registry)

	info := asMap(t, doc["info"])
	if info["title"] != "API" || info["version"] != "0.0.0" {
		t.Fatalf("default info = %#v", info)
	}
	if paths := asMap(t, doc["paths"]); len(paths) != 0 {
		t.Fatalf("paths = %#v, want empty", paths)
	}
}

func TestRegistryOpenAPIOutputIsDeterministic(t *testing.T) {
	first := NewRegistry(Info{Title: "Deterministic", Version: "1"})
	second := NewRegistry(Info{Title: "Deterministic", Version: "1"})
	ops := []Operation{
		{Method: http.MethodPost, Path: "/widgets"},
		{Method: http.MethodGet, Path: "/accounts"},
		{Method: http.MethodDelete, Path: "/widgets/{id}"},
		{Method: http.MethodGet, Path: "/widgets"},
	}
	for _, op := range ops {
		first.Register(op)
	}
	for i := len(ops) - 1; i >= 0; i-- {
		second.Register(ops[i])
	}

	firstDoc, err := first.OpenAPI()
	if err != nil {
		t.Fatalf("first OpenAPI() error = %v", err)
	}
	secondDoc, err := second.OpenAPI()
	if err != nil {
		t.Fatalf("second OpenAPI() error = %v", err)
	}
	if string(firstDoc) != string(secondDoc) {
		t.Fatalf("OpenAPI output differs\nfirst:  %s\nsecond: %s", firstDoc, secondDoc)
	}
}

func TestRegistryProviderUsesDefaultsAndRegistryDocument(t *testing.T) {
	registry := NewRegistry(Info{Title: "Registry", Version: "v1"})
	registry.Register(Operation{Method: http.MethodGet, Path: "/healthz"})
	provider := NewRegistryProvider(registry, ports.DocsInfo{
		Title:       "Docs",
		Description: "Documentation",
		Version:     "v1",
	}, "")

	if got, want := provider.GetInfo().Title, "Docs"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
	if got, err := provider.GetVersion(); err != nil || got != "v1" {
		t.Fatalf("GetVersion() = (%q, %v), want (v1, nil)", got, err)
	}
	html, err := provider.GetHTML()
	if err != nil {
		t.Fatalf("GetHTML() error = %v", err)
	}
	if !containsAll(html, "Docs", "Documentation", ports.DefaultDocsPaths().OpenAPI) {
		t.Fatalf("GetHTML() missing docs metadata: %s", html)
	}
	openAPI, err := provider.GetOpenAPI()
	if err != nil {
		t.Fatalf("GetOpenAPI() error = %v", err)
	}
	doc := decodeOpenAPIBytes(t, openAPI)
	_ = operationAt(t, doc, "/healthz", "get")
}

func TestRegistryProviderRequiresRegistry(t *testing.T) {
	provider := NewRegistryProvider(nil, ports.DocsInfo{}, "")
	if _, err := provider.GetOpenAPI(); err == nil {
		t.Fatal("expected error for missing registry")
	}
}

func decodeOpenAPI(t *testing.T, registry *Registry) map[string]any {
	t.Helper()

	data, err := registry.OpenAPI()
	if err != nil {
		t.Fatalf("OpenAPI() error = %v", err)
	}
	return decodeOpenAPIBytes(t, data)
}

func decodeOpenAPIBytes(t *testing.T, data []byte) map[string]any {
	t.Helper()

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode OpenAPI JSON: %v\n%s", err, data)
	}
	return doc
}

func operationAt(t *testing.T, doc map[string]any, path string, method string) map[string]any {
	t.Helper()

	paths := asMap(t, doc["paths"])
	pathItem := asMap(t, paths[path])
	return asMap(t, pathItem[method])
}

func assertContentType(t *testing.T, content map[string]any, contentType string) {
	t.Helper()

	entry := asMap(t, content[contentType])
	schema := asMap(t, entry["schema"])
	if schema["type"] != "object" {
		t.Fatalf("%s schema = %#v", contentType, schema)
	}
}

func asMap(t *testing.T, value any) map[string]any {
	t.Helper()

	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value has type %T, want map[string]any: %#v", value, value)
	}
	return out
}

func asSlice(t *testing.T, value any) []any {
	t.Helper()

	out, ok := value.([]any)
	if !ok {
		t.Fatalf("value has type %T, want []any: %#v", value, value)
	}
	return out
}

func asStringSlice(t *testing.T, value any) []string {
	t.Helper()

	values := asSlice(t, value)
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("slice value has type %T, want string: %#v", value, value)
		}
		out = append(out, text)
	}
	return out
}

func containsAll(text string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			return false
		}
	}
	return true
}
