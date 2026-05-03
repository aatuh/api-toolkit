package specs

import (
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

var componentNamePattern = regexp.MustCompile(`[^A-Za-z0-9]+`)

// ProblemSchema returns the reusable OpenAPI schema for RFC 9457 Problem Details.
func ProblemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":              map[string]any{"type": "string", "format": "uri"},
			"title":             map[string]any{"type": "string"},
			"status":            map[string]any{"type": "integer", "format": "int32"},
			"detail":            map[string]any{"type": "string"},
			"instance":          map[string]any{"type": "string"},
			"code":              map[string]any{"type": "string"},
			"retryable":         map[string]any{"type": "boolean"},
			"documentation_url": map[string]any{"type": "string", "format": "uri"},
			"log_level":         map[string]any{"type": "string"},
		},
	}
}

// ProblemResponse returns a reusable Problem Details response object.
func ProblemResponse(description string) Response {
	if strings.TrimSpace(description) == "" {
		description = "Problem Details"
	}
	return Response{
		Description: description,
		Content: map[string]MediaType{
			"application/problem+json": {SchemaRef: "#/components/schemas/Problem"},
		},
	}
}

// ValidationProblemResponse returns a reusable validation Problem Details response object.
func ValidationProblemResponse(description string) Response {
	if strings.TrimSpace(description) == "" {
		description = "Validation Problem Details"
	}
	return Response{
		Description: description,
		Content: map[string]MediaType{
			"application/problem+json": {SchemaRef: "#/components/schemas/ValidationProblem"},
		},
	}
}

// RegisterProblemCatalog registers reusable Problem Details schemas and responses.
func RegisterProblemCatalog(registry *Registry, catalog *httpx.ProblemCatalog) {
	if registry == nil {
		return
	}
	registry.RegisterSchema("Problem", ProblemSchema())
	validation := ProblemSchema()
	validation["properties"].(map[string]any)["errors"] = map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"field":   map[string]any{"type": "string"},
				"message": map[string]any{"type": "string"},
				"code":    map[string]any{"type": "string"},
			},
		},
	}
	registry.RegisterSchema("ValidationProblem", validation)
	definitions := catalog.Definitions()
	if len(definitions) == 0 {
		definitions = httpx.DefaultProblemCatalog().Definitions()
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Code < definitions[j].Code })
	for _, definition := range definitions {
		name := problemResponseName(definition)
		response := ProblemResponse(definition.Title)
		if definition.Status == http.StatusBadRequest && definition.Code == httpx.ProblemCode("validation") {
			response = ValidationProblemResponse(definition.Title)
		}
		registry.RegisterResponse(name, response)
	}
}

func problemResponseName(definition httpx.ProblemDefinition) string {
	base := strings.TrimSpace(string(definition.Code))
	if base == "" {
		base = http.StatusText(definition.Status)
	}
	if base == "" {
		base = "problem"
	}
	parts := componentNamePattern.Split(base, -1)
	var name strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		name.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			name.WriteString(part[1:])
		}
	}
	if name.Len() == 0 {
		return "ProblemResponse"
	}
	return name.String() + "ProblemResponse"
}
