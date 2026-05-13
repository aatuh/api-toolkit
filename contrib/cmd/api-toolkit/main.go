package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/aatuh/api-toolkit/v2/routepolicy"
	"github.com/aatuh/api-toolkit/v2/specs"
)

const toolVersion = "dev"

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: api-toolkit <new|contracts|version>")
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "api-toolkit %s\n", toolVersion)
		return 0
	case "new":
		return runNew(ctx, args[1:], stdout, stderr)
	case "contracts":
		return runContracts(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func runNew(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "service" {
		fmt.Fprintln(stderr, "usage: api-toolkit new service --module <module> [--dir <path>] [--profile saas-api]")
		return 2
	}
	fs := flag.NewFlagSet("new service", flag.ContinueOnError)
	fs.SetOutput(stderr)
	module := fs.String("module", "", "Go module path")
	profile := fs.String("profile", "saas-api", "service profile")
	dir := fs.String("dir", ".", "output directory")
	coreReplace := fs.String("core-replace", "", "optional local replace path for github.com/aatuh/api-toolkit/v2")
	contribReplace := fs.String("contrib-replace", "", "optional local replace path for github.com/aatuh/api-toolkit/contrib/v2")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if err := ctx.Err(); err != nil {
		fmt.Fprintf(stderr, "context canceled: %v\n", err)
		return 1
	}
	if strings.TrimSpace(*profile) != "saas-api" {
		fmt.Fprintf(stderr, "unsupported profile %q\n", *profile)
		return 2
	}
	cfg := scaffoldConfig{
		Module:         strings.TrimSpace(*module),
		Dir:            strings.TrimSpace(*dir),
		CoreReplace:    strings.TrimSpace(*coreReplace),
		ContribReplace: strings.TrimSpace(*contribReplace),
	}
	if err := generateService(cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "created %s\n", cfg.Dir)
	return 0
}

func runContracts(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: api-toolkit contracts <lint|diff>")
		return 2
	}
	switch args[0] {
	case "lint":
		fs := flag.NewFlagSet("contracts lint", flag.ContinueOnError)
		fs.SetOutput(stderr)
		openAPIPath := fs.String("openapi", "", "OpenAPI JSON file")
		publicPaths := newStringListFlag(defaultContractLintPublicPaths())
		adminPaths := newStringListFlag(defaultContractLintAdminPaths())
		fs.Var(&publicPaths, "public-path", "additional public path allowed without security metadata; repeatable")
		fs.Var(&adminPaths, "admin-path", "additional operator-only path that must declare admin policy metadata; repeatable")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := ctx.Err(); err != nil {
			fmt.Fprintf(stderr, "context canceled: %v\n", err)
			return 1
		}
		loaded, err := loadOpenAPI(*openAPIPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		operations := operationsFromOpenAPIDocument(loaded.doc)
		findings := routepolicy.LintOperations(operations, routepolicy.LintOptions{
			RequireOperationID:                true,
			RequireUniqueOperationID:          true,
			RequireSecurity:                   true,
			RequireProblemResponse:            true,
			RequireUnsafeWriteAuth:            true,
			RequireUnsafeWriteTenant:          true,
			RequireUnsafeWriteIdempotency:     true,
			RequireUnsafeWriteRateLimit:       true,
			RequireUnsafeWriteRequestBody:     true,
			RequireUnsafeWriteSuccessResponse: true,
			RequireUnsafeWriteProblemResponse: true,
			PublicPaths:                       publicPaths.Values(),
			AdminPaths:                        adminPaths.Values(),
		})
		if len(findings) > 0 {
			for _, finding := range findings {
				fmt.Fprintln(stderr, finding.Error())
			}
			return 1
		}
		if err := loaded.validate(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "contracts lint passed")
		return 0
	case "diff":
		fs := flag.NewFlagSet("contracts diff", flag.ContinueOnError)
		fs.SetOutput(stderr)
		base := fs.String("base", "", "base OpenAPI JSON file")
		head := fs.String("head", "", "head OpenAPI JSON file")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := diffOpenAPI(*base, *head); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "contracts diff passed")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown contracts command %q\n", args[0])
		return 2
	}
}

type stringListFlag struct {
	values []string
}

func newStringListFlag(defaults []string) stringListFlag {
	return stringListFlag{values: append([]string(nil), defaults...)}
}

func (f *stringListFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(f.values, ",")
}

func (f *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("path must not be empty")
	}
	f.values = append(f.values, value)
	return nil
}

func (f stringListFlag) Values() []string {
	return append([]string(nil), f.values...)
}

func defaultContractLintPublicPaths() []string {
	return []string{
		specs.Livez,
		specs.Readyz,
		specs.Healthz,
		specs.Health,
		specs.Docs,
		specs.Docs + "/*",
		specs.DocsOpenAPI,
		specs.DocsVersion,
		specs.DocsInfo,
		specs.Version,
	}
}

func defaultContractLintAdminPaths() []string {
	return []string{
		specs.PprofIndex,
		specs.PprofIndex + "*",
		specs.Metrics,
		specs.HealthDetailed,
	}
}

type scaffoldConfig struct {
	Module         string
	Dir            string
	CoreReplace    string
	ContribReplace string
}

