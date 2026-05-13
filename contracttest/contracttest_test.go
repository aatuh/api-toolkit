package contracttest

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/routecontracts"
	"github.com/aatuh/api-toolkit/v2/routepolicy"
	"github.com/aatuh/api-toolkit/v2/specs"
)

func TestContractAssertionsPassForCoveredRoute(t *testing.T) {
	specRegistry := specs.NewRegistry(specs.Info{Title: "Contracts", Version: "1"})
	specRegistry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	specRegistry.Register(specs.Operation{
		OperationID: "listWidgets",
		Method:      http.MethodGet,
		Path:        "/widgets",
		Security: []specs.SecurityRequirement{{
			Name:   "ApiKeyAuth",
			Scopes: []string{"widgets:read"},
		}},
		Responses: map[int]specs.Response{
			http.StatusOK:         {Description: "ok"},
			http.StatusBadRequest: specs.ProblemResponse("bad request"),
		},
		Extensions: map[string]any{
			routepolicy.ExtensionTenant:         map[string]any{"required": true, "source": "header"},
			routepolicy.ExtensionIdempotencyKey: map[string]any{"required": true, "header": "Idempotency-Key"},
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
	AssertOperationHasSecurityScopes(t, specRegistry, http.MethodGet, "/widgets", "ApiKeyAuth", "widgets:read")
	AssertSecuritySchemesDefined(t, specRegistry)
	AssertOperationID(t, specRegistry, http.MethodGet, "/widgets", "listWidgets")
	AssertAllOperationsHaveOperationID(t, specRegistry)
	AssertUniqueOperationIDs(t, specRegistry)
	AssertOperationHasProblemResponse(t, specRegistry, http.MethodGet, "/widgets", http.StatusBadRequest)
	AssertOperationHasProblemResponses(t, specRegistry, http.MethodGet, "/widgets", http.StatusBadRequest)
	AssertOperationHasTenantPolicy(t, specRegistry, http.MethodGet, "/widgets")
	AssertOperationHasTenantPolicySource(t, specRegistry, http.MethodGet, "/widgets", "header")
	AssertOperationHasIdempotencyPolicy(t, specRegistry, http.MethodGet, "/widgets")
	AssertOperationHasIdempotencyPolicyHeader(t, specRegistry, http.MethodGet, "/widgets", "Idempotency-Key")
	AssertOperationHasRateLimitPolicy(t, specRegistry, http.MethodGet, "/widgets", "read-standard")
	AssertOperationHasAdminPolicy(t, specRegistry, http.MethodGet, "/widgets")
	AssertOperationHasAdminPolicyNamed(t, specRegistry, http.MethodGet, "/widgets", "admin")
	AssertProblemCatalogHas(t, httpx.DefaultProblemCatalog(), httpx.ProblemCode(httpx.TypeBadRequest))
}

func TestSecuritySchemeDefinitionFindingsReportsUndefinedSchemes(t *testing.T) {
	registry := specs.NewRegistry(specs.Info{Title: "Contracts", Version: "1"})
	registry.Register(specs.Operation{
		OperationID: "listWidgets",
		Method:      http.MethodGet,
		Path:        "/widgets",
		Security: []specs.SecurityRequirement{{
			Name:   "MissingAuth",
			Scopes: []string{"widgets:read"},
		}},
		Responses: map[int]specs.Response{http.StatusOK: {Description: "ok"}},
	})

	findings := SecuritySchemeDefinitionFindings(registry)
	if len(findings) != 1 || !strings.Contains(findings[0], "security_scheme_undefined GET /widgets MissingAuth") {
		t.Fatalf("findings = %v", findings)
	}

	registry.RegisterSecurityScheme("MissingAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	if findings := SecuritySchemeDefinitionFindings(registry); len(findings) != 0 {
		t.Fatalf("findings after registering scheme = %v", findings)
	}
}

func TestSecuritySchemeDefinitionFindingsReportsUndefinedGlobalSchemes(t *testing.T) {
	registry := specs.NewRegistry(specs.Info{Title: "Contracts", Version: "1"})
	registry.SetSecurity([]specs.SecurityRequirement{{
		Name:   "MissingGlobalAuth",
		Scopes: []string{"widgets:read"},
	}})
	registry.Register(specs.Operation{
		OperationID: "listWidgets",
		Method:      http.MethodGet,
		Path:        "/widgets",
		Responses:   map[int]specs.Response{http.StatusOK: {Description: "ok"}},
	})

	findings := SecuritySchemeDefinitionFindings(registry)
	if len(findings) != 1 || !strings.Contains(findings[0], "security_scheme_undefined GLOBAL MissingGlobalAuth") {
		t.Fatalf("findings = %v", findings)
	}

	registry.RegisterSecurityScheme("MissingGlobalAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	if findings := SecuritySchemeDefinitionFindings(registry); len(findings) != 0 {
		t.Fatalf("findings after registering scheme = %v", findings)
	}
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

func TestOpenAPICompatibilityFindingsReportGlobalSecurityDrift(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"security": [{"ApiKeyAuth": ["widgets:read"]}],
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}}
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
		`global_security_changed "ApiKeyAuth:widgets:read" -> ""`,
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

func TestOpenAPICompatibilityFindingsReportRoutePolicyDrift(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"post": {
					"operationId": "createWidget",
					"deprecated": true,
					"requestBody": {
						"required": true,
						"content": {"application/json": {"schema": {"type": "object"}}}
					},
					"responses": {
						"201": {"description": "created"},
						"400": {
							"description": "bad request",
							"content": {"application/problem+json": {"schema": {"type": "object"}}}
						}
					},
					"security": [{"ApiKeyAuth": ["widgets:write"]}],
					"x-tenant": {"required": true, "source": "header"},
					"x-idempotency-key": {"required": true, "header": "Idempotency-Key"},
					"x-rate-limit": "write-standard",
					"x-sunset": "Wed, 01 Jul 2026 00:00:00 GMT"
				}
			},
			"/metrics": {
				"get": {
					"operationId": "getMetrics",
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["admin:read"]}],
					"x-admin-policy": "admin"
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
					"responses": {
						"201": {"description": "created"},
						"400": {
							"description": "bad request",
							"content": {"application/problem+json": {"schema": {"type": "object"}}}
						}
					},
					"security": [{"ApiKeyAuth": ["widgets:write"]}],
					"x-tenant": {"required": false, "source": "path"},
					"x-rate-limit": "write-burst"
				}
			},
			"/metrics": {
				"get": {
					"operationId": "getMetrics",
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["admin:read"]}]
				}
			}
		}
	}`)

	findings, err := OpenAPICompatibilityFindings(base, head)
	if err != nil {
		t.Fatalf("compatibility findings: %v", err)
	}
	for _, want := range []string{
		"tenant_policy_changed POST /widgets",
		"idempotency_policy_changed POST /widgets",
		"rate_limit_policy_changed POST /widgets",
		"deprecation_policy_changed POST /widgets",
		"admin_policy_changed GET /metrics",
	} {
		if !containsString(findings, want) {
			t.Fatalf("findings missing %q: %v", want, findings)
		}
	}
}

func TestOpenAPICompatibilityFindingsReportComponentSchemaDrift(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"schemas": {
				"Legacy": {"type": "object", "properties": {"id": {"type": "string"}}},
				"Widget": {
					"type": "object",
					"required": ["name"],
					"properties": {
						"name": {"type": "string"},
						"status": {"type": "string", "enum": ["active", "disabled"]},
						"tenant_id": {"type": "string"}
					}
				}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`)
	head := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"schemas": {
				"Widget": {
					"type": "object",
					"required": ["name", "status"],
					"properties": {
						"name": {"type": "integer"},
						"status": {"type": "string", "enum": ["active"]}
					}
				}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
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
		"schema_removed Legacy",
		"schema_required_property_added Widget status",
		"schema_property_removed Widget tenant_id",
		"schema_type_changed Widget.name",
		"schema_enum_value_removed Widget.status \"disabled\"",
	} {
		if !containsString(findings, want) {
			t.Fatalf("findings missing %q: %v", want, findings)
		}
	}
}

