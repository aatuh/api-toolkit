package routecontracts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/apiclient"
	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/specs"
)

type openAPIClientRoundTripCreateWidget struct {
	Name string `json:"name"`
}

type openAPIClientRoundTripWidget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestRegistryGeneratedOpenAPIDrivesExampleJSONClient(t *testing.T) {
	router := &openAPIClientRoundTripRouter{}
	specRegistry := specs.NewRegistry(specs.Info{Title: "Widget API", Version: "1.0.0"})
	if err := specs.RegisterSchemaFrom[openAPIClientRoundTripCreateWidget](specRegistry, "CreateWidget", specs.SchemaOptions{}); err != nil {
		t.Fatalf("register CreateWidget schema: %v", err)
	}
	if err := specs.RegisterSchemaFrom[openAPIClientRoundTripWidget](specRegistry, "Widget", specs.SchemaOptions{}); err != nil {
		t.Fatalf("register Widget schema: %v", err)
	}

	registry := NewRegistry(router, specRegistry)
	err := registry.Post("/widgets", specs.Operation{
		OperationID: "createWidget",
		RequestBody: &specs.RequestBody{
			Required: true,
			Content: map[string]specs.MediaType{
				"application/json": {SchemaRef: "#/components/schemas/CreateWidget"},
			},
		},
		Responses: map[int]specs.Response{
			http.StatusCreated: {
				Description: "Created",
				Content: map[string]specs.MediaType{
					"application/json": {SchemaRef: "#/components/schemas/Widget"},
				},
			},
			http.StatusBadRequest: specs.ValidationProblemResponse("Invalid widget"),
		},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openAPIClientRoundTripCreateWidget
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeValidation),
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
				Detail: "invalid request",
			})
			return
		}
		if strings.TrimSpace(request.Name) == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeValidation),
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
				Detail: "name is required",
			})
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, openAPIClientRoundTripWidget{ID: "widget_123", Name: request.Name})
	}))
	if err != nil {
		t.Fatalf("register route: %v", err)
	}
	if err := registry.Validate(); err != nil {
		t.Fatalf("route registry validation: %v", err)
	}

	openAPI, err := specRegistry.OpenAPI()
	if err != nil {
		t.Fatalf("OpenAPI() error = %v", err)
	}
	operation := openAPIClientOperationFromDocument(t, openAPI, "createWidget")
	if operation.Method != http.MethodPost || operation.Path != "/widgets" {
		t.Fatalf("client operation = %#v", operation)
	}
	if operation.RequestSchemaRef != "#/components/schemas/CreateWidget" {
		t.Fatalf("request schema ref = %q", operation.RequestSchemaRef)
	}
	if operation.ResponseSchemaRef != "#/components/schemas/Widget" {
		t.Fatalf("response schema ref = %q", operation.ResponseSchemaRef)
	}
	if operation.ProblemContentType != "application/problem+json" {
		t.Fatalf("problem content type = %q", operation.ProblemContentType)
	}

	client := &http.Client{Transport: localRoundTripper{handler: router}}
	baseURL := "https://api.example.test"

	created, resp, err := apiclient.DoJSON[openAPIClientRoundTripWidget](
		context.Background(),
		client,
		operation.Method,
		baseURL+operation.Path,
		openAPIClientRoundTripCreateWidget{Name: "primary"},
	)
	if err != nil {
		t.Fatalf("client create widget error = %v", err)
	}
	if resp == nil {
		t.Fatal("client create widget response is nil")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || created.ID != "widget_123" || created.Name != "primary" {
		t.Fatalf("client create widget response = %#v status=%d", created, resp.StatusCode)
	}

	_, resp, err = apiclient.DoJSON[openAPIClientRoundTripWidget](
		context.Background(),
		client,
		operation.Method,
		baseURL+operation.Path,
		openAPIClientRoundTripCreateWidget{},
	)
	if resp == nil {
		t.Fatal("client problem response is nil")
	}
	defer resp.Body.Close()
	var problem *apiclient.ProblemError
	if !errors.As(err, &problem) {
		t.Fatalf("client problem error = %T %v", err, err)
	}
	if resp.StatusCode != http.StatusBadRequest || problem.Problem.Detail != "name is required" {
		t.Fatalf("client problem = %#v status=%d", problem.Problem, resp.StatusCode)
	}
}

type openAPIClientRoundTripOperation struct {
	Method             string
	Path               string
	RequestSchemaRef   string
	ResponseSchemaRef  string
	ProblemContentType string
}

