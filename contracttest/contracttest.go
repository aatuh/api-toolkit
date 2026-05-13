package contracttest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/routecontracts"
	"github.com/aatuh/api-toolkit/v2/routepolicy"
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

// AssertOperationHasSecurityScopes fails the test when an OpenAPI operation
// lacks the expected security requirement scopes for a scheme.
func AssertOperationHasSecurityScopes(t testing.TB, registry *specs.Registry, method, path, scheme string, scopes ...string) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	auth, ok := routepolicy.AuthPolicyFromOperation(operation)
	if !ok {
		t.Fatalf("operation %s %s has no security requirements", method, path)
	}
	wantScopes := sortedNonEmptyStrings(scopes)
	scheme = strings.TrimSpace(scheme)
	for _, requirement := range auth.Security {
		if strings.TrimSpace(requirement.Name) != scheme {
			continue
		}
		gotScopes := sortedNonEmptyStrings(requirement.Scopes)
		if sameStrings(gotScopes, wantScopes) {
			return
		}
		t.Fatalf("operation %s %s security scopes for %q = %v, want %v", method, path, scheme, gotScopes, wantScopes)
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

// AssertUniqueOperationIDs fails the test when two operations share an operationId.
func AssertUniqueOperationIDs(t testing.TB, registry *specs.Registry) {
	t.Helper()
	if registry == nil {
		t.Fatalf("spec registry is nil")
	}
	seen := map[string]specs.Operation{}
	for _, operation := range registry.Operations() {
		operationID := strings.TrimSpace(operation.OperationID)
		if operationID == "" {
			continue
		}
		if previous, ok := seen[operationID]; ok {
			t.Fatalf("operation %s %s duplicates operationId %q from %s %s", operation.Method, operation.Path, operationID, previous.Method, previous.Path)
		}
		seen[operationID] = operation
	}
}

// AssertOperationHasProblemResponse fails when an operation response is not application/problem+json.
func AssertOperationHasProblemResponse(t testing.TB, registry *specs.Registry, method, path string, status int) {
	t.Helper()
	AssertOperationHasProblemResponses(t, registry, method, path, status)
}

// AssertOperationHasProblemResponses fails when an operation lacks documented
// Problem Details responses for any expected status.
func AssertOperationHasProblemResponses(t testing.TB, registry *specs.Registry, method, path string, statuses ...int) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	problemStatuses := routepolicy.ProblemResponseStatuses(operation)
	if len(problemStatuses) == 0 {
		t.Fatalf("operation %s %s has no Problem Details responses", method, path)
	}
	for _, status := range statuses {
		if !containsInt(problemStatuses, status) {
			t.Fatalf("operation %s %s missing Problem Details response %d; got %v", method, path, status, problemStatuses)
		}
	}
}

// AssertOperationHasTenantPolicy fails when an operation lacks required tenant metadata.
func AssertOperationHasTenantPolicy(t testing.TB, registry *specs.Registry, method, path string) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	tenant, ok := routepolicy.TenantPolicyFromOperation(operation)
	if !ok {
		t.Fatalf("operation %s %s missing %s metadata", method, path, routepolicy.ExtensionTenant)
	}
	if !tenant.Required {
		t.Fatalf("operation %s %s tenant metadata is not required", method, path)
	}
}

// AssertOperationHasTenantPolicySource fails when an operation lacks required
// tenant metadata with the expected source.
func AssertOperationHasTenantPolicySource(t testing.TB, registry *specs.Registry, method, path, source string) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	tenant, ok := routepolicy.TenantPolicyFromOperation(operation)
	if !ok {
		t.Fatalf("operation %s %s missing %s metadata", method, path, routepolicy.ExtensionTenant)
	}
	if !tenant.Required {
		t.Fatalf("operation %s %s tenant metadata is not required", method, path)
	}
	if got, want := strings.TrimSpace(tenant.Source), strings.TrimSpace(source); got != want {
		t.Fatalf("operation %s %s tenant source = %q, want %q", method, path, got, want)
	}
}

