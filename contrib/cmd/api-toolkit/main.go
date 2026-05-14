package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/aatuh/api-toolkit/v2/routepolicy"
	"github.com/aatuh/api-toolkit/v2/specs"
)

const toolVersion = "dev"

var (
	buildCommit = "unknown"
	buildDate   = "unknown"
)

const (
	coreModulePath    = "github.com/aatuh/api-toolkit/v2"
	contribModulePath = "github.com/aatuh/api-toolkit/contrib/v2"
)

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
		fmt.Fprintln(stderr, "usage: api-toolkit <new|contracts|clients|version>")
		return 2
	}
	switch args[0] {
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "new":
		return runNew(ctx, args[1:], stdout, stderr)
	case "contracts":
		return runContracts(ctx, args[1:], stdout, stderr)
	case "clients":
		return runClients(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

type versionMetadata struct {
	ToolVersion    string `json:"tool_version"`
	GoVersion      string `json:"go_version"`
	MainPath       string `json:"main_path"`
	MainVersion    string `json:"main_version"`
	CoreVersion    string `json:"core_version"`
	ContribVersion string `json:"contrib_version"`
	BuildCommit    string `json:"build_commit"`
	BuildDate      string `json:"build_date"`
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	switch len(args) {
	case 0:
		printVersion(stdout)
		return 0
	case 1:
		if args[0] == "--json" {
			if err := printVersionJSON(stdout); err != nil {
				fmt.Fprintf(stderr, "write version json: %v\n", err)
				return 1
			}
			return 0
		}
	}
	fmt.Fprintln(stderr, "usage: api-toolkit version [--json]")
	return 2
}

func printVersion(stdout io.Writer) {
	info := collectVersionMetadata()
	fmt.Fprintf(stdout, "api-toolkit %s\n", info.ToolVersion)
	fmt.Fprintf(stdout, "go %s\n", info.GoVersion)
	fmt.Fprintf(stdout, "main %s %s\n", info.MainPath, info.MainVersion)
	fmt.Fprintf(stdout, "core %s %s\n", coreModulePath, info.CoreVersion)
	fmt.Fprintf(stdout, "contrib %s %s\n", contribModulePath, info.ContribVersion)
	fmt.Fprintf(stdout, "build_commit %s\n", info.BuildCommit)
	fmt.Fprintf(stdout, "build_date %s\n", info.BuildDate)
}

func printVersionJSON(stdout io.Writer) error {
	return json.NewEncoder(stdout).Encode(collectVersionMetadata())
}

func collectVersionMetadata() versionMetadata {
	info := versionMetadata{
		ToolVersion:    toolVersion,
		GoVersion:      runtime.Version(),
		MainPath:       "unknown",
		MainVersion:    "unknown",
		CoreVersion:    "unknown",
		ContribVersion: "unknown",
		BuildCommit:    buildCommit,
		BuildDate:      buildDate,
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok && buildInfo != nil {
		info.MainPath = versionValue(buildInfo.Main.Path)
		info.MainVersion = versionValue(buildInfo.Main.Version)
		if buildInfo.Main.Path == contribModulePath || strings.HasPrefix(buildInfo.Main.Path, contribModulePath+"/") {
			info.ContribVersion = versionValue(buildInfo.Main.Version)
		}
		for _, dep := range buildInfo.Deps {
			if dep == nil {
				continue
			}
			version := versionValue(dep.Version)
			if dep.Replace != nil {
				version = versionValue(dep.Replace.Version)
				if version == "unknown" {
					version = "local"
				}
			}
			switch dep.Path {
			case coreModulePath:
				info.CoreVersion = version
			case contribModulePath:
				info.ContribVersion = version
			}
		}
	}
	return info
}

func versionValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "(devel)" {
		return "dev"
	}
	return value
}

func runNew(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "service" {
		fmt.Fprintln(stderr, "usage: api-toolkit new service --module <module> [--dir <path>] [--profile saas-api|saas-api-full|dev-api] [--auth api-key|jwt|clerk|oidc|dev-headers]")
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
	scaffoldProfileSaaSAPI     = "saas-api"
	scaffoldProfileSaaSAPIFull = "saas-api-full"
	scaffoldProfileDevAPI      = "dev-api"
	scaffoldAuthAPIKey         = "api-key"
	scaffoldAuthJWT            = "jwt"
	scaffoldAuthClerk          = "clerk"
	scaffoldAuthOIDC           = "oidc"
	scaffoldAuthDevHeaders     = "dev-headers"
)

func isSupportedScaffoldProfile(profile string) bool {
	switch strings.TrimSpace(profile) {
	case scaffoldProfileSaaSAPI, scaffoldProfileSaaSAPIFull, scaffoldProfileDevAPI:
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
	case scaffoldAuthOIDC:
		if profile != scaffoldProfileSaaSAPIFull {
			return "", fmt.Errorf("auth mode %q requires profile %q", authMode, scaffoldProfileSaaSAPIFull)
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

func runClients(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "go" {
		fmt.Fprintln(stderr, "usage: api-toolkit clients go --openapi <openapi.json> --out <dir> --package <name>")
		return 2
	}
	fs := flag.NewFlagSet("clients go", flag.ContinueOnError)
	fs.SetOutput(stderr)
	openAPIPath := fs.String("openapi", "", "OpenAPI JSON file")
	outDir := fs.String("out", "", "output directory")
	packageName := fs.String("package", "apiclient", "Go package name")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if err := ctx.Err(); err != nil {
		fmt.Fprintf(stderr, "context canceled: %v\n", err)
		return 1
	}
	cfg := goClientConfig{
		OpenAPIPath: strings.TrimSpace(*openAPIPath),
		OutDir:      strings.TrimSpace(*outDir),
		Package:     strings.TrimSpace(*packageName),
	}
	if err := generateGoClient(cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "generated %s\n", filepath.Join(cfg.OutDir, "client.go"))
	return 0
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

type goClientConfig struct {
	OpenAPIPath string
	OutDir      string
	Package     string
}

func generateGoClient(cfg goClientConfig) error {
	if cfg.OpenAPIPath == "" {
		return errors.New("--openapi is required")
	}
	if strings.TrimSpace(cfg.OutDir) == "" {
		return errors.New("--out is required")
	}
	if !validGoPackageName(cfg.Package) {
		return fmt.Errorf("invalid Go package name %q", cfg.Package)
	}
	loaded, err := loadOpenAPI(cfg.OpenAPIPath)
	if err != nil {
		return err
	}
	if err := loaded.validate(); err != nil {
		return err
	}
	operations := operationsFromOpenAPIDocument(loaded.doc)
	rendered := renderGoClient(cfg.Package, operations)
	formatted, err := format.Source(rendered)
	if err != nil {
		return fmt.Errorf("format generated client: %w", err)
	}
	outDir, err := safeOutputDir(cfg.OutDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	root, err := os.OpenRoot(outDir)
	if err != nil {
		return fmt.Errorf("open output root: %w", err)
	}
	defer root.Close()
	return writeGeneratedFileReplace(root, "client.go", formatted)
}

func validGoPackageName(name string) bool {
	if !token.IsIdentifier(name) {
		return false
	}
	return !token.Lookup(name).IsKeyword()
}

func writeGeneratedFileReplace(root *os.Root, name string, data []byte) error {
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
	file, err := root.OpenFile(clean, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func renderGoClient(packageName string, operations []specs.Operation) []byte {
	operations = append([]specs.Operation(nil), operations...)
	sort.SliceStable(operations, func(i, j int) bool {
		if operations[i].Path != operations[j].Path {
			return operations[i].Path < operations[j].Path
		}
		if operations[i].Method != operations[j].Method {
			return operations[i].Method < operations[j].Method
		}
		return operations[i].OperationID < operations[j].OperationID
	})
	seenMethods := map[string]int{}
	var methods strings.Builder
	for _, operation := range operations {
		operationID := strings.TrimSpace(operation.OperationID)
		if operationID == "" {
			continue
		}
		methodName := exportedGoIdentifier(operationID)
		seenMethods[methodName]++
		if seenMethods[methodName] > 1 {
			methodName = fmt.Sprintf("%s%d", methodName, seenMethods[methodName])
		}
		renderGoClientOperation(&methods, methodName, operation)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "package %s\n\n", packageName)
	out.WriteString(goClientRuntimeTemplate)
	out.WriteString(methods.String())
	return []byte(out.String())
}

func renderGoClientOperation(out *strings.Builder, methodName string, operation specs.Operation) {
	pathParams := operationPathParameters(operation)
	args := []string{"ctx context.Context"}
	for _, param := range pathParams {
		args = append(args, goParamName(param.Name)+" string")
	}
	bodyArg := "nil"
	if operation.RequestBody != nil {
		args = append(args, "body any")
		bodyArg = "body"
	}
	args = append(args, "opts ...RequestOption")
	fmt.Fprintf(out, "// %s calls %s %s.\n", methodName, strings.ToUpper(operation.Method), operation.Path)
	fmt.Fprintf(out, "func (c *Client) %s(%s) (*http.Response, error) {\n", methodName, strings.Join(args, ", "))
	if len(pathParams) > 0 {
		out.WriteString("\topts = append([]RequestOption{\n")
		for _, param := range pathParams {
			fmt.Fprintf(out, "\t\tPathParam(%q, %s),\n", param.Name, goParamName(param.Name))
		}
		out.WriteString("\t}, opts...)\n")
	}
	fmt.Fprintf(out, "\treturn c.do(ctx, %q, %q, %s, opts...)\n", strings.ToUpper(operation.Method), operation.Path, bodyArg)
	out.WriteString("}\n\n")
}

func operationPathParameters(operation specs.Operation) []specs.Parameter {
	var out []specs.Parameter
	for _, parameter := range operation.Parameters {
		if strings.EqualFold(parameter.In, "path") && strings.TrimSpace(parameter.Name) != "" {
			out = append(out, parameter)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func exportedGoIdentifier(value string) string {
	parts := identifierParts(value)
	if len(parts) == 0 {
		return "Operation"
	}
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	out := strings.Join(parts, "")
	if out == "" || !isASCIILetter(out[0]) {
		return "Operation" + out
	}
	return out
}

func goParamName(value string) string {
	parts := identifierParts(value)
	if len(parts) == 0 {
		return "param"
	}
	for i := range parts {
		if i == 0 {
			parts[i] = strings.ToLower(parts[i][:1]) + parts[i][1:]
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	out := strings.Join(parts, "")
	if out == "" || !isASCIILetter(out[0]) {
		out = "param" + out
	}
	if token.Lookup(out).IsKeyword() {
		out += "Value"
	}
	return out
}

func identifierParts(value string) []string {
	var parts []string
	var current strings.Builder
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if isASCIILetter(ch) || isASCIIDigit(ch) {
			current.WriteByte(ch)
			continue
		}
		if current.Len() > 0 {
			parts = append(parts, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func isASCIILetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isASCIIDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
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
	files := scaffoldFilesForProfile(cfg.Profile)
	for _, file := range files {
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
	var golden []byte
	if cfg.Profile == scaffoldProfileSaaSAPIFull {
		golden, err = renderSaaSAPIFullOpenAPIGolden()
	} else {
		golden, err = renderSaaSAPIOpenAPIGolden(cfg.AuthMode)
	}
	if err != nil {
		return err
	}
	if err := writeGeneratedFile(root, "testdata/openapi.golden.json", golden); err != nil {
		return err
	}
	if cfg.Profile == scaffoldProfileSaaSAPIFull {
		client, err := renderSaaSAPIFullGoClient()
		if err != nil {
			return err
		}
		if err := writeGeneratedFile(root, "internal/client/apiclient/client.go", client); err != nil {
			return err
		}
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

func renderSaaSAPIFullOpenAPIGolden() ([]byte, error) {
	registry := specs.NewRegistry(specs.Info{
		Title:       "Full SaaS API",
		Description: "Generated api-toolkit full SaaS/API profile.",
		Version:     "dev",
	})
	authSchemeName := "ApiKeyAuth"
	registry.RegisterSecurityScheme(authSchemeName, specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	registry.SetSecurity([]specs.SecurityRequirement{{Name: authSchemeName}})
	registerFullScaffoldSchemas(registry)
	specs.RegisterProblemCatalog(registry, nil)
	for _, operation := range fullScaffoldOperations(authSchemeName) {
		registry.Register(operation)
	}
	doc, err := registry.OpenAPI()
	if err != nil {
		return nil, fmt.Errorf("render full scaffold openapi: %w", err)
	}
	return normalizeJSON(doc)
}

func renderSaaSAPIFullGoClient() ([]byte, error) {
	client := renderGoClient("apiclient", fullScaffoldOperations("ApiKeyAuth"))
	formatted, err := format.Source(client)
	if err != nil {
		return nil, fmt.Errorf("format full scaffold Go client: %w", err)
	}
	return formatted, nil
}

func registerFullScaffoldSchemas(registry *specs.Registry) {
	registry.RegisterSchema("Widget", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "name", "version"},
		"properties": map[string]any{
			"id":        map[string]any{"type": "string"},
			"tenant_id": map[string]any{"type": "string"},
			"name":      map[string]any{"type": "string"},
			"version":   map[string]any{"type": "integer", "format": "int64"},
		},
	})
	registry.RegisterSchema("WidgetCreateRequest", map[string]any{
		"type":     "object",
		"required": []string{"name"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		},
		"additionalProperties": false,
	})
	registry.RegisterSchema("WidgetList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Widget"}},
			"next_cursor": map[string]any{"type": "string", "nullable": true},
		},
	})
}

func fullScaffoldOperations(authSchemeName string) []specs.Operation {
	auth := func(scopes ...string) []specs.SecurityRequirement {
		return []specs.SecurityRequirement{{Name: authSchemeName, Scopes: scopes}}
	}
	problemStatuses := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusPreconditionFailed, http.StatusTooManyRequests}
	jsonBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetCreateRequest"},
		},
	}
	widgetResponse := specs.Response{
		Description: "Widget",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Widget"},
		},
	}
	return []specs.Operation{
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getReadiness",
			Method:      http.MethodGet,
			Path:        "/readyz",
			Summary:     "Readiness",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Ready"}},
		}),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOpenAPI",
			Method:      http.MethodGet,
			Path:        "/docs/openapi.json",
			Summary:     "OpenAPI document",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "OpenAPI document"}},
		}),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getDetailedHealth",
			Method:      http.MethodGet,
			Path:        "/health/detailed",
			Summary:     "Detailed health",
			Security:    auth("admin:read"),
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Detailed health"}},
		}, routepolicy.WithAdminPolicy("admin"), routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getMetrics",
			Method:      http.MethodGet,
			Path:        "/metrics",
			Summary:     "Metrics",
			Security:    auth("admin:read"),
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Metrics"}},
		}, routepolicy.WithAdminPolicy("admin"), routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listWidgets",
			Method:      http.MethodGet,
			Path:        "/widgets",
			Summary:     "List widgets",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "cursor", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
				{Name: "limit", In: "query", Required: false, Schema: map[string]any{"type": "integer", "minimum": 1, "maximum": 100}},
			},
			Security: auth("widgets:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Widget list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/WidgetList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createWidget",
			Method:      http.MethodPost,
			Path:        "/widgets",
			Summary:     "Create widget",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: jsonBody,
			Responses:   map[int]specs.Response{http.StatusCreated: widgetResponse, http.StatusOK: widgetResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "updateWidget",
			Method:      http.MethodPatch,
			Path:        "/widgets/{id}",
			Summary:     "Update widget",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "If-Match", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: jsonBody,
			Responses:   map[int]specs.Response{http.StatusOK: widgetResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "deleteWidget",
			Method:      http.MethodDelete,
			Path:        "/widgets/{id}",
			Summary:     "Delete widget",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("widgets:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Deleted"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
	}
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
	return authMode == scaffoldAuthJWT || authMode == scaffoldAuthClerk || authMode == scaffoldAuthOIDC
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
	{Name: ".github/workflows/ci.yml", Body: ciWorkflowTemplate},
	{Name: "Dockerfile", Body: dockerfileTemplate},
	{Name: "docker-compose.yml", Body: composeTemplate},
	{Name: "README.md", Body: readmeTemplate},
}

func scaffoldFilesForProfile(profile string) []scaffoldFile {
	if profile == scaffoldProfileSaaSAPIFull {
		return fullScaffoldFiles
	}
	return scaffoldFiles
}

var fullScaffoldFiles = []scaffoldFile{
	{Name: "go.mod", Body: fullGoModTemplate},
	{Name: "cmd/api/main.go", Body: fullCmdMainTemplate},
	{Name: "internal/domain/widget.go", Body: fullDomainWidgetTemplate},
	{Name: "internal/app/widgets.go", Body: fullAppWidgetsTemplate},
	{Name: "internal/adapters/postgres/postgres.go", Body: fullPostgresAdapterTemplate},
	{Name: "internal/httpapi/openapi.go", Body: fullHTTPAPIOpenAPITemplate},
	{Name: "internal/httpapi/router.go", Body: fullHTTPAPIRouterTemplate},
	{Name: "internal/httpapi/router_test.go", Body: fullHTTPAPIRouterTestTemplate},
	{Name: "migrations/0001_platform.sql", Body: fullMigrationTemplate},
	{Name: "Makefile", Body: fullMakefileTemplate},
	{Name: ".env.example", Body: fullEnvTemplate},
	{Name: ".gitignore", Body: fullGitignoreTemplate},
	{Name: ".dockerignore", Body: fullDockerignoreTemplate},
	{Name: ".github/workflows/ci.yml", Body: fullCIWorkflowTemplate},
	{Name: "Dockerfile", Body: fullDockerfileTemplate},
	{Name: "docker-compose.yml", Body: fullComposeTemplate},
	{Name: "deploy/kubernetes/deployment.yaml", Body: fullKubernetesDeploymentTemplate},
	{Name: "deploy/kubernetes/service.yaml", Body: fullKubernetesServiceTemplate},
	{Name: "deploy/kubernetes/admin-service.yaml", Body: fullKubernetesAdminServiceTemplate},
	{Name: "README.md", Body: fullReadmeTemplate},
}

const goClientRuntimeTemplate = `import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	baseURL     string
	httpClient  *http.Client
	apiKey      string
	bearerToken string
	headers     http.Header
}

type Option func(*Client)

func New(baseURL string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q", baseURL)
	}
	client := &Client{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
		headers:    http.Header{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	if client.httpClient == nil {
		return nil, errors.New("http client is required")
	}
	return client, nil
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

func WithAPIKey(apiKey string) Option {
	return func(client *Client) {
		client.apiKey = strings.TrimSpace(apiKey)
	}
}

func WithBearerToken(token string) Option {
	return func(client *Client) {
		client.bearerToken = strings.TrimSpace(token)
	}
}

func WithHeader(name, value string) Option {
	return func(client *Client) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if client.headers == nil {
			client.headers = http.Header{}
		}
		client.headers.Set(name, value)
	}
}

type RequestOption func(*requestOptions)

type requestOptions struct {
	pathParams map[string]string
	query      url.Values
	headers    http.Header
}

func PathParam(name, value string) RequestOption {
	return func(opts *requestOptions) {
		if opts.pathParams == nil {
			opts.pathParams = map[string]string{}
		}
		opts.pathParams[name] = value
	}
}

func QueryParam(name, value string) RequestOption {
	return func(opts *requestOptions) {
		if opts.query == nil {
			opts.query = url.Values{}
		}
		opts.query.Set(name, value)
	}
}

func Header(name, value string) RequestOption {
	return func(opts *requestOptions) {
		if opts.headers == nil {
			opts.headers = http.Header{}
		}
		opts.headers.Set(name, value)
	}
}

type Problem struct {
	Type     string         ` + "`json:\"type,omitempty\"`" + `
	Title    string         ` + "`json:\"title,omitempty\"`" + `
	Status   int            ` + "`json:\"status,omitempty\"`" + `
	Detail   string         ` + "`json:\"detail,omitempty\"`" + `
	Instance string         ` + "`json:\"instance,omitempty\"`" + `
	Ext      map[string]any ` + "`json:\"-\"`" + `
}

type Error struct {
	Response *http.Response
	Problem  *Problem
	Body     []byte
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	if err.Problem != nil && err.Problem.Title != "" {
		return err.Problem.Title
	}
	if err.Response != nil {
		return err.Response.Status
	}
	return "request failed"
}

func (c *Client) do(ctx context.Context, method, path string, body any, opts ...RequestOption) (*http.Response, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}
	requestOpts := requestOptions{
		pathParams: map[string]string{},
		query:      url.Values{},
		headers:    http.Header{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&requestOpts)
		}
	}
	expandedPath, err := expandPath(path, requestOpts.pathParams)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(c.baseURL, "/") + expandedPath
	if encodedQuery := requestOpts.query.Encode(); encodedQuery != "" {
		endpoint += "?" + encodedQuery
	}
	var reader io.Reader
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = &buf
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	copyHeaders(req.Header, c.headers)
	copyHeaders(req.Header, requestOpts.headers)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return resp, decodeError(resp)
	}
	return resp, nil
}

func expandPath(path string, params map[string]string) (string, error) {
	expanded := path
	for name, value := range params {
		expanded = strings.ReplaceAll(expanded, "{"+name+"}", url.PathEscape(value))
	}
	if strings.Contains(expanded, "{") || strings.Contains(expanded, "}") {
		return "", fmt.Errorf("missing path parameter for %s", path)
	}
	return expanded, nil
}

func decodeError(resp *http.Response) error {
	apiErr := &Error{Response: resp}
	if resp == nil || resp.Body == nil {
		return apiErr
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/problem+json") {
		return apiErr
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apiErr
	}
	apiErr.Body = body
	var problem Problem
	if err := json.Unmarshal(body, &problem); err == nil {
		apiErr.Problem = &problem
	}
	return apiErr
}

func copyHeaders(dst, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

`

const fullGoModTemplate = `module {{ .Module }}

go 1.25.0

require (
	github.com/aatuh/api-toolkit/v2 v2.1.0
	github.com/aatuh/api-toolkit/contrib/v2 v2.1.0
)

{{ .CoreReplace }}{{ .ContribReplace }}`

const fullCmdMainTemplate = `package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"{{ .Module }}/internal/app"
	"{{ .Module }}/internal/httpapi"
)

const (
	appVersion  = "dev"
	buildCommit = "unknown"
	buildDate   = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := httpapi.ConfigFromEnv()
	if err != nil {
		return err
	}
	widgets := app.NewWidgetService()
	publicServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(httpapi.RouterConfig{Widgets: widgets, APIKey: cfg.APIKey, AdminKey: cfg.AdminKey}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	servers := []*http.Server{publicServer}
	errCh := make(chan error, 2)
	for _, srv := range servers {
		server := srv
		go func() {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}
	if cfg.AdminAddr != "" {
		adminServer := &http.Server{
			Addr:              cfg.AdminAddr,
			Handler:           httpapi.NewAdminRouter(httpapi.RouterConfig{AdminKey: cfg.AdminKey}),
			ReadHeaderTimeout: 5 * time.Second,
		}
		servers = append(servers, adminServer)
		go func() {
			if err := adminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, srv := range servers {
			_ = srv.Shutdown(shutdownCtx)
		}
		return nil
	case err := <-errCh:
		return err
	}
}
`

const fullDomainWidgetTemplate = `package domain

import (
	"fmt"
	"time"
)

type Widget struct {
	ID        string
	TenantID  string
	Name      string
	Version   int64
	Deleted   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (w Widget) ETag() string {
	return fmt.Sprintf("%q", w.Version)
}

func (w Widget) Public() map[string]any {
	return map[string]any{
		"id":        w.ID,
		"tenant_id": w.TenantID,
		"name":      w.Name,
		"version":   w.Version,
	}
}
`

// #nosec G101 -- generated source uses idempotency-key variable names, not hardcoded secrets.
const fullAppWidgetsTemplate = `package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"{{ .Module }}/internal/domain"
)

var (
	ErrValidation         = errors.New("validation failed")
	ErrNotFound           = errors.New("not found")
	ErrPreconditionFailed = errors.New("precondition failed")
)

type WidgetService struct {
	mu            sync.Mutex
	next          int
	widgets       map[string]domain.Widget
	createReplays map[string]domain.Widget
	updateReplays map[string]domain.Widget
	deleteReplays map[string]struct{}
	now           func() time.Time
}

func NewWidgetService() *WidgetService {
	return &WidgetService{
		widgets:       map[string]domain.Widget{},
		createReplays: map[string]domain.Widget{},
		updateReplays: map[string]domain.Widget{},
		deleteReplays: map[string]struct{}{},
		now:           time.Now,
	}
}

func (s *WidgetService) List(ctx context.Context, tenantID string) ([]domain.Widget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Widget, 0, len(s.widgets))
	for _, widget := range s.widgets {
		if widget.TenantID == tenantID && !widget.Deleted {
			out = append(out, widget)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *WidgetService) Create(ctx context.Context, tenantID, name, idempotencyKey string) (domain.Widget, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Widget{}, false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	name = strings.TrimSpace(name)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if tenantID == "" || name == "" || idempotencyKey == "" {
		return domain.Widget{}, false, ErrValidation
	}
	replayKey := tenantID + "\x00create\x00" + idempotencyKey
	s.mu.Lock()
	defer s.mu.Unlock()
	if widget, ok := s.createReplays[replayKey]; ok {
		return widget, true, nil
	}
	s.next++
	now := s.now().UTC()
	widget := domain.Widget{
		ID:        fmt.Sprintf("wgt_%06d", s.next),
		TenantID:  tenantID,
		Name:      name,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.widgets[widget.ID] = widget
	s.createReplays[replayKey] = widget
	return widget, false, nil
}

func (s *WidgetService) Update(ctx context.Context, tenantID, id, name, ifMatch, idempotencyKey string) (domain.Widget, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Widget{}, false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	ifMatch = strings.TrimSpace(ifMatch)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if tenantID == "" || id == "" || name == "" || ifMatch == "" || idempotencyKey == "" {
		return domain.Widget{}, false, ErrValidation
	}
	replayKey := tenantID + "\x00update\x00" + id + "\x00" + idempotencyKey
	s.mu.Lock()
	defer s.mu.Unlock()
	if widget, ok := s.updateReplays[replayKey]; ok {
		return widget, true, nil
	}
	widget, ok := s.widgets[id]
	if !ok || widget.Deleted || widget.TenantID != tenantID {
		return domain.Widget{}, false, ErrNotFound
	}
	if widget.ETag() != ifMatch {
		return domain.Widget{}, false, ErrPreconditionFailed
	}
	widget.Name = name
	widget.Version++
	widget.UpdatedAt = s.now().UTC()
	s.widgets[id] = widget
	s.updateReplays[replayKey] = widget
	return widget, false, nil
}

func (s *WidgetService) Delete(ctx context.Context, tenantID, id, idempotencyKey string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if tenantID == "" || id == "" || idempotencyKey == "" {
		return false, ErrValidation
	}
	replayKey := tenantID + "\x00delete\x00" + id + "\x00" + idempotencyKey
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.deleteReplays[replayKey]; ok {
		return true, nil
	}
	widget, ok := s.widgets[id]
	if !ok || widget.Deleted || widget.TenantID != tenantID {
		return false, ErrNotFound
	}
	widget.Deleted = true
	widget.Version++
	widget.UpdatedAt = s.now().UTC()
	s.widgets[id] = widget
	s.deleteReplays[replayKey] = struct{}{}
	return false, nil
}
`

const fullPostgresAdapterTemplate = `package postgres

import (
	"context"
	"errors"
)

var ErrPoolRequired = errors.New("postgres pool is required")

type Pinger interface {
	Ping(context.Context) error
}

type HealthChecker struct {
	Pool Pinger
}

func (h HealthChecker) Check(ctx context.Context) error {
	if h.Pool == nil {
		return ErrPoolRequired
	}
	return h.Pool.Ping(ctx)
}
`

const fullHTTPAPIOpenAPITemplate = `package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aatuh/api-toolkit/v2/routepolicy"
	"github.com/aatuh/api-toolkit/v2/specs"
)

func OpenAPIDocument() ([]byte, error) {
	registry := specs.NewRegistry(specs.Info{
		Title:       "Full SaaS API",
		Description: "Generated api-toolkit full SaaS/API profile.",
		Version:     "dev",
	})
	registry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	registry.SetSecurity([]specs.SecurityRequirement{ {Name: "ApiKeyAuth"} })
	registerSchemas(registry)
	specs.RegisterProblemCatalog(registry, nil)
	for _, operation := range operations() {
		registry.Register(operation)
	}
	doc, err := registry.OpenAPI()
	if err != nil {
		return nil, fmt.Errorf("render openapi: %w", err)
	}
	return normalizeJSON(doc)
}

func registerSchemas(registry *specs.Registry) {
	registry.RegisterSchema("Widget", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "name", "version"},
		"properties": map[string]any{
			"id":        map[string]any{"type": "string"},
			"tenant_id": map[string]any{"type": "string"},
			"name":      map[string]any{"type": "string"},
			"version":   map[string]any{"type": "integer", "format": "int64"},
		},
	})
	registry.RegisterSchema("WidgetCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"name"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		},
	})
	registry.RegisterSchema("WidgetList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Widget"}},
			"next_cursor": map[string]any{"type": "string", "nullable": true},
		},
	})
}

func operations() []specs.Operation {
	auth := func(scopes ...string) []specs.SecurityRequirement {
		return []specs.SecurityRequirement{ {Name: "ApiKeyAuth", Scopes: scopes} }
	}
	problemStatuses := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusPreconditionFailed, http.StatusTooManyRequests}
	jsonBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetCreateRequest"},
		},
	}
	widgetResponse := specs.Response{
		Description: "Widget",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Widget"},
		},
	}
	return []specs.Operation{
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getReadiness",
			Method:      http.MethodGet,
			Path:        "/readyz",
			Summary:     "Readiness",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Ready"}},
		}),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOpenAPI",
			Method:      http.MethodGet,
			Path:        "/docs/openapi.json",
			Summary:     "OpenAPI document",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "OpenAPI document"}},
		}),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getDetailedHealth",
			Method:      http.MethodGet,
			Path:        "/health/detailed",
			Summary:     "Detailed health",
			Security:    auth("admin:read"),
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Detailed health"}},
		}, routepolicy.WithAdminPolicy("admin"), routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getMetrics",
			Method:      http.MethodGet,
			Path:        "/metrics",
			Summary:     "Metrics",
			Security:    auth("admin:read"),
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Metrics"}},
		}, routepolicy.WithAdminPolicy("admin"), routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listWidgets",
			Method:      http.MethodGet,
			Path:        "/widgets",
			Summary:     "List widgets",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "cursor", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
				{Name: "limit", In: "query", Required: false, Schema: map[string]any{"type": "integer", "minimum": 1, "maximum": 100}},
			},
			Security: auth("widgets:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Widget list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/WidgetList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createWidget",
			Method:      http.MethodPost,
			Path:        "/widgets",
			Summary:     "Create widget",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: jsonBody,
			Responses:   map[int]specs.Response{http.StatusCreated: widgetResponse, http.StatusOK: widgetResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "updateWidget",
			Method:      http.MethodPatch,
			Path:        "/widgets/{id}",
			Summary:     "Update widget",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "If-Match", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: jsonBody,
			Responses:   map[int]specs.Response{http.StatusOK: widgetResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "deleteWidget",
			Method:      http.MethodDelete,
			Path:        "/widgets/{id}",
			Summary:     "Delete widget",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("widgets:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Deleted"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
	}
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
`

const fullHTTPAPIRouterTemplate = `package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/aatuh/api-toolkit/v2/httpx"

	"{{ .Module }}/internal/app"
)

type Config struct {
	Addr         string
	AdminAddr    string
	APIKey       string
	AdminKey     string
	DatabaseURL  string
	RedisAddr    string
	APIKeyPepper string
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Addr:         envDefault("API_ADDR", ":8080"),
		AdminAddr:    strings.TrimSpace(os.Getenv("ADMIN_ADDR")),
		APIKey:       envDefault("API_KEY", "local-dev-key"),
		AdminKey:     envDefault("ADMIN_KEY", "local-admin-key"),
		DatabaseURL:  strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RedisAddr:    envDefault("REDIS_ADDR", "localhost:6379"),
		APIKeyPepper: strings.TrimSpace(os.Getenv("API_KEY_PEPPER")),
	}
	if strings.EqualFold(os.Getenv("ENV"), "production") {
		var missing []string
		if cfg.DatabaseURL == "" {
			missing = append(missing, "DATABASE_URL")
		}
		if cfg.RedisAddr == "" {
			missing = append(missing, "REDIS_ADDR")
		}
		if cfg.APIKeyPepper == "" {
			missing = append(missing, "API_KEY_PEPPER")
		}
		if cfg.APIKey == "" || cfg.APIKey == "local-dev-key" {
			missing = append(missing, "API_KEY")
		}
		if cfg.AdminKey == "" || cfg.AdminKey == "local-admin-key" {
			missing = append(missing, "ADMIN_KEY")
		}
		if len(missing) > 0 {
			return Config{}, errors.New("production configuration missing: " + strings.Join(missing, ", "))
		}
	}
	return cfg, nil
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

type RouterConfig struct {
	Widgets  *app.WidgetService
	APIKey   string
	AdminKey string
}

func NewRouter(cfg RouterConfig) http.Handler {
	cfg = cfg.withDefaults()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", handleReady)
	mux.HandleFunc("GET /docs/openapi.json", handleOpenAPI)
	mux.HandleFunc("GET /widgets", cfg.handleListWidgets)
	mux.HandleFunc("POST /widgets", cfg.handleCreateWidget)
	mux.HandleFunc("PATCH /widgets/{id}", cfg.handleUpdateWidget)
	mux.HandleFunc("DELETE /widgets/{id}", cfg.handleDeleteWidget)
	return mux
}

func NewAdminRouter(cfg RouterConfig) http.Handler {
	cfg = cfg.withDefaults()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/detailed", cfg.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))
	mux.HandleFunc("GET /metrics", cfg.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte("# HELP api_requests_total Total API requests\n# TYPE api_requests_total counter\napi_requests_total 0\n"))
	}))
	mux.HandleFunc("GET /debug/pprof/", cfg.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"pprof": "mounted on the admin listener"})
	}))
	return mux
}

func (cfg RouterConfig) withDefaults() RouterConfig {
	if cfg.Widgets == nil {
		cfg.Widgets = app.NewWidgetService()
	}
	if cfg.APIKey == "" {
		cfg.APIKey = "local-dev-key"
	}
	if cfg.AdminKey == "" {
		cfg.AdminKey = "local-admin-key"
	}
	return cfg
}

func handleReady(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	doc, err := OpenAPIDocument()
	if err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "openapi document unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc)
}

func (cfg RouterConfig) handleListWidgets(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	widgets, err := cfg.Widgets.List(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(widgets))
	for _, widget := range widgets {
		items = append(items, widget.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

func (cfg RouterConfig) handleCreateWidget(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	req, ok := decodeWidgetRequest(w, r)
	if !ok {
		return
	}
	widget, replayed, err := cfg.Widgets.Create(r.Context(), tenantID, req.Name, idempotencyKey)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("ETag", widget.ETag())
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, widget.Public())
}

func (cfg RouterConfig) handleUpdateWidget(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	ifMatch, ok := requireHeader(w, r, "If-Match")
	if !ok {
		return
	}
	req, ok := decodeWidgetRequest(w, r)
	if !ok {
		return
	}
	widget, _, err := cfg.Widgets.Update(r.Context(), tenantID, r.PathValue("id"), req.Name, ifMatch, idempotencyKey)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("ETag", widget.ETag())
	writeJSON(w, http.StatusOK, widget.Public())
}

func (cfg RouterConfig) handleDeleteWidget(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	if _, err := cfg.Widgets.Delete(r.Context(), tenantID, r.PathValue("id"), idempotencyKey); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type widgetRequest struct {
	Name string
}

func decodeWidgetRequest(w http.ResponseWriter, r *http.Request) (widgetRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]string
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return widgetRequest{}, false
	}
	name := strings.TrimSpace(raw["name"])
	if name == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "name is required"})
		return widgetRequest{}, false
	}
	return widgetRequest{Name: name}, true
}

func (cfg RouterConfig) authenticateTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !sameSecret(r.Header.Get("X-API-Key"), cfg.APIKey) {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid API key required"})
		return "", false
	}
	tenantID, ok := requireHeader(w, r, "X-Tenant-ID")
	if !ok {
		return "", false
	}
	return tenantID, true
}

func (cfg RouterConfig) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameSecret(r.Header.Get("X-Admin-Key"), cfg.AdminKey) {
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "admin authentication required"})
			return
		}
		next(w, r)
	}
}

func requireHeader(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	value := strings.TrimSpace(r.Header.Get(name))
	if value == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: name + " header is required"})
		return "", false
	}
	return value, true
}

func sameSecret(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func writeAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrValidation):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "request validation failed"})
	case errors.Is(err, app.ErrNotFound):
		httpx.WriteProblem(w, http.StatusNotFound, httpx.Problem{Title: http.StatusText(http.StatusNotFound), Detail: "resource not found"})
	case errors.Is(err, app.ErrPreconditionFailed):
		httpx.WriteProblem(w, http.StatusPreconditionFailed, httpx.Problem{Title: http.StatusText(http.StatusPreconditionFailed), Detail: "If-Match does not match current resource version"})
	default:
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "request failed"})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
`

const fullHTTPAPIRouterTestTemplate = `package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"{{ .Module }}/internal/app"
)

func TestReadinessAndOpenAPI(t *testing.T) {
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), APIKey: "test-key"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"operationId\": \"createWidget\"") {
		t.Fatalf("openapi missing createWidget operation: %s", rec.Body.String())
	}
}

func TestCreateWidgetRequiresAuth(t *testing.T) {
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), APIKey: "test-key"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"alpha\"}"))
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", "idem_1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/problem+json") {
		t.Fatalf("auth failure content type = %q", got)
	}
}

func TestCreateWidgetValidatesBody(t *testing.T) {
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), APIKey: "test-key"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"\"}"))
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", "idem_1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("validation status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateWidgetReplaysIdempotencyKey(t *testing.T) {
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), APIKey: "test-key"})
	first := createWidget(t, handler, "idem_1")
	second := createWidget(t, handler, "idem_1")
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d body=%s", first.Code, first.Body.String())
	}
	if second.Code != http.StatusOK {
		t.Fatalf("second replay status = %d body=%s", second.Code, second.Body.String())
	}
	var firstBody, secondBody map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decode first body: %v", err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode second body: %v", err)
	}
	if firstBody["id"] != secondBody["id"] {
		t.Fatalf("idempotent replay changed id: first=%v second=%v", firstBody["id"], secondBody["id"])
	}
	if first.Header().Get("ETag") == "" || second.Header().Get("ETag") == "" {
		t.Fatalf("missing ETag on create/replay")
	}
}

func TestUpdateWidgetRequiresMatchingETag(t *testing.T) {
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), APIKey: "test-key"})
	created := createWidget(t, handler, "idem_create")
	var body map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode created body: %v", err)
	}
	id, _ := body["id"].(string)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/widgets/"+id, strings.NewReader("{\"name\":\"beta\"}"))
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", "idem_update")
	req.Header.Set("If-Match", "\"999\"")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("conflict status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOpenAPIGolden(t *testing.T) {
	got, err := OpenAPIDocument()
	if err != nil {
		t.Fatalf("render openapi: %v", err)
	}
	goldenPath := filepath.Join("..", "..", "testdata", "openapi.golden.json")
	if os.Getenv("UPDATE_OPENAPI") == "1" {
		if err := os.WriteFile(goldenPath, got, 0o600); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("openapi golden drift\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func createWidget(t *testing.T, handler http.Handler, idem string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"alpha\"}"))
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", idem)
	handler.ServeHTTP(rec, req)
	return rec
}
`

const fullMigrationTemplate = `CREATE TABLE organizations (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE invitations (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	email TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
	token_hash BYTEA NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	accepted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	prefix TEXT NOT NULL,
	key_hash BYTEA NOT NULL,
	scopes TEXT[] NOT NULL DEFAULT '{}',
	expires_at TIMESTAMPTZ,
	last_used_at TIMESTAMPTZ,
	revoked_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE widgets (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	version BIGINT NOT NULL DEFAULT 1,
	deleted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE operations (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	state TEXT NOT NULL,
	result JSONB,
	error JSONB,
	lease_owner TEXT,
	lease_expires_at TIMESTAMPTZ,
	retry_count INTEGER NOT NULL DEFAULT 0,
	next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	event_type TEXT NOT NULL,
	payload JSONB NOT NULL,
	state TEXT NOT NULL DEFAULT 'pending',
	lease_owner TEXT,
	lease_expires_at TIMESTAMPTZ,
	retry_count INTEGER NOT NULL DEFAULT 0,
	next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_events (
	id TEXT PRIMARY KEY,
	organization_id TEXT,
	actor_type TEXT NOT NULL,
	actor_id TEXT,
	action TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	resource_id TEXT,
	result TEXT NOT NULL,
	request_id TEXT,
	metadata JSONB NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_endpoints (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	url TEXT NOT NULL,
	event_types TEXT[] NOT NULL,
	secret_hash BYTEA NOT NULL,
	disabled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_deliveries (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	endpoint_id TEXT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
	event_type TEXT NOT NULL,
	payload JSONB NOT NULL,
	state TEXT NOT NULL DEFAULT 'pending',
	attempts INTEGER NOT NULL DEFAULT 0,
	next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_error TEXT,
	delivered_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

const fullMakefileTemplate = `GO ?= go
API_TOOLKIT ?= $(GO) run -mod=mod github.com/aatuh/api-toolkit/contrib/v2/cmd/api-toolkit
OPENAPI ?= testdata/openapi.golden.json
OPENAPI_BASE ?= $(OPENAPI)
COMPOSE ?= docker compose

.PHONY: test fmt build openapi-check openapi-update contracts-lint contracts-diff client-check integration-check clean finalize

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

build:
	$(GO) build -trimpath -o bin/api ./cmd/api

openapi-check:
	$(GO) test ./internal/httpapi -run TestOpenAPIGolden

openapi-update:
	UPDATE_OPENAPI=1 $(GO) test ./internal/httpapi -run TestOpenAPIGolden

contracts-lint:
	$(API_TOOLKIT) contracts lint --openapi $(OPENAPI)

contracts-diff:
	$(API_TOOLKIT) contracts diff --base $(OPENAPI_BASE) --head $(OPENAPI)

client-check:
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	cp internal/client/apiclient/client.go "$$tmp/client.go"; \
	$(API_TOOLKIT) clients go --openapi $(OPENAPI) --out internal/client/apiclient --package apiclient; \
	cmp -s "$$tmp/client.go" internal/client/apiclient/client.go || { echo "generated Go client is out of date"; diff -u "$$tmp/client.go" internal/client/apiclient/client.go; exit 1; }
	$(GO) test ./internal/client/apiclient

integration-check:
	$(COMPOSE) up -d postgres redis
	$(GO) test ./...
	$(COMPOSE) down -v

clean:
	$(GO) clean -testcache

finalize: fmt test build openapi-check contracts-lint contracts-diff clean
`

const fullEnvTemplate = `ENV=development
API_ADDR=:8080
ADMIN_ADDR=:9090
DATABASE_URL=
REDIS_ADDR=localhost:6379
API_KEY=local-dev-key
API_KEY_PEPPER=
ADMIN_KEY=local-admin-key
IDEMPOTENCY_KEY_PREFIX=idempotency:
RATE_LIMIT_KEY_PREFIX=ratelimit:
OIDC_ISSUER=
OIDC_AUDIENCE=saas-api-full
OIDC_JWKS_URL=
OIDC_TENANT_CLAIM=tenant_id
`

const fullGitignoreTemplate = `.env
.env.*
!.env.example
bin/
coverage.out
.ci-result/
.tools/
tmp/
api
*.test
internal/client/apiclient/
`

const fullDockerignoreTemplate = `.git
.env
.env.*
!.env.example
bin/
coverage.out
.ci-result
.tools
tmp/
`

const fullCIWorkflowTemplate = `name: ci

on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
  verify:
    runs-on: ubuntu-latest
    env:
      GOTOOLCHAIN: local
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # actions/checkout v4
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # actions/setup-go v5
        with:
          go-version: 1.25.x
          check-latest: true
      - name: Finalize
        run: make finalize
`

const fullDockerfileTemplate = `FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/api /api
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/api"]
`

// #nosec G101 -- generated compose credentials are local development placeholders documented as non-production defaults.
const fullComposeTemplate = `services:
  api:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"
    env_file:
      - .env
    environment:
      DATABASE_URL: postgres://api:api@postgres:5432/api?sslmode=disable
      REDIS_ADDR: redis:6379
      ADMIN_ADDR: :9090
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_USER: api
      POSTGRES_PASSWORD: api
      POSTGRES_DB: api
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U api -d api"]
      interval: 5s
      timeout: 3s
      retries: 10
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10
  minio:
    image: minio/minio:latest
    profiles: [objectstore]
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minio
      MINIO_ROOT_PASSWORD: minio123
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio-data:/data

volumes:
  postgres-data:
  redis-data:
  minio-data:
`

const fullKubernetesDeploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
        - name: api
          image: example/api:dev
          ports:
            - name: public
              containerPort: 8080
            - name: admin
              containerPort: 9090
          env:
            - name: API_ADDR
              value: ":8080"
            - name: ADMIN_ADDR
              value: ":9090"
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: database-url
            - name: REDIS_ADDR
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: redis-addr
            - name: API_KEY_PEPPER
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: api-key-pepper
          readinessProbe:
            httpGet:
              path: /readyz
              port: public
          livenessProbe:
            httpGet:
              path: /readyz
              port: public
`

const fullKubernetesServiceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: api
spec:
  selector:
    app: api
  ports:
    - name: http
      port: 80
      targetPort: public
`

const fullKubernetesAdminServiceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: api-admin
spec:
  selector:
    app: api
  ports:
    - name: admin
      port: 9090
      targetPort: admin
`

const fullReadmeTemplate = `# Generated api-toolkit Full SaaS API

Generated profile: ` + "`{{ .Profile }}`" + `.
Generated auth mode: ` + "`{{ .AuthMode }}`" + `.

Run locally:

` + "```sh" + `
go test ./...
go run ./cmd/api
` + "```" + `

Postgres stores tenants, API keys, widgets, operations, outbox, audit, and webhook delivery state.
Redis is reserved for shared idempotency, rate limiting, and cache state.
The generated HTTP layer starts with API-key tenant isolation and tenant-scoped idempotent widget writes; JWT, Clerk, and OIDC modes are accepted by the generator and are wired in later platform slices.

Useful checks:

` + "```sh" + `
make openapi-check
make contracts-lint
make contracts-diff
make integration-check
` + "```" + `

` + "`make integration-check`" + ` is opt-in and starts Postgres and Redis through Docker Compose. The default finalize target stays local and deterministic.

Admin routes are intended for a separate listener when ` + "`ADMIN_ADDR`" + ` is set. Keep ` + "`/health/detailed`" + `, ` + "`/metrics`" + `, and ` + "`/debug/pprof/`" + ` behind admin authentication and network isolation.
`

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
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/ratelimitredis"
	"github.com/aatuh/api-toolkit/contrib/v2/bootstrap"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	requestlog "github.com/aatuh/api-toolkit/contrib/v2/middleware/requestlog"
	"github.com/aatuh/api-toolkit/contrib/v2/telemetry"
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

var (
	appVersion  = "dev"
	buildCommit = "unknown"
	buildDate   = "unknown"
)

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
	tracingShutdown, err := newTracingShutdown(context.Background())
	if err != nil {
		return nil, err
	}
	metricsRecorder, err := metricsmw.NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		return nil, err
	}
	routerConfig, err := bootstrap.DefaultRouterConfigFromEnv(nil)
	if err != nil {
		return nil, err
	}
	routerConfig.Metrics = metricsRecorder
	rateLimiter, rateLimitShutdown, err := newRateLimitLimiter(routerConfig.RateLimit.Capacity, routerConfig.RateLimit.RefillRate)
	if err != nil {
		return nil, err
	}
	if rateLimiter != nil {
		routerConfig.RateLimit.Limiter = rateLimiter
	}
	router, err := bootstrap.NewDefaultRouterWithConfig(log, routerConfig)
	if err != nil {
		return nil, err
	}

specRegistry := specs.NewRegistry(specs.Info{Title: "SaaS API", Version: appVersion})
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
		Version:     appVersion,
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
	idempotencyStore, idempotencyShutdown, err := newIdempotencyStore()
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
	shutdownHooks := []bootstrap.ShutdownHook{}
	if tracingShutdown.Hook != nil {
		shutdownHooks = append(shutdownHooks, tracingShutdown)
	}
	if rateLimitShutdown.Hook != nil {
		shutdownHooks = append(shutdownHooks, rateLimitShutdown)
	}
	if idempotencyShutdown.Hook != nil {
		shutdownHooks = append(shutdownHooks, idempotencyShutdown)
	}
{{ if eq .AuthMode "jwt" }}	shutdownHooks = append(shutdownHooks, bootstrap.ShutdownHook{Name: "jwt", Hook: func(context.Context) error {
		jwtMiddleware.Close()
		return nil
	}})
{{ else if eq .AuthMode "clerk" }}	shutdownHooks = append(shutdownHooks, bootstrap.ShutdownHook{Name: "clerk", Hook: func(context.Context) error {
		clerkMiddleware.Close()
		return nil
	}})
{{ end }}

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
		ShutdownHooks: shutdownHooks,
		SystemEndpoints: bootstrap.SystemEndpoints{
			Health:  health.NewHandler(healthManager),
			Docs:    docs.NewHandler(docsManager),
			Version: version.NewHandler(version.Config{Info: ports.VersionInfo{Version: appVersion, Commit: buildCommit, Date: buildDate}}),
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

func newTracingShutdown(ctx context.Context) (bootstrap.ShutdownHook, error) {
	cfg := telemetry.TraceConfigFromEnv()
	if cfg.Enabled && strings.TrimSpace(cfg.Endpoint) == "" {
		return bootstrap.ShutdownHook{}, errors.New("OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_TRACING_ENABLED=true")
	}
	shutdown, enabled, err := telemetry.InitTracing(ctx, cfg)
	if err != nil {
		return bootstrap.ShutdownHook{}, err
	}
	if !enabled {
		return bootstrap.ShutdownHook{}, nil
	}
	return bootstrap.ShutdownHook{Name: "otel-tracing", Hook: shutdown}, nil
}

func newRateLimitLimiter(capacity, refillRate float64) (ports.RateLimiter, bootstrap.ShutdownHook, error) {
	store := strings.ToLower(strings.TrimSpace(os.Getenv("RATE_LIMIT_STORE")))
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
			return nil, bootstrap.ShutdownHook{}, errors.New("RATE_LIMIT_STORE=memory is not allowed when ENV=production; use redis")
		}
		return nil, bootstrap.ShutdownHook{}, nil
	case "redis":
		addr := strings.TrimSpace(os.Getenv("RATE_LIMIT_REDIS_ADDR"))
		if addr == "" {
			addr = strings.TrimSpace(os.Getenv("REDIS_ADDR"))
		}
		if addr == "" {
			if isProduction() {
				return nil, bootstrap.ShutdownHook{}, errors.New("RATE_LIMIT_REDIS_ADDR or REDIS_ADDR is required when RATE_LIMIT_STORE=redis")
			}
			addr = "localhost:6379"
		}
		addrs := splitCSV(addr)
		if len(addrs) == 0 {
			return nil, bootstrap.ShutdownHook{}, errors.New("RATE_LIMIT_REDIS_ADDR or REDIS_ADDR must include at least one address")
		}
		client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
		return ratelimitredis.New(client, ratelimitredis.Options{
			Capacity:   capacity,
			RefillRate: refillRate,
			KeyPrefix:  env("RATE_LIMIT_KEY_PREFIX", "ratelimit:"),
		}), bootstrap.ShutdownHook{Name: "rate-limit-redis", Hook: func(context.Context) error {
			return client.Close()
		}}, nil
	default:
		return nil, bootstrap.ShutdownHook{}, fmt.Errorf("unsupported RATE_LIMIT_STORE %q", store)
	}
}

func newIdempotencyStore() (ports.IdempotencyStore, bootstrap.ShutdownHook, error) {
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
			return nil, bootstrap.ShutdownHook{}, errors.New("IDEMPOTENCY_STORE=memory is not allowed when ENV=production; use redis")
		}
		return idempotency.NewMemoryStore(), bootstrap.ShutdownHook{}, nil
	case "redis":
		addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
		if addr == "" {
			if isProduction() {
				return nil, bootstrap.ShutdownHook{}, errors.New("REDIS_ADDR is required when IDEMPOTENCY_STORE=redis")
			}
			addr = "localhost:6379"
		}
		addrs := splitCSV(addr)
		if len(addrs) == 0 {
			return nil, bootstrap.ShutdownHook{}, errors.New("REDIS_ADDR must include at least one address")
		}
		client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
		return idempotencyredis.New(client, idempotencyredis.Options{
			KeyPrefix: env("IDEMPOTENCY_KEY_PREFIX", "idempotency:"),
		}), bootstrap.ShutdownHook{Name: "idempotency-redis", Hook: func(context.Context) error {
			return client.Close()
		}}, nil
	default:
		return nil, bootstrap.ShutdownHook{}, fmt.Errorf("unsupported IDEMPOTENCY_STORE %q", store)
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
	"context"
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

	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, specs.Version, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("version status = %d body=%s", rec.Code, rec.Body.String())
	}
	var versionInfo map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &versionInfo); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	for key, want := range map[string]string{"version": "dev", "commit": "unknown", "date": "unknown"} {
		if versionInfo[key] != want {
			t.Fatalf("version %s = %q, want %q", key, versionInfo[key], want)
		}
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

func TestGeneratedVersionEndpointUsesBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := appVersion, buildCommit, buildDate
	appVersion = "1.2.3"
	buildCommit = "abc123"
	buildDate = "2026-05-13T00:00:00Z"
	t.Cleanup(func() {
		appVersion, buildCommit, buildDate = oldVersion, oldCommit, oldDate
	})
	setLocalTestEnv(t)
	service, err := newService()
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	rec := httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, specs.Version, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("version status = %d body=%s", rec.Code, rec.Body.String())
	}
	var versionInfo map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &versionInfo); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	for key, want := range map[string]string{"version": "1.2.3", "commit": "abc123", "date": "2026-05-13T00:00:00Z"} {
		if versionInfo[key] != want {
			t.Fatalf("version %s = %q, want %q", key, versionInfo[key], want)
		}
	}
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

func TestGeneratedIdempotencyRedisStoreHasShutdownHook(t *testing.T) {
	setLocalTestEnv(t)
	t.Setenv("IDEMPOTENCY_STORE", "redis")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	_, hook, err := newIdempotencyStore()
	if err != nil {
		t.Fatalf("new redis idempotency store: %v", err)
	}
	if hook.Name != "idempotency-redis" || hook.Hook == nil {
		t.Fatalf("redis idempotency shutdown hook = %#v", hook)
	}
	if err := hook.Hook(context.Background()); err != nil {
		t.Fatalf("redis idempotency shutdown hook failed: %v", err)
	}
}

func TestGeneratedRateLimitRedisLimiterHasShutdownHook(t *testing.T) {
	setLocalTestEnv(t)
	t.Setenv("RATE_LIMIT_STORE", "redis")
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "localhost:6379")
	limiter, hook, err := newRateLimitLimiter(30, 15)
	if err != nil {
		t.Fatalf("new redis rate limiter: %v", err)
	}
	if limiter == nil {
		t.Fatal("redis rate limiter = nil")
	}
	if hook.Name != "rate-limit-redis" || hook.Hook == nil {
		t.Fatalf("redis rate-limit shutdown hook = %#v", hook)
	}
	if err := hook.Hook(context.Background()); err != nil {
		t.Fatalf("redis rate-limit shutdown hook failed: %v", err)
	}
}