func openAPIClientOperationFromDocument(t *testing.T, data []byte, operationID string) openAPIClientRoundTripOperation {
	t.Helper()

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode OpenAPI document: %v", err)
	}
	components := routeOpenAPIMap(t, doc["components"])
	schemas := routeOpenAPIMap(t, components["schemas"])
	paths := routeOpenAPIMap(t, doc["paths"])
	for path, rawPathItem := range paths {
		pathItem := routeOpenAPIMap(t, rawPathItem)
		for method, rawOperation := range pathItem {
			operation := routeOpenAPIMap(t, rawOperation)
			if operation["operationId"] != operationID {
				continue
			}
			requestSchemaRef := routeOpenAPISchemaRef(t, operation, "requestBody", "application/json")
			responseSchemaRef := routeOpenAPIResponseSchemaRef(t, operation, "201", "application/json")
			if _, ok := schemas[strings.TrimPrefix(requestSchemaRef, "#/components/schemas/")]; !ok {
				t.Fatalf("request schema ref %q is missing from components", requestSchemaRef)
			}
			if _, ok := schemas[strings.TrimPrefix(responseSchemaRef, "#/components/schemas/")]; !ok {
				t.Fatalf("response schema ref %q is missing from components", responseSchemaRef)
			}
			return openAPIClientRoundTripOperation{
				Method:             strings.ToUpper(method),
				Path:               path,
				RequestSchemaRef:   requestSchemaRef,
				ResponseSchemaRef:  responseSchemaRef,
				ProblemContentType: routeOpenAPIProblemContentType(t, operation, "400"),
			}
		}
	}
	t.Fatalf("operationId %q not found in generated OpenAPI", operationID)
	return openAPIClientRoundTripOperation{}
}

func routeOpenAPISchemaRef(t *testing.T, operation map[string]any, field, contentType string) string {
	t.Helper()
	requestBody := routeOpenAPIMap(t, operation[field])
	content := routeOpenAPIMap(t, requestBody["content"])
	media := routeOpenAPIMap(t, content[contentType])
	return routeOpenAPIRef(t, routeOpenAPIMap(t, media["schema"]))
}

func routeOpenAPIResponseSchemaRef(t *testing.T, operation map[string]any, status, contentType string) string {
	t.Helper()
	responses := routeOpenAPIMap(t, operation["responses"])
	response := routeOpenAPIMap(t, responses[status])
	content := routeOpenAPIMap(t, response["content"])
	media := routeOpenAPIMap(t, content[contentType])
	return routeOpenAPIRef(t, routeOpenAPIMap(t, media["schema"]))
}

func routeOpenAPIProblemContentType(t *testing.T, operation map[string]any, status string) string {
	t.Helper()
	responses := routeOpenAPIMap(t, operation["responses"])
	response := routeOpenAPIMap(t, responses[status])
	content := routeOpenAPIMap(t, response["content"])
	if _, ok := content["application/problem+json"]; !ok {
		t.Fatalf("response %s missing application/problem+json content", status)
	}
	return "application/problem+json"
}

func routeOpenAPIRef(t *testing.T, schema map[string]any) string {
	t.Helper()
	ref, ok := schema["$ref"].(string)
	if !ok || strings.TrimSpace(ref) == "" {
		t.Fatalf("schema ref missing: %#v", schema)
	}
	return ref
}

func routeOpenAPIMap(t *testing.T, value any) map[string]any {
	t.Helper()
	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value has type %T, want map[string]any: %#v", value, value)
	}
	return out
}

type openAPIClientRoundTripRouter struct {
	handlers map[string]http.HandlerFunc
}

func (r *openAPIClientRoundTripRouter) Get(pattern string, h http.HandlerFunc) {
	r.set(http.MethodGet, pattern, h)
}

func (r *openAPIClientRoundTripRouter) Post(pattern string, h http.HandlerFunc) {
	r.set(http.MethodPost, pattern, h)
}

func (r *openAPIClientRoundTripRouter) Put(pattern string, h http.HandlerFunc) {
	r.set(http.MethodPut, pattern, h)
}

func (r *openAPIClientRoundTripRouter) Delete(pattern string, h http.HandlerFunc) {
	r.set(http.MethodDelete, pattern, h)
}

func (r *openAPIClientRoundTripRouter) set(method, pattern string, h http.HandlerFunc) {
	if r.handlers == nil {
		r.handlers = map[string]http.HandlerFunc{}
	}
	r.handlers[method+" "+pattern] = h
}

func (r *openAPIClientRoundTripRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r == nil {
		http.NotFound(w, req)
		return
	}
	key := req.Method + " " + req.URL.Path
	handler, ok := r.handlers[key]
	if !ok {
		http.NotFound(w, req)
		return
	}
	handler.ServeHTTP(w, req)
}

func (op openAPIClientRoundTripOperation) String() string {
	return fmt.Sprintf("%s %s", op.Method, op.Path)
}

type localRoundTripper struct {
	handler http.Handler
}

func (transport localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if transport.handler == nil {
		return nil, fmt.Errorf("handler is required")
	}
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}
