package specs

import (
	"encoding/json"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestRegisterProblemCatalogAddsReusableComponents(t *testing.T) {
	registry := NewRegistry(Info{Title: "API", Version: "v1"})
	RegisterProblemCatalog(registry, httpx.DefaultProblemCatalog())

	body, err := registry.OpenAPI()
	if err != nil {
		t.Fatalf("openapi: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}
	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	if schemas["Problem"] == nil || schemas["ValidationProblem"] == nil {
		t.Fatalf("problem schemas missing: %#v", schemas)
	}
	responses := components["responses"].(map[string]any)
	if len(responses) == 0 {
		t.Fatalf("problem responses missing")
	}
}

func TestProblemResponseUsesProblemMediaType(t *testing.T) {
	response := ProblemResponse("Bad request")
	media := response.Content["application/problem+json"]
	if media.SchemaRef != "#/components/schemas/Problem" {
		t.Fatalf("schema ref = %q", media.SchemaRef)
	}
}

func TestRegisterHTTPProblemResponses(t *testing.T) {
	registry := NewRegistry(Info{Title: "API", Version: "v1"})
	RegisterHTTPProblemResponses(registry, 401, 409)
	registry.Register(Operation{
		Method: "GET",
		Path:   "/widgets",
		Responses: map[int]Response{
			401: HTTPProblemResponseRef(401),
			409: HTTPProblemResponseRef(409),
		},
	})

	doc := decodeOpenAPI(t, registry)
	components := asMap(t, doc["components"])
	responses := asMap(t, components["responses"])
	if _, ok := responses["UnauthorizedProblemResponse"]; !ok {
		t.Fatalf("responses missing UnauthorizedProblemResponse: %#v", responses)
	}
	if _, ok := responses["ConflictProblemResponse"]; !ok {
		t.Fatalf("responses missing ConflictProblemResponse: %#v", responses)
	}
	operation := operationAt(t, doc, "/widgets", "get")
	operationResponses := asMap(t, operation["responses"])
	unauthorized := asMap(t, operationResponses["401"])
	if unauthorized["$ref"] != "#/components/responses/UnauthorizedProblemResponse" {
		t.Fatalf("401 response ref = %#v", unauthorized)
	}
	if got := HTTPProblemResponseName(499); got != "Status499ProblemResponse" {
		t.Fatalf("custom status response name = %q", got)
	}
}
