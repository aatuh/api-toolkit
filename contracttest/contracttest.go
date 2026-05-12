package contracttest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/routecontracts"
	"github.com/aatuh/api-toolkit/v2/specs"
)

// AssertRegistryValid fails the test when a route contract registry has coverage errors.
func AssertRegistryValid(t testing.TB, registry *routecontracts.Registry) {
	t.Helper()
	if registry == nil {
		t.Fatalf("route contract registry is nil")
	}
	if err := registry.Validate(); err != nil {
		t.Fatalf("route contract registry is invalid: %v", err)
	}
}

// AssertRouteCoverage fails the test when method and pattern are not registered.
func AssertRouteCoverage(t testing.TB, registry *routecontracts.Registry, method, pattern string) {
	t.Helper()
	AssertRegistryValid(t, registry)
	method = strings.ToUpper(strings.TrimSpace(method))
	pattern = strings.TrimSpace(pattern)
	for _, route := range registry.Routes() {
		if strings.ToUpper(route.Method) == method && strings.TrimSpace(route.Pattern) == pattern {
			return
		}
	}
	t.Fatalf("route contract %s %s is not registered", method, pattern)
}

// AssertOperationHasResponse fails the test when an OpenAPI operation lacks a response status.
func AssertOperationHasResponse(t testing.TB, registry *specs.Registry, method, path string, status int) {
	t.Helper()
	operation := openAPIOperation(t, registry, method, path)
	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		t.Fatalf("operation %s %s has no responses", method, path)
	}
	if _, ok := responses[strconv.Itoa(status)]; !ok {
		t.Fatalf("operation %s %s missing response %d", method, path, status)
	}
}

// AssertOperationHasSecurity fails the test when an OpenAPI operation lacks a security requirement.
func AssertOperationHasSecurity(t testing.TB, registry *specs.Registry, method, path, scheme string) {
	t.Helper()
	operation := openAPIOperation(t, registry, method, path)
	security, ok := operation["security"].([]any)
	if !ok || len(security) == 0 {
		t.Fatalf("operation %s %s has no security requirements", method, path)
	}
	for _, entry := range security {
		requirement, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := requirement[scheme]; ok {
			return
		}
	}
	t.Fatalf("operation %s %s missing security scheme %q", method, path, scheme)
}

// AssertOperationID fails the test when an OpenAPI operation lacks the expected operationId.
func AssertOperationID(t testing.TB, registry *specs.Registry, method, path, operationID string) {
	t.Helper()
	operation := openAPIOperation(t, registry, method, path)
	if got := strings.TrimSpace(fmtString(operation["operationId"])); got != strings.TrimSpace(operationID) {
		t.Fatalf("operation %s %s operationId = %q, want %q", method, path, got, operationID)
	}
}

// AssertAllOperationsHaveOperationID fails the test when any registered operation lacks operationId.
func AssertAllOperationsHaveOperationID(t testing.TB, registry *specs.Registry) {
	t.Helper()
	if registry == nil {
		t.Fatalf("spec registry is nil")
	}
	for _, operation := range registry.Operations() {
		if strings.TrimSpace(operation.OperationID) == "" {
			t.Fatalf("operation %s %s is missing operationId", operation.Method, operation.Path)
		}
	}
}

// AssertOperationHasProblemResponse fails when an operation response is not application/problem+json.
func AssertOperationHasProblemResponse(t testing.TB, registry *specs.Registry, method, path string, status int) {
	t.Helper()
	operation := openAPIOperation(t, registry, method, path)
	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		t.Fatalf("operation %s %s has no responses", method, path)
	}
	response, ok := responses[strconv.Itoa(status)].(map[string]any)
	if !ok {
		t.Fatalf("operation %s %s missing response %d", method, path, status)
	}
	if ref, _ := response["$ref"].(string); strings.Contains(ref, "Problem") {
		return
	}
	content, ok := response["content"].(map[string]any)
	if !ok {
		t.Fatalf("operation %s %s response %d has no content", method, path, status)
	}
	if _, ok := content["application/problem+json"]; !ok {
		t.Fatalf("operation %s %s response %d missing application/problem+json", method, path, status)
	}
}

// AssertProblemCatalogHas fails the test when a problem catalog lacks a code.
func AssertProblemCatalogHas(t testing.TB, catalog *httpx.ProblemCatalog, code httpx.ProblemCode) {
	t.Helper()
	if catalog == nil {
		catalog = httpx.DefaultProblemCatalog()
	}
	if _, ok := catalog.Definition(code); !ok {
		t.Fatalf("problem catalog missing code %q", code)
	}
}

// NormalizeOpenAPI returns deterministic indented JSON for OpenAPI comparison.
func NormalizeOpenAPI(doc []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(doc, &value); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// GoldenOpenAPI fails the test when normalized OpenAPI JSON differs from golden JSON.
func GoldenOpenAPI(t testing.TB, got, golden []byte) {
	t.Helper()
	normalizedGot, err := NormalizeOpenAPI(got)
	if err != nil {
		t.Fatalf("normalize OpenAPI output: %v", err)
	}
	normalizedGolden, err := NormalizeOpenAPI(golden)
	if err != nil {
		t.Fatalf("normalize golden OpenAPI: %v", err)
	}
	if !bytes.Equal(normalizedGot, normalizedGolden) {
		t.Fatalf("OpenAPI golden mismatch\nwant:\n%s\ngot:\n%s", normalizedGolden, normalizedGot)
	}
}

func openAPIOperation(t testing.TB, registry *specs.Registry, method, path string) map[string]any {
	t.Helper()
	if registry == nil {
		t.Fatalf("spec registry is nil")
	}
	doc, err := registry.OpenAPI()
	if err != nil {
		t.Fatalf("OpenAPI() error = %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(doc, &root); err != nil {
		t.Fatalf("OpenAPI JSON error = %v", err)
	}
	paths, ok := root["paths"].(map[string]any)
	if !ok {
		t.Fatalf("OpenAPI document has no paths")
	}
	pathItem, ok := paths[path].(map[string]any)
	if !ok {
		t.Fatalf("OpenAPI path %q is missing", path)
	}
	operation, ok := pathItem[strings.ToLower(strings.TrimSpace(method))].(map[string]any)
	if !ok {
		t.Fatalf("OpenAPI operation %s %s is missing", method, path)
	}
	return operation
}

func fmtString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
