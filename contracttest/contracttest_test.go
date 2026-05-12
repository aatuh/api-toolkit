package contracttest

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/routecontracts"
	"github.com/aatuh/api-toolkit/v2/routepolicy"
	"github.com/aatuh/api-toolkit/v2/specs"
)

func TestContractAssertionsPassForCoveredRoute(t *testing.T) {
	specRegistry := specs.NewRegistry(specs.Info{Title: "Contracts", Version: "1"})
	specRegistry.Register(specs.Operation{
		OperationID: "listWidgets",
		Method:      http.MethodGet,
		Path:        "/widgets",
		Security: []specs.SecurityRequirement{{
			Name: "ApiKeyAuth",
		}},
		Responses: map[int]specs.Response{
			http.StatusOK:         {Description: "ok"},
			http.StatusBadRequest: specs.ProblemResponse("bad request"),
		},
		Extensions: map[string]any{
			routepolicy.ExtensionTenant:         map[string]any{"required": true, "source": "header"},
			routepolicy.ExtensionIdempotencyKey: map[string]any{"required": true},
			routepolicy.ExtensionRateLimit:      "read-standard",
			routepolicy.ExtensionAdminPolicy:    "admin",
		},
	})
	routeRegistry := routecontracts.NewRegistry(fakeRouter{}, nil)
	if err := routeRegistry.Get("/widgets", specs.Operation{Method: http.MethodGet, Path: "/widgets"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})); err != nil {
		t.Fatalf("register route: %v", err)
	}

	AssertRegistryValid(t, routeRegistry)
	AssertRouteCoverage(t, routeRegistry, http.MethodGet, "/widgets")
	AssertOperationHasResponse(t, specRegistry, http.MethodGet, "/widgets", http.StatusOK)
	AssertOperationHasSecurity(t, specRegistry, http.MethodGet, "/widgets", "ApiKeyAuth")
	AssertOperationID(t, specRegistry, http.MethodGet, "/widgets", "listWidgets")
	AssertAllOperationsHaveOperationID(t, specRegistry)
	AssertUniqueOperationIDs(t, specRegistry)
	AssertOperationHasProblemResponse(t, specRegistry, http.MethodGet, "/widgets", http.StatusBadRequest)
	AssertOperationHasTenantPolicy(t, specRegistry, http.MethodGet, "/widgets")
	AssertOperationHasIdempotencyPolicy(t, specRegistry, http.MethodGet, "/widgets")
	AssertOperationHasRateLimitPolicy(t, specRegistry, http.MethodGet, "/widgets", "read-standard")
	AssertOperationHasAdminPolicy(t, specRegistry, http.MethodGet, "/widgets")
	AssertProblemCatalogHas(t, httpx.DefaultProblemCatalog(), httpx.ProblemCode(httpx.TypeBadRequest))
}

func TestNormalizeAndGoldenOpenAPI(t *testing.T) {
	got, err := NormalizeOpenAPI([]byte(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatalf("NormalizeOpenAPI() error = %v", err)
	}
	want := []byte("{\n  \"a\": 1,\n  \"b\": 2\n}\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("normalized = %s", got)
	}
	GoldenOpenAPI(t, []byte(`{"a":1}`), []byte(`{"a":1}`))
	if _, err := NormalizeOpenAPI([]byte(`{`)); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestAssertOpenAPICompatibleAllowsAdditiveChanges(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": []}]
				}
			}
		}
	}`)
	head := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}, "400": {"description": "bad request"}},
					"security": [{"ApiKeyAuth": []}]
				}
			},
			"/widget-exports": {
				"post": {
					"operationId": "createWidgetExport",
					"responses": {"202": {"description": "accepted"}},
					"security": [{"ApiKeyAuth": ["exports:write"]}]
				}
			}
		}
	}`)

	AssertOpenAPICompatible(t, base, head)
}

func TestOpenAPICompatibilityFindingsReportBreakingChanges(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}, "401": {"description": "unauthorized"}},
					"security": [{"ApiKeyAuth": []}]
				},
				"post": {
					"operationId": "createWidget",
					"responses": {"201": {"description": "created"}}
				}
			}
		}
	}`)
	head := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgetsRenamed",
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`)

	findings, err := OpenAPICompatibilityFindings(base, head)
	if err != nil {
		t.Fatalf("compatibility findings: %v", err)
	}
	for _, want := range []string{
		"operation_removed POST /widgets",
		"operation_id_changed GET /widgets",
		"response_removed GET /widgets 401",
		"security_changed GET /widgets",
	} {
		if !containsString(findings, want) {
			t.Fatalf("findings missing %q: %v", want, findings)
		}
	}
}