func TestGeneratedTracingRequiresEndpointWhenEnabled(t *testing.T) {
	setLocalTestEnv(t)
	t.Setenv("OTEL_TRACING_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if _, err := newTracingShutdown(context.Background()); err == nil {
		t.Fatal("expected tracing startup to require an OTLP endpoint when enabled")
	} else if !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Fatalf("startup error = %v, want OTLP endpoint requirement", err)
	}
}

{{ if eq .AuthMode "jwt" }}func TestGeneratedServiceRejectsProductionMissingJWTConfig(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "localhost:6379")
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
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "localhost:6379")
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
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "localhost:6379")
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
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "localhost:6379")
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
func TestGeneratedServiceRejectsProductionMemoryRateLimit(t *testing.T) {
	t.Setenv("ENV", "production")
	setProductionAuthEnv(t)
	t.Setenv("ADMIN_KEY", "prod-admin-key")
	t.Setenv("RATE_LIMIT_STORE", "memory")
	t.Setenv("IDEMPOTENCY_STORE", "redis")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to reject memory rate limiting")
	} else if !strings.Contains(err.Error(), "RATE_LIMIT_STORE=memory") {
		t.Fatalf("startup error = %v, want memory rate-limit rejection", err)
	}
}

func TestGeneratedServiceRejectsProductionMissingRateLimitRedisAddress(t *testing.T) {
	t.Setenv("ENV", "production")
	setProductionAuthEnv(t)
	t.Setenv("ADMIN_KEY", "prod-admin-key")
	t.Setenv("RATE_LIMIT_STORE", "redis")
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "")
	t.Setenv("REDIS_ADDR", "")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require Redis rate-limit address")
	} else if !strings.Contains(err.Error(), "RATE_LIMIT_REDIS_ADDR") {
		t.Fatalf("startup error = %v, want rate-limit Redis address requirement", err)
	}
}