// AssertOperationHasIdempotencyPolicy fails when an operation lacks idempotency metadata.
func AssertOperationHasIdempotencyPolicy(t testing.TB, registry *specs.Registry, method, path string) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	policy, ok := routepolicy.IdempotencyPolicyFromOperation(operation)
	if !ok {
		t.Fatalf("operation %s %s missing %s metadata", method, path, routepolicy.ExtensionIdempotencyKey)
	}
	if !policy.Required {
		t.Fatalf("operation %s %s idempotency metadata is not required", method, path)
	}
}

// AssertOperationHasIdempotencyPolicyHeader fails when an operation lacks
// required idempotency metadata with the expected header.
func AssertOperationHasIdempotencyPolicyHeader(t testing.TB, registry *specs.Registry, method, path, header string) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	policy, ok := routepolicy.IdempotencyPolicyFromOperation(operation)
	if !ok {
		t.Fatalf("operation %s %s missing %s metadata", method, path, routepolicy.ExtensionIdempotencyKey)
	}
	if !policy.Required {
		t.Fatalf("operation %s %s idempotency metadata is not required", method, path)
	}
	if got, want := strings.TrimSpace(policy.Header), strings.TrimSpace(header); got != want {
		t.Fatalf("operation %s %s idempotency header = %q, want %q", method, path, got, want)
	}
}

// AssertOperationHasRateLimitPolicy fails when an operation lacks the expected rate-limit policy metadata.
func AssertOperationHasRateLimitPolicy(t testing.TB, registry *specs.Registry, method, path, policy string) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	got, ok := routepolicy.RateLimitPolicyFromOperation(operation)
	policy = strings.TrimSpace(policy)
	if !ok {
		t.Fatalf("operation %s %s missing %s metadata", method, path, routepolicy.ExtensionRateLimit)
	}
	if policy != "" && got != policy {
		t.Fatalf("operation %s %s rate-limit policy = %q, want %q", method, path, got, policy)
	}
}

// AssertOperationHasAdminPolicy fails when an operation lacks admin policy metadata.
func AssertOperationHasAdminPolicy(t testing.TB, registry *specs.Registry, method, path string) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	if _, ok := routepolicy.AdminPolicyFromOperation(operation); !ok {
		t.Fatalf("operation %s %s missing %s metadata", method, path, routepolicy.ExtensionAdminPolicy)
	}
}

