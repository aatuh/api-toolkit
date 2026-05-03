package specs

import (
	"encoding/json"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
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