func TestGeneratedServiceRejectsProductionMemoryIdempotency(t *testing.T) {
	t.Setenv("ENV", "production")
	setProductionAuthEnv(t)
	t.Setenv("ADMIN_KEY", "prod-admin-key")
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "localhost:6379")
	t.Setenv("IDEMPOTENCY_STORE", "memory")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to reject memory idempotency")
	} else if !strings.Contains(err.Error(), "IDEMPOTENCY_STORE=memory") {
		t.Fatalf("startup error = %v, want memory-store rejection", err)
	}
}

func TestGeneratedServiceRejectsProductionMissingRedisAddress(t *testing.T) {
	t.Setenv("ENV", "production")
	setProductionAuthEnv(t)
	t.Setenv("ADMIN_KEY", "prod-admin-key")
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "localhost:6379")
	t.Setenv("IDEMPOTENCY_STORE", "redis")
	t.Setenv("REDIS_ADDR", "")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require Redis address")
	} else if !strings.Contains(err.Error(), "REDIS_ADDR") {
		t.Fatalf("startup error = %v, want REDIS_ADDR requirement", err)
	}
}
{{ end }}

{{ if ne .AuthMode "dev-headers" }}func setProductionAuthEnv(t *testing.T) {
	t.Helper()
{{ if eq .AuthMode "jwt" }}	setJWTAuthEnv(t)
{{ else if eq .AuthMode "clerk" }}	setClerkAuthEnv(t)
{{ else }}	t.Setenv("API_KEY", "prod-api-key")
	t.Setenv("API_TENANT_ID", "tenant_1")
{{ end }}}

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
OUTPUT_DIR ?= .ci-result
TOOLS_DIR ?= .tools
SYFT ?= syft
COVERAGE_MIN ?= 70.0
GOVULNCHECK ?= $(CURDIR)/$(TOOLS_DIR)/bin/govulncheck
GOVULNCHECK_VERSION ?= v1.2.0
VERSION ?= dev
BUILD_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
LDFLAGS ?= -s -w -X main.appVersion=$(VERSION) -X main.buildCommit=$(BUILD_COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: tools test fmt build coverage coverage-check test-race vuln openapi-check openapi-update contracts-lint contracts-diff fast-check audit-check sbom-local clean finalize

tools: $(GOVULNCHECK)

$(GOVULNCHECK):
	mkdir -p "$(CURDIR)/$(TOOLS_DIR)/bin"
	GOBIN="$(CURDIR)/$(TOOLS_DIR)/bin" $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/api .

coverage:
	$(GO) test ./... -coverprofile=coverage.out

coverage-check: coverage
	@coverage="$$($(GO) tool cover -func=coverage.out | awk '/^total:/ {gsub(/%/, "", $$3); print $$3}')"; \
	awk -v got="$$coverage" -v min="$(COVERAGE_MIN)" 'BEGIN { if ((got + 0) < (min + 0)) { printf "coverage %.1f%% below required %.1f%%\n", got, min; exit 1 } printf "coverage %.1f%% >= %.1f%%\n", got, min }'

test-race:
	$(GO) test ./... -race -count=1

vuln: tools
	"$(GOVULNCHECK)" ./...

openapi-check:
	$(GO) test ./... -run TestGeneratedServiceOpenAPIGolden

openapi-update:
	$(GO) test ./... -run TestGeneratedServiceOpenAPIGolden -update-openapi

contracts-lint:
	$(API_TOOLKIT) contracts lint --openapi $(OPENAPI)

contracts-diff:
	$(API_TOOLKIT) contracts diff --base $(OPENAPI_BASE) --head $(OPENAPI)

fast-check: test build openapi-check contracts-lint contracts-diff

audit-check: coverage-check test-race build openapi-check contracts-lint contracts-diff vuln

sbom-local:
	rm -rf "$(OUTPUT_DIR)/sbom"
	mkdir -p "$(OUTPUT_DIR)/sbom"
	"$(SYFT)" dir:. -o spdx-json >"$(OUTPUT_DIR)/sbom/sbom.spdx.json"

clean:
	$(GO) clean -testcache

finalize: fmt audit-check clean
`

const ciWorkflowTemplate = `name: ci

on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
  verify:
    runs-on: ubuntu-latest
    env:
      GOTOOLCHAIN: local
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # actions/checkout v4
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # actions/setup-go v5
        with:
          go-version: 1.25.x
          check-latest: true
      - name: Finalize
        run: make finalize
`

const envTemplate = `ENV=development
API_ADDR=:8080
TRUSTED_PROXIES=
RATE_LIMIT_SKIP_ENABLED=false
RATE_LIMIT_SKIP_HEADER=
RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES=false
RATE_LIMIT_STORE=memory
RATE_LIMIT_REDIS_ADDR=
RATE_LIMIT_KEY_PREFIX=ratelimit:
OTEL_TRACING_ENABLED=false
OTEL_SERVICE_NAME=api
OTEL_EXPORTER_OTLP_ENDPOINT=
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_SAMPLE_RATIO=1
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
.tools/
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
.tools
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
ARG VERSION=dev
ARG BUILD_COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.appVersion=${VERSION} -X main.buildCommit=${BUILD_COMMIT} -X main.buildDate=${BUILD_DATE}" -o /out/api .

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
    environment:
      REDIS_ADDR: redis:6379
      RATE_LIMIT_REDIS_ADDR: redis:6379
    depends_on:
      redis:
        condition: service_healthy
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  redis-data:
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

Run the API with its local Redis dependency:

` + "```sh" + `
docker compose up --build
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
Local development uses ` + "`RATE_LIMIT_STORE=memory`" + `. In production, the generated service defaults to ` + "`RATE_LIMIT_STORE=redis`" + ` and requires ` + "`RATE_LIMIT_REDIS_ADDR`" + ` or ` + "`REDIS_ADDR`" + ` so rate limits are shared across instances.
OpenTelemetry tracing is disabled by default with ` + "`OTEL_TRACING_ENABLED=false`" + `. ` + "`OTEL_EXPORTER_OTLP_ENDPOINT`" + ` is required when tracing is enabled, and the tracer provider is closed through the service shutdown hooks.
` + "`make build`" + ` produces ` + "`bin/api`" + ` and stamps ` + "`/version`" + ` with ` + "`VERSION`" + `, ` + "`BUILD_COMMIT`" + `, and ` + "`BUILD_DATE`" + `. The Dockerfile accepts matching build args and defaults to ` + "`dev`" + `/` + "`unknown`" + ` metadata.
Generated CI runs ` + "`make finalize`" + ` on pushes and pull requests with a pinned checkout/setup-go workflow. The generated Makefile also includes ` + "`make fast-check`" + ` for local iteration, ` + "`make audit-check`" + ` for coverage floor, race, vulnerability, OpenAPI, and contract review, and optional ` + "`make sbom-local`" + ` for SPDX JSON SBOM output under ` + "`.ci-result/sbom`" + ` when Syft is installed. Generated Go tools are installed under ` + "`.tools/bin`" + ` by default.
Local ` + "`.env`" + ` files, coverage output, temporary files, and built binaries are ignored by default.
`