// AssertOperationHasAdminPolicyNamed fails when an operation lacks the expected
// admin policy metadata.
func AssertOperationHasAdminPolicyNamed(t testing.TB, registry *specs.Registry, method, path, policy string) {
	t.Helper()
	operation := openAPISpecOperation(t, registry, method, path)
	got, ok := routepolicy.AdminPolicyFromOperation(operation)
	if !ok {
		t.Fatalf("operation %s %s missing %s metadata", method, path, routepolicy.ExtensionAdminPolicy)
	}
	if want := strings.TrimSpace(policy); want != "" && got != want {
		t.Fatalf("operation %s %s admin policy = %q, want %q", method, path, got, want)
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

// AssertOpenAPICompatible fails when head removes or changes base operations in
// a way that requires compatibility review.
func AssertOpenAPICompatible(t testing.TB, base, head []byte) {
	t.Helper()
	findings, err := OpenAPICompatibilityFindings(base, head)
	if err != nil {
		t.Fatalf("OpenAPI compatibility parse error: %v", err)
	}
	if len(findings) > 0 {
		t.Fatalf("OpenAPI compatibility findings:\n%s", strings.Join(findings, "\n"))
	}
}

// OpenAPICompatibilityFindings reports conservative compatibility findings
// between two OpenAPI JSON documents. Additive operations and responses are
// compatible; removed operations, changed operation IDs, removed documented
// parameters, added required parameters, removed documented responses,
// request-body tightening or content removal, response content removal, and
// changed security requirements are findings.
func OpenAPICompatibilityFindings(base, head []byte) ([]string, error) {
	baseOperations, err := openAPIOperationSnapshots(base)
	if err != nil {
		return nil, fmt.Errorf("base: %w", err)
	}
	headOperations, err := openAPIOperationSnapshots(head)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	headByKey := make(map[string]openAPIOperationSnapshot, len(headOperations))
	for _, operation := range headOperations {
		headByKey[operation.key()] = operation
	}
	var findings []string
	for _, baseOperation := range baseOperations {
		headOperation, ok := headByKey[baseOperation.key()]
		if !ok {
			findings = append(findings, fmt.Sprintf("operation_removed %s %s", baseOperation.Method, baseOperation.Path))
			continue
		}
		if strings.TrimSpace(baseOperation.OperationID) != "" && strings.TrimSpace(baseOperation.OperationID) != strings.TrimSpace(headOperation.OperationID) {
			findings = append(findings, fmt.Sprintf("operation_id_changed %s %s", baseOperation.Method, baseOperation.Path))
		}
		findings = append(findings, parameterCompatibilityFindings(baseOperation, headOperation)...)
		for _, status := range baseOperation.ResponseStatuses {
			if !containsValue(headOperation.ResponseStatuses, status) {
				findings = append(findings, fmt.Sprintf("response_removed %s %s %s", baseOperation.Method, baseOperation.Path, status))
				continue
			}
			for _, contentType := range baseOperation.ResponseContentTypes[status] {
				if !containsValue(headOperation.ResponseContentTypes[status], contentType) {
					findings = append(findings, fmt.Sprintf("response_content_removed %s %s %s %s", baseOperation.Method, baseOperation.Path, status, contentType))
				}
			}
		}
		if baseOperation.RequestBodyPresent && !headOperation.RequestBodyPresent {
			findings = append(findings, fmt.Sprintf("request_body_removed %s %s", baseOperation.Method, baseOperation.Path))
		}
		if !baseOperation.RequestBodyRequired && headOperation.RequestBodyRequired {
			findings = append(findings, fmt.Sprintf("request_body_required_added %s %s", baseOperation.Method, baseOperation.Path))
		}
		if baseOperation.RequestBodyPresent && headOperation.RequestBodyPresent {
			for _, contentType := range baseOperation.RequestBodyContentTypes {
				if !containsValue(headOperation.RequestBodyContentTypes, contentType) {
					findings = append(findings, fmt.Sprintf("request_body_content_removed %s %s %s", baseOperation.Method, baseOperation.Path, contentType))
				}
			}
		}
		if strings.Join(baseOperation.Security, "|") != strings.Join(headOperation.Security, "|") {
			findings = append(findings, fmt.Sprintf("security_changed %s %s", baseOperation.Method, baseOperation.Path))
		}
	}
	return findings, nil
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

func openAPISpecOperation(t testing.TB, registry *specs.Registry, method, path string) specs.Operation {
	t.Helper()
	raw := openAPIOperation(t, registry, method, path)
	return specs.Operation{
		OperationID: fmtString(raw["operationId"]),
		Method:      strings.ToUpper(strings.TrimSpace(method)),
		Path:        strings.TrimSpace(path),
		Deprecated:  boolValue(raw["deprecated"]),
		Sunset:      fmtString(raw["x-sunset"]),
		Security:    specSecurityRequirements(raw["security"]),
		Scopes:      securityScopes(raw["x-scopes"]),
		Responses:   specResponses(raw["responses"]),
		Extensions:  specExtensions(raw),
	}
}

func specExtensions(raw map[string]any) map[string]any {
	extensions := map[string]any{}
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if strings.HasPrefix(key, "x-") {
			extensions[key] = value
		}
	}
	if len(extensions) == 0 {
		return nil
	}
	return extensions
}

func specSecurityRequirements(raw any) []specs.SecurityRequirement {
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []specs.SecurityRequirement
	for _, entry := range entries {
		requirement, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for scheme, rawScopes := range requirement {
			scheme = strings.TrimSpace(scheme)
			if scheme == "" {
				continue
			}
			out = append(out, specs.SecurityRequirement{
				Name:   scheme,
				Scopes: securityScopes(rawScopes),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func specResponses(raw any) map[int]specs.Response {
	responses, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[int]specs.Response, len(responses))
	for rawStatus, rawResponse := range responses {
		status, err := strconv.Atoi(strings.TrimSpace(rawStatus))
		if err != nil {
			continue
		}
		response, ok := rawResponse.(map[string]any)
		if !ok {
			continue
		}
		out[status] = specs.Response{
			Description:  fmtString(response["description"]),
			ContentTypes: specResponseContentTypes(response["content"]),
			Content:      specResponseContent(response["content"]),
			Ref:          fmtString(response["$ref"]),
		}
	}
	return out
}

func specResponseContentTypes(raw any) []string {
	content, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return sortedMapKeys(content)
}

func specResponseContent(raw any) map[string]specs.MediaType {
	content, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]specs.MediaType, len(content))
	for contentType := range content {
		contentType = strings.TrimSpace(contentType)
		if contentType != "" {
			out[contentType] = specs.MediaType{}
		}
	}
	return out
}

type openAPIOperationSnapshot struct {
	Method                  string
	Path                    string
	OperationID             string
	Parameters              []openAPIParameterSnapshot
	RequestBodyPresent      bool
	RequestBodyRequired     bool
	RequestBodyContentTypes []string
	ResponseStatuses        []string
	ResponseContentTypes    map[string][]string
	Security                []string
}

func (operation openAPIOperationSnapshot) key() string {
	return strings.ToUpper(strings.TrimSpace(operation.Method)) + " " + strings.TrimSpace(operation.Path)
}

func openAPIOperationSnapshots(doc []byte) ([]openAPIOperationSnapshot, error) {
	var root map[string]any
	if err := json.Unmarshal(doc, &root); err != nil {
		return nil, err
	}
	paths, ok := root["paths"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("OpenAPI document has no paths")
	}
	var operations []openAPIOperationSnapshot
	for path, rawPathItem := range paths {
		pathItem, ok := rawPathItem.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range []string{httpMethodGet, httpMethodPost, httpMethodPut, httpMethodPatch, httpMethodDelete} {
			rawOperation, ok := pathItem[method].(map[string]any)
			if !ok {
				continue
			}
			operations = append(operations, openAPIOperationSnapshot{
				Method:                  strings.ToUpper(method),
				Path:                    path,
				OperationID:             fmtString(rawOperation["operationId"]),
				Parameters:              operationParameters(pathItem["parameters"], rawOperation["parameters"]),
				RequestBodyPresent:      hasRequestBody(rawOperation["requestBody"]),
				RequestBodyRequired:     requestBodyRequired(rawOperation["requestBody"]),
				RequestBodyContentTypes: requestBodyContentTypes(rawOperation["requestBody"]),
				ResponseStatuses:        responseStatuses(rawOperation["responses"]),
				ResponseContentTypes:    responseContentTypes(rawOperation["responses"]),
				Security:                securityRequirements(rawOperation["security"]),
			})
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Path == operations[j].Path {
			return operations[i].Method < operations[j].Method
		}
		return operations[i].Path < operations[j].Path
	})
	return operations, nil
}

const (
	httpMethodGet    = "get"
	httpMethodPost   = "post"
	httpMethodPut    = "put"
	httpMethodPatch  = "patch"
	httpMethodDelete = "delete"
)

func responseStatuses(raw any) []string {
	responses, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	statuses := make([]string, 0, len(responses))
	for status := range responses {
		status = strings.TrimSpace(status)
		if status != "" {
			statuses = append(statuses, status)
		}
	}
	sort.Strings(statuses)
	return statuses
}

type openAPIParameterSnapshot struct {
	Name     string
	In       string
	Required bool
}

func (parameter openAPIParameterSnapshot) key() string {
	return parameterLabel(parameter.In, parameter.Name)
}

func parameterCompatibilityFindings(baseOperation, headOperation openAPIOperationSnapshot) []string {
	headByKey := make(map[string]openAPIParameterSnapshot, len(headOperation.Parameters))
	for _, parameter := range headOperation.Parameters {
		headByKey[parameter.key()] = parameter
	}
	baseByKey := make(map[string]openAPIParameterSnapshot, len(baseOperation.Parameters))
	var findings []string
	for _, baseParameter := range baseOperation.Parameters {
		baseByKey[baseParameter.key()] = baseParameter
		headParameter, ok := headByKey[baseParameter.key()]
		if !ok {
			findings = append(findings, fmt.Sprintf("parameter_removed %s %s %s", baseOperation.Method, baseOperation.Path, baseParameter.key()))
			continue
		}
		if !baseParameter.Required && headParameter.Required {
			findings = append(findings, fmt.Sprintf("parameter_required_added %s %s %s", baseOperation.Method, baseOperation.Path, baseParameter.key()))
		}
	}
	for _, headParameter := range headOperation.Parameters {
		if _, ok := baseByKey[headParameter.key()]; !ok && headParameter.Required {
			findings = append(findings, fmt.Sprintf("required_parameter_added %s %s %s", baseOperation.Method, baseOperation.Path, headParameter.key()))
		}
	}
	return findings
}

func operationParameters(rawPathParameters, rawOperationParameters any) []openAPIParameterSnapshot {
	return mergeParameterSnapshots(parameterSnapshots(rawPathParameters), parameterSnapshots(rawOperationParameters))
}

func parameterSnapshots(raw any) []openAPIParameterSnapshot {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]openAPIParameterSnapshot, 0, len(values))
	for _, value := range values {
		parameter, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name := fmtString(parameter["name"])
		in := strings.ToLower(fmtString(parameter["in"]))
		if name == "" || in == "" {
			continue
		}
		required, _ := parameter["required"].(bool)
		out = append(out, openAPIParameterSnapshot{Name: name, In: in, Required: required})
	}
	return out
}

func mergeParameterSnapshots(groups ...[]openAPIParameterSnapshot) []openAPIParameterSnapshot {
	indexByKey := map[string]int{}
	var out []openAPIParameterSnapshot
	for _, group := range groups {
		for _, parameter := range group {
			key := parameter.key()
			if index, ok := indexByKey[key]; ok {
				out[index] = parameter
				continue
			}
			indexByKey[key] = len(out)
			out = append(out, parameter)
		}
	}
	return out
}

func parameterLabel(in, name string) string {
	return strings.ToLower(strings.TrimSpace(in)) + ":" + strings.TrimSpace(name)
}

func responseContentTypes(raw any) map[string][]string {
	responses, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string][]string, len(responses))
	for status, rawResponse := range responses {
		status = strings.TrimSpace(status)
		response, ok := rawResponse.(map[string]any)
		if status == "" || !ok {
			continue
		}
		content, ok := response["content"].(map[string]any)
		if !ok {
			continue
		}
		out[status] = sortedMapKeys(content)
	}
	return out
}

func hasRequestBody(raw any) bool {
	_, ok := raw.(map[string]any)
	return ok
}

func requestBodyRequired(raw any) bool {
	requestBody, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	required, _ := requestBody["required"].(bool)
	return required
}

func requestBodyContentTypes(raw any) []string {
	requestBody, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		return nil
	}
	contentTypes := make([]string, 0, len(content))
	for contentType := range content {
		contentType = strings.TrimSpace(contentType)
		if contentType != "" {
			contentTypes = append(contentTypes, contentType)
		}
	}
	sort.Strings(contentTypes)
	return contentTypes
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func securityRequirements(raw any) []string {
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, entry := range entries {
		requirement, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for scheme, rawScopes := range requirement {
			out = append(out, strings.TrimSpace(scheme)+":"+strings.Join(securityScopes(rawScopes), ","))
		}
	}
	sort.Strings(out)
	return out
}

func securityScopes(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	scopes := make([]string, 0, len(values))
	for _, value := range values {
		scope := fmtString(value)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	sort.Strings(scopes)
	return scopes
}

func containsValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sortedNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func boolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func fmtString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