func TestOpenAPICompatibilityFindingsReportRequestBodyBreakingChanges(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"post": {
					"operationId": "createWidget",
					"requestBody": {
						"required": false,
						"content": {
							"application/json": {"schema": {"type": "object"}},
							"application/vnd.widgets+json": {"schema": {"type": "object"}}
						}
					},
					"responses": {"201": {"description": "created"}},
					"security": [{"ApiKeyAuth": ["widgets:write"]}]
				}
			},
			"/widget-imports": {
				"post": {
					"operationId": "importWidgets",
					"requestBody": {
						"required": true,
						"content": {"application/json": {"schema": {"type": "object"}}}
					},
					"responses": {"202": {"description": "accepted"}},
					"security": [{"ApiKeyAuth": ["widgets:write"]}]
				}
			}
		}
	}`)
	head := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"post": {
					"operationId": "createWidget",
					"requestBody": {
						"required": true,
						"content": {"application/json": {"schema": {"type": "object"}}}
					},
					"responses": {"201": {"description": "created"}},
					"security": [{"ApiKeyAuth": ["widgets:write"]}]
				}
			},
			"/widget-imports": {
				"post": {
					"operationId": "importWidgets",
					"responses": {"202": {"description": "accepted"}},
					"security": [{"ApiKeyAuth": ["widgets:write"]}]
				}
			}
		}
	}`)

	findings, err := OpenAPICompatibilityFindings(base, head)
	if err != nil {
		t.Fatalf("compatibility findings: %v", err)
	}
	for _, want := range []string{
		"request_body_required_added POST /widgets",
		"request_body_content_removed POST /widgets application/vnd.widgets+json",
		"request_body_removed POST /widget-imports",
	} {
		if !containsString(findings, want) {
			t.Fatalf("findings missing %q: %v", want, findings)
		}
	}
}

func TestOpenAPICompatibilityFindingsReportResponseContentBreakingChanges(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {
						"200": {
							"description": "ok",
							"content": {
								"application/json": {"schema": {"type": "object"}},
								"application/vnd.widgets+json": {"schema": {"type": "object"}}
							}
						},
						"400": {
							"description": "bad request",
							"content": {"application/problem+json": {"schema": {"type": "object"}}}
						}
					},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`)
	head := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {
						"200": {
							"description": "ok",
							"content": {"application/json": {"schema": {"type": "object"}}}
						},
						"400": {"description": "bad request"}
					},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`)

	findings, err := OpenAPICompatibilityFindings(base, head)
	if err != nil {
		t.Fatalf("compatibility findings: %v", err)
	}
	for _, want := range []string{
		"response_content_removed GET /widgets 200 application/vnd.widgets+json",
		"response_content_removed GET /widgets 400 application/problem+json",
	} {
		if !containsString(findings, want) {
			t.Fatalf("findings missing %q: %v", want, findings)
		}
	}
}

func TestOpenAPICompatibilityFindingsReportParameterBreakingChanges(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"parameters": [
						{"name": "cursor", "in": "query", "required": false, "schema": {"type": "string"}},
						{"name": "filter", "in": "query", "required": false, "schema": {"type": "string"}}
					],
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`)
	head := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"parameters": [
						{"name": "filter", "in": "query", "required": true, "schema": {"type": "string"}},
						{"name": "X-Client-Version", "in": "header", "required": true, "schema": {"type": "string"}},
						{"name": "expand", "in": "query", "required": false, "schema": {"type": "string"}}
					],
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`)

	findings, err := OpenAPICompatibilityFindings(base, head)
	if err != nil {
		t.Fatalf("compatibility findings: %v", err)
	}
	for _, want := range []string{
		"parameter_removed GET /widgets query:cursor",
		"parameter_required_added GET /widgets query:filter",
		"required_parameter_added GET /widgets header:X-Client-Version",
	} {
		if !containsString(findings, want) {
			t.Fatalf("findings missing %q: %v", want, findings)
		}
	}
}

type fakeRouter struct{}

func (fakeRouter) Get(pattern string, h http.HandlerFunc)    {}
func (fakeRouter) Post(pattern string, h http.HandlerFunc)   {}
func (fakeRouter) Put(pattern string, h http.HandlerFunc)    {}
func (fakeRouter) Delete(pattern string, h http.HandlerFunc) {}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
