package specs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type openAPIRoundTripCreateWidget struct {
	Name string `json:"name" required:"true" example:"primary"`
	Mode string `json:"mode" enum:"draft,active" example:"active"`
}

type openAPIRoundTripWidget struct {
	ID   string `json:"id" required:"true" example:"widget_123"`
	Name string `json:"name" required:"true" example:"primary"`
	Mode string `json:"mode" required:"true" enum:"draft,active" example:"active"`
}

func TestRegistryOpenAPIRoundTripValidatesSchemaRefsAndExamples(t *testing.T) {
	registry := NewRegistryWithOptions(Info{
		Title:   "Widget API",
		Version: "2026-06-11",
	}, RegistryOptions{OpenAPIVersion: OpenAPIVersion31})
	if err := RegisterSchemaFrom[openAPIRoundTripCreateWidget](registry, "CreateWidget", SchemaOptions{}); err != nil {
		t.Fatalf("register CreateWidget schema: %v", err)
	}
	if err := RegisterSchemaFrom[openAPIRoundTripWidget](registry, "Widget", SchemaOptions{}); err != nil {
		t.Fatalf("register Widget schema: %v", err)
	}
	registry.RegisterSecurityScheme("ApiKeyAuth", SecurityScheme{
		Type: "apiKey",
		Name: "X-API-Key",
		In:   "header",
	})
	RegisterProblemCatalog(registry, nil)
	registry.RegisterResponse("ValidationProblem", ValidationProblemResponse("Invalid widget"))
	registry.Register(Operation{
		OperationID: "createWidget",
		Method:      http.MethodPost,
		Path:        "/tenants/{tenant_id}/widgets",
		Parameters: []Parameter{
			{Name: "tenant_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
			{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
		},
		Security: []SecurityRequirement{{Name: "ApiKeyAuth", Scopes: []string{"widgets:write"}}},
		RequestBody: &RequestBody{
			Required: true,
			Content: map[string]MediaType{
				"application/json": {
					SchemaRef: "#/components/schemas/CreateWidget",
					Examples: map[string]any{
						"default": Example{Summary: "Create widget", Value: map[string]any{"name": "primary", "mode": "active"}},
					},
				},
			},
		},
		Responses: map[int]Response{
			http.StatusCreated: {
				Description: "Created",
				Content: map[string]MediaType{
					"application/json": {
						SchemaRef: "#/components/schemas/Widget",
						Example:   map[string]any{"id": "widget_123", "name": "primary", "mode": "active"},
					},
				},
			},
			http.StatusBadRequest: {Ref: "#/components/responses/ValidationProblem"},
		},
	})

	data, err := registry.OpenAPI()
	if err != nil {
		t.Fatalf("OpenAPI() error = %v", err)
	}
	doc := decodeOpenAPIBytes(t, data)
	roundTrip, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("remarshal OpenAPI document: %v", err)
	}
	doc = decodeOpenAPIBytes(t, roundTrip)

	validateGeneratedOpenAPIForClientUse(t, doc)
}

func validateGeneratedOpenAPIForClientUse(t *testing.T, doc map[string]any) {
	t.Helper()

	if version, _ := doc["openapi"].(string); version != "3.0.0" && version != "3.1.0" {
		t.Fatalf("openapi version = %q", version)
	}
	info := asMap(t, doc["info"])
	if strings.TrimSpace(fmt.Sprint(info["title"])) == "" || strings.TrimSpace(fmt.Sprint(info["version"])) == "" {
		t.Fatalf("info is not client-usable: %#v", info)
	}
	resolver := openAPITestResolver{schemas: asMap(t, asMap(t, doc["components"])["schemas"])}
	if responses, ok := asMap(t, doc["components"])["responses"]; ok {
		resolver.responses = asMap(t, responses)
	}
	if securitySchemes, ok := asMap(t, doc["components"])["securitySchemes"]; ok {
		resolver.securitySchemes = asMap(t, securitySchemes)
	}
	validateComponentExamples(t, resolver)
	validateComponentResponses(t, resolver)
	validateSecurityRequirements(t, doc["security"], resolver, "GLOBAL")

	paths := asMap(t, doc["paths"])
	if len(paths) == 0 {
		t.Fatal("OpenAPI document has no paths")
	}
	for path, rawPathItem := range paths {
		if !strings.HasPrefix(path, "/") {
			t.Fatalf("path %q must start with /", path)
		}
		pathItem := asMap(t, rawPathItem)
		for method, rawOperation := range pathItem {
			if !validOpenAPIMethod(method) {
				continue
			}
			operation := asMap(t, rawOperation)
			context := strings.ToUpper(method) + " " + path
			if strings.TrimSpace(fmt.Sprint(operation["operationId"])) == "" {
				t.Fatalf("%s is missing operationId", context)
			}
			validatePathParameters(t, path, operation, context)
			validateSecurityRequirements(t, operation["security"], resolver, context)
			if requestBody, ok := operation["requestBody"]; ok {
				validateRequestBody(t, asMap(t, requestBody), resolver, context)
			}
			validateResponses(t, operation["responses"], resolver, context)
		}
	}
}

type openAPITestResolver struct {
	schemas         map[string]any
	responses       map[string]any
	securitySchemes map[string]any
}

func validateComponentExamples(t *testing.T, resolver openAPITestResolver) {
	t.Helper()
	for name, rawSchema := range resolver.schemas {
		validateSchemaObject(t, resolver, asMap(t, rawSchema), "#/components/schemas/"+name)
	}
}

func validateComponentResponses(t *testing.T, resolver openAPITestResolver) {
	t.Helper()
	for name, rawResponse := range resolver.responses {
		response := asMap(t, rawResponse)
		context := "#/components/responses/" + name
		if strings.TrimSpace(fmt.Sprint(response["description"])) == "" {
			t.Fatalf("%s is missing description", context)
		}
		if rawContent, ok := response["content"]; ok {
			for contentType, rawMedia := range asMap(t, rawContent) {
				validateMediaType(t, resolver, asMap(t, rawMedia), context+" "+contentType)
			}
		}
	}
}

func validateSchemaObject(t *testing.T, resolver openAPITestResolver, schema map[string]any, context string) {
	t.Helper()
	if ref, _ := schema["$ref"].(string); ref != "" {
		_ = resolver.schema(t, ref, context)
		return
	}
	if example, ok := schema["example"]; ok {
		validateExampleAgainstSchema(t, resolver, example, schema, context+" example")
	}
	if properties, ok := schema["properties"]; ok {
		for property, rawPropertySchema := range asMap(t, properties) {
			validateSchemaObject(t, resolver, asMap(t, rawPropertySchema), context+"."+property)
		}
	}
	if items, ok := schema["items"]; ok {
		validateSchemaObject(t, resolver, asMap(t, items), context+" items")
	}
}

func validateRequestBody(t *testing.T, requestBody map[string]any, resolver openAPITestResolver, context string) {
	t.Helper()
	content := asMap(t, requestBody["content"])
	if len(content) == 0 {
		t.Fatalf("%s requestBody has no content", context)
	}
	for contentType, rawMedia := range content {
		validateMediaType(t, resolver, asMap(t, rawMedia), context+" requestBody "+contentType)
	}
}

func validateResponses(t *testing.T, rawResponses any, resolver openAPITestResolver, context string) {
	t.Helper()
	responses := asMap(t, rawResponses)
	if len(responses) == 0 {
		t.Fatalf("%s has no responses", context)
	}
	for status, rawResponse := range responses {
		response := asMap(t, rawResponse)
		if ref, _ := response["$ref"].(string); ref != "" {
			_ = resolver.response(t, ref, context+" response "+status)
			continue
		}
		if strings.TrimSpace(fmt.Sprint(response["description"])) == "" {
			t.Fatalf("%s response %s is missing description", context, status)
		}
		if rawContent, ok := response["content"]; ok {
			content := asMap(t, rawContent)
			for contentType, rawMedia := range content {
				validateMediaType(t, resolver, asMap(t, rawMedia), context+" response "+status+" "+contentType)
			}
		}
	}
}

func validateMediaType(t *testing.T, resolver openAPITestResolver, media map[string]any, context string) {
	t.Helper()
	schema := asMap(t, media["schema"])
	validateSchemaObject(t, resolver, schema, context)
	if example, ok := media["example"]; ok {
		validateExampleAgainstSchema(t, resolver, example, schema, context+" example")
	}
	if rawExamples, ok := media["examples"]; ok {
		for name, rawExample := range asMap(t, rawExamples) {
			example := asMap(t, rawExample)
			value, ok := example["value"]
			if !ok {
				continue
			}
			validateExampleAgainstSchema(t, resolver, value, schema, context+" examples."+name)
		}
	}
}

func validateExampleAgainstSchema(t *testing.T, resolver openAPITestResolver, example any, schema map[string]any, context string) {
	t.Helper()
	if ref, _ := schema["$ref"].(string); ref != "" {
		validateExampleAgainstSchema(t, resolver, example, resolver.schema(t, ref, context), context)
		return
	}
	if rawEnum, ok := schema["enum"]; ok {
		enum := asSlice(t, rawEnum)
		for _, value := range enum {
			if reflect.DeepEqual(value, example) {
				return
			}
		}
		t.Fatalf("%s = %#v is not in enum %#v", context, example, enum)
	}
	switch schema["type"] {
	case "object":
		object := asMap(t, example)
		required := schemaStrings(rawStringSlice(schema["required"]))
		for _, field := range required {
			if _, ok := object[field]; !ok {
				t.Fatalf("%s missing required field %q in example %#v", context, field, object)
			}
		}
		if properties, ok := schema["properties"]; ok {
			for name, rawPropertySchema := range asMap(t, properties) {
				if value, ok := object[name]; ok {
					validateExampleAgainstSchema(t, resolver, value, asMap(t, rawPropertySchema), context+"."+name)
				}
			}
		}
	case "array":
		values := asSlice(t, example)
		if rawItems, ok := schema["items"]; ok {
			items := asMap(t, rawItems)
			for i, value := range values {
				validateExampleAgainstSchema(t, resolver, value, items, fmt.Sprintf("%s[%d]", context, i))
			}
		}
	case "string":
		if _, ok := example.(string); !ok {
			t.Fatalf("%s has type %T, want string: %#v", context, example, example)
		}
	case "integer":
		switch example.(type) {
		case float64, int, int64, json.Number:
		default:
			t.Fatalf("%s has type %T, want integer-compatible value: %#v", context, example, example)
		}
	case "number":
		switch example.(type) {
		case float64, int, int64, json.Number:
		default:
			t.Fatalf("%s has type %T, want number-compatible value: %#v", context, example, example)
		}
	case "boolean":
		if _, ok := example.(bool); !ok {
			t.Fatalf("%s has type %T, want boolean: %#v", context, example, example)
		}
	}
}

func validatePathParameters(t *testing.T, path string, operation map[string]any, context string) {
	t.Helper()
	declared := map[string]map[string]any{}
	if rawParameters, ok := operation["parameters"]; ok {
		for _, rawParameter := range asSlice(t, rawParameters) {
			parameter := asMap(t, rawParameter)
			if parameter["in"] == "path" {
				declared[fmt.Sprint(parameter["name"])] = parameter
			}
			if _, ok := parameter["schema"]; !ok {
				t.Fatalf("%s parameter %#v is missing schema", context, parameter)
			}
		}
	}
	for _, name := range pathTemplateParameters(path) {
		parameter, ok := declared[name]
		if !ok {
			t.Fatalf("%s missing path parameter %q", context, name)
		}
		if parameter["required"] != true {
			t.Fatalf("%s path parameter %q must be required", context, name)
		}
	}
}

func validateSecurityRequirements(t *testing.T, raw any, resolver openAPITestResolver, context string) {
	t.Helper()
	if raw == nil {
		return
	}
	for _, rawRequirement := range asSlice(t, raw) {
		for name := range asMap(t, rawRequirement) {
			if _, ok := resolver.securitySchemes[name]; !ok {
				t.Fatalf("%s references undefined security scheme %q", context, name)
			}
		}
	}
}

func (resolver openAPITestResolver) schema(t *testing.T, ref string, context string) map[string]any {
	t.Helper()
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		t.Fatalf("%s has unsupported schema ref %q", context, ref)
	}
	name := strings.TrimPrefix(ref, prefix)
	raw, ok := resolver.schemas[name]
	if !ok {
		t.Fatalf("%s references missing schema %q", context, ref)
	}
	return asMap(t, raw)
}

func (resolver openAPITestResolver) response(t *testing.T, ref string, context string) map[string]any {
	t.Helper()
	const prefix = "#/components/responses/"
	if !strings.HasPrefix(ref, prefix) {
		t.Fatalf("%s has unsupported response ref %q", context, ref)
	}
	name := strings.TrimPrefix(ref, prefix)
	raw, ok := resolver.responses[name]
	if !ok {
		t.Fatalf("%s references missing response %q", context, ref)
	}
	return asMap(t, raw)
}

func validOpenAPIMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		return true
	default:
		return false
	}
}

func pathTemplateParameters(path string) []string {
	var out []string
	for {
		start := strings.Index(path, "{")
		if start < 0 {
			return out
		}
		path = path[start+1:]
		end := strings.Index(path, "}")
		if end < 0 {
			return out
		}
		if name := strings.TrimSpace(path[:end]); name != "" {
			out = append(out, name)
		}
		path = path[end+1:]
	}
}

func rawStringSlice(raw any) []string {
	if raw == nil {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return append([]string(nil), values...)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func schemaStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
