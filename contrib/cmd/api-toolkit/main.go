package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
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
		fmt.Fprintln(stderr, "usage: api-toolkit new service --module <module> [--dir <path>] [--profile saas-api|dev-api] [--auth api-key|jwt|clerk|dev-headers]")
		return 2
	}
	fs := flag.NewFlagSet("new service", flag.ContinueOnError)
	fs.SetOutput(stderr)
	module := fs.String("module", "", "Go module path")
	profile := fs.String("profile", "saas-api", "service profile")
	authMode := fs.String("auth", "api-key", "authentication mode")
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
	profileName := strings.TrimSpace(*profile)
	if !isSupportedScaffoldProfile(profileName) {
		fmt.Fprintf(stderr, "unsupported profile %q\n", *profile)
		return 2
	}
	authName, err := validateScaffoldAuthMode(profileName, *authMode)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cfg := scaffoldConfig{
		Module:         strings.TrimSpace(*module),
		Dir:            strings.TrimSpace(*dir),
		Profile:        profileName,
		AuthMode:       authName,
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

const (
	scaffoldProfileSaaSAPI = "saas-api"
	scaffoldProfileDevAPI  = "dev-api"
	scaffoldAuthAPIKey     = "api-key"
	scaffoldAuthJWT        = "jwt"
	scaffoldAuthClerk      = "clerk"
	scaffoldAuthDevHeaders = "dev-headers"
)

func isSupportedScaffoldProfile(profile string) bool {
	switch strings.TrimSpace(profile) {
	case scaffoldProfileSaaSAPI, scaffoldProfileDevAPI:
		return true
	default:
		return false
	}
}

func validateScaffoldAuthMode(profile, authMode string) (string, error) {
	profile = strings.TrimSpace(profile)
	authMode = strings.ToLower(strings.TrimSpace(authMode))
	if authMode == "" {
		if profile == scaffoldProfileDevAPI {
			authMode = scaffoldAuthDevHeaders
		} else {
			authMode = scaffoldAuthAPIKey
		}
	}
	switch authMode {
	case scaffoldAuthAPIKey, scaffoldAuthJWT, scaffoldAuthClerk:
		if profile == scaffoldProfileDevAPI {
			return "", fmt.Errorf("auth mode %q is not supported for profile %q", authMode, profile)
		}
		return authMode, nil
	case scaffoldAuthDevHeaders:
		if profile == scaffoldProfileDevAPI {
			return authMode, nil
		}
		return "", fmt.Errorf("auth mode %q requires an explicit development profile", authMode)
	default:
		return "", fmt.Errorf("unsupported auth mode %q", authMode)
	}
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
		securityFindings := lintUndefinedSecuritySchemes(loaded.doc, operations)
		if len(findings) > 0 || len(securityFindings) > 0 {
			for _, finding := range findings {
				fmt.Fprintln(stderr, finding.Error())
			}
			for _, finding := range securityFindings {
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
	Profile        string
	AuthMode       string
	CoreReplace    string
	ContribReplace string
}

func generateService(cfg scaffoldConfig) error {
	if err := validateModulePath(cfg.Module); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Profile) == "" {
		cfg.Profile = scaffoldProfileSaaSAPI
	}
	if !isSupportedScaffoldProfile(cfg.Profile) {
		return fmt.Errorf("unsupported profile %q", cfg.Profile)
	}
	if strings.TrimSpace(cfg.AuthMode) == "" {
		authMode, err := validateScaffoldAuthMode(cfg.Profile, "")
		if err != nil {
			return err
		}
		cfg.AuthMode = authMode
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
		"Profile":        cfg.Profile,
		"AuthMode":       cfg.AuthMode,
		"AuthSchemeName": scaffoldAuthSecuritySchemeName(cfg.AuthMode),
		"CoreReplace":    replaceLine("github.com/aatuh/api-toolkit/v2", cfg.CoreReplace),
		"ContribReplace": replaceLine("github.com/aatuh/api-toolkit/contrib/v2", cfg.ContribReplace),
	}
	for _, file := range scaffoldFiles {
		rendered, err := renderTemplate(file.Name, file.Body, data)
		if err != nil {
			return err
		}
		if strings.HasSuffix(file.Name, ".go") {
			formatted, err := format.Source(rendered)
			if err != nil {
				return fmt.Errorf("format %s: %w", file.Name, err)
			}
			rendered = formatted
		}
		if err := writeGeneratedFile(root, file.Name, rendered); err != nil {
			return err
		}
	}
	golden, err := renderSaaSAPIOpenAPIGolden(cfg.AuthMode)
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

func renderSaaSAPIOpenAPIGolden(authMode string) ([]byte, error) {
	registry := specs.NewRegistry(specs.Info{Title: "SaaS API", Version: "dev"})
	authSchemeName := scaffoldAuthSecuritySchemeName(authMode)
	registry.RegisterSecurityScheme(authSchemeName, scaffoldAuthSecurityScheme(authMode))
	registry.SetSecurity([]specs.SecurityRequirement{{Name: authSchemeName}})
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
		routepolicy.WithAuth(scaffoldAuthSecuritySchemeName(authMode), "widgets:write"),
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

func scaffoldAuthSecuritySchemeName(authMode string) string {
	if isScaffoldBearerAuth(authMode) {
		return "BearerAuth"
	}
	if authMode == scaffoldAuthDevHeaders {
		return "DevHeaderAuth"
	}
	return "ApiKeyAuth"
}

func scaffoldAuthSecurityScheme(authMode string) specs.SecurityScheme {
	if isScaffoldBearerAuth(authMode) {
		return specs.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"}
	}
	if authMode == scaffoldAuthDevHeaders {
		return specs.SecurityScheme{Type: "apiKey", Name: "X-Debug-User", In: "header"}
	}
	return specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"}
}

func isScaffoldBearerAuth(authMode string) bool {
	return authMode == scaffoldAuthJWT || authMode == scaffoldAuthClerk
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

type openAPILintFinding struct {
	Code    string
	Method  string
	Path    string
	Message string
}

func (f openAPILintFinding) Error() string {
	method := strings.ToUpper(strings.TrimSpace(f.Method))
	path := strings.TrimSpace(f.Path)
	if path == "" {
		return fmt.Sprintf("%s: %s: %s", method, f.Code, f.Message)
	}
	return fmt.Sprintf("%s %s: %s: %s", method, path, f.Code, f.Message)
}

func lintUndefinedSecuritySchemes(doc *openapi3.T, operations []specs.Operation) []openAPILintFinding {
	defined := definedSecuritySchemes(doc)
	var findings []openAPILintFinding
	if doc != nil {
		seenGlobal := map[string]struct{}{}
		for _, requirement := range doc.Security {
			for name := range requirement {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				if _, seen := seenGlobal[name]; seen {
					continue
				}
				seenGlobal[name] = struct{}{}
				if _, ok := defined[name]; ok {
					continue
				}
				findings = append(findings, openAPILintFinding{
					Code:    "security_scheme_undefined",
					Method:  "GLOBAL",
					Message: fmt.Sprintf("security scheme %q is referenced by top-level security but not defined in components.securitySchemes", name),
				})
			}
		}
	}
	for _, operation := range operations {
		for _, requirement := range operation.Security {
			name := strings.TrimSpace(requirement.Name)
			if name == "" {
				continue
			}
			if _, ok := defined[name]; ok {
				continue
			}
			findings = append(findings, openAPILintFinding{
				Code:    "security_scheme_undefined",
				Method:  operation.Method,
				Path:    operation.Path,
				Message: fmt.Sprintf("security scheme %q is referenced but not defined in components.securitySchemes", name),
			})
		}
	}
	return findings
}

func definedSecuritySchemes(doc *openapi3.T) map[string]struct{} {
	out := map[string]struct{}{}
	if doc == nil || doc.Components == nil {
		return out
	}
	for name := range doc.Components.SecuritySchemes {
		name = strings.TrimSpace(name)
		if name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func operationsFromOpenAPIDocument(doc *openapi3.T) []specs.Operation {
	if doc == nil || doc.Paths == nil {
		return nil
	}
	globalSecurity := securityRequirementsFromOpenAPI(doc.Security)
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
			operation.Security = effectiveOperationSecurity(globalSecurity, op.Security)
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
	findings = append(findings, diffSecuritySchemes(baseLoaded.doc, headLoaded.doc)...)
	findings = append(findings, diffGlobalSecurity(baseLoaded.doc, headLoaded.doc)...)
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

func diffGlobalSecurity(baseDoc, headDoc *openapi3.T) []openAPIDiffFinding {
	baseSecurity := securityFingerprint(securityRequirementsFromOpenAPI(baseDoc.Security))
	headSecurity := securityFingerprint(securityRequirementsFromOpenAPI(headDoc.Security))
	if baseSecurity == headSecurity {
		return nil
	}
	return []openAPIDiffFinding{{
		Code:   "global_security_changed",
		Detail: fmt.Sprintf("%q -> %q", baseSecurity, headSecurity),
	}}
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

func diffSecuritySchemes(baseDoc, headDoc *openapi3.T) []openAPIDiffFinding {
	baseSchemes := componentSecuritySchemes(baseDoc)
	headSchemes := componentSecuritySchemes(headDoc)
	var findings []openAPIDiffFinding
	for _, name := range sortedSecuritySchemeNames(baseSchemes) {
		baseScheme := baseSchemes[name]
		headScheme, ok := headSchemes[name]
		if !ok {
			findings = append(findings, openAPIDiffFinding{
				Code:   "security_scheme_removed",
				Detail: name,
			})
			continue
		}
		if securitySchemeRefFingerprint(baseScheme) != securitySchemeRefFingerprint(headScheme) {
			findings = append(findings, openAPIDiffFinding{
				Code:   "security_scheme_changed",
				Detail: name,
			})
		}
	}
	return findings
}

func componentSecuritySchemes(doc *openapi3.T) openapi3.SecuritySchemes {
	if doc == nil || doc.Components == nil {
		return nil
	}
	return doc.Components.SecuritySchemes
}

func sortedSecuritySchemeNames(schemes openapi3.SecuritySchemes) []string {
	names := make([]string, 0, len(schemes))
	for name := range schemes {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func securitySchemeRefFingerprint(ref *openapi3.SecuritySchemeRef) string {
	if ref == nil {
		return ""
	}
	if refName := strings.TrimSpace(ref.Ref); refName != "" {
		return "ref=" + refName
	}
	if ref.Value == nil {
		return ""
	}
	scheme := ref.Value
	payload := map[string]any{
		"type":             strings.TrimSpace(scheme.Type),
		"name":             strings.TrimSpace(scheme.Name),
		"in":               strings.TrimSpace(scheme.In),
		"scheme":           strings.TrimSpace(scheme.Scheme),
		"bearerFormat":     strings.TrimSpace(scheme.BearerFormat),
		"openIdConnectUrl": strings.TrimSpace(scheme.OpenIdConnectUrl),
	}
	if scheme.Flows != nil {
		payload["flows"] = scheme.Flows
	}
	if len(scheme.Extensions) > 0 {
		payload["extensions"] = scheme.Extensions
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprint(payload)
	}
	return string(encoded)
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

func effectiveOperationSecurity(global []specs.SecurityRequirement, operation *openapi3.SecurityRequirements) []specs.SecurityRequirement {
	if operation != nil {
		return securityRequirementsFromOpenAPI(*operation)
	}
	return cloneSecurityRequirements(global)
}

func securityRequirementsFromOpenAPI(requirements openapi3.SecurityRequirements) []specs.SecurityRequirement {
	var out []specs.SecurityRequirement
	for _, requirement := range requirements {
		for name, scopes := range requirement {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			out = append(out, specs.SecurityRequirement{
				Name:   name,
				Scopes: append([]string(nil), scopes...),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return strings.Join(out[i].Scopes, ",") < strings.Join(out[j].Scopes, ",")
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func cloneSecurityRequirements(requirements []specs.SecurityRequirement) []specs.SecurityRequirement {
	if len(requirements) == 0 {
		return nil
	}
	out := make([]specs.SecurityRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		out = append(out, specs.SecurityRequirement{
			Name:   requirement.Name,
			Scopes: append([]string(nil), requirement.Scopes...),
		})
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
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") }}	github.com/golang-jwt/jwt/v5 v5.3.0
{{ end }}	github.com/redis/go-redis/v9 v9.19.0
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
	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	requestlog "github.com/aatuh/api-toolkit/contrib/v2/middleware/requestlog"
{{ if eq .AuthMode "dev-headers" }}	"github.com/aatuh/api-toolkit/contrib/v2/config"
	"github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/devheaders"
{{ end }}
{{ if eq .AuthMode "clerk" }}	clerkauth "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/clerk"
{{ end }}	"github.com/redis/go-redis/v9"
	"github.com/aatuh/api-toolkit/v2/authorization"
	"github.com/aatuh/api-toolkit/v2/binding"
	"github.com/aatuh/api-toolkit/v2/endpoints/docs"
	"github.com/aatuh/api-toolkit/v2/endpoints/health"
	"github.com/aatuh/api-toolkit/v2/endpoints/version"
	"github.com/aatuh/api-toolkit/v2/httpx"
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "dev-headers") }}	jwtauth "github.com/aatuh/api-toolkit/v2/middleware/auth/jwt"
{{ end }}{{ if eq .AuthMode "api-key" }}	"github.com/aatuh/api-toolkit/v2/middleware/auth/apikey"
{{ end }}	"github.com/aatuh/api-toolkit/v2/middleware/auth/tenant"
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
	log := ports.NopLogger{}
	metricsRecorder, err := metricsmw.NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		return nil, err
	}
	routerConfig, err := bootstrap.DefaultRouterConfigFromEnv(nil)
	if err != nil {
		return nil, err
	}
	routerConfig.Metrics = metricsRecorder
	router, err := bootstrap.NewDefaultRouterWithConfig(log, routerConfig)
	if err != nil {
		return nil, err
	}

specRegistry := specs.NewRegistry(specs.Info{Title: "SaaS API", Version: "dev"})
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") }}	specRegistry.RegisterSecurityScheme("BearerAuth", specs.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"})
{{ else if eq .AuthMode "dev-headers" }}	specRegistry.RegisterSecurityScheme("DevHeaderAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-Debug-User", In: "header"})
{{ else }}	specRegistry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
{{ end }}
	specRegistry.SetSecurity([]specs.SecurityRequirement{
		{Name: "{{ .AuthSchemeName }}"},
	})
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
		ReadinessChecks: []string{"basic"{{ if eq .AuthMode "jwt" }}, "jwt"{{ else if eq .AuthMode "clerk" }}, "clerk"{{ end }}},
	})
	healthManager.RegisterChecker(health.NewBasicChecker())

{{ if eq .AuthMode "jwt" }}	jwtMiddleware, jwtConfig, err := newJWTMiddleware(context.Background())
	if err != nil {
		return nil, err
	}
	healthManager.RegisterChecker(jwtauth.HealthChecker(jwtConfig, nil))
{{ else if eq .AuthMode "clerk" }}	clerkMiddleware, clerkConfig, err := newClerkMiddleware(context.Background())
	if err != nil {
		return nil, err
	}
	healthManager.RegisterChecker(clerkauth.HealthChecker(clerkConfig, nil))
{{ else if eq .AuthMode "dev-headers" }}	devHeadersMiddleware, err := newDevHeadersMiddleware()
	if err != nil {
		return nil, err
	}
{{ else }}
	apiKeyMiddleware, err := newAPIKeyMiddleware()
	if err != nil {
		return nil, err
	}
{{ end }}
	healthScheduler := health.NewScheduler(healthManager, health.SchedulerConfig{
		Interval:       30 * time.Second,
		Logger:         log,
		OnStatusChange: metricsmw.HealthStatusChangeHook(metricsRecorder),
	})
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
		Store:          idempotencyStore,
		StorageKeyFunc: idempotencymw.TenantScopedStorageKeyFunc(),
		RequireKey:     true,
		OnOutcome: idempotencyOutcomeHooks(
			metricsmw.IdempotencyOutcomeHook(metricsRecorder),
			requestlog.IdempotencyOutcomeLogHook(log),
		),
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
		Log:                     log,
		Router:                  router,
		MiddlewareOrder:         bootstrap.StrictSaaSAPIMiddlewareOrder(),
		RequiredMiddlewareOrder: bootstrap.StrictSaaSAPIMiddlewareOrder(),
		BackgroundTasks: []bootstrap.BackgroundTask{
			{
				Name: "health-scheduler",
				Run: func(ctx context.Context) error {
					healthScheduler.Start(ctx)
					return nil
				},
			},
		},
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
				routepolicy.WithAuth("{{ .AuthSchemeName }}", "widgets:write"),
				routepolicy.WithTenantRequired("header"),
				routepolicy.WithIdempotencyRequired(),
				routepolicy.WithRateLimit("write-standard"),
				routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict),
			)
			widgetHandler := http.Handler(http.HandlerFunc(createWidget))
			widgetHandler = idempotencyMiddleware.Handler(widgetHandler)
{{ if eq .AuthMode "jwt" }}			widgetHandler = requireJWTScope("widgets:write")(widgetHandler)
			widgetHandler = tenantMiddleware.Handler(widgetHandler)
			widgetHandler = withJWTAuthorizationScope(widgetHandler)
			widgetHandler = jwtMiddleware.Handler(widgetHandler)
{{ else if eq .AuthMode "clerk" }}			widgetHandler = requireClerkScope("widgets:write")(widgetHandler)
			widgetHandler = tenantMiddleware.Handler(widgetHandler)
			widgetHandler = withClerkAuthorizationScope(widgetHandler)
			widgetHandler = clerkMiddleware.Handler(widgetHandler)
{{ else if eq .AuthMode "dev-headers" }}			widgetHandler = requireDevHeaderScope("widgets:write")(widgetHandler)
			widgetHandler = tenantMiddleware.Handler(widgetHandler)
			widgetHandler = withDevHeaderAuthorizationScope(widgetHandler)
			widgetHandler = devHeadersMiddleware.Handler(widgetHandler)
{{ else }}
			widgetHandler = apikey.RequireScopeMiddleware("widgets:write")(widgetHandler)
			widgetHandler = tenantMiddleware.Handler(widgetHandler)
			widgetHandler = apiKeyMiddleware.Handler(widgetHandler)
{{ end }}
			if err := contracts.Post("/widgets", operation, widgetHandler); err != nil {
				return err
			}
			return contracts.Validate()
		},
{{ if eq .AuthMode "jwt" }}		ShutdownHooks: []bootstrap.ShutdownHook{
			{Name: "jwt", Hook: func(context.Context) error {
				jwtMiddleware.Close()
				return nil
			}},
		},
{{ else if eq .AuthMode "clerk" }}		ShutdownHooks: []bootstrap.ShutdownHook{
			{Name: "clerk", Hook: func(context.Context) error {
				clerkMiddleware.Close()
				return nil
			}},
		},
{{ end }}
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

{{ if eq .AuthMode "jwt" }}func newJWTMiddleware(ctx context.Context) (*jwtauth.Middleware, jwtauth.Config, error) {
	jwksURL, err := requiredEnv("JWT_JWKS_URL")
	if err != nil {
		return nil, jwtauth.Config{}, err
	}
	issuer, err := requiredEnv("JWT_ISSUER")
	if err != nil {
		return nil, jwtauth.Config{}, err
	}
	audience, err := requiredEnv("JWT_AUDIENCE")
	if err != nil {
		return nil, jwtauth.Config{}, err
	}
	cfg := jwtauth.Config{
		Enabled:             true,
		JWKSURL:             jwksURL,
		Issuer:              issuer,
		Audience:            audience,
		AllowedAlgorithms:   splitCSV(env("JWT_ALLOWED_ALGORITHMS", "RS256")),
		AllowedClockSkew:    30 * time.Second,
		JWKSRefreshTimeout:  5 * time.Second,
		JWKSRefreshInterval: 10 * time.Minute,
	}
	mw, err := jwtauth.NewMiddleware(ctx, cfg, ports.NopLogger{})
	if err != nil {
		return nil, cfg, err
	}
	return mw, cfg, nil
}

{{ else if eq .AuthMode "clerk" }}func newClerkMiddleware(ctx context.Context) (*clerkauth.Middleware, clerkauth.Config, error) {
	jwksURL, err := requiredEnv("CLERK_JWKS_URL")
	if err != nil {
		return nil, clerkauth.Config{}, err
	}
	issuer, err := requiredEnv("CLERK_ISSUER")
	if err != nil {
		return nil, clerkauth.Config{}, err
	}
	audience, err := requiredEnv("CLERK_AUDIENCE")
	if err != nil {
		return nil, clerkauth.Config{}, err
	}
	cfg := clerkauth.Config{
		Enabled:             true,
		JWKSURL:             jwksURL,
		Issuer:              issuer,
		Audience:            audience,
		AllowedAlgorithms:   splitCSV(env("CLERK_ALLOWED_ALGORITHMS", "RS256")),
		AllowedClockSkew:    30 * time.Second,
		JWKSRefreshTimeout:  5 * time.Second,
		JWKSRefreshInterval: 10 * time.Minute,
	}
	mw, err := clerkauth.NewMiddleware(ctx, cfg, ports.NopLogger{})
	if err != nil {
		return nil, cfg, err
	}
	return mw, cfg, nil
}

{{ else if eq .AuthMode "dev-headers" }}func newDevHeadersMiddleware() (*devheaders.Middleware, error) {
	if isProduction() {
		return nil, errors.New("dev header auth is not allowed when ENV=production")
	}
	cfg := devheaders.LoadConfig(config.NewLoader())
	if !cfg.Enabled {
		return nil, errors.New("DEV_AUTH_FALLBACK_ENABLED=true is required for dev-header auth")
	}
	if !cfg.AllowDangerousDevBypasses {
		return nil, errors.New("DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES=true is required for dev-header auth")
	}
	return devheaders.New(cfg, ports.NopLogger{})
}

{{ else }}
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
{{ end }}

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

func idempotencyOutcomeHooks(handlers ...idempotencymw.OutcomeHandler) idempotencymw.OutcomeHandler {
	return func(ctx context.Context, event idempotencymw.OutcomeEvent) {
		for _, handler := range handlers {
			if handler != nil {
				handler(ctx, event)
			}
		}
	}
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

{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "dev-headers") }}{{ if eq .AuthMode "jwt" }}func withJWTAuthorizationScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subj, ok := jwtauth.SubjectFromContext(r.Context())
		if !ok {
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "authentication token required"})
			return
		}
		tenantID := tenantIDFromJWTSubject(subj)
		if tenantID == "" {
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "tenant claim required"})
			return
		}
		ctx := authorization.WithScope(r.Context(), authorization.Scope{TenantID: tenantID, UserID: subj.UserID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireJWTScope(required string) func(http.Handler) http.Handler {
	required = strings.TrimSpace(required)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subj, ok := jwtauth.SubjectFromContext(r.Context())
			if !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "authentication token required"})
				return
			}
			if !jwtSubjectHasScope(subj, required) {
				httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "required JWT scope missing"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func tenantIDFromJWTSubject(subj jwtauth.Subject) string {
	for _, key := range []string{"tenant_id", "tid", "org_id"} {
		if tenantID := stringJWTClaim(subj.Claims, key); tenantID != "" {
			return tenantID
		}
	}
	return ""
}

func jwtSubjectHasScope(subj jwtauth.Subject, required string) bool {
	if required == "" {
		return true
	}
	for _, claim := range []string{"scope", "scp", "permissions"} {
		for _, scope := range jwtScopeValues(subj.Claims[claim]) {
			if strings.EqualFold(scope, required) {
				return true
			}
		}
	}
	return false
}

func stringJWTClaim(claims map[string]any, key string) string {
	if claims == nil {
		return ""
	}
	value, ok := claims[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func jwtScopeValues(value any) []string {
	switch v := value.(type) {
	case string:
		return splitScopeString(v)
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if scope, ok := item.(string); ok {
				if scope = strings.TrimSpace(scope); scope != "" {
					out = append(out, scope)
				}
			}
		}
		return out
	default:
		return nil
	}
}

{{ else if eq .AuthMode "clerk" }}func withClerkAuthorizationScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subj, ok := clerkauth.SubjectFromContext(r.Context())
		if !ok {
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "authentication token required"})
			return
		}
		if strings.TrimSpace(subj.TenantID) == "" {
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "tenant claim required"})
			return
		}
		ctx := authorization.WithScope(r.Context(), authorization.Scope{TenantID: strings.TrimSpace(subj.TenantID), UserID: subj.UserID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireClerkScope(required string) func(http.Handler) http.Handler {
	required = strings.TrimSpace(required)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subj, ok := clerkauth.SubjectFromContext(r.Context())
			if !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "authentication token required"})
				return
			}
			if !clerkSubjectHasScope(subj, required) {
				httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "required Clerk scope missing"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clerkSubjectHasScope(subj clerkauth.Subject, required string) bool {
	if required == "" {
		return true
	}
	for _, scope := range splitScopeString(subj.Scope) {
		if strings.EqualFold(scope, required) {
			return true
		}
	}
	return false
}

{{ else if eq .AuthMode "dev-headers" }}func withDevHeaderAuthorizationScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subj, ok := jwtauth.SubjectFromContext(r.Context())
		if !ok {
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "development auth headers required"})
			return
		}
		tenantID := strings.TrimSpace(r.Header.Get(devTenantHeader()))
		if tenantID == "" {
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "development tenant header required"})
			return
		}
		ctx := authorization.WithScope(r.Context(), authorization.Scope{TenantID: tenantID, UserID: subj.UserID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireDevHeaderScope(required string) func(http.Handler) http.Handler {
	required = strings.TrimSpace(required)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := jwtauth.SubjectFromContext(r.Context()); !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "development auth headers required"})
				return
			}
			if !devHeaderHasScope(r, required) {
				httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "required development scope missing"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func devHeaderHasScope(r *http.Request, required string) bool {
	if required == "" {
		return true
	}
	if r == nil {
		return false
	}
	for _, scope := range splitScopeString(r.Header.Get(devScopeHeader())) {
		if strings.EqualFold(scope, required) {
			return true
		}
	}
	return false
}

func devTenantHeader() string {
	return env("DEV_AUTH_TENANT_HEADER", "X-Debug-Tenant-ID")
}

func devScopeHeader() string {
	return env("DEV_AUTH_SCOPE_HEADER", "X-Debug-Scopes")
}

{{ end }}
func splitScopeString(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == ','
	})
	out := parts[:0]
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func requiredEnv(key string) (string, error) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("%s is required", key)
}