func generateService(cfg scaffoldConfig) error {
	if err := validateModulePath(cfg.Module); err != nil {
		return err
	}
	outDir, err := safeOutputDir(cfg.Dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("read output directory: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("output directory must be empty: %s", outDir)
	}
	root, err := os.OpenRoot(outDir)
	if err != nil {
		return fmt.Errorf("open output root: %w", err)
	}
	defer root.Close()
	data := map[string]string{
		"Module":         cfg.Module,
		"CoreReplace":    replaceLine("github.com/aatuh/api-toolkit/v2", cfg.CoreReplace),
		"ContribReplace": replaceLine("github.com/aatuh/api-toolkit/contrib/v2", cfg.ContribReplace),
	}
	for _, file := range scaffoldFiles {
		rendered, err := renderTemplate(file.Name, file.Body, data)
		if err != nil {
			return err
		}
		if err := writeGeneratedFile(root, file.Name, rendered); err != nil {
			return err
		}
	}
	golden, err := renderSaaSAPIOpenAPIGolden()
	if err != nil {
		return err
	}
	if err := writeGeneratedFile(root, "testdata/openapi.golden.json", golden); err != nil {
		return err
	}
	return nil
}

func validateModulePath(module string) error {
	if module == "" {
		return errors.New("module is required")
	}
	if strings.ContainsAny(module, " \t\r\n") || strings.HasPrefix(module, ".") || strings.Contains(module, "..") {
		return fmt.Errorf("invalid module path %q", module)
	}
	if !strings.Contains(module, ".") || strings.ContainsAny(module, `\`) {
		return fmt.Errorf("invalid module path %q", module)
	}
	return nil
}

func safeOutputDir(raw string) (string, error) {
	if raw == "" {
		raw = "."
	}
	cleaned := filepath.Clean(raw)
	if !filepath.IsAbs(cleaned) {
		if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return "", errors.New("output directory must stay under the current working directory")
		}
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		cleaned = filepath.Join(wd, cleaned)
	}
	return cleaned, nil
}

func writeGeneratedFile(root *os.Root, name string, data []byte) error {
	if root == nil {
		return errors.New("output root is required")
	}
	clean := filepath.Clean(name)
	if clean != name || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return fmt.Errorf("unsafe generated path %q", name)
	}
	if parent := filepath.Dir(clean); parent != "." {
		if err := root.MkdirAll(parent, 0o750); err != nil {
			return fmt.Errorf("create parent for %s: %w", name, err)
		}
	}
	file, err := root.OpenFile(clean, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func renderTemplate(name, body string, data map[string]string) ([]byte, error) {
	tmpl, err := template.New(name).Parse(body)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

type scaffoldWidgetResponse struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
}

func renderSaaSAPIOpenAPIGolden() ([]byte, error) {
	registry := specs.NewRegistry(specs.Info{Title: "SaaS API", Version: "dev"})
	registry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	if err := specs.RegisterSchemaFrom[scaffoldWidgetResponse](registry, "Widget", specs.SchemaOptions{}); err != nil {
		return nil, fmt.Errorf("register scaffold schema: %w", err)
	}
	specs.RegisterProblemCatalog(registry, nil)
	registry.Register(routepolicy.ApplyMetadata(specs.Operation{
		Method:  http.MethodPost,
		Path:    "/widgets",
		Summary: "Create widget",
		RequestBody: &specs.RequestBody{
			Required: true,
			Content: map[string]specs.MediaType{
				"application/json": {Schema: map[string]any{"type": "object"}},
			},
		},
		Responses: map[int]specs.Response{
			http.StatusCreated: {
				Description: "Widget created",
				Content: map[string]specs.MediaType{
					"application/json": {SchemaRef: "#/components/schemas/Widget"},
				},
			},
		},
	},
		routepolicy.WithOperationID("createWidget"),
		routepolicy.WithAuth("ApiKeyAuth", "widgets:write"),
		routepolicy.WithTenantRequired("header"),
		routepolicy.WithIdempotencyRequired(),
		routepolicy.WithRateLimit("write-standard"),
		routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict),
	))
	doc, err := registry.OpenAPI()
	if err != nil {
		return nil, fmt.Errorf("render scaffold openapi: %w", err)
	}
	return normalizeJSON(doc)
}

func normalizeJSON(data []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func replaceLine(module, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return fmt.Sprintf("replace %s => %s\n", module, filepath.Clean(path))
}

type loadedOpenAPI struct {
	doc    *openapi3.T
	loader *openapi3.Loader
}

func loadOpenAPI(path string) (loadedOpenAPI, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return loadedOpenAPI{}, errors.New("--openapi is required")
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return loadedOpenAPI{}, fmt.Errorf("load openapi: %w", err)
	}
	return loadedOpenAPI{doc: doc, loader: loader}, nil
}

func (loaded loadedOpenAPI) validate() error {
	if loaded.doc == nil || loaded.loader == nil {
		return errors.New("load openapi: document is nil")
	}
	if err := loaded.doc.Validate(loaded.loader.Context); err != nil {
		return fmt.Errorf("validate openapi: %w", err)
	}
	return nil
}

func operationsFromOpenAPIDocument(doc *openapi3.T) []specs.Operation {
	if doc == nil || doc.Paths == nil {
		return nil
	}
	var operations []specs.Operation
	for routePath, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			operation := specs.Operation{
				OperationID: op.OperationID,
				Method:      strings.ToUpper(method),
				Path:        routePath,
				Deprecated:  op.Deprecated,
				Responses:   map[int]specs.Response{},
				Extensions:  map[string]any{},
			}
			operation.Parameters = mergeParameters(parametersFromOpenAPI(item.Parameters), parametersFromOpenAPI(op.Parameters))
			if op.RequestBody != nil {
				operation.RequestBody = &specs.RequestBody{}
				if op.RequestBody.Ref != "" {
					operation.RequestBody.Description = op.RequestBody.Ref
				}
				if op.RequestBody.Value != nil {
					operation.RequestBody.Description = op.RequestBody.Value.Description
					operation.RequestBody.Required = op.RequestBody.Value.Required
					operation.RequestBody.Content = map[string]specs.MediaType{}
					for contentType := range op.RequestBody.Value.Content {
						operation.RequestBody.Content[contentType] = specs.MediaType{}
					}
				}
			}
			if op.Security != nil && len(*op.Security) > 0 {
				for _, req := range *op.Security {
					for name, scopes := range req {
						operation.Security = append(operation.Security, specs.SecurityRequirement{Name: name, Scopes: scopes})
					}
				}
			}
			for name, value := range op.Extensions {
				if strings.HasPrefix(name, "x-") {
					operation.Extensions[name] = value
				}
			}
			if sunset, ok := operation.Extensions["x-sunset"].(string); ok {
				operation.Sunset = strings.TrimSpace(sunset)
			}
			for status, responseRef := range op.Responses.Map() {
				if status == "default" {
					continue
				}
				code := 0
				if _, err := fmt.Sscanf(status, "%d", &code); err != nil || code == 0 {
					continue
				}
				response := specs.Response{}
				if responseRef != nil && responseRef.Ref != "" {
					response.Ref = responseRef.Ref
				}
				if responseRef != nil && responseRef.Value != nil {
					response.Description = stringPtrValue(responseRef.Value.Description)
					response.Content = map[string]specs.MediaType{}
					for contentType := range responseRef.Value.Content {
						response.Content[contentType] = specs.MediaType{}
					}
				}
				operation.Responses[code] = response
			}
			operations = append(operations, operation)
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Path == operations[j].Path {
			return operations[i].Method < operations[j].Method
		}
		return operations[i].Path < operations[j].Path
	})
	return operations
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func diffOpenAPI(basePath, headPath string) error {
	baseLoaded, err := loadOpenAPI(basePath)
	if err != nil {
		return fmt.Errorf("base: %w", err)
	}
	if err := baseLoaded.validate(); err != nil {
		return fmt.Errorf("base: %w", err)
	}
	headLoaded, err := loadOpenAPI(headPath)
	if err != nil {
		return fmt.Errorf("head: %w", err)
	}
	if err := headLoaded.validate(); err != nil {
		return fmt.Errorf("head: %w", err)
	}
	base := operationsFromOpenAPIDocument(baseLoaded.doc)
	head := operationsFromOpenAPIDocument(headLoaded.doc)
	findings := diffOperations(base, head)
	findings = append(findings, diffOperationSchemas(baseLoaded.doc, headLoaded.doc)...)
	findings = append(findings, diffComponentSchemas(baseLoaded.doc, headLoaded.doc)...)
	if len(findings) > 0 {
		return openAPIDiffError{Findings: findings}
	}
	return nil
}

type openAPIDiffFinding struct {
	Code   string
	Method string
	Path   string
	Detail string
}

func (f openAPIDiffFinding) String() string {
	parts := []string{f.Code}
	if f.Method != "" {
		parts = append(parts, f.Method)
	}
	if f.Path != "" {
		parts = append(parts, f.Path)
	}
	if f.Detail != "" {
		parts = append(parts, f.Detail)
	}
	return strings.Join(parts, " ")
}

type openAPIDiffError struct {
	Findings []openAPIDiffFinding
}

func (e openAPIDiffError) Error() string {
	lines := make([]string, 0, len(e.Findings)+1)
	lines = append(lines, "breaking OpenAPI changes:")
	for _, finding := range e.Findings {
		lines = append(lines, "- "+finding.String())
	}
	return strings.Join(lines, "\n")
}

func diffOperations(base, head []specs.Operation) []openAPIDiffFinding {
	headByKey := make(map[string]specs.Operation, len(head))
	for _, operation := range head {
		headByKey[operationKey(operation.Method, operation.Path)] = operation
	}
	var findings []openAPIDiffFinding
	for _, baseOperation := range base {
		headOperation, ok := headByKey[operationKey(baseOperation.Method, baseOperation.Path)]
		if !ok {
			findings = append(findings, openAPIDiffFinding{
				Code:   "operation_removed",
				Method: baseOperation.Method,
				Path:   baseOperation.Path,
			})
			continue
		}
		if strings.TrimSpace(baseOperation.OperationID) != "" && strings.TrimSpace(baseOperation.OperationID) != strings.TrimSpace(headOperation.OperationID) {
			findings = append(findings, openAPIDiffFinding{
				Code:   "operation_id_changed",
				Method: baseOperation.Method,
				Path:   baseOperation.Path,
				Detail: fmt.Sprintf("%q -> %q", strings.TrimSpace(baseOperation.OperationID), strings.TrimSpace(headOperation.OperationID)),
			})
		}
		findings = append(findings, diffParameters(baseOperation, headOperation)...)
		for _, status := range sortedResponseStatuses(baseOperation.Responses) {
			headResponse, ok := headOperation.Responses[status]
			if !ok {
				findings = append(findings, openAPIDiffFinding{
					Code:   "response_removed",
					Method: baseOperation.Method,
					Path:   baseOperation.Path,
					Detail: fmt.Sprintf("%d", status),
				})
				continue
			}
			findings = append(findings, diffResponseContent(baseOperation, status, baseOperation.Responses[status], headResponse)...)
		}
		findings = append(findings, diffRequestBody(baseOperation, headOperation)...)
		baseSecurity := securityFingerprint(baseOperation.Security)
		headSecurity := securityFingerprint(headOperation.Security)
		if baseSecurity != headSecurity {
			findings = append(findings, openAPIDiffFinding{
				Code:   "security_changed",
				Method: baseOperation.Method,
				Path:   baseOperation.Path,
				Detail: fmt.Sprintf("%q -> %q", baseSecurity, headSecurity),
			})
		}
		findings = append(findings, diffRoutePolicies(baseOperation, headOperation)...)
	}
	return findings
}

func diffParameters(baseOperation, headOperation specs.Operation) []openAPIDiffFinding {
	headByKey := make(map[string]specs.Parameter, len(headOperation.Parameters))
	for _, parameter := range headOperation.Parameters {
		if key := parameterKey(parameter); key != "" {
			headByKey[key] = parameter
		}
	}
	baseByKey := make(map[string]specs.Parameter, len(baseOperation.Parameters))
	var findings []openAPIDiffFinding
	for _, baseParameter := range baseOperation.Parameters {
		key := parameterKey(baseParameter)
		if key == "" {
			continue
		}
		baseByKey[key] = baseParameter
		headParameter, ok := headByKey[key]
		if !ok {
			findings = append(findings, openAPIDiffFinding{
				Code:   "parameter_removed",
				Method: baseOperation.Method,
				Path:   baseOperation.Path,
				Detail: key,
			})
			continue
		}
		if !baseParameter.Required && headParameter.Required {
			findings = append(findings, openAPIDiffFinding{
				Code:   "parameter_required_added",
				Method: baseOperation.Method,
				Path:   baseOperation.Path,
				Detail: key,
			})
		}
	}
	for _, headParameter := range headOperation.Parameters {
		key := parameterKey(headParameter)
		if key == "" {
			continue
		}
		if _, ok := baseByKey[key]; !ok && headParameter.Required {
			findings = append(findings, openAPIDiffFinding{
				Code:   "required_parameter_added",
				Method: baseOperation.Method,
				Path:   baseOperation.Path,
				Detail: key,
			})
		}
	}
	return findings
}

func diffResponseContent(baseOperation specs.Operation, status int, baseResponse, headResponse specs.Response) []openAPIDiffFinding {
	var findings []openAPIDiffFinding
	headContentTypes := stringSet(responseContentTypes(headResponse))
	for _, contentType := range responseContentTypes(baseResponse) {
		if _, ok := headContentTypes[contentType]; !ok {
			findings = append(findings, openAPIDiffFinding{
				Code:   "response_content_removed",
				Method: baseOperation.Method,
				Path:   baseOperation.Path,
				Detail: fmt.Sprintf("%d %s", status, contentType),
			})
		}
	}
	return findings
}

func diffRequestBody(baseOperation, headOperation specs.Operation) []openAPIDiffFinding {
	var findings []openAPIDiffFinding
	baseBody := baseOperation.RequestBody
	headBody := headOperation.RequestBody
	if baseBody != nil && headBody == nil {
		findings = append(findings, openAPIDiffFinding{
			Code:   "request_body_removed",
			Method: baseOperation.Method,
			Path:   baseOperation.Path,
		})
		return findings
	}
	if !requestBodyRequired(baseBody) && requestBodyRequired(headBody) {
		findings = append(findings, openAPIDiffFinding{
			Code:   "request_body_required_added",
			Method: baseOperation.Method,
			Path:   baseOperation.Path,
		})
	}
	if baseBody == nil || headBody == nil {
		return findings
	}
	headContentTypes := stringSet(requestBodyContentTypes(headBody))
	for _, contentType := range requestBodyContentTypes(baseBody) {
		if _, ok := headContentTypes[contentType]; !ok {
			findings = append(findings, openAPIDiffFinding{
				Code:   "request_body_content_removed",
				Method: baseOperation.Method,
				Path:   baseOperation.Path,
				Detail: contentType,
			})
		}
	}
	return findings
}

func diffOperationSchemas(baseDoc, headDoc *openapi3.T) []openAPIDiffFinding {
	if baseDoc == nil || baseDoc.Paths == nil || headDoc == nil || headDoc.Paths == nil {
		return nil
	}
	var findings []openAPIDiffFinding
	for routePath, baseItem := range baseDoc.Paths.Map() {
		if baseItem == nil {
			continue
		}
		headItem := headDoc.Paths.Value(routePath)
		if headItem == nil {
			continue
		}
		headOperations := headItem.Operations()
		for method, baseOperation := range baseItem.Operations() {
			if baseOperation == nil {
				continue
			}
			headOperation := headOperations[method]
			if headOperation == nil {
				continue
			}
			method = strings.ToUpper(method)
			findings = append(findings, diffOperationRequestSchemas(method, routePath, baseOperation, headOperation)...)
			findings = append(findings, diffOperationResponseSchemas(method, routePath, baseOperation, headOperation)...)
		}
	}
	return findings
}

func diffOperationRequestSchemas(method, routePath string, baseOperation, headOperation *openapi3.Operation) []openAPIDiffFinding {
	baseBody := requestBodyValue(baseOperation.RequestBody)
	headBody := requestBodyValue(headOperation.RequestBody)
	if baseBody == nil || headBody == nil {
		return nil
	}
	var findings []openAPIDiffFinding
	headContent := headBody.Content
	for _, contentType := range sortedOpenAPIContentTypes(baseBody.Content) {
		baseMedia := baseBody.Content[contentType]
		headMedia := headContent[contentType]
		if baseMedia == nil || headMedia == nil {
			continue
		}
		schemaPath := "requestBody " + contentType
		findings = append(findings, diffOperationSchemaRef(method, routePath, schemaPath, baseMedia.Schema, headMedia.Schema)...)
	}
	return findings
}

func diffOperationResponseSchemas(method, routePath string, baseOperation, headOperation *openapi3.Operation) []openAPIDiffFinding {
	if baseOperation.Responses == nil || headOperation.Responses == nil {
		return nil
	}
	headResponses := headOperation.Responses.Map()
	var findings []openAPIDiffFinding
	for _, status := range sortedOpenAPIResponseStatuses(baseOperation.Responses.Map()) {
		baseResponse := responseValue(baseOperation.Responses.Map()[status])
		headResponse := responseValue(headResponses[status])
		if baseResponse == nil || headResponse == nil {
			continue
		}
		headContent := headResponse.Content
		for _, contentType := range sortedOpenAPIContentTypes(baseResponse.Content) {
			baseMedia := baseResponse.Content[contentType]
			headMedia := headContent[contentType]
			if baseMedia == nil || headMedia == nil {
				continue
			}
			schemaPath := fmt.Sprintf("response %s %s", status, contentType)
			findings = append(findings, diffOperationSchemaRef(method, routePath, schemaPath, baseMedia.Schema, headMedia.Schema)...)
		}
	}
	return findings
}

func diffOperationSchemaRef(method, routePath, schemaPath string, baseRef, headRef *openapi3.SchemaRef) []openAPIDiffFinding {
	findings := diffSchemaRef(schemaPath, baseRef, headRef)
	for i := range findings {
		findings[i].Method = method
		findings[i].Path = routePath
	}
	return findings
}

func requestBodyValue(ref *openapi3.RequestBodyRef) *openapi3.RequestBody {
	if ref == nil {
		return nil
	}
	return ref.Value
}

func responseValue(ref *openapi3.ResponseRef) *openapi3.Response {
	if ref == nil {
		return nil
	}
	return ref.Value
}

func sortedOpenAPIContentTypes(content openapi3.Content) []string {
	types := make([]string, 0, len(content))
	for contentType := range content {
		contentType = strings.TrimSpace(contentType)
		if contentType != "" {
			types = append(types, contentType)
		}
	}
	sort.Strings(types)
	return types
}

func sortedOpenAPIResponseStatuses(responses map[string]*openapi3.ResponseRef) []string {
	statuses := make([]string, 0, len(responses))
	for status := range responses {
		status = strings.TrimSpace(status)
		if status != "" && status != "default" {
			statuses = append(statuses, status)
		}
	}
	sort.Strings(statuses)
	return statuses
}

func diffRoutePolicies(baseOperation, headOperation specs.Operation) []openAPIDiffFinding {
	checks := []struct {
		code string
		base string
		head string
	}{
		{code: "tenant_policy_changed", base: tenantPolicyFingerprint(baseOperation), head: tenantPolicyFingerprint(headOperation)},
		{code: "idempotency_policy_changed", base: idempotencyPolicyFingerprint(baseOperation), head: idempotencyPolicyFingerprint(headOperation)},
		{code: "rate_limit_policy_changed", base: rateLimitPolicyFingerprint(baseOperation), head: rateLimitPolicyFingerprint(headOperation)},
		{code: "admin_policy_changed", base: adminPolicyFingerprint(baseOperation), head: adminPolicyFingerprint(headOperation)},
		{code: "deprecation_policy_changed", base: deprecationPolicyFingerprint(baseOperation), head: deprecationPolicyFingerprint(headOperation)},
	}
	var findings []openAPIDiffFinding
	for _, check := range checks {
		if check.base == check.head {
			continue
		}
		findings = append(findings, openAPIDiffFinding{
			Code:   check.code,
			Method: baseOperation.Method,
			Path:   baseOperation.Path,
			Detail: fmt.Sprintf("%q -> %q", check.base, check.head),
		})
	}
	return findings
}

func tenantPolicyFingerprint(operation specs.Operation) string {
	policy, ok := routepolicy.TenantPolicyFromOperation(operation)
	if !ok {
		return ""
	}
	return fmt.Sprintf("required=%t;source=%s", policy.Required, strings.TrimSpace(policy.Source))
}

func idempotencyPolicyFingerprint(operation specs.Operation) string {
	policy, ok := routepolicy.IdempotencyPolicyFromOperation(operation)
	if !ok {
		return ""
	}
	return fmt.Sprintf("required=%t;header=%s", policy.Required, strings.TrimSpace(policy.Header))
}

func rateLimitPolicyFingerprint(operation specs.Operation) string {
	policy, ok := routepolicy.RateLimitPolicyFromOperation(operation)
	if !ok {
		return ""
	}
	return policy
}

func adminPolicyFingerprint(operation specs.Operation) string {
	policy, ok := routepolicy.AdminPolicyFromOperation(operation)
	if !ok {
		return ""
	}
	return policy
}

func deprecationPolicyFingerprint(operation specs.Operation) string {
	policy, ok := routepolicy.DeprecationPolicyFromOperation(operation)
	if !ok {
		return ""
	}
	return fmt.Sprintf("deprecated=%t;sunset=%s", policy.Deprecated, strings.TrimSpace(policy.Sunset))
}

func diffComponentSchemas(baseDoc, headDoc *openapi3.T) []openAPIDiffFinding {
	baseSchemas := componentSchemas(baseDoc)
	headSchemas := componentSchemas(headDoc)
	var findings []openAPIDiffFinding
	for _, name := range sortedSchemaNames(baseSchemas) {
		baseSchema := baseSchemas[name]
		headSchema, ok := headSchemas[name]
		if !ok {
			findings = append(findings, openAPIDiffFinding{
				Code:   "schema_removed",
				Detail: name,
			})
			continue
		}
		findings = append(findings, diffSchemaRef(name, baseSchema, headSchema)...)
	}
	return findings
}

func componentSchemas(doc *openapi3.T) openapi3.Schemas {
	if doc == nil || doc.Components == nil {
		return nil
	}
	return doc.Components.Schemas
}

func sortedSchemaNames(schemas openapi3.Schemas) []string {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func diffSchemaRef(path string, baseRef, headRef *openapi3.SchemaRef) []openAPIDiffFinding {
	if baseRef == nil {
		return nil
	}
	if headRef == nil {
		return []openAPIDiffFinding{{
			Code:   "schema_removed",
			Detail: path,
		}}
	}
	baseRefName := strings.TrimSpace(baseRef.Ref)
	headRefName := strings.TrimSpace(headRef.Ref)
	if baseRefName != headRefName {
		return []openAPIDiffFinding{{
			Code:   "schema_ref_changed",
			Detail: fmt.Sprintf("%s %q -> %q", path, baseRefName, headRefName),
		}}
	}
	if baseRef.Value == nil || headRef.Value == nil {
		return nil
	}
	return diffSchemaValue(path, baseRef.Value, headRef.Value)
}

func diffSchemaValue(path string, baseSchema, headSchema *openapi3.Schema) []openAPIDiffFinding {
	var findings []openAPIDiffFinding
	baseType := schemaTypeFingerprint(baseSchema)
	headType := schemaTypeFingerprint(headSchema)
	if baseType != headType {
		findings = append(findings, openAPIDiffFinding{
			Code:   "schema_type_changed",
			Detail: fmt.Sprintf("%s %q -> %q", path, baseType, headType),
		})
	}
	for _, required := range sortedStringSetDiff(schemaRequiredSet(headSchema), schemaRequiredSet(baseSchema)) {
		findings = append(findings, openAPIDiffFinding{
			Code:   "schema_required_property_added",
			Detail: path + " " + required,
		})
	}
	for _, enumValue := range sortedStringSetDiff(schemaEnumSet(baseSchema), schemaEnumSet(headSchema)) {
		findings = append(findings, openAPIDiffFinding{
			Code:   "schema_enum_value_removed",
			Detail: path + " " + enumValue,
		})
	}
	headProperties := headSchema.Properties
	for _, property := range sortedSchemaNames(baseSchema.Properties) {
		baseProperty := baseSchema.Properties[property]
		headProperty, ok := headProperties[property]
		propertyPath := path + "." + property
		if !ok {
			findings = append(findings, openAPIDiffFinding{
				Code:   "schema_property_removed",
				Detail: path + " " + property,
			})
			continue
		}
		findings = append(findings, diffSchemaRef(propertyPath, baseProperty, headProperty)...)
	}
	return findings
}

func schemaTypeFingerprint(schema *openapi3.Schema) string {
	if schema == nil || schema.Type == nil {
		return ""
	}
	types := append([]string(nil), schema.Type.Slice()...)
	sort.Strings(types)
	return strings.Join(types, "|")
}

func schemaRequiredSet(schema *openapi3.Schema) map[string]struct{} {
	out := map[string]struct{}{}
	if schema == nil {
		return out
	}
	for _, required := range schema.Required {
		required = strings.TrimSpace(required)
		if required != "" {
			out[required] = struct{}{}
		}
	}
	return out
}

func schemaEnumSet(schema *openapi3.Schema) map[string]struct{} {
	out := map[string]struct{}{}
	if schema == nil {
		return out
	}
	for _, value := range schema.Enum {
		encoded, err := json.Marshal(value)
		if err != nil {
			encoded = []byte(fmt.Sprint(value))
		}
		out[string(encoded)] = struct{}{}
	}
	return out
}

func sortedStringSetDiff(left, right map[string]struct{}) []string {
	var out []string
	for value := range left {
		if _, ok := right[value]; !ok {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func operationKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
}

func sortedResponseStatuses(responses map[int]specs.Response) []int {
	statuses := make([]int, 0, len(responses))
	for status := range responses {
		statuses = append(statuses, status)
	}
	sort.Ints(statuses)
	return statuses
}

func parametersFromOpenAPI(parameters openapi3.Parameters) []specs.Parameter {
	out := make([]specs.Parameter, 0, len(parameters))
	for _, parameterRef := range parameters {
		if parameterRef == nil || parameterRef.Value == nil {
			continue
		}
		parameter := parameterRef.Value
		name := strings.TrimSpace(parameter.Name)
		in := strings.ToLower(strings.TrimSpace(parameter.In))
		if name == "" || in == "" {
			continue
		}
		out = append(out, specs.Parameter{
			Name:        name,
			In:          in,
			Description: parameter.Description,
			Required:    parameter.Required,
		})
	}
	return out
}

func mergeParameters(groups ...[]specs.Parameter) []specs.Parameter {
	indexByKey := map[string]int{}
	var out []specs.Parameter
	for _, group := range groups {
		for _, parameter := range group {
			key := parameterKey(parameter)
			if key == "" {
				continue
			}
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

func parameterKey(parameter specs.Parameter) string {
	name := strings.TrimSpace(parameter.Name)
	in := strings.ToLower(strings.TrimSpace(parameter.In))
	if name == "" || in == "" {
		return ""
	}
	return in + ":" + name
}

func responseContentTypes(response specs.Response) []string {
	seen := map[string]struct{}{}
	for contentType := range response.Content {
		contentType = strings.TrimSpace(contentType)
		if contentType != "" {
			seen[contentType] = struct{}{}
		}
	}
	for _, contentType := range response.ContentTypes {
		contentType = strings.TrimSpace(contentType)
		if contentType != "" {
			seen[contentType] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for contentType := range seen {
		out = append(out, contentType)
	}
	sort.Strings(out)
	return out
}

func requestBodyRequired(body *specs.RequestBody) bool {
	return body != nil && body.Required
}

func requestBodyContentTypes(body *specs.RequestBody) []string {
	if body == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for contentType := range body.Content {
		contentType = strings.TrimSpace(contentType)
		if contentType != "" {
			seen[contentType] = struct{}{}
		}
	}
	for _, contentType := range body.ContentTypes {
		contentType = strings.TrimSpace(contentType)
		if contentType != "" {
			seen[contentType] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for contentType := range seen {
		out = append(out, contentType)
	}
	sort.Strings(out)
	return out
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func securityFingerprint(requirements []specs.SecurityRequirement) string {
	if len(requirements) == 0 {
		return ""
	}
	parts := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		scopes := append([]string(nil), requirement.Scopes...)
		sort.Strings(scopes)
		parts = append(parts, requirement.Name+":"+strings.Join(scopes, ","))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

type scaffoldFile struct {
	Name string
	Body string
}

var scaffoldFiles = []scaffoldFile{
	{Name: "go.mod", Body: goModTemplate},
	{Name: "main.go", Body: mainGoTemplate},
	{Name: "main_test.go", Body: mainTestTemplate},
	{Name: "Makefile", Body: makefileTemplate},
	{Name: ".env.example", Body: envTemplate},
	{Name: ".gitignore", Body: gitignoreTemplate},
	{Name: ".dockerignore", Body: dockerignoreTemplate},
	{Name: "Dockerfile", Body: dockerfileTemplate},
	{Name: "docker-compose.yml", Body: composeTemplate},
	{Name: "README.md", Body: readmeTemplate},
}

const goModTemplate = `module {{ .Module }}

go 1.25.0

require (
	github.com/aatuh/api-toolkit/v2 v2.1.0
	github.com/aatuh/api-toolkit/contrib/v2 v2.1.0
	github.com/redis/go-redis/v9 v9.19.0
)

{{ .CoreReplace }}{{ .ContribReplace }}`

const mainGoTemplate = `package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/idempotency"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/idempotencyredis"
	"github.com/aatuh/api-toolkit/contrib/v2/bootstrap"
	"github.com/redis/go-redis/v9"
	"github.com/aatuh/api-toolkit/v2/authorization"
	"github.com/aatuh/api-toolkit/v2/binding"
	"github.com/aatuh/api-toolkit/v2/endpoints/docs"
	"github.com/aatuh/api-toolkit/v2/endpoints/health"
	"github.com/aatuh/api-toolkit/v2/endpoints/version"
	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/middleware/auth/apikey"
	"github.com/aatuh/api-toolkit/v2/middleware/auth/tenant"
	idempotencymw "github.com/aatuh/api-toolkit/v2/middleware/idempotency"
	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/routecontracts"
	"github.com/aatuh/api-toolkit/v2/routepolicy"
	"github.com/aatuh/api-toolkit/v2/specs"
)

type createWidgetRequest struct {
	Name string ` + "`json:\"name\" required:\"true\"`" + `
}

type widgetResponse struct {
	ID       string ` + "`json:\"id\"`" + `
	TenantID string ` + "`json:\"tenant_id\"`" + `
	Name     string ` + "`json:\"name\"`" + `
}

func main() {
	service, err := newService()
	if err != nil {
		panic(err)
	}
	if err := service.Start(context.Background()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func newService() (*bootstrap.APIService, error) {
	specRegistry := specs.NewRegistry(specs.Info{Title: "SaaS API", Version: "dev"})
	specRegistry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	if err := specs.RegisterSchemaFrom[widgetResponse](specRegistry, "Widget", specs.SchemaOptions{}); err != nil {
		return nil, err
	}
	specs.RegisterProblemCatalog(specRegistry, nil)

	docsManager := docs.NewWithConfig(ports.DocsConfig{
		Title:       "SaaS API",
		Description: "Generated api-toolkit service",
		Version:     "dev",
		Paths:       ports.DefaultDocsPaths(),
		EnableHTML:  true,
		EnableJSON:  true,
		HTMLMode:    ports.DocsHTMLModeStatic,
	})
	docsManager.RegisterProvider(specs.NewRegistryProvider(specRegistry, docsManager.GetInfo(), ""))

	healthManager := health.NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         5 * time.Second,
		CacheDuration:   5 * time.Second,
		EnableCaching:   true,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic"},
	})
	healthManager.RegisterChecker(health.NewBasicChecker())

	apiKeyMiddleware, err := newAPIKeyMiddleware()
	if err != nil {
		return nil, err
	}
	tenantMiddleware, err := tenant.New(tenant.Options{
		HeaderName:        "X-Tenant-ID",
		TenantFromContext: authorization.TenantIDFromContext,
		RequireAllSources: true,
	})
	if err != nil {
		return nil, err
	}
	idempotencyStore, err := newIdempotencyStore()
	if err != nil {
		return nil, err
	}
	idempotencyMiddleware, err := idempotencymw.New(idempotencymw.Options{
		Store: idempotencyStore,
	})
	if err != nil {
		return nil, err
	}
	adminKey, err := secretEnv("ADMIN_KEY", "local-admin-key")
	if err != nil {
		return nil, err
	}

	return bootstrap.NewAPIService(bootstrap.APIServiceConfig{
		Addr:                    env("API_ADDR", ":8080"),
		Log:                     ports.NopLogger{},
		MiddlewareOrder:         bootstrap.StrictSaaSAPIMiddlewareOrder(),
		RequiredMiddlewareOrder: bootstrap.StrictSaaSAPIMiddlewareOrder(),
		RegisterRoutes: func(r ports.HTTPRouter) error {
			contracts := routecontracts.NewRegistry(r, specRegistry)
			operation := routepolicy.ApplyMetadata(specs.Operation{
				Method:  http.MethodPost,
				Path:    "/widgets",
				Summary: "Create widget",
				RequestBody: &specs.RequestBody{
					Required: true,
					Content: map[string]specs.MediaType{
						"application/json": {Schema: map[string]any{"type": "object"}},
					},
				},
				Responses: map[int]specs.Response{
					http.StatusCreated: {
						Description: "Widget created",
						Content: map[string]specs.MediaType{
							"application/json": {SchemaRef: "#/components/schemas/Widget"},
						},
					},
				},
			},
				routepolicy.WithOperationID("createWidget"),
				routepolicy.WithAuth("ApiKeyAuth", "widgets:write"),
				routepolicy.WithTenantRequired("header"),
				routepolicy.WithIdempotencyRequired(),
				routepolicy.WithRateLimit("write-standard"),
				routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict),
			)
			widgetHandler := http.Handler(http.HandlerFunc(createWidget))
			widgetHandler = idempotencyMiddleware.Handler(widgetHandler)
			widgetHandler = apikey.RequireScopeMiddleware("widgets:write")(widgetHandler)
			widgetHandler = tenantMiddleware.Handler(widgetHandler)
			widgetHandler = apiKeyMiddleware.Handler(widgetHandler)
			if err := contracts.Post("/widgets", operation, widgetHandler); err != nil {
				return err
			}
			return contracts.Validate()
		},
		SystemEndpoints: bootstrap.SystemEndpoints{
			Health:  health.NewHandler(healthManager),
			Docs:    docs.NewHandler(docsManager),
			Version: version.NewHandler(version.Config{Info: ports.VersionInfo{Version: "dev"}}),
			Metrics: bootstrap.PrometheusMetricsHandler(),
			Pprof:   http.DefaultServeMux,
		},
		Admin: bootstrap.SystemEndpointAdminOptions{
			RequireAdmin: requireAdmin(adminKey),
			EnablePprof:  true,
		},
	})
}

func newAPIKeyMiddleware() (*apikey.Middleware, error) {
	expectedKey, err := secretEnv("API_KEY", "local-dev-key")
	if err != nil {
		return nil, err
	}
	tenantID := env("API_TENANT_ID", "tenant_1")
	return apikey.NewMiddleware(apikey.Config{
		HeaderNames: []string{"X-API-Key"},
		Verifier: apikey.VerifierFunc(func(_ context.Context, key apikey.PresentedKey) (apikey.Principal, error) {
			if key.Value != expectedKey {
				return apikey.Principal{}, errors.New("invalid api key")
			}
			return apikey.Principal{
				ID:       "local-api-key",
				TenantID: tenantID,
				Scopes:   []string{"widgets:write"},
			}, nil
		}),
	})
}

func newIdempotencyStore() (ports.IdempotencyStore, error) {
	store := strings.ToLower(strings.TrimSpace(os.Getenv("IDEMPOTENCY_STORE")))
	if store == "" {
		if isProduction() {
			store = "redis"
		} else {
			store = "memory"
		}
	}
	switch store {
	case "memory":
		if isProduction() {
			return nil, errors.New("IDEMPOTENCY_STORE=memory is not allowed when ENV=production; use redis")
		}
		return idempotency.NewMemoryStore(), nil
	case "redis":
		addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
		if addr == "" {
			if isProduction() {
				return nil, errors.New("REDIS_ADDR is required when IDEMPOTENCY_STORE=redis")
			}
			addr = "localhost:6379"
		}
		addrs := splitCSV(addr)
		if len(addrs) == 0 {
			return nil, errors.New("REDIS_ADDR must include at least one address")
		}
		client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
		return idempotencyredis.New(client, idempotencyredis.Options{
			KeyPrefix: env("IDEMPOTENCY_KEY_PREFIX", "idempotency:"),
		}), nil
	default:
		return nil, fmt.Errorf("unsupported IDEMPOTENCY_STORE %q", store)
	}
}

func createWidget(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := authorization.TenantIDFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "tenant scope required"})
		return
	}
	input, err := binding.DecodeJSON[createWidgetRequest](r, binding.JSONConfig{RequireObject: true})
	if err != nil {
		binding.WriteValidationProblem(w, err)
		return
	}
	if strings.TrimSpace(input.Name) == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeBadRequest), Title: http.StatusText(http.StatusBadRequest), Detail: "name is required"})
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, widgetResponse{ID: "w_123", TenantID: tenantID, Name: strings.TrimSpace(input.Name)})
}

func requireAdmin(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Key") != expectedKey {
				httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "admin authentication required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func secretEnv(key, fallback string) (string, error) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value, nil
	}
	if isProduction() {
		return "", fmt.Errorf("%s is required when ENV=production", key)
	}
	return fallback, nil
}

func isProduction() bool {
	return strings.EqualFold(env("ENV", "development"), "production")
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := parts[:0]
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
`

const mainTestTemplate = `package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/contracttest"
	"github.com/aatuh/api-toolkit/v2/specs"
)

var updateOpenAPI = flag.Bool("update-openapi", false, "rewrite testdata/openapi.golden.json")

func TestGeneratedServiceHealthAndOpenAPI(t *testing.T) {
	setLocalTestEnv(t)
	service, err := newService()
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	rec := httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, specs.Readyz, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, specs.DocsOpenAPI, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi status = %d body=%s", rec.Code, rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}
	operation := doc["paths"].(map[string]any)["/widgets"].(map[string]any)["post"].(map[string]any)
	if operation["operationId"] != "createWidget" {
		t.Fatalf("operationId = %v", operation["operationId"])
	}
}

func TestGeneratedServiceOpenAPIGolden(t *testing.T) {
	setLocalTestEnv(t)
	service, err := newService()
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	rec := httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, specs.DocsOpenAPI, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi status = %d body=%s", rec.Code, rec.Body.String())
	}
	goldenPath := filepath.Join("testdata", "openapi.golden.json")
	if *updateOpenAPI {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o750); err != nil {
			t.Fatalf("create golden directory: %v", err)
		}
		normalized, err := contracttest.NormalizeOpenAPI(rec.Body.Bytes())
		if err != nil {
			t.Fatalf("normalize openapi: %v", err)
		}
		if err := os.WriteFile(goldenPath, normalized, 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	contracttest.GoldenOpenAPI(t, rec.Body.Bytes(), golden)
}

func TestGeneratedServiceAuthValidationAndIdempotency(t *testing.T) {
	setLocalTestEnv(t)
	service, err := newService()
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"starter"}` + "`" + `))
	req.Header.Set("Content-Type", "application/json")
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"starter"}` + "`" + `))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "local-dev-key")
	req.Header.Set("X-Tenant-ID", "tenant_2")
	req.Header.Set("Idempotency-Key", "tenant-mismatch-key")
	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tenant mismatch status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tenant scope mismatch") {
		t.Fatalf("tenant mismatch body = %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"   "}` + "`" + `))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "local-dev-key")
	req.Header.Set("X-Tenant-ID", "tenant_1")
	req.Header.Set("Idempotency-Key", "validation-key")
	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("validation status = %d body=%s", rec.Code, rec.Body.String())
	}

	body := []byte(` + "`" + `{"name":"starter"}` + "`" + `)
	for i := 0; i < 2; i++ {
		req = httptest.NewRequest(http.MethodPost, "/widgets", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "local-dev-key")
		req.Header.Set("X-Tenant-ID", "tenant_1")
		req.Header.Set("Idempotency-Key", "create-key")
		rec = httptest.NewRecorder()
		service.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create status iteration %d = %d body=%s", i, rec.Code, rec.Body.String())
		}
	}
	if got := rec.Header().Get("Idempotency-Replayed"); got != "true" {
		t.Fatalf("replay header = %q", got)
	}
}

func TestGeneratedServiceProtectsOperatorRoutes(t *testing.T) {
	setLocalTestEnv(t)
	service, err := newService()
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	rec := httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, specs.Metrics, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("metrics without admin status = %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, specs.Metrics, nil)
	req.Header.Set("X-Admin-Key", "local-admin-key")
	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics with admin status = %d", rec.Code)
	}
}

func TestGeneratedServiceRejectsProductionDefaultSecrets(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("API_KEY", "")
	t.Setenv("ADMIN_KEY", "")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require explicit secrets")
	} else if !strings.Contains(err.Error(), "API_KEY") {
		t.Fatalf("startup error = %v, want API_KEY requirement", err)
	}
}

func TestGeneratedServiceRejectsProductionMemoryIdempotency(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("API_KEY", "prod-api-key")
	t.Setenv("ADMIN_KEY", "prod-admin-key")
	t.Setenv("API_TENANT_ID", "tenant_1")
	t.Setenv("IDEMPOTENCY_STORE", "memory")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to reject memory idempotency")
	} else if !strings.Contains(err.Error(), "IDEMPOTENCY_STORE=memory") {
		t.Fatalf("startup error = %v, want memory-store rejection", err)
	}
}

func TestGeneratedServiceRejectsProductionMissingRedisAddress(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("API_KEY", "prod-api-key")
	t.Setenv("ADMIN_KEY", "prod-admin-key")
	t.Setenv("API_TENANT_ID", "tenant_1")
	t.Setenv("IDEMPOTENCY_STORE", "redis")
	t.Setenv("REDIS_ADDR", "")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require Redis address")
	} else if !strings.Contains(err.Error(), "REDIS_ADDR") {
		t.Fatalf("startup error = %v, want REDIS_ADDR requirement", err)
	}
}

func setLocalTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENV", "development")
	t.Setenv("API_KEY", "local-dev-key")
	t.Setenv("API_TENANT_ID", "tenant_1")
	t.Setenv("ADMIN_KEY", "local-admin-key")
	t.Setenv("IDEMPOTENCY_STORE", "memory")
}
`

const makefileTemplate = `GO ?= go
API_TOOLKIT ?= $(GO) run -mod=mod github.com/aatuh/api-toolkit/contrib/v2/cmd/api-toolkit
OPENAPI ?= testdata/openapi.golden.json
OPENAPI_BASE ?= $(OPENAPI)

.PHONY: test fmt openapi-check openapi-update contracts-lint contracts-diff finalize

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

openapi-check:
	$(GO) test ./... -run TestGeneratedServiceOpenAPIGolden

openapi-update:
	$(GO) test ./... -run TestGeneratedServiceOpenAPIGolden -update-openapi

contracts-lint:
	$(API_TOOLKIT) contracts lint --openapi $(OPENAPI)

contracts-diff:
	$(API_TOOLKIT) contracts diff --base $(OPENAPI_BASE) --head $(OPENAPI)

finalize: fmt test openapi-check contracts-lint contracts-diff
`

const envTemplate = `ENV=development
API_ADDR=:8080
API_KEY=local-dev-key
API_TENANT_ID=tenant_1
ADMIN_KEY=local-admin-key
IDEMPOTENCY_STORE=memory
REDIS_ADDR=localhost:6379
IDEMPOTENCY_KEY_PREFIX=idempotency:
`

const gitignoreTemplate = `.env
.env.*
!.env.example
.ci-result/
coverage.out
bin/
tmp/
api
*.test
`

const dockerignoreTemplate = `.git
.env
.env.*
!.env.example
.ci-result
coverage.out
tmp/
bin/
`

const dockerfileTemplate = `FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
`

const composeTemplate = `services:
  api:
    build: .
    ports:
      - "8080:8080"
    env_file:
      - .env
`

const readmeTemplate = `# Generated api-toolkit Service

Run locally:

` + "```sh" + `
go test ./...
go run .
` + "```" + `

Build the production-style container image:

` + "```sh" + `
docker build -t my-api .
` + "```" + `

Refresh and check the OpenAPI golden:

` + "```sh" + `
make openapi-update
make openapi-check
` + "```" + `

Default routes:

- ` + "`GET /readyz`" + `
- ` + "`GET /docs/openapi.json`" + `
- ` + "`POST /widgets`" + ` with ` + "`X-API-Key`" + `, ` + "`X-Tenant-ID`" + `, and ` + "`Idempotency-Key`" + `
- ` + "`GET /metrics`" + ` with ` + "`X-Admin-Key`" + `

The default API key is scoped to ` + "`API_TENANT_ID`" + `, and write requests fail when ` + "`X-Tenant-ID`" + ` does not match that authenticated tenant.
When ` + "`ENV=production`" + `, startup requires explicit non-empty ` + "`API_KEY`" + ` and ` + "`ADMIN_KEY`" + ` values instead of local fallback keys.
Local development uses ` + "`IDEMPOTENCY_STORE=memory`" + `. In production, the generated service defaults to ` + "`IDEMPOTENCY_STORE=redis`" + ` and requires ` + "`REDIS_ADDR`" + ` so unsafe writes can be replayed across instances.
Local ` + "`.env`" + ` files, coverage output, temporary files, and built binaries are ignored by default.
`