func TestOpenAPICompatibilityFindingsReportSecuritySchemeDrift(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"},
				"WebhookAuth": {"type": "apiKey", "in": "header", "name": "X-Webhook-Signature"}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`)
	head := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key-V2"}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
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
		"security_scheme_changed ApiKeyAuth",
		"security_scheme_removed WebhookAuth",
	} {
		if !containsString(findings, want) {
			t.Fatalf("findings missing %q: %v", want, findings)
		}
	}
}

func TestOpenAPICompatibilityFindingsReportInlineSchemaDrift(t *testing.T) {
	base := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/widgets": {
				"post": {
					"operationId": "createWidget",
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"required": ["name"],
									"properties": {
										"name": {"type": "string"},
										"status": {"type": "string", "enum": ["active", "disabled"]}
									}
								}
							}
						}
					},
					"responses": {
						"201": {
							"description": "created",
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"properties": {
											"id": {"type": "string"},
											"status": {"type": "string"}
										}
									}
								}
							}
						}
					},
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
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"required": ["name", "status"],
									"properties": {
										"name": {"type": "integer"},
										"status": {"type": "string", "enum": ["active"]}
									}
								}
							}
						}
					},
					"responses": {
						"201": {
							"description": "created",
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"properties": {
											"id": {"type": "integer"}
										}
									}
								}
							}
						}
					},
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
		"schema_required_property_added POST /widgets requestBody application/json status",
		"schema_type_changed POST /widgets requestBody application/json.name",
		"schema_enum_value_removed POST /widgets requestBody application/json.status \"disabled\"",
		"schema_type_changed POST /widgets response 201 application/json.id",
		"schema_property_removed POST /widgets response 201 application/json status",
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