{{ end }}
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
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") }}	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
{{ end }}	"encoding/json"
	"flag"
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") }}	"math/big"
{{ end }}	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") }}	"time"

	"github.com/golang-jwt/jwt/v5"
{{ end }}
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
	authorizeWidgetRequest(t, req, "tenant_1")
	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency key status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "idempotency key is required") {
		t.Fatalf("missing idempotency key body = %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"starter"}` + "`" + `))
	req.Header.Set("Content-Type", "application/json")
	authorizeWidgetRequest(t, req, "tenant_1")
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
	authorizeWidgetRequest(t, req, "tenant_1")
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
		authorizeWidgetRequest(t, req, "tenant_1")
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
	if got := rec.Header().Get("Idempotency-Key"); got != "create-key" {
		t.Fatalf("replay idempotency key = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, specs.Metrics, nil)
	req.Header.Set("X-Admin-Key", "local-admin-key")
	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"idempotency_outcomes_total", ` + "`outcome=\"completed_stored\"`" + `, ` + "`outcome=\"replayed\"`" + `} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("metrics body missing %q:\n%s", want, rec.Body.String())
		}
	}
}

func TestGeneratedServiceProtectsOperatorRoutes(t *testing.T) {
	setLocalTestEnv(t)
	service, err := newService()
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	for _, path := range []string{specs.HealthDetailed, specs.Metrics, specs.PprofIndex} {
		rec := httptest.NewRecorder()
		service.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without admin status = %d", path, rec.Code)
		}
	}

	for _, path := range []string{specs.HealthDetailed, specs.Metrics, specs.PprofIndex} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Admin-Key", "local-admin-key")
		rec := httptest.NewRecorder()
		service.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s with admin status = %d body=%s", path, rec.Code, rec.Body.String())
		}
		if path != specs.Metrics {
			continue
		}
		if !strings.Contains(rec.Body.String(), "http_requests_total") {
			t.Fatalf("metrics body missing HTTP metrics:\n%s", rec.Body.String())
		}
	}
}

func TestGeneratedServiceRejectsMalformedTrustedProxies(t *testing.T) {
	setLocalTestEnv(t)
	t.Setenv("TRUSTED_PROXIES", "not-a-cidr")
	if _, err := newService(); err == nil {
		t.Fatal("expected service startup to reject malformed trusted proxies")
	} else if !strings.Contains(err.Error(), "parse trusted proxies") {
		t.Fatalf("startup error = %v, want trusted proxy parse failure", err)
	}
}

func TestGeneratedServiceRejectsUnsafeRateLimitBypassConfig(t *testing.T) {
	setLocalTestEnv(t)
	t.Setenv("RATE_LIMIT_SKIP_ENABLED", "true")
	t.Setenv("RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES", "true")
	t.Setenv("RATE_LIMIT_SKIP_HEADER", "X-Rate-Limit-Bypass")
	t.Setenv("TRUSTED_PROXIES", "")
	if _, err := newService(); err == nil {
		t.Fatal("expected service startup to reject rate-limit bypass without trusted proxies")
	} else if !strings.Contains(err.Error(), "trusted proxies are required") {
		t.Fatalf("startup error = %v, want trusted proxy requirement", err)
	}
}

{{ if eq .AuthMode "jwt" }}func TestGeneratedServiceRejectsProductionMissingJWTConfig(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_JWKS_URL", "")
	t.Setenv("JWT_ISSUER", "https://issuer.example.com")
	t.Setenv("JWT_AUDIENCE", "saas-api")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require JWT config")
	} else if !strings.Contains(err.Error(), "JWT_JWKS_URL") {
		t.Fatalf("startup error = %v, want JWT_JWKS_URL requirement", err)
	}
}

func TestGeneratedServiceRejectsProductionMissingAdminKey(t *testing.T) {
	t.Setenv("ENV", "production")
	setJWTAuthEnv(t)
	t.Setenv("ADMIN_KEY", "")
	t.Setenv("IDEMPOTENCY_STORE", "redis")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require admin key")
	} else if !strings.Contains(err.Error(), "ADMIN_KEY") {
		t.Fatalf("startup error = %v, want ADMIN_KEY requirement", err)
	}
}

{{ else if eq .AuthMode "clerk" }}func TestGeneratedServiceRejectsProductionMissingClerkConfig(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "")
	t.Setenv("CLERK_ISSUER", "https://issuer.example.com")
	t.Setenv("CLERK_AUDIENCE", "saas-api")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require Clerk config")
	} else if !strings.Contains(err.Error(), "CLERK_JWKS_URL") {
		t.Fatalf("startup error = %v, want CLERK_JWKS_URL requirement", err)
	}
}

func TestGeneratedServiceRejectsProductionMissingAdminKey(t *testing.T) {
	t.Setenv("ENV", "production")
	setClerkAuthEnv(t)
	t.Setenv("ADMIN_KEY", "")
	t.Setenv("IDEMPOTENCY_STORE", "redis")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require admin key")
	} else if !strings.Contains(err.Error(), "ADMIN_KEY") {
		t.Fatalf("startup error = %v, want ADMIN_KEY requirement", err)
	}
}

{{ else if eq .AuthMode "dev-headers" }}func TestGeneratedServiceRejectsProductionDevHeaders(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DEV_AUTH_FALLBACK_ENABLED", "true")
	t.Setenv("DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES", "true")
	t.Setenv("DEV_AUTH_TRUSTED_PROXIES", "127.0.0.1/32")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to reject dev-header auth")
	} else if !strings.Contains(err.Error(), "dev header auth is not allowed") {
		t.Fatalf("startup error = %v, want dev-header production rejection", err)
	}
}

{{ else }}func TestGeneratedServiceRejectsProductionDefaultSecrets(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("API_KEY", "")
	t.Setenv("ADMIN_KEY", "")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require explicit secrets")
	} else if !strings.Contains(err.Error(), "API_KEY") {
		t.Fatalf("startup error = %v, want API_KEY requirement", err)
	}
}

{{ end }}
{{ if ne .AuthMode "dev-headers" }}
func TestGeneratedServiceRejectsProductionMemoryIdempotency(t *testing.T) {
	t.Setenv("ENV", "production")
{{ if eq .AuthMode "jwt" }}	setJWTAuthEnv(t)
{{ else if eq .AuthMode "clerk" }}	setClerkAuthEnv(t)
{{ else }}
	t.Setenv("API_KEY", "prod-api-key")
	t.Setenv("API_TENANT_ID", "tenant_1")
{{ end }}
	t.Setenv("ADMIN_KEY", "prod-admin-key")
	t.Setenv("IDEMPOTENCY_STORE", "memory")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to reject memory idempotency")
	} else if !strings.Contains(err.Error(), "IDEMPOTENCY_STORE=memory") {
		t.Fatalf("startup error = %v, want memory-store rejection", err)
	}
}

func TestGeneratedServiceRejectsProductionMissingRedisAddress(t *testing.T) {
	t.Setenv("ENV", "production")
{{ if eq .AuthMode "jwt" }}	setJWTAuthEnv(t)
{{ else if eq .AuthMode "clerk" }}	setClerkAuthEnv(t)
{{ else }}
	t.Setenv("API_KEY", "prod-api-key")
	t.Setenv("API_TENANT_ID", "tenant_1")
{{ end }}
	t.Setenv("ADMIN_KEY", "prod-admin-key")
	t.Setenv("IDEMPOTENCY_STORE", "redis")
	t.Setenv("REDIS_ADDR", "")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require Redis address")
	} else if !strings.Contains(err.Error(), "REDIS_ADDR") {
		t.Fatalf("startup error = %v, want REDIS_ADDR requirement", err)
	}
}
{{ end }}

func setLocalTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENV", "development")
{{ if eq .AuthMode "jwt" }}	setJWTAuthEnv(t)
{{ else if eq .AuthMode "clerk" }}	setClerkAuthEnv(t)
{{ else if eq .AuthMode "dev-headers" }}	setDevHeaderAuthEnv(t)
{{ else }}
	t.Setenv("API_KEY", "local-dev-key")
	t.Setenv("API_TENANT_ID", "tenant_1")
{{ end }}
	t.Setenv("ADMIN_KEY", "local-admin-key")
	t.Setenv("IDEMPOTENCY_STORE", "memory")
}

func authorizeWidgetRequest(t *testing.T, req *http.Request, tenantID string) {
	t.Helper()
{{ if eq .AuthMode "jwt" }}	req.Header.Set("Authorization", "Bearer "+testJWT(t, tenantID, "widgets:write"))
{{ else if eq .AuthMode "clerk" }}	req.Header.Set("Authorization", "Bearer "+testClerkJWT(t, tenantID, "widgets:write"))
{{ else if eq .AuthMode "dev-headers" }}	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Debug-User", "user_123")
	req.Header.Set("X-Debug-Tenant-ID", tenantID)
	req.Header.Set("X-Debug-Scopes", "widgets:write")
{{ else }}	req.Header.Set("X-API-Key", "local-dev-key")
{{ end }}	req.Header.Set("X-Tenant-ID", tenantID)
}

{{ if eq .AuthMode "dev-headers" }}func setDevHeaderAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DEV_AUTH_FALLBACK_ENABLED", "true")
	t.Setenv("DEV_AUTH_USER_HEADER", "X-Debug-User")
	t.Setenv("DEV_AUTH_EMAIL_HEADER", "X-Debug-Email")
	t.Setenv("DEV_AUTH_FIRST_NAME_HEADER", "X-Debug-First-Name")
	t.Setenv("DEV_AUTH_LAST_NAME_HEADER", "X-Debug-Last-Name")
	t.Setenv("DEV_AUTH_DEFAULT_LANGUAGE", "en")
	t.Setenv("DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES", "true")
	t.Setenv("DEV_AUTH_TRUSTED_PROXIES", "127.0.0.1/32,::1/128")
	t.Setenv("DEV_AUTH_TENANT_HEADER", "X-Debug-Tenant-ID")
	t.Setenv("DEV_AUTH_SCOPE_HEADER", "X-Debug-Scopes")
}

{{ end }}
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") }}const (
	testBearerKeyID = "test-kid"
{{ if eq .AuthMode "jwt" }}	testJWTIssuer   = "https://issuer.example.test"
	testJWTAudience = "saas-api"
{{ else if eq .AuthMode "clerk" }}	testClerkIssuer   = "https://clerk.example.test"
	testClerkAudience = "saas-api"
{{ end }}
)

var testBearerPrivateKey *rsa.PrivateKey

{{ if eq .AuthMode "jwt" }}
func setJWTAuthEnv(t *testing.T) {
	t.Helper()
	setBearerAuthEnv(t, "JWT", testJWTIssuer, testJWTAudience)
}

func testJWT(t *testing.T, tenantID string, scopes ...string) string {
	t.Helper()
	return testBearerJWT(t, testJWTIssuer, testJWTAudience, tenantID, scopes...)
}

{{ else if eq .AuthMode "clerk" }}func setClerkAuthEnv(t *testing.T) {
	t.Helper()
	setBearerAuthEnv(t, "CLERK", testClerkIssuer, testClerkAudience)
}

func testClerkJWT(t *testing.T, tenantID string, scopes ...string) string {
	t.Helper()
	return testBearerJWT(t, testClerkIssuer, testClerkAudience, tenantID, scopes...)
}

{{ end }}func setBearerAuthEnv(t *testing.T, envPrefix, issuer, audience string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate bearer test key: %v", err)
	}
	testBearerPrivateKey = key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwkFromRSAPublicKey(&key.PublicKey)}})
	}))
	t.Cleanup(server.Close)
	t.Setenv(envPrefix+"_JWKS_URL", server.URL)
	t.Setenv(envPrefix+"_ISSUER", issuer)
	t.Setenv(envPrefix+"_AUDIENCE", audience)
	t.Setenv(envPrefix+"_ALLOWED_ALGORITHMS", "RS256")
}

func testBearerJWT(t *testing.T, issuer, audience, tenantID string, scopes ...string) string {
	t.Helper()
	if testBearerPrivateKey == nil {
		t.Fatal("bearer test key is not configured")
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":       "user_123",
		"tenant_id": tenantID,
		"scope":     strings.Join(scopes, " "),
		"iss":       issuer,
		"aud":       audience,
		"iat":       now.Unix(),
		"exp":       now.Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testBearerKeyID
	signed, err := token.SignedString(testBearerPrivateKey)
	if err != nil {
		t.Fatalf("sign bearer test JWT: %v", err)
	}
	return signed
}

func jwkFromRSAPublicKey(key *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"use": "sig",
		"kid": testBearerKeyID,
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

{{ end }}
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
TRUSTED_PROXIES=
RATE_LIMIT_SKIP_ENABLED=false
RATE_LIMIT_SKIP_HEADER=
RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES=false
{{ if eq .AuthMode "jwt" }}JWT_JWKS_URL=
JWT_ISSUER=
JWT_AUDIENCE=saas-api
JWT_ALLOWED_ALGORITHMS=RS256
{{ else if eq .AuthMode "clerk" }}CLERK_JWKS_URL=
CLERK_ISSUER=
CLERK_AUDIENCE=saas-api
CLERK_ALLOWED_ALGORITHMS=RS256
{{ else if eq .AuthMode "dev-headers" }}DEV_AUTH_FALLBACK_ENABLED=true
DEV_AUTH_USER_HEADER=X-Debug-User
DEV_AUTH_EMAIL_HEADER=X-Debug-Email
DEV_AUTH_FIRST_NAME_HEADER=X-Debug-First-Name
DEV_AUTH_LAST_NAME_HEADER=X-Debug-Last-Name
DEV_AUTH_DEFAULT_LANGUAGE=en
DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES=true
DEV_AUTH_TRUSTED_PROXIES=127.0.0.1/32,::1/128
DEV_AUTH_TENANT_HEADER=X-Debug-Tenant-ID
DEV_AUTH_SCOPE_HEADER=X-Debug-Scopes
{{ else }}
API_KEY=local-dev-key
API_TENANT_ID=tenant_1
{{ end }}
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
- ` + "`POST /widgets`" + ` with {{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") }}` + "`Authorization: Bearer <token>`" + `{{ else if eq .AuthMode "dev-headers" }}` + "`X-Debug-User`" + `, ` + "`X-Debug-Tenant-ID`" + `, ` + "`X-Debug-Scopes`" + `{{ else }}` + "`X-API-Key`" + `{{ end }}, ` + "`X-Tenant-ID`" + `, and ` + "`Idempotency-Key`" + `
- ` + "`GET /health/detailed`" + ` with ` + "`X-Admin-Key`" + `
- ` + "`GET /metrics`" + ` with ` + "`X-Admin-Key`" + `
- ` + "`GET /debug/pprof/`" + ` with ` + "`X-Admin-Key`" + `

Generated auth mode: ` + "`{{ .AuthMode }}`" + `.
The generated OpenAPI document declares ` + "`{{ .AuthSchemeName }}`" + ` as the top-level security default and keeps operation scopes explicit on protected writes.
Unsafe writes without ` + "`Idempotency-Key`" + ` fail with Problem Details 400 before the handler runs. Idempotency storage keys are tenant and actor scoped before they reach the memory or Redis store.
{{ if eq .AuthMode "jwt" }}JWT mode validates bearer tokens with ` + "`JWT_JWKS_URL`" + `, ` + "`JWT_ISSUER`" + `, and ` + "`JWT_AUDIENCE`" + `. The ` + "`tenant_id`" + ` token claim must match ` + "`X-Tenant-ID`" + `, and write requests require the ` + "`widgets:write`" + ` scope.
When ` + "`ENV=production`" + `, startup requires explicit JWT configuration and ` + "`ADMIN_KEY`" + `.
{{ else if eq .AuthMode "clerk" }}Clerk mode validates bearer tokens with ` + "`CLERK_JWKS_URL`" + `, ` + "`CLERK_ISSUER`" + `, and ` + "`CLERK_AUDIENCE`" + `. The ` + "`tenant_id`" + ` or ` + "`org_id`" + ` token claim must match ` + "`X-Tenant-ID`" + `, and write requests require the ` + "`widgets:write`" + ` scope.
When ` + "`ENV=production`" + `, startup requires explicit Clerk configuration and ` + "`ADMIN_KEY`" + `.
{{ else if eq .AuthMode "dev-headers" }}Development-header mode is only generated by the explicit ` + "`dev-api`" + ` profile. ` + "`X-Debug-Tenant-ID`" + ` must match ` + "`X-Tenant-ID`" + `, ` + "`X-Debug-Scopes`" + ` must include ` + "`widgets:write`" + `, and startup refuses this auth mode when ` + "`ENV=production`" + `.
{{ else }}
The default API key is scoped to ` + "`API_TENANT_ID`" + `, and write requests fail when ` + "`X-Tenant-ID`" + ` does not match that authenticated tenant.
When ` + "`ENV=production`" + `, startup requires explicit non-empty ` + "`API_KEY`" + ` and ` + "`ADMIN_KEY`" + ` values instead of local fallback keys.
{{ end }}
Local development uses ` + "`IDEMPOTENCY_STORE=memory`" + `. In production, the generated service defaults to ` + "`IDEMPOTENCY_STORE=redis`" + ` and requires ` + "`REDIS_ADDR`" + ` so unsafe writes can be replayed across instances.
Local ` + "`.env`" + ` files, coverage output, temporary files, and built binaries are ignored by default.
`
