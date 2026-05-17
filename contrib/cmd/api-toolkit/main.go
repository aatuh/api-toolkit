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
	"os/exec"
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

const defaultScaffoldModuleVersion = "v2.1.0"

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
		fmt.Fprintln(stderr, "usage: api-toolkit <new|generate|contracts|clients|version>")
		return 2
	}
	switch args[0] {
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "new":
		return runNew(ctx, args[1:], stdout, stderr)
	case "generate":
		return runGenerate(ctx, args[1:], stdout, stderr)
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

func scaffoldDependencyVersion(info versionMetadata) string {
	for _, candidate := range []string{info.ContribVersion, info.MainVersion, info.CoreVersion} {
		if isSemVerModuleVersion(candidate) {
			return candidate
		}
	}
	return defaultScaffoldModuleVersion
}

func isSemVerModuleVersion(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "v") {
		return false
	}
	rest := strings.TrimPrefix(value, "v")
	parts := strings.SplitN(rest, ".", 3)
	if len(parts) != 3 {
		return false
	}
	if !isNonEmptyDigits(parts[0]) || !isNonEmptyDigits(parts[1]) {
		return false
	}
	patch := parts[2]
	if patch == "" {
		return false
	}
	digitCount := 0
	for digitCount < len(patch) && patch[digitCount] >= '0' && patch[digitCount] <= '9' {
		digitCount++
	}
	if digitCount == 0 {
		return false
	}
	if digitCount == len(patch) {
		return true
	}
	return patch[digitCount] == '-' || patch[digitCount] == '+'
}

func isNonEmptyDigits(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func runNew(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "service" {
		fmt.Fprintln(stderr, "usage: api-toolkit new service --module <module> [--dir <path>] [--profile saas-api|saas-api-full|dev-api] [--auth api-key|jwt|clerk|oidc|dev-headers] [--with stripe-billing|resend-email|clerk-webhooks]")
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
	var providerWorkflows providerWorkflowFlag
	fs.Var(&providerWorkflows, "with", "optional provider workflow for saas-api-full; repeatable: stripe-billing, resend-email, clerk-webhooks")
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
	providers, err := validateScaffoldProviderWorkflows(profileName, providerWorkflows.Values())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cfg := scaffoldConfig{
		Module:         strings.TrimSpace(*module),
		Dir:            strings.TrimSpace(*dir),
		Profile:        profileName,
		AuthMode:       authName,
		Providers:      providers,
		CoreReplace:    strings.TrimSpace(*coreReplace),
		ContribReplace: strings.TrimSpace(*contribReplace),
		ToolkitVersion: scaffoldDependencyVersion(collectVersionMetadata()),
	}
	if err := generateService(cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "created %s\n", cfg.Dir)
	return 0
}

func runGenerate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "resource" {
		fmt.Fprintln(stderr, "usage: api-toolkit generate resource --name <singular> --plural <plural> --tenant-scoped --crud [--postgres] [--soft-delete] [--etag] [--audit] [--webhooks] [--dir <path>]")
		return 2
	}
	fs := flag.NewFlagSet("generate resource", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "generated service directory")
	name := fs.String("name", "", "singular resource name")
	plural := fs.String("plural", "", "plural resource name")
	tenantScoped := fs.Bool("tenant-scoped", false, "generate tenant-scoped resource")
	crud := fs.Bool("crud", false, "generate CRUD endpoints")
	postgres := fs.Bool("postgres", false, "generate Postgres adapter and migration")
	softDelete := fs.Bool("soft-delete", false, "generate soft delete semantics")
	etag := fs.Bool("etag", false, "generate optimistic concurrency with ETags")
	audit := fs.Bool("audit", false, "record audit hooks")
	webhooks := fs.Bool("webhooks", false, "emit webhook hooks")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if err := ctx.Err(); err != nil {
		fmt.Fprintf(stderr, "context canceled: %v\n", err)
		return 1
	}
	cfg := resourceConfig{
		Dir:          strings.TrimSpace(*dir),
		Name:         strings.TrimSpace(*name),
		Plural:       strings.TrimSpace(*plural),
		TenantScoped: *tenantScoped,
		CRUD:         *crud,
		Postgres:     *postgres,
		SoftDelete:   *softDelete,
		ETag:         *etag,
		Audit:        *audit,
		Webhooks:     *webhooks,
	}
	if err := generateResource(ctx, cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "generated resource %s\n", cfg.Plural)
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
	scaffoldProviderStripe     = "stripe-billing"
	scaffoldProviderResend     = "resend-email"
	scaffoldProviderClerkHooks = "clerk-webhooks"
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

type providerWorkflowFlag struct {
	values []string
}

func (f *providerWorkflowFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(f.values, ",")
}

func (f *providerWorkflowFlag) Set(value string) error {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return errors.New("provider workflow must not be empty")
	}
	f.values = append(f.values, value)
	return nil
}

func (f providerWorkflowFlag) Values() []string {
	return append([]string(nil), f.values...)
}

func validateScaffoldProviderWorkflows(profile string, workflows []string) ([]string, error) {
	if len(workflows) == 0 {
		return nil, nil
	}
	profile = strings.TrimSpace(profile)
	if profile != scaffoldProfileSaaSAPIFull {
		return nil, fmt.Errorf("provider workflows require profile %q", scaffoldProfileSaaSAPIFull)
	}
	seen := map[string]bool{}
	for _, workflow := range workflows {
		workflow = strings.ToLower(strings.TrimSpace(workflow))
		switch workflow {
		case scaffoldProviderStripe, scaffoldProviderResend, scaffoldProviderClerkHooks:
			seen[workflow] = true
		default:
			return nil, fmt.Errorf("unsupported provider workflow %q", workflow)
		}
	}
	ordered := make([]string, 0, len(seen))
	for _, workflow := range []string{scaffoldProviderStripe, scaffoldProviderResend, scaffoldProviderClerkHooks} {
		if seen[workflow] {
			ordered = append(ordered, workflow)
		}
	}
	return ordered, nil
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
		clientFindings := lintGoClientCompatibility(loaded.doc, operations)
		if len(findings) > 0 || len(securityFindings) > 0 || len(clientFindings) > 0 {
			for _, finding := range findings {
				fmt.Fprintln(stderr, finding.Error())
			}
			for _, finding := range securityFindings {
				fmt.Fprintln(stderr, finding.Error())
			}
			for _, finding := range clientFindings {
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
		fmt.Fprintln(stderr, "usage: api-toolkit clients go --openapi <openapi.json> --out <dir> --package <name> [--style raw|typed]")
		return 2
	}
	fs := flag.NewFlagSet("clients go", flag.ContinueOnError)
	fs.SetOutput(stderr)
	openAPIPath := fs.String("openapi", "", "OpenAPI JSON file")
	outDir := fs.String("out", "", "output directory")
	packageName := fs.String("package", "apiclient", "Go package name")
	style := fs.String("style", goClientStyleRaw, "Go client style: raw or typed")
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
		Style:       strings.ToLower(strings.TrimSpace(*style)),
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
	Style       string
}

const (
	goClientStyleRaw   = "raw"
	goClientStyleTyped = "typed"
)

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
	if cfg.Style == "" {
		cfg.Style = goClientStyleRaw
	}
	if cfg.Style != goClientStyleRaw && cfg.Style != goClientStyleTyped {
		return fmt.Errorf("unsupported Go client style %q", cfg.Style)
	}
	loaded, err := loadOpenAPI(cfg.OpenAPIPath)
	if err != nil {
		return err
	}
	if err := loaded.validate(); err != nil {
		return err
	}
	var rendered []byte
	if cfg.Style == goClientStyleTyped {
		rendered = renderTypedGoClient(cfg.Package, loaded.doc)
	} else {
		operations := operationsFromOpenAPIDocument(loaded.doc)
		rendered = renderGoClient(cfg.Package, operations)
	}
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

type resourceConfig struct {
	Dir          string
	Name         string
	Plural       string
	TenantScoped bool
	CRUD         bool
	Postgres     bool
	SoftDelete   bool
	ETag         bool
	Audit        bool
	Webhooks     bool
}

type resourceManifest struct {
	Profile       string
	Module        string
	OpenAPI       string
	ClientPath    string
	ClientPackage string
	Resources     map[string]bool
}

type resourceTemplateData struct {
	Module     string
	Name       string
	Plural     string
	Type       string
	Field      string
	Var        string
	Table      string
	ScopeRead  string
	ScopeWrite string
	Migration  string
	Prefix     string
}

func generateResource(ctx context.Context, cfg resourceConfig) error {
	if err := validateResourceConfig(cfg); err != nil {
		return err
	}
	rootDir, err := safeOutputDir(cfg.Dir)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(rootDir, "api-toolkit.yaml")
	// #nosec G304 -- rootDir is validated and the manifest filename is fixed.
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("api-toolkit.yaml is required; run this inside a generated saas-api-full project")
		}
		return fmt.Errorf("read api-toolkit.yaml: %w", err)
	}
	manifest := parseResourceManifest(manifestBytes)
	if manifest.Profile != scaffoldProfileSaaSAPIFull {
		return fmt.Errorf("api-toolkit.yaml profile must be %q", scaffoldProfileSaaSAPIFull)
	}
	if manifest.Module == "" {
		return errors.New("api-toolkit.yaml module is required")
	}
	if manifest.Resources[cfg.Name] || manifest.Resources[cfg.Plural] {
		return fmt.Errorf("resource %q already exists", cfg.Name)
	}
	data := resourceTemplateData{
		Module:     manifest.Module,
		Name:       cfg.Name,
		Plural:     cfg.Plural,
		Type:       exportedGoIdentifier(cfg.Name),
		Field:      exportedGoIdentifier(cfg.Plural),
		Var:        goParamName(cfg.Name),
		Table:      cfg.Plural,
		ScopeRead:  cfg.Plural + ":read",
		ScopeWrite: cfg.Plural + ":write",
		Migration:  nextResourceMigrationName(rootDir, cfg.Plural),
		Prefix:     resourceIDPrefix(cfg.Name),
	}
	if err := assertResourceAnchors(rootDir); err != nil {
		return err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return fmt.Errorf("open project root: %w", err)
	}
	defer root.Close()
	files := []struct {
		name string
		body string
	}{
		{fmt.Sprintf("internal/domain/%s.go", cfg.Name), resourceDomainTemplate},
		{fmt.Sprintf("internal/app/%s.go", cfg.Plural), resourceAppTemplate},
		{fmt.Sprintf("internal/app/%s_test.go", cfg.Plural), resourceAppTestTemplate},
		{fmt.Sprintf("internal/adapters/postgres/%s.go", cfg.Plural), resourcePostgresTemplate},
		{fmt.Sprintf("internal/adapters/postgres/%s_test.go", cfg.Plural), resourcePostgresTestTemplate},
		{fmt.Sprintf("internal/httpapi/%s.go", cfg.Plural), resourceHTTPTemplate},
		{fmt.Sprintf("internal/httpapi/%s_test.go", cfg.Plural), resourceHTTPTestTemplate},
		{filepath.Join("migrations", data.Migration), resourceMigrationTemplate},
	}
	for _, file := range files {
		rendered, err := renderResourceTemplate(file.name, file.body, data)
		if err != nil {
			return err
		}
		if strings.HasSuffix(file.name, ".go") {
			rendered, err = format.Source(rendered)
			if err != nil {
				return fmt.Errorf("format %s: %w", file.name, err)
			}
		}
		if err := writeGeneratedFile(root, file.name, rendered); err != nil {
			return err
		}
	}
	patches := []resourcePatch{
		{Path: "api-toolkit.yaml", Anchor: "  # api-toolkit:manifest-resources", Insert: renderResourceManifestEntry(cfg)},
		{Path: "cmd/api/main.go", Anchor: "\t// api-toolkit:main-service-defaults", Insert: fmt.Sprintf("\t%s := app.New%sService()\n", data.Plural, data.Type)},
		{Path: "cmd/api/main.go", Anchor: "\t\t// api-toolkit:main-postgres-stores", Insert: fmt.Sprintf("\t\t%s = app.New%sServiceWithStore(postgres.New%sStore(pool))\n", data.Plural, data.Type, data.Type)},
		{Path: "cmd/api/main.go", Anchor: "\t\t// api-toolkit:main-router-config", Insert: fmt.Sprintf("\t\t%s: %s,\n", data.Field, data.Plural)},
		{Path: "internal/adapters/postgres/postgres.go", Anchor: "\t// api-toolkit:postgres-required-tables", Insert: fmt.Sprintf("\t%q,\n", data.Table)},
		{Path: "internal/httpapi/router.go", Anchor: "\t// api-toolkit:router-config-fields", Insert: fmt.Sprintf("\t%s *app.%sService\n", data.Field, data.Type)},
		{Path: "internal/httpapi/router.go", Anchor: "\t// api-toolkit:router-register-routes", Insert: renderResourceRouteRegistrations(data)},
		{Path: "internal/httpapi/router.go", Anchor: "\t// api-toolkit:router-default-services", Insert: fmt.Sprintf("\tif cfg.%s == nil {\n\t\tcfg.%s = app.New%sService()\n\t}\n", data.Field, data.Field, data.Type)},
		{Path: "internal/httpapi/openapi.go", Anchor: "\t// api-toolkit:openapi-schemas", Insert: renderResourceOpenAPISchemas(data)},
		{Path: "internal/httpapi/openapi.go", Anchor: "\t\t\t\t// api-toolkit:openapi-webhook-event-types", Insert: fmt.Sprintf("\t\t\t\t%q,\n\t\t\t\t%q,\n\t\t\t\t%q,\n", data.Name+".created", data.Name+".updated", data.Name+".deleted")},
		{Path: "internal/httpapi/openapi.go", Anchor: "\t// api-toolkit:openapi-operation-variables", Insert: renderResourceOpenAPIVariables(data)},
		{Path: "internal/httpapi/openapi.go", Anchor: "\t\t// api-toolkit:openapi-operations", Insert: renderResourceOpenAPIOperations(data)},
		{Path: "internal/app/webhooks.go", Anchor: "\t// api-toolkit:webhook-event-types", Insert: fmt.Sprintf("\t%q,\n\t%q,\n\t%q,\n", data.Name+".created", data.Name+".updated", data.Name+".deleted")},
	}
	for _, patch := range patches {
		if err := applyResourcePatch(rootDir, patch); err != nil {
			return err
		}
	}
	for _, name := range []string{
		"cmd/api/main.go",
		"internal/adapters/postgres/postgres.go",
		"internal/httpapi/router.go",
		"internal/httpapi/openapi.go",
		"internal/app/webhooks.go",
	} {
		if err := formatGoFile(filepath.Join(rootDir, name)); err != nil {
			return err
		}
	}
	if err := runResourceCommand(ctx, rootDir, []string{"go", "mod", "tidy"}, nil); err != nil {
		return err
	}
	if err := runResourceCommand(ctx, rootDir, []string{"go", "test", "./internal/httpapi", "-run", "TestOpenAPIGolden"}, []string{"UPDATE_OPENAPI=1"}); err != nil {
		return err
	}
	openAPIPath := manifest.OpenAPI
	if openAPIPath == "" {
		openAPIPath = "testdata/openapi.golden.json"
	}
	clientPath := manifest.ClientPath
	if clientPath == "" {
		clientPath = "internal/client/apiclient"
	}
	clientPackage := manifest.ClientPackage
	if clientPackage == "" {
		clientPackage = "apiclient"
	}
	return generateGoClient(goClientConfig{
		OpenAPIPath: filepath.Join(rootDir, openAPIPath),
		OutDir:      filepath.Join(rootDir, clientPath),
		Package:     clientPackage,
		Style:       goClientStyleTyped,
	})
}

func validateResourceConfig(cfg resourceConfig) error {
	if !validResourceName(cfg.Name) {
		return fmt.Errorf("invalid resource name %q", cfg.Name)
	}
	if !validResourceName(cfg.Plural) {
		return fmt.Errorf("invalid resource plural %q", cfg.Plural)
	}
	if cfg.Name == cfg.Plural {
		return errors.New("--plural must differ from --name")
	}
	if !cfg.TenantScoped {
		return errors.New("--tenant-scoped is required")
	}
	if !cfg.CRUD {
		return errors.New("--crud is required")
	}
	return nil
}

func validResourceName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value != strings.ToLower(value) {
		return false
	}
	if !isASCIILetter(value[0]) {
		return false
	}
	for i := 0; i < len(value); i++ {
		if isASCIILetter(value[i]) || isASCIIDigit(value[i]) || value[i] == '_' {
			continue
		}
		return false
	}
	return true
}

func parseResourceManifest(data []byte) resourceManifest {
	manifest := resourceManifest{Resources: map[string]bool{}}
	lines := strings.Split(string(data), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(line, "profile:"):
			manifest.Profile = strings.TrimSpace(strings.TrimPrefix(line, "profile:"))
		case strings.HasPrefix(line, "module:"):
			manifest.Module = strings.TrimSpace(strings.TrimPrefix(line, "module:"))
		case strings.HasPrefix(line, "openapi:"):
			manifest.OpenAPI = strings.TrimSpace(strings.TrimPrefix(line, "openapi:"))
		case strings.HasPrefix(line, "path:") && manifest.ClientPath == "":
			manifest.ClientPath = strings.TrimSpace(strings.TrimPrefix(line, "path:"))
		case strings.HasPrefix(line, "package:") && manifest.ClientPackage == "":
			manifest.ClientPackage = strings.TrimSpace(strings.TrimPrefix(line, "package:"))
		case strings.HasPrefix(line, "- name:"):
			name := strings.TrimSpace(strings.TrimPrefix(line, "- name:"))
			if name != "" {
				manifest.Resources[name] = true
			}
		}
	}
	return manifest
}

func assertResourceAnchors(rootDir string) error {
	anchors := []resourcePatch{
		{Path: "api-toolkit.yaml", Anchor: "  # api-toolkit:manifest-resources"},
		{Path: "cmd/api/main.go", Anchor: "\t// api-toolkit:main-service-defaults"},
		{Path: "cmd/api/main.go", Anchor: "\t\t// api-toolkit:main-postgres-stores"},
		{Path: "cmd/api/main.go", Anchor: "\t\t// api-toolkit:main-router-config"},
		{Path: "internal/adapters/postgres/postgres.go", Anchor: "\t// api-toolkit:postgres-required-tables"},
		{Path: "internal/httpapi/router.go", Anchor: "\t// api-toolkit:router-config-fields"},
		{Path: "internal/httpapi/router.go", Anchor: "\t// api-toolkit:router-register-routes"},
		{Path: "internal/httpapi/router.go", Anchor: "\t// api-toolkit:router-default-services"},
		{Path: "internal/httpapi/openapi.go", Anchor: "\t// api-toolkit:openapi-schemas"},
		{Path: "internal/httpapi/openapi.go", Anchor: "\t\t\t\t// api-toolkit:openapi-webhook-event-types"},
		{Path: "internal/httpapi/openapi.go", Anchor: "\t// api-toolkit:openapi-operation-variables"},
		{Path: "internal/httpapi/openapi.go", Anchor: "\t\t// api-toolkit:openapi-operations"},
		{Path: "internal/app/webhooks.go", Anchor: "\t// api-toolkit:webhook-event-types"},
	}
	for _, anchor := range anchors {
		// #nosec G304 -- rootDir is validated and anchor paths are fixed generator-owned relative paths.
		data, err := os.ReadFile(filepath.Join(rootDir, anchor.Path))
		if err != nil {
			return fmt.Errorf("read %s: %w", anchor.Path, err)
		}
		if strings.Count(string(data), anchor.Anchor) != 1 {
			return fmt.Errorf("generated anchors missing or dirty in %s", anchor.Path)
		}
	}
	return nil
}

type resourcePatch struct {
	Path   string
	Anchor string
	Insert string
}

func applyResourcePatch(rootDir string, patch resourcePatch) error {
	path := filepath.Join(rootDir, patch.Path)
	// #nosec G304 -- rootDir is validated and patch paths are fixed generator-owned relative paths.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", patch.Path, err)
	}
	content := string(data)
	if strings.Count(content, patch.Anchor) != 1 {
		return fmt.Errorf("generated anchors missing or dirty in %s", patch.Path)
	}
	content = strings.Replace(content, patch.Anchor, patch.Insert+patch.Anchor, 1)
	// #nosec G703 -- rootDir is validated and patch paths are fixed generator-owned relative paths.
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", patch.Path, err)
	}
	return nil
}

func renderResourceTemplate(name, body string, data resourceTemplateData) ([]byte, error) {
	tmpl, err := template.New(name).Parse(body)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return out.Bytes(), nil
}

func formatGoFile(path string) error {
	// #nosec G304 -- caller passes fixed generator-owned Go files under the validated scaffold root.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	formatted, err := format.Source(data)
	if err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	// #nosec G703 -- path is a fixed generator-owned Go file under the validated scaffold root.
	return os.WriteFile(path, formatted, 0o600)
}

func runResourceCommand(ctx context.Context, dir string, args []string, env []string) error {
	if len(args) == 0 {
		return errors.New("resource command is required")
	}
	// #nosec G204 -- callers pass fixed go command vectors, not user-controlled shell input.
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\n%s", strings.Join(args, " "), err, output)
	}
	return nil
}

func nextResourceMigrationName(rootDir, plural string) string {
	matches, _ := filepath.Glob(filepath.Join(rootDir, "migrations", "*.up.sql"))
	version := 20260517000100 + len(matches)*100
	return fmt.Sprintf("%014d_%s.up.sql", version, plural)
}

func resourceIDPrefix(name string) string {
	parts := identifierParts(name)
	if len(parts) == 0 {
		return "res"
	}
	value := strings.ToLower(parts[0])
	if len(value) >= 3 {
		return value[:3]
	}
	return value
}

func renderResourceManifestEntry(cfg resourceConfig) string {
	return fmt.Sprintf("  - name: %s\n    plural: %s\n    tenant_scoped: %t\n    crud: %t\n    postgres: %t\n    soft_delete: %t\n    etag: %t\n    audit: %t\n    webhooks: %t\n", cfg.Name, cfg.Plural, cfg.TenantScoped, cfg.CRUD, cfg.Postgres, cfg.SoftDelete, cfg.ETag, cfg.Audit, cfg.Webhooks)
}

func renderResourceRouteRegistrations(data resourceTemplateData) string {
	return fmt.Sprintf("\tr.Get(%q, cfg.protect(%q, http.HandlerFunc(cfg.handleList%s)).ServeHTTP)\n\tr.Post(%q, cfg.protect(%q, cfg.idempotent(http.HandlerFunc(cfg.handleCreate%s))).ServeHTTP)\n\tregisterPatch(r, %q, cfg.protect(%q, cfg.idempotent(http.HandlerFunc(cfg.handleUpdate%s))).ServeHTTP)\n\tr.Delete(%q, cfg.protect(%q, cfg.idempotent(http.HandlerFunc(cfg.handleDelete%s))).ServeHTTP)\n",
		"/"+data.Plural, data.ScopeRead, data.Field,
		"/"+data.Plural, data.ScopeWrite, data.Type,
		"/"+data.Plural+"/{id}", data.ScopeWrite, data.Type,
		"/"+data.Plural+"/{id}", data.ScopeWrite, data.Type,
	)
}

func renderResourceOpenAPISchemas(data resourceTemplateData) string {
	return fmt.Sprintf(`	registry.RegisterSchema("%s", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "name", "version"},
		"properties": map[string]any{
			"id":         map[string]any{"type": "string"},
			"tenant_id":  map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"version":    map[string]any{"type": "integer", "format": "int64"},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
			"updated_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("%sCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"name"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		},
	})
	registry.RegisterSchema("%sList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/%s"}},
			"next_cursor": map[string]any{"type": "string", "nullable": true},
		},
	})
`, data.Type, data.Type, data.Type, data.Type)
}

func renderResourceOpenAPIVariables(data resourceTemplateData) string {
	varName := goParamName(data.Name)
	return fmt.Sprintf(`	%sBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/%sCreateRequest"},
		},
	}
	%sResponse := specs.Response{
		Description: "%s",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/%s"},
		},
	}
	%sListResponse := specs.Response{
		Description: "%s list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/%sList"},
		},
	}
`, varName, data.Type, varName, data.Type, data.Type, varName, data.Type, data.Type)
}

func renderResourceOpenAPIOperations(data resourceTemplateData) string {
	varName := goParamName(data.Name)
	return fmt.Sprintf(`		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "list%s",
			Method:      http.MethodGet,
			Path:        "/%s",
			Summary:     "List %s",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "cursor", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
				{Name: "limit", In: "query", Required: false, Schema: map[string]any{"type": "integer", "minimum": 1, "maximum": 100}},
			},
			Security: auth("%s"),
			Responses: map[int]specs.Response{http.StatusOK: %sListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "create%s",
			Method:      http.MethodPost,
			Path:        "/%s",
			Summary:     "Create %s",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("%s"),
			RequestBody: %sBody,
			Responses:   map[int]specs.Response{http.StatusCreated: %sResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "update%s",
			Method:      http.MethodPatch,
			Path:        "/%s/{id}",
			Summary:     "Update %s",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "If-Match", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("%s"),
			RequestBody: %sBody,
			Responses:   map[int]specs.Response{http.StatusOK: %sResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "delete%s",
			Method:      http.MethodDelete,
			Path:        "/%s/{id}",
			Summary:     "Delete %s",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("%s"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Deleted"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
`, data.Field, data.Plural, data.Plural, data.ScopeRead, varName, data.Type, data.Plural, data.Name, data.ScopeWrite, varName, varName, data.Type, data.Plural, data.Name, data.ScopeWrite, varName, varName, data.Type, data.Plural, data.Name, data.ScopeWrite)
}

const resourceDomainTemplate = `package domain

import (
	"fmt"
	"time"
)

type {{ .Type }} struct {
	ID        string
	TenantID  string
	Name      string
	Version   int64
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r {{ .Type }}) ETag() string {
	return fmt.Sprintf("%q", r.Version)
}

func (r {{ .Type }}) Public() map[string]any {
	return map[string]any{
		"id":         r.ID,
		"tenant_id":  r.TenantID,
		"name":       r.Name,
		"version":    r.Version,
		"created_at": r.CreatedAt,
		"updated_at": r.UpdatedAt,
	}
}
`

const resourceAppTemplate = `package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"{{ .Module }}/internal/domain"
)

type {{ .Type }}Store interface {
	Create(context.Context, domain.{{ .Type }}) error
	Save(context.Context, domain.{{ .Type }}) error
	Get(context.Context, string, string) (domain.{{ .Type }}, bool, error)
	List(context.Context, string, string, int) ([]domain.{{ .Type }}, string, error)
}

type {{ .Type }}Service struct {
	mu     sync.Mutex
	next   int
	now    func() time.Time
	store  {{ .Type }}Store
	items  map[string]domain.{{ .Type }}
}

func New{{ .Type }}Service() *{{ .Type }}Service {
	return &{{ .Type }}Service{
		now:   time.Now,
		items: map[string]domain.{{ .Type }}{},
	}
}

func New{{ .Type }}ServiceWithStore(store {{ .Type }}Store) *{{ .Type }}Service {
	service := New{{ .Type }}Service()
	service.store = store
	return service
}

func (s *{{ .Type }}Service) List(ctx context.Context, tenantID, cursor string, limit int) ([]domain.{{ .Type }}, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if s == nil {
		return nil, "", ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	cursor = strings.TrimSpace(cursor)
	if tenantID == "" {
		return nil, "", ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if s.store != nil {
		return s.store.List(ctx, tenantID, cursor, limit)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]domain.{{ .Type }}, 0, len(s.items))
	for _, item := range s.items {
		if item.TenantID != tenantID || item.DeletedAt != nil {
			continue
		}
		if cursor != "" && item.ID <= cursor {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	next := ""
	if len(items) > limit {
		next = items[limit-1].ID
		items = items[:limit]
	}
	return items, next, nil
}

func (s *{{ .Type }}Service) Create(ctx context.Context, tenantID, name string) (domain.{{ .Type }}, error) {
	if err := ctx.Err(); err != nil {
		return domain.{{ .Type }}{}, err
	}
	if s == nil {
		return domain.{{ .Type }}{}, ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	name = strings.TrimSpace(name)
	if tenantID == "" || name == "" {
		return domain.{{ .Type }}{}, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	now := s.now().UTC()
	item := domain.{{ .Type }}{
		ID:        fmt.Sprintf("{{ .Prefix }}_%06d", s.next),
		TenantID:  tenantID,
		Name:      name,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if s.store != nil {
		if err := s.store.Create(ctx, item); err != nil {
			return domain.{{ .Type }}{}, err
		}
	} else {
		s.items[item.ID] = item
	}
	return item, nil
}

func (s *{{ .Type }}Service) Update(ctx context.Context, tenantID, id, name, ifMatch string) (domain.{{ .Type }}, error) {
	if err := ctx.Err(); err != nil {
		return domain.{{ .Type }}{}, err
	}
	if s == nil {
		return domain.{{ .Type }}{}, ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	ifMatch = strings.TrimSpace(ifMatch)
	if tenantID == "" || id == "" || name == "" || ifMatch == "" {
		return domain.{{ .Type }}{}, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, found, err := s.getLocked(ctx, tenantID, id)
	if err != nil {
		return domain.{{ .Type }}{}, err
	}
	if !found {
		return domain.{{ .Type }}{}, ErrNotFound
	}
	if item.ETag() != ifMatch {
		return domain.{{ .Type }}{}, ErrPreconditionFailed
	}
	item.Name = name
	item.Version++
	item.UpdatedAt = s.now().UTC()
	if s.store != nil {
		if err := s.store.Save(ctx, item); err != nil {
			return domain.{{ .Type }}{}, err
		}
	} else {
		s.items[item.ID] = item
	}
	return item, nil
}

func (s *{{ .Type }}Service) Delete(ctx context.Context, tenantID, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	if tenantID == "" || id == "" {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, found, err := s.getLocked(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	now := s.now().UTC()
	item.DeletedAt = &now
	item.Version++
	item.UpdatedAt = now
	if s.store != nil {
		return s.store.Save(ctx, item)
	}
	s.items[item.ID] = item
	return nil
}

func (s *{{ .Type }}Service) getLocked(ctx context.Context, tenantID, id string) (domain.{{ .Type }}, bool, error) {
	if s.store != nil {
		item, found, err := s.store.Get(ctx, tenantID, id)
		if err != nil || !found || item.DeletedAt != nil {
			return domain.{{ .Type }}{}, false, err
		}
		return item, true, nil
	}
	item, found := s.items[id]
	if !found || item.TenantID != tenantID || item.DeletedAt != nil {
		return domain.{{ .Type }}{}, false, nil
	}
	return item, true, nil
}
`

const resourceAppTestTemplate = `package app

import (
	"context"
	"errors"
	"testing"
)

func Test{{ .Type }}ServiceCRUDIsTenantScoped(t *testing.T) {
	service := New{{ .Type }}Service()
	ctx := context.Background()
	created, err := service.Create(ctx, "org_1", "Alpha")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.TenantID != "org_1" || created.Version != 1 {
		t.Fatalf("created {{ .Name }} = %#v", created)
	}
	items, _, err := service.List(ctx, "org_1", "", 50)
	if err != nil || len(items) != 1 {
		t.Fatalf("List() items=%#v err=%v", items, err)
	}
	other, _, err := service.List(ctx, "org_2", "", 50)
	if err != nil || len(other) != 0 {
		t.Fatalf("cross tenant List() items=%#v err=%v", other, err)
	}
	updated, err := service.Update(ctx, "org_1", created.ID, "Beta", created.ETag())
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Beta" || updated.Version != 2 {
		t.Fatalf("updated {{ .Name }} = %#v", updated)
	}
	if _, err := service.Update(ctx, "org_1", created.ID, "Gamma", created.ETag()); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("stale Update() error = %v, want %v", err, ErrPreconditionFailed)
	}
	if err := service.Delete(ctx, "org_1", created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	items, _, err = service.List(ctx, "org_1", "", 50)
	if err != nil || len(items) != 0 {
		t.Fatalf("List() after delete items=%#v err=%v", items, err)
	}
}
`

const resourcePostgresTemplate = `package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"{{ .Module }}/internal/domain"
)

var (
	Err{{ .Type }}StoreRequired = errors.New("postgres {{ .Name }} store db is required")
	Err{{ .Type }}Invalid       = errors.New("postgres {{ .Name }} record is invalid")
)

type {{ .Type }}DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type {{ .Type }}Store struct {
	db {{ .Type }}DB
}

func New{{ .Type }}Store(db {{ .Type }}DB) *{{ .Type }}Store {
	return &{{ .Type }}Store{db: db}
}

func (s *{{ .Type }}Store) Create(ctx context.Context, item domain.{{ .Type }}) error {
	return s.Save(ctx, item)
}

func (s *{{ .Type }}Store) Save(ctx context.Context, item domain.{{ .Type }}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return Err{{ .Type }}StoreRequired
	}
	item.ID = strings.TrimSpace(item.ID)
	item.TenantID = strings.TrimSpace(item.TenantID)
	item.Name = strings.TrimSpace(item.Name)
	if item.ID == "" || item.TenantID == "" || item.Name == "" || item.Version < 1 || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return Err{{ .Type }}Invalid
	}
	_, err := s.db.Exec(ctx,
		"insert into {{ .Table }} (id, organization_id, name, version, deleted_at, created_at, updated_at) values ($1, $2, $3, $4, $5, $6, $7) "+
			"on conflict (id) do update set name=excluded.name, version=excluded.version, deleted_at=excluded.deleted_at, updated_at=excluded.updated_at",
		item.ID, item.TenantID, item.Name, item.Version, item.DeletedAt, item.CreatedAt.UTC(), item.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("save {{ .Name }}: %w", err)
	}
	return nil
}

func (s *{{ .Type }}Store) Get(ctx context.Context, tenantID, id string) (domain.{{ .Type }}, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.{{ .Type }}{}, false, err
	}
	if s == nil || s.db == nil {
		return domain.{{ .Type }}{}, false, Err{{ .Type }}StoreRequired
	}
	row := s.db.QueryRow(ctx, "select id, organization_id, name, version, deleted_at, created_at, updated_at from {{ .Table }} where organization_id=$1 and id=$2", strings.TrimSpace(tenantID), strings.TrimSpace(id))
	item, err := scan{{ .Type }}(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.{{ .Type }}{}, false, nil
	}
	if err != nil {
		return domain.{{ .Type }}{}, false, fmt.Errorf("get {{ .Name }}: %w", err)
	}
	return item, true, nil
}

func (s *{{ .Type }}Store) List(ctx context.Context, tenantID, cursor string, limit int) ([]domain.{{ .Type }}, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if s == nil || s.db == nil {
		return nil, "", Err{{ .Type }}StoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	cursor = strings.TrimSpace(cursor)
	if tenantID == "" {
		return nil, "", Err{{ .Type }}Invalid
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(ctx,
		"select id, organization_id, name, version, deleted_at, created_at, updated_at from {{ .Table }} where organization_id=$1 and deleted_at is null and ($2 = '' or id > $2) order by id limit $3",
		tenantID, cursor, limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list {{ .Plural }}: %w", err)
	}
	defer rows.Close()
	var out []domain.{{ .Type }}
	for rows.Next() {
		item, err := scan{{ .Type }}(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("list {{ .Plural }} rows: %w", err)
	}
	next := ""
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

type {{ .Var }}Scanner interface {
	Scan(dest ...any) error
}

func scan{{ .Type }}(row {{ .Var }}Scanner) (domain.{{ .Type }}, error) {
	var item domain.{{ .Type }}
	var deleted pgtype.Timestamptz
	if err := row.Scan(&item.ID, &item.TenantID, &item.Name, &item.Version, &deleted, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return domain.{{ .Type }}{}, err
	}
	if deleted.Valid {
		value := deleted.Time.UTC()
		item.DeletedAt = &value
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}

var _ = time.Time{}
`

const resourcePostgresTestTemplate = `package postgres

import (
	"context"
	"errors"
	"testing"

	"{{ .Module }}/internal/domain"
)

func Test{{ .Type }}StoreRequiresDatabase(t *testing.T) {
	store := New{{ .Type }}Store(nil)
	if err := store.Save(context.Background(), domain.{{ .Type }}{}); !errors.Is(err, Err{{ .Type }}StoreRequired) {
		t.Fatalf("Save() error = %v, want %v", err, Err{{ .Type }}StoreRequired)
	}
	if _, _, err := store.Get(context.Background(), "org_1", "{{ .Prefix }}_1"); !errors.Is(err, Err{{ .Type }}StoreRequired) {
		t.Fatalf("Get() error = %v, want %v", err, Err{{ .Type }}StoreRequired)
	}
	if _, _, err := store.List(context.Background(), "org_1", "", 50); !errors.Is(err, Err{{ .Type }}StoreRequired) {
		t.Fatalf("List() error = %v, want %v", err, Err{{ .Type }}StoreRequired)
	}
}
`

const resourceHTTPTemplate = `package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"{{ .Module }}/internal/app"
)

type {{ .Var }}Request struct {
	Name string ` + "`json:\"name\"`" + `
}

func (cfg RouterConfig) handleList{{ .Field }}(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	limit, err := query{{ .Type }}Limit(r, 50)
	if err != nil {
		writeAppError(w, app.ErrValidation)
		return
	}
	items, next, err := cfg.{{ .Field }}.List(r.Context(), tenantID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeAppError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, item.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "next_cursor": nullable{{ .Type }}String(next)})
}

func (cfg RouterConfig) handleCreate{{ .Type }}(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decode{{ .Type }}Request(w, r)
	if !ok {
		return
	}
	item, err := cfg.{{ .Field }}.Create(r.Context(), tenantID, req.Name)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("ETag", item.ETag())
	if cfg.Webhooks != nil {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "{{ .Name }}.created", map[string]any{"id": item.ID, "tenant_id": tenantID, "version": item.Version}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "{{ .Name }}.create", "{{ .Name }}", item.ID, nil)
	writeJSON(w, http.StatusCreated, item.Public())
}

func (cfg RouterConfig) handleUpdate{{ .Type }}(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	ifMatch, ok := requireHeader(w, r, "If-Match")
	if !ok {
		return
	}
	req, ok := decode{{ .Type }}Request(w, r)
	if !ok {
		return
	}
	item, err := cfg.{{ .Field }}.Update(r.Context(), tenantID, r.PathValue("id"), req.Name, ifMatch)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("ETag", item.ETag())
	if cfg.Webhooks != nil {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "{{ .Name }}.updated", map[string]any{"id": item.ID, "tenant_id": tenantID, "version": item.Version}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "{{ .Name }}.update", "{{ .Name }}", item.ID, nil)
	writeJSON(w, http.StatusOK, item.Public())
}

func (cfg RouterConfig) handleDelete{{ .Type }}(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if err := cfg.{{ .Field }}.Delete(r.Context(), tenantID, id); err != nil {
		writeAppError(w, err)
		return
	}
	if cfg.Webhooks != nil {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "{{ .Name }}.deleted", map[string]any{"id": id, "tenant_id": tenantID}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "{{ .Name }}.delete", "{{ .Name }}", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func decode{{ .Type }}Request(w http.ResponseWriter, r *http.Request) ({{ .Var }}Request, bool) {
	var req {{ .Var }}Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAppError(w, app.ErrValidation)
		return req, false
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeAppError(w, app.ErrValidation)
		return req, false
	}
	return req, true
}

func query{{ .Type }}Limit(r *http.Request, fallback int) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get("limit"))
	if value == "" {
		return fallback, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 100 {
		return 0, app.ErrValidation
	}
	return limit, nil
}

func nullable{{ .Type }}String(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
`

const resourceHTTPTestTemplate = `package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerated{{ .Type }}ResourceCRUD(t *testing.T) {
	handler := newTestRouter(t)
	orgID := createOrganization(t, handler, "owner_{{ .Name }}", "Resource Org")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/{{ .Plural }}", strings.NewReader("{\"name\":\"Alpha\"}"))
	authorizeTestRequestAs(t, req, orgID, "owner_{{ .Name }}", "{{ .ScopeWrite }}")
	req.Header.Set("Idempotency-Key", "create-{{ .Name }}")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create {{ .Name }} status = %d body=%s", rec.Code, rec.Body.String())
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("create {{ .Name }} missing ETag")
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("create body missing id: %#v", created)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/{{ .Plural }}", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_{{ .Name }}", "{{ .ScopeRead }}")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), id) {
		t.Fatalf("list {{ .Plural }} status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/{{ .Plural }}/"+id, strings.NewReader("{\"name\":\"Beta\"}"))
	authorizeTestRequestAs(t, req, orgID, "owner_{{ .Name }}", "{{ .ScopeWrite }}")
	req.Header.Set("If-Match", etag)
	req.Header.Set("Idempotency-Key", "update-{{ .Name }}")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Beta") {
		t.Fatalf("update {{ .Name }} status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/{{ .Plural }}/"+id, nil)
	authorizeTestRequestAs(t, req, orgID, "owner_{{ .Name }}", "{{ .ScopeWrite }}")
	req.Header.Set("Idempotency-Key", "delete-{{ .Name }}")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete {{ .Name }} status = %d body=%s", rec.Code, rec.Body.String())
	}
}
`

const resourceMigrationTemplate = `CREATE TABLE {{ .Table }} (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	version BIGINT NOT NULL DEFAULT 1,
	deleted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX {{ .Table }}_organization_id_idx ON {{ .Table }}(organization_id, id) WHERE deleted_at IS NULL;
`

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

type typedClientOperation struct {
	Method      string
	Path        string
	OperationID string
	Parameters  []typedClientParameter
	Operation   *openapi3.Operation
}

type typedClientParameter struct {
	Name     string
	In       string
	Required bool
	Schema   *openapi3.SchemaRef
}

func renderTypedGoClient(packageName string, doc *openapi3.T) []byte {
	operations := typedClientOperationsFromOpenAPI(doc)
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
		renderTypedGoClientOperation(&methods, methodName, operation)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "package %s\n\n", packageName)
	out.WriteString(goClientRuntimeTemplate)
	renderTypedGoClientSchemas(&out, doc)
	out.WriteString(methods.String())
	return []byte(out.String())
}

func typedClientOperationsFromOpenAPI(doc *openapi3.T) []typedClientOperation {
	if doc == nil || doc.Paths == nil {
		return nil
	}
	var operations []typedClientOperation
	for routePath, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		itemParams := typedParametersFromOpenAPI(item.Parameters)
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			operations = append(operations, typedClientOperation{
				Method:      strings.ToUpper(method),
				Path:        routePath,
				OperationID: op.OperationID,
				Parameters:  mergeTypedParameters(itemParams, typedParametersFromOpenAPI(op.Parameters)),
				Operation:   op,
			})
		}
	}
	sort.SliceStable(operations, func(i, j int) bool {
		if operations[i].Path != operations[j].Path {
			return operations[i].Path < operations[j].Path
		}
		if operations[i].Method != operations[j].Method {
			return operations[i].Method < operations[j].Method
		}
		return operations[i].OperationID < operations[j].OperationID
	})
	return operations
}

func renderTypedGoClientSchemas(out *strings.Builder, doc *openapi3.T) {
	if doc == nil || doc.Components == nil || len(doc.Components.Schemas) == 0 {
		return
	}
	names := make([]string, 0, len(doc.Components.Schemas))
	for name := range doc.Components.Schemas {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.EqualFold(name, "Problem") {
			continue
		}
		goName := exportedGoIdentifier(name)
		ref := doc.Components.Schemas[name]
		if ref == nil || ref.Value == nil {
			continue
		}
		schema := ref.Value
		if schema.Type != nil && !schema.Type.Is("object") && len(schema.Properties) == 0 {
			fmt.Fprintf(out, "type %s %s\n\n", goName, goTypeForSchemaRef(ref, true))
			continue
		}
		required := stringSet(schema.Required)
		propertyNames := make([]string, 0, len(schema.Properties))
		for property := range schema.Properties {
			if strings.TrimSpace(property) != "" {
				propertyNames = append(propertyNames, property)
			}
		}
		sort.Strings(propertyNames)
		fmt.Fprintf(out, "type %s struct {\n", goName)
		for _, property := range propertyNames {
			propertyRef := schema.Properties[property]
			if propertyRef == nil {
				continue
			}
			fieldName := exportedGoIdentifier(property)
			requiredField := false
			if _, ok := required[property]; ok {
				requiredField = true
			}
			fieldType := goTypeForSchemaRef(propertyRef, requiredField)
			tag := property
			if !requiredField {
				tag += ",omitempty"
			}
			fmt.Fprintf(out, "\t%s %s `json:%q`\n", fieldName, fieldType, tag)
		}
		out.WriteString("}\n\n")
	}
}

func renderTypedGoClientOperation(out *strings.Builder, methodName string, operation typedClientOperation) {
	pathParams := typedOperationPathParameters(operation)
	optionParams := typedOperationOptionParameters(operation)
	paramsType := methodName + "Params"
	if len(optionParams) > 0 {
		renderTypedGoClientParams(out, paramsType, optionParams)
	}
	args := []string{"ctx context.Context"}
	for _, param := range pathParams {
		args = append(args, goParamName(param.Name)+" "+goTypeForParameter(param))
	}
	if len(optionParams) > 0 {
		args = append(args, "params "+paramsType)
	}
	requestType, hasRequestBody := typedOperationRequestBodyType(operation.Operation)
	if hasRequestBody {
		args = append(args, "body "+requestType)
	}
	args = append(args, "opts ...RequestOption")

	responseType := typedOperationResponseType(operation.Operation)
	rawArgs := append([]string(nil), args...)
	if hasRequestBody {
		for i, arg := range rawArgs {
			if strings.HasPrefix(arg, "body ") {
				rawArgs[i] = "body any"
				break
			}
		}
	}

	fmt.Fprintf(out, "// %s calls %s %s and decodes the primary JSON response.\n", methodName, operation.Method, operation.Path)
	if responseType != "" {
		fmt.Fprintf(out, "func (c *Client) %s(%s) (*%s, *http.Response, error) {\n", methodName, strings.Join(args, ", "), responseType)
		rawCallArgs := typedRawCallArgs(pathParams, len(optionParams) > 0, hasRequestBody)
		fmt.Fprintf(out, "\tresp, err := c.%sRaw(%s)\n", methodName, strings.Join(rawCallArgs, ", "))
		out.WriteString("\tif err != nil {\n")
		fmt.Fprintf(out, "\t\treturn nil, resp, err\n")
		out.WriteString("\t}\n")
		fmt.Fprintf(out, "\tdecoded, err := DecodeJSONResponse[%s](resp)\n", responseType)
		out.WriteString("\tif err != nil {\n")
		fmt.Fprintf(out, "\t\treturn nil, resp, err\n")
		out.WriteString("\t}\n")
		out.WriteString("\treturn decoded, resp, nil\n")
		out.WriteString("}\n\n")
	} else {
		fmt.Fprintf(out, "// %s calls %s %s.\n", methodName, operation.Method, operation.Path)
		fmt.Fprintf(out, "func (c *Client) %s(%s) (*http.Response, error) {\n", methodName, strings.Join(args, ", "))
		rawCallArgs := typedRawCallArgs(pathParams, len(optionParams) > 0, hasRequestBody)
		fmt.Fprintf(out, "\treturn c.%sRaw(%s)\n", methodName, strings.Join(rawCallArgs, ", "))
		out.WriteString("}\n\n")
	}

	fmt.Fprintf(out, "// %sRaw calls %s %s and returns the raw HTTP response.\n", methodName, operation.Method, operation.Path)
	fmt.Fprintf(out, "func (c *Client) %sRaw(%s) (*http.Response, error) {\n", methodName, strings.Join(rawArgs, ", "))
	if len(pathParams) > 0 || len(optionParams) > 0 {
		out.WriteString("\trequestOpts := []RequestOption{\n")
		for _, param := range pathParams {
			fmt.Fprintf(out, "\t\tPathParam(%q, formatParamValue(%s)),\n", param.Name, goParamName(param.Name))
		}
		out.WriteString("\t}\n")
		if len(optionParams) > 0 {
			out.WriteString("\trequestOpts = append(requestOpts, params.requestOptions()...)\n")
		}
		out.WriteString("\topts = append(requestOpts, opts...)\n")
	}
	bodyArg := "nil"
	if hasRequestBody {
		bodyArg = "body"
	}
	fmt.Fprintf(out, "\treturn c.do(ctx, %q, %q, %s, opts...)\n", operation.Method, operation.Path, bodyArg)
	out.WriteString("}\n\n")
}

func renderTypedGoClientParams(out *strings.Builder, typeName string, params []typedClientParameter) {
	fmt.Fprintf(out, "type %s struct {\n", typeName)
	for _, param := range params {
		fmt.Fprintf(out, "\t%s %s\n", exportedGoIdentifier(param.Name), goTypeForParameter(param))
	}
	out.WriteString("}\n\n")
	fmt.Fprintf(out, "func (params %s) requestOptions() []RequestOption {\n", typeName)
	out.WriteString("\tvar opts []RequestOption\n")
	for _, param := range params {
		field := "params." + exportedGoIdentifier(param.Name)
		optionFunc := "QueryParam"
		if strings.EqualFold(param.In, "header") {
			optionFunc = "Header"
		}
		fieldType := goTypeForParameter(param)
		switch {
		case param.Required:
			fmt.Fprintf(out, "\topts = append(opts, %s(%q, formatParamValue(%s)))\n", optionFunc, param.Name, field)
		case strings.HasPrefix(fieldType, "*"):
			fmt.Fprintf(out, "\tif %s != nil {\n", field)
			fmt.Fprintf(out, "\t\topts = append(opts, %s(%q, formatParamValue(*%s)))\n", optionFunc, param.Name, field)
			out.WriteString("\t}\n")
		default:
			fmt.Fprintf(out, "\tif %s != nil {\n", field)
			fmt.Fprintf(out, "\t\topts = append(opts, %s(%q, formatParamValue(%s)))\n", optionFunc, param.Name, field)
			out.WriteString("\t}\n")
		}
	}
	out.WriteString("\treturn opts\n")
	out.WriteString("}\n\n")
}

func typedRawCallArgs(pathParams []typedClientParameter, hasOptionParams bool, hasRequestBody bool) []string {
	args := []string{"ctx"}
	for _, param := range pathParams {
		args = append(args, goParamName(param.Name))
	}
	if hasOptionParams {
		args = append(args, "params")
	}
	if hasRequestBody {
		args = append(args, "body")
	}
	args = append(args, "opts...")
	return args
}

func typedOperationPathParameters(operation typedClientOperation) []typedClientParameter {
	var out []typedClientParameter
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

func typedOperationOptionParameters(operation typedClientOperation) []typedClientParameter {
	var out []typedClientParameter
	for _, parameter := range operation.Parameters {
		if (strings.EqualFold(parameter.In, "query") || strings.EqualFold(parameter.In, "header")) && strings.TrimSpace(parameter.Name) != "" {
			out = append(out, parameter)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].In != out[j].In {
			return out[i].In < out[j].In
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func typedOperationRequestBodyType(operation *openapi3.Operation) (string, bool) {
	if operation == nil || operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return "", false
	}
	schema := jsonMediaSchema(operation.RequestBody.Value.Content)
	if schema == nil {
		return "any", true
	}
	return goTypeForSchemaRef(schema, true), true
}

func typedOperationResponseType(operation *openapi3.Operation) string {
	if operation == nil || operation.Responses == nil {
		return ""
	}
	statuses := make([]int, 0, len(operation.Responses.Map()))
	byStatus := map[int]*openapi3.ResponseRef{}
	for status, response := range operation.Responses.Map() {
		var code int
		if _, err := fmt.Sscanf(status, "%d", &code); err != nil {
			continue
		}
		if code >= 200 && code < 300 {
			statuses = append(statuses, code)
			byStatus[code] = response
		}
	}
	sort.Ints(statuses)
	for _, status := range statuses {
		response := byStatus[status]
		if response == nil || response.Value == nil {
			continue
		}
		schema := jsonMediaSchema(response.Value.Content)
		if schema == nil {
			continue
		}
		return strings.TrimPrefix(goTypeForSchemaRef(schema, true), "*")
	}
	return ""
}

func jsonMediaSchema(content openapi3.Content) *openapi3.SchemaRef {
	if len(content) == 0 {
		return nil
	}
	if media := content.Get("application/json"); media != nil {
		return media.Schema
	}
	return nil
}

func goTypeForParameter(param typedClientParameter) string {
	if param.Schema == nil {
		if param.Required {
			return "string"
		}
		return "*string"
	}
	return goTypeForSchemaRef(param.Schema, param.Required)
}

func goTypeForSchemaRef(ref *openapi3.SchemaRef, required bool) string {
	if ref == nil {
		return "any"
	}
	if name := schemaNameFromRef(ref.Ref); name != "" {
		typeName := exportedGoIdentifier(name)
		if required && !schemaRefNullable(ref) {
			return typeName
		}
		return "*" + typeName
	}
	if ref.Value == nil {
		return "any"
	}
	schema := ref.Value
	var goType string
	switch {
	case schema.Type != nil && schema.Type.Includes("array"):
		goType = "[]" + strings.TrimPrefix(goTypeForSchemaRef(schema.Items, true), "*")
	case schema.Type != nil && schema.Type.Includes("integer"):
		if schema.Format == "int64" {
			goType = "int64"
		} else {
			goType = "int"
		}
	case schema.Type != nil && schema.Type.Includes("number"):
		goType = "float64"
	case schema.Type != nil && schema.Type.Includes("boolean"):
		goType = "bool"
	case schema.Type != nil && schema.Type.Includes("string"):
		goType = "string"
	case schema.Type != nil && schema.Type.Includes("object"):
		goType = "map[string]any"
	default:
		goType = "any"
	}
	if required && !schemaRefNullable(ref) {
		return goType
	}
	if strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[") || goType == "any" {
		return goType
	}
	return "*" + goType
}

func schemaNameFromRef(ref string) string {
	const prefix = "#/components/schemas/"
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	name := strings.TrimPrefix(ref, prefix)
	if strings.Contains(name, "/") || name == "" {
		return ""
	}
	return name
}

func schemaRefNullable(ref *openapi3.SchemaRef) bool {
	if ref == nil || ref.Value == nil {
		return false
	}
	schema := ref.Value
	return schema.Nullable || (schema.Type != nil && schema.Type.Includes("null"))
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
		parts[i] = exportedIdentifierPart(parts[i])
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
		parts[i] = exportedIdentifierPart(parts[i])
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

func exportedIdentifierPart(value string) string {
	switch strings.ToLower(value) {
	case "api":
		return "API"
	case "id":
		return "ID"
	case "json":
		return "JSON"
	case "jwt":
		return "JWT"
	case "oidc":
		return "OIDC"
	case "url":
		return "URL"
	default:
		return strings.ToUpper(value[:1]) + value[1:]
	}
}

func identifierParts(value string) []string {
	var parts []string
	var current strings.Builder
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || isASCIIDigit(ch) {
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
	Providers      []string
	CoreReplace    string
	ContribReplace string
	ToolkitVersion string
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
	if !isSemVerModuleVersion(cfg.ToolkitVersion) {
		cfg.ToolkitVersion = defaultScaffoldModuleVersion
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
		"Module":               cfg.Module,
		"Profile":              cfg.Profile,
		"AuthMode":             cfg.AuthMode,
		"AuthSchemeName":       scaffoldAuthSecuritySchemeName(cfg.AuthMode),
		"CoreVersion":          cfg.ToolkitVersion,
		"ContribVersion":       cfg.ToolkitVersion,
		"CoreReplace":          replaceLine("github.com/aatuh/api-toolkit/v2", cfg.CoreReplace),
		"ContribReplace":       replaceLine("github.com/aatuh/api-toolkit/contrib/v2", cfg.ContribReplace),
		"HasProviderWorkflows": boolTemplateValue(len(cfg.Providers) > 0),
		"ProviderWorkflows":    providerWorkflowInlineList(cfg.Providers),
		"HasStripeBilling":     boolTemplateValue(hasScaffoldProvider(cfg.Providers, scaffoldProviderStripe)),
		"HasResendEmail":       boolTemplateValue(hasScaffoldProvider(cfg.Providers, scaffoldProviderResend)),
		"HasClerkWebhooks":     boolTemplateValue(hasScaffoldProvider(cfg.Providers, scaffoldProviderClerkHooks)),
	}
	files := scaffoldFilesForConfig(cfg)
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
		golden, err = renderSaaSAPIFullOpenAPIGolden(cfg.AuthMode)
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

func boolTemplateValue(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func hasScaffoldProvider(providers []string, provider string) bool {
	for _, current := range providers {
		if current == provider {
			return true
		}
	}
	return false
}

func providerWorkflowInlineList(providers []string) string {
	if len(providers) == 0 {
		return "none"
	}
	quoted := make([]string, 0, len(providers))
	for _, provider := range providers {
		quoted = append(quoted, "`"+provider+"`")
	}
	return strings.Join(quoted, ", ")
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

func renderSaaSAPIFullOpenAPIGolden(authMode string) ([]byte, error) {
	doc, err := renderSaaSAPIFullOpenAPIBytes(authMode)
	if err != nil {
		return nil, err
	}
	return normalizeJSON(doc)
}

func renderSaaSAPIFullOpenAPIDoc(authMode string) (*openapi3.T, error) {
	doc, err := renderSaaSAPIFullOpenAPIBytes(authMode)
	if err != nil {
		return nil, err
	}
	loaded, err := openapi3.NewLoader().LoadFromData(doc)
	if err != nil {
		return nil, fmt.Errorf("load full scaffold openapi: %w", err)
	}
	return loaded, nil
}

func renderSaaSAPIFullOpenAPIBytes(authMode string) ([]byte, error) {
	registry := specs.NewRegistryWithOptions(specs.Info{
		Title:       "Full SaaS API",
		Description: "Generated api-toolkit full SaaS/API profile.",
		Version:     "dev",
	}, specs.RegistryOptions{
		OpenAPIVersion: specs.OpenAPIVersion31,
	})
	authSchemeName := scaffoldAuthSecuritySchemeName(authMode)
	if isScaffoldBearerAuth(authMode) {
		registry.RegisterSecurityScheme(authSchemeName, specs.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"})
	} else {
		registry.RegisterSecurityScheme(authSchemeName, specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	}
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
	return doc, nil
}

func renderSaaSAPIFullGoClient() ([]byte, error) {
	doc, err := renderSaaSAPIFullOpenAPIDoc(scaffoldAuthAPIKey)
	if err != nil {
		return nil, err
	}
	client := renderTypedGoClient("apiclient", doc)
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
	registry.RegisterSchema("WidgetImportItem", map[string]any{
		"type":                 "object",
		"required":             []string{"name"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		},
	})
	registry.RegisterSchema("WidgetImportRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"items"},
		"additionalProperties": false,
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WidgetImportItem"}, "minItems": 1, "maxItems": 100},
		},
	})
	registry.RegisterSchema("WidgetImportResult", map[string]any{
		"type":     "object",
		"required": []string{"created", "widget_ids"},
		"properties": map[string]any{
			"created":    map[string]any{"type": "integer", "minimum": 0},
			"widget_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	})
	registry.RegisterSchema("OperationAccepted", map[string]any{
		"type":     "object",
		"required": []string{"state"},
		"properties": map[string]any{
			"id":       map[string]any{"type": "string"},
			"state":    map[string]any{"type": "string", "enum": []string{"pending"}},
			"location": map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("WidgetImportOperation", map[string]any{
		"type":     "object",
		"required": []string{"id", "state"},
		"properties": map[string]any{
			"id":      map[string]any{"type": "string"},
			"state":   map[string]any{"type": "string", "enum": []string{"pending", "running", "succeeded", "failed", "canceled"}},
			"result":  map[string]any{"$ref": "#/components/schemas/WidgetImportResult", "nullable": true},
			"problem": map[string]any{"$ref": "#/components/schemas/Problem", "nullable": true},
		},
	})
	registry.RegisterSchema("Organization", map[string]any{
		"type":     "object",
		"required": []string{"id", "name", "created_at", "updated_at"},
		"properties": map[string]any{
			"id":         map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
			"updated_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("OrganizationCreateRequest", map[string]any{
		"type":     "object",
		"required": []string{"name"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 160},
		},
		"additionalProperties": false,
	})
	registry.RegisterSchema("OrganizationList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Organization"}},
		},
	})
	registry.RegisterSchema("Membership", map[string]any{
		"type":     "object",
		"required": []string{"organization_id", "user_id", "role", "created_at"},
		"properties": map[string]any{
			"organization_id": map[string]any{"type": "string"},
			"user_id":         map[string]any{"type": "string"},
			"role":            map[string]any{"type": "string", "enum": []string{"owner", "admin", "member", "viewer"}},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("MembershipList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Membership"}},
		},
	})
	registry.RegisterSchema("Invitation", map[string]any{
		"type":     "object",
		"required": []string{"id", "organization_id", "email", "role", "token_prefix", "expires_at", "created_at"},
		"properties": map[string]any{
			"id":              map[string]any{"type": "string"},
			"organization_id": map[string]any{"type": "string"},
			"email":           map[string]any{"type": "string", "format": "email"},
			"role":            map[string]any{"type": "string", "enum": []string{"owner", "admin", "member", "viewer"}},
			"token_prefix":    map[string]any{"type": "string"},
			"expires_at":      map[string]any{"type": "string", "format": "date-time"},
			"accepted_at":     map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("InvitationCreateRequest", map[string]any{
		"type":     "object",
		"required": []string{"email", "role"},
		"properties": map[string]any{
			"email": map[string]any{"type": "string", "format": "email"},
			"role":  map[string]any{"type": "string", "enum": []string{"admin", "member", "viewer"}},
		},
		"additionalProperties": false,
	})
	registry.RegisterSchema("InvitationCreated", map[string]any{
		"type":     "object",
		"required": []string{"invitation", "token"},
		"properties": map[string]any{
			"invitation": map[string]any{"$ref": "#/components/schemas/Invitation"},
			"token":      map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("InvitationAcceptRequest", map[string]any{
		"type":     "object",
		"required": []string{"token"},
		"properties": map[string]any{
			"token": map[string]any{"type": "string", "minLength": 1},
		},
		"additionalProperties": false,
	})
	registry.RegisterSchema("APIKey", map[string]any{
		"type":     "object",
		"required": []string{"id", "organization_id", "name", "prefix", "scopes", "created_at"},
		"properties": map[string]any{
			"id":              map[string]any{"type": "string"},
			"organization_id": map[string]any{"type": "string"},
			"name":            map[string]any{"type": "string"},
			"prefix":          map[string]any{"type": "string"},
			"scopes":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"expires_at":      map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"last_used_at":    map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"revoked_at":      map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("APIKeyCreateRequest", map[string]any{
		"type":     "object",
		"required": []string{"name", "scopes"},
		"properties": map[string]any{
			"name":       map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
			"scopes":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
			"expires_at": map[string]any{"type": "string", "format": "date-time"},
		},
		"additionalProperties": false,
	})
	registry.RegisterSchema("APIKeyCreated", map[string]any{
		"type":     "object",
		"required": []string{"api_key", "secret"},
		"properties": map[string]any{
			"api_key": map[string]any{"$ref": "#/components/schemas/APIKey"},
			"secret":  map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("APIKeyList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/APIKey"}},
		},
	})
	registry.RegisterSchema("WebhookEventCatalog", map[string]any{
		"type":     "object",
		"required": []string{"event_types"},
		"properties": map[string]any{
			"event_types": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{
				"widget.created",
				"widget.updated",
				"widget.deleted",
				"widget.import.completed",
				// api-toolkit:openapi-webhook-event-types
			}}},
		},
	})
	registry.RegisterSchema("Object", map[string]any{
		"type":     "object",
		"required": []string{"tenant_id", "key", "content_type", "size", "created_at", "updated_at"},
		"properties": map[string]any{
			"tenant_id":    map[string]any{"type": "string"},
			"key":          map[string]any{"type": "string"},
			"content_type": map[string]any{"type": "string"},
			"size":         map[string]any{"type": "integer", "minimum": 0},
			"created_at":   map[string]any{"type": "string", "format": "date-time"},
			"updated_at":   map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("ObjectPutRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"key", "content_type", "content_base64"},
		"additionalProperties": false,
		"properties": map[string]any{
			"key":            map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
			"content_type":   map[string]any{"type": "string", "enum": []string{"application/json", "application/pdf", "image/jpeg", "image/png", "text/plain"}},
			"content_base64": map[string]any{"type": "string", "format": "byte"},
		},
	})
	registry.RegisterSchema("ObjectRead", map[string]any{
		"type":     "object",
		"required": []string{"object", "content_base64"},
		"properties": map[string]any{
			"object":         map[string]any{"$ref": "#/components/schemas/Object"},
			"content_base64": map[string]any{"type": "string", "format": "byte"},
		},
	})
	registry.RegisterSchema("ObjectList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Object"}},
		},
	})
	registry.RegisterSchema("WebhookEndpoint", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "url", "events", "created_at"},
		"properties": map[string]any{
			"id":         map[string]any{"type": "string"},
			"tenant_id":  map[string]any{"type": "string"},
			"url":        map[string]any{"type": "string", "format": "uri"},
			"events":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"disabled":   map[string]any{"type": "boolean"},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
			"updated_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("WebhookEndpointCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"url", "events"},
		"additionalProperties": false,
		"properties": map[string]any{
			"url":    map[string]any{"type": "string", "format": "uri"},
			"events": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
		},
	})
	registry.RegisterSchema("WebhookEndpointCreated", map[string]any{
		"type":     "object",
		"required": []string{"endpoint", "secret"},
		"properties": map[string]any{
			"endpoint": map[string]any{"$ref": "#/components/schemas/WebhookEndpoint"},
			"secret":   map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("WebhookEndpointList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WebhookEndpoint"}},
		},
	})
	registry.RegisterSchema("WebhookDelivery", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "endpoint_id", "event_id", "event_type", "url", "state", "attempt", "next_at", "created_at", "updated_at"},
		"properties": map[string]any{
			"id":               map[string]any{"type": "string"},
			"tenant_id":        map[string]any{"type": "string"},
			"endpoint_id":      map[string]any{"type": "string"},
			"event_id":         map[string]any{"type": "string"},
			"event_type":       map[string]any{"type": "string"},
			"url":              map[string]any{"type": "string", "format": "uri"},
			"state":            map[string]any{"type": "string", "enum": []string{"pending", "leased", "succeeded", "failed", "dead_letter"}},
			"attempt":          map[string]any{"type": "integer", "minimum": 0},
			"next_at":          map[string]any{"type": "string", "format": "date-time"},
			"last_status_code": map[string]any{"type": "integer", "nullable": true},
			"last_error":       map[string]any{"type": "string", "nullable": true},
			"created_at":       map[string]any{"type": "string", "format": "date-time"},
			"updated_at":       map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("WebhookDeliveryList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WebhookDelivery"}},
		},
	})
	registry.RegisterSchema("WebhookReplayRequest", map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           map[string]any{},
	})
	// api-toolkit:openapi-schemas
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
	widgetImportBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetImportRequest"},
		},
	}
	operationAcceptedResponse := specs.Response{
		Description: "Operation accepted",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/OperationAccepted"},
		},
	}
	operationResponse := specs.Response{
		Description: "Operation",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetImportOperation"},
		},
	}
	organizationCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/OrganizationCreateRequest"},
		},
	}
	invitationCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationCreateRequest"},
		},
	}
	invitationAcceptBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationAcceptRequest"},
		},
	}
	organizationResponse := specs.Response{
		Description: "Organization",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Organization"},
		},
	}
	membershipResponse := specs.Response{
		Description: "Membership",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Membership"},
		},
	}
	invitationCreatedResponse := specs.Response{
		Description: "Invitation created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationCreated"},
		},
	}
	apiKeyCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/APIKeyCreateRequest"},
		},
	}
	apiKeyCreatedResponse := specs.Response{
		Description: "API key created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/APIKeyCreated"},
		},
	}
	webhookEventCatalogResponse := specs.Response{
		Description: "Webhook event catalog",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEventCatalog"},
		},
	}
	webhookEndpointCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointCreateRequest"},
		},
	}
	webhookEndpointCreatedResponse := specs.Response{
		Description: "Webhook endpoint created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointCreated"},
		},
	}
	webhookEndpointListResponse := specs.Response{
		Description: "Webhook endpoint list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointList"},
		},
	}
	webhookDeliveryListResponse := specs.Response{
		Description: "Webhook delivery list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookDeliveryList"},
		},
	}
	webhookDeliveryResponse := specs.Response{
		Description: "Webhook delivery",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookDelivery"},
		},
	}
	webhookReplayBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookReplayRequest"},
		},
	}
	objectPutBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectPutRequest"},
		},
	}
	objectResponse := specs.Response{
		Description: "Object",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Object"},
		},
	}
	objectReadResponse := specs.Response{
		Description: "Object content",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectRead"},
		},
	}
	objectListResponse := specs.Response{
		Description: "Object list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectList"},
		},
	}
	// api-toolkit:openapi-operation-variables
	return []specs.Operation{
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getLiveness",
			Method:      http.MethodGet,
			Path:        "/livez",
			Summary:     "Liveness",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Live"}},
		}),
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
			OperationID: "listOrganizations",
			Method:      http.MethodGet,
			Path:        "/organizations",
			Summary:     "List organizations",
			Security:    auth("organizations:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Organization list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/OrganizationList"},
					},
				},
			},
		}, routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganization",
			Method:      http.MethodPost,
			Path:        "/organizations",
			Summary:     "Create organization",
			Parameters: []specs.Parameter{
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("organizations:write"),
			RequestBody: organizationCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: organizationResponse},
		}, routepolicy.WithTenantRequired("actor"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationMembers",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/members",
			Summary:     "List organization members",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security: auth("members:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Membership list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/MembershipList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationInvitation",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/invitations",
			Summary:     "Create organization invitation",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("invitations:write"),
			RequestBody: invitationCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: invitationCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationAPIKeys",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/api-keys",
			Summary:     "List organization API keys",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security: auth("api-keys:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "API key list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/APIKeyList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationAPIKey",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/api-keys",
			Summary:     "Create organization API key",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("api-keys:write"),
			RequestBody: apiKeyCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: apiKeyCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "revokeOrganizationAPIKey",
			Method:      http.MethodDelete,
			Path:        "/organizations/{organization_id}/api-keys/{api_key_id}",
			Summary:     "Revoke organization API key",
			Parameters: []specs.Parameter{
				{Name: "api_key_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("api-keys:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Revoked"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listWebhookEvents",
			Method:      http.MethodGet,
			Path:        "/webhook-events",
			Summary:     "List webhook event types",
			Security:    auth("webhooks:read"),
			Responses:   map[int]specs.Response{http.StatusOK: webhookEventCatalogResponse},
		}, routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationWebhookEndpoints",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/webhook-endpoints",
			Summary:     "List organization webhook endpoints",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("webhooks:read"),
			Responses: map[int]specs.Response{http.StatusOK: webhookEndpointListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationWebhookEndpoint",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/webhook-endpoints",
			Summary:     "Create organization webhook endpoint",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("webhooks:write"),
			RequestBody: webhookEndpointCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: webhookEndpointCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationWebhookDeliveries",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/webhook-deliveries",
			Summary:     "List organization webhook deliveries",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("webhooks:read"),
			Responses: map[int]specs.Response{http.StatusOK: webhookDeliveryListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "replayOrganizationWebhookDelivery",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/webhook-deliveries/{delivery_id}/replay",
			Summary:     "Replay organization webhook delivery",
			Parameters: []specs.Parameter{
				{Name: "delivery_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("webhooks:write"),
			RequestBody: webhookReplayBody,
			Responses:   map[int]specs.Response{http.StatusAccepted: webhookDeliveryResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationObjects",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/objects",
			Summary:     "List organization objects",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:read"),
			Responses: map[int]specs.Response{http.StatusOK: objectListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "putOrganizationObject",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/objects",
			Summary:     "Put organization object",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("objects:write"),
			RequestBody: objectPutBody,
			Responses:   map[int]specs.Response{http.StatusCreated: objectResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOrganizationObject",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/objects/{object_key}",
			Summary:     "Get organization object",
			Parameters: []specs.Parameter{
				{Name: "object_key", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:read"),
			Responses: map[int]specs.Response{http.StatusOK: objectReadResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "deleteOrganizationObject",
			Method:      http.MethodDelete,
			Path:        "/organizations/{organization_id}/objects/{object_key}",
			Summary:     "Delete organization object",
			Parameters: []specs.Parameter{
				{Name: "object_key", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Deleted"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "acceptInvitation",
			Method:      http.MethodPost,
			Path:        "/invitations/{id}/accept",
			Summary:     "Accept invitation",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("invitations:accept"),
			RequestBody: invitationAcceptBody,
			Responses:   map[int]specs.Response{http.StatusOK: membershipResponse},
		}, routepolicy.WithTenantRequired("invitation"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOperation",
			Method:      http.MethodGet,
			Path:        "/operations/{id}",
			Summary:     "Get operation",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("operations:read"),
			Responses: map[int]specs.Response{http.StatusOK: operationResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
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
			OperationID: "createWidgetImport",
			Method:      http.MethodPost,
			Path:        "/widgets/imports",
			Summary:     "Create widget import",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: widgetImportBody,
			Responses:   map[int]specs.Response{http.StatusAccepted: operationAcceptedResponse},
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
		// api-toolkit:openapi-operations
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
	data, err := readOpenAPIFile(path)
	if err != nil {
		return loadedOpenAPI{}, fmt.Errorf("load openapi: %w", err)
	}
	data, err = normalizeOpenAPI31NullableTypes(data)
	if err != nil {
		return loadedOpenAPI{}, fmt.Errorf("load openapi: %w", err)
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return loadedOpenAPI{}, fmt.Errorf("load openapi: %w", err)
	}
	return loadedOpenAPI{doc: doc, loader: loader}, nil
}

func readOpenAPIFile(path string) ([]byte, error) {
	clean := filepath.Clean(path)
	rootPath := "."
	rootName := clean
	if filepath.IsAbs(clean) {
		rootPath = string(filepath.Separator)
		rootName = strings.TrimPrefix(clean, rootPath)
	}
	if rootName == "." || rootName == "" {
		return nil, errors.New("openapi path must name a file")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(rootName)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	const maxOpenAPIBytes = 16 << 20
	data, err := io.ReadAll(io.LimitReader(file, maxOpenAPIBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxOpenAPIBytes {
		return nil, fmt.Errorf("openapi file exceeds %d bytes", maxOpenAPIBytes)
	}
	return data, nil
}

func normalizeOpenAPI31NullableTypes(data []byte) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	root, ok := doc.(map[string]any)
	if !ok {
		return data, nil
	}
	version, _ := root["openapi"].(string)
	if !strings.HasPrefix(strings.TrimSpace(version), "3.1") {
		return data, nil
	}
	normalizeNullableTypeArrays(root)
	normalized, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizeNullableTypeArrays(value any) {
	switch typed := value.(type) {
	case map[string]any:
		normalizeSchemaExamples(typed)
		if rawType, ok := typed["type"]; ok {
			if nullable, replacement, ok := nullableTypeReplacement(rawType); ok {
				typed["nullable"] = nullable
				typed["type"] = replacement
			}
		}
		for _, child := range typed {
			normalizeNullableTypeArrays(child)
		}
	case []any:
		for _, child := range typed {
			normalizeNullableTypeArrays(child)
		}
	}
}

func normalizeSchemaExamples(value map[string]any) {
	if !looksLikeSchemaObject(value) {
		return
	}
	rawExamples, ok := value["examples"]
	if !ok {
		return
	}
	examples, ok := rawExamples.([]any)
	if !ok || len(examples) == 0 {
		delete(value, "examples")
		return
	}
	if _, hasExample := value["example"]; !hasExample {
		value["example"] = examples[0]
	}
	delete(value, "examples")
}

func looksLikeSchemaObject(value map[string]any) bool {
	for _, key := range []string{
		"$ref",
		"additionalProperties",
		"allOf",
		"anyOf",
		"enum",
		"format",
		"items",
		"nullable",
		"oneOf",
		"properties",
		"required",
		"type",
	} {
		if _, ok := value[key]; ok {
			return true
		}
	}
	return false
}

func nullableTypeReplacement(raw any) (bool, any, bool) {
	values, ok := raw.([]any)
	if !ok {
		return false, nil, false
	}
	nonNull := make([]any, 0, len(values))
	nullable := false
	for _, value := range values {
		if text, ok := value.(string); ok && text == "null" {
			nullable = true
			continue
		}
		nonNull = append(nonNull, value)
	}
	if !nullable || len(nonNull) == 0 {
		return false, nil, false
	}
	if len(nonNull) == 1 {
		return true, nonNull[0], true
	}
	return true, nonNull, true
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

func lintGoClientCompatibility(doc *openapi3.T, operations []specs.Operation) []openAPILintFinding {
	var findings []openAPILintFinding
	methodOwners := map[string]specs.Operation{}
	for _, operation := range operations {
		operationID := strings.TrimSpace(operation.OperationID)
		if operationID == "" {
			continue
		}
		methodName := exportedGoIdentifier(operationID)
		if previous, ok := methodOwners[methodName]; ok && strings.TrimSpace(previous.OperationID) != operationID {
			findings = append(findings, openAPILintFinding{
				Code:    "go_client_method_id_conflict",
				Method:  operation.Method,
				Path:    operation.Path,
				Message: fmt.Sprintf("operationId %q and %q both generate Go method %s", previous.OperationID, operationID, methodName),
			})
			continue
		}
		methodOwners[methodName] = operation
	}
	if doc != nil && doc.Components != nil {
		schemaOwners := map[string]string{}
		for _, schemaName := range sortedSchemaNames(doc.Components.Schemas) {
			if strings.EqualFold(schemaName, "Problem") {
				continue
			}
			typeName := exportedGoIdentifier(schemaName)
			if goClientReservedTypeNames()[typeName] {
				findings = append(findings, openAPILintFinding{
					Code:    "go_client_schema_id_conflict",
					Method:  "GLOBAL",
					Message: fmt.Sprintf("schema %q generates reserved Go type %s", schemaName, typeName),
				})
				continue
			}
			if previous, ok := schemaOwners[typeName]; ok && previous != schemaName {
				findings = append(findings, openAPILintFinding{
					Code:    "go_client_schema_id_conflict",
					Method:  "GLOBAL",
					Message: fmt.Sprintf("schemas %q and %q both generate Go type %s", previous, schemaName, typeName),
				})
				continue
			}
			schemaOwners[typeName] = schemaName
		}
	}
	for _, operation := range typedClientOperationsFromOpenAPI(doc) {
		findings = append(findings, lintGoClientPathParameterIdentifiers(operation)...)
		findings = append(findings, lintGoClientOptionParameterIdentifiers(operation)...)
	}
	return findings
}

func goClientReservedTypeNames() map[string]bool {
	return map[string]bool{
		"Client":        true,
		"Error":         true,
		"Option":        true,
		"Problem":       true,
		"RequestOption": true,
	}
}

func lintGoClientPathParameterIdentifiers(operation typedClientOperation) []openAPILintFinding {
	return lintGoClientParameterIdentifiers(operation, typedOperationPathParameters(operation), goParamName, "path parameter")
}

func lintGoClientOptionParameterIdentifiers(operation typedClientOperation) []openAPILintFinding {
	return lintGoClientParameterIdentifiers(operation, typedOperationOptionParameters(operation), exportedGoIdentifier, "request parameter")
}

func lintGoClientParameterIdentifiers(operation typedClientOperation, parameters []typedClientParameter, identifier func(string) string, label string) []openAPILintFinding {
	owners := map[string]string{}
	var findings []openAPILintFinding
	for _, parameter := range parameters {
		name := strings.TrimSpace(parameter.Name)
		if name == "" {
			continue
		}
		goName := identifier(name)
		key := strings.ToLower(strings.TrimSpace(parameter.In)) + ":" + name
		if previous, ok := owners[goName]; ok && previous != key {
			findings = append(findings, openAPILintFinding{
				Code:    "go_client_parameter_id_conflict",
				Method:  operation.Method,
				Path:    operation.Path,
				Message: fmt.Sprintf("%s %s and %s both generate Go identifier %s", label, previous, key, goName),
			})
			continue
		}
		owners[goName] = key
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
		if schema != nil && schema.Nullable {
			return "null"
		}
		return ""
	}
	types := append([]string(nil), schema.Type.Slice()...)
	if schema.Nullable {
		types = append(types, "null")
	}
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

func typedParametersFromOpenAPI(parameters openapi3.Parameters) []typedClientParameter {
	out := make([]typedClientParameter, 0, len(parameters))
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
		out = append(out, typedClientParameter{
			Name:     name,
			In:       in,
			Required: parameter.Required,
			Schema:   parameter.Schema,
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

func mergeTypedParameters(groups ...[]typedClientParameter) []typedClientParameter {
	indexByKey := map[string]int{}
	var out []typedClientParameter
	for _, group := range groups {
		for _, parameter := range group {
			key := typedParameterKey(parameter)
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

func typedParameterKey(parameter typedClientParameter) string {
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

func scaffoldFilesForConfig(cfg scaffoldConfig) []scaffoldFile {
	files := append([]scaffoldFile(nil), scaffoldFilesForProfile(cfg.Profile)...)
	if cfg.Profile != scaffoldProfileSaaSAPIFull {
		return files
	}
	if hasScaffoldProvider(cfg.Providers, scaffoldProviderStripe) {
		files = append(files, stripeBillingScaffoldFiles...)
	}
	if hasScaffoldProvider(cfg.Providers, scaffoldProviderResend) {
		files = append(files, resendEmailScaffoldFiles...)
	}
	if hasScaffoldProvider(cfg.Providers, scaffoldProviderClerkHooks) {
		files = append(files, clerkWebhooksScaffoldFiles...)
	}
	return files
}

var fullScaffoldFiles = []scaffoldFile{
	{Name: "go.mod", Body: fullGoModTemplate},
	{Name: "api-toolkit.yaml", Body: fullManifestTemplate},
	{Name: "cmd/api/main.go", Body: fullCmdMainTemplate},
	{Name: "cmd/worker/main.go", Body: fullCmdWorkerTemplate},
	{Name: "cmd/migrate/main.go", Body: fullCmdMigrateTemplate},
	{Name: "internal/domain/api_key.go", Body: fullDomainAPIKeyTemplate},
	{Name: "internal/domain/tenancy.go", Body: fullDomainTenancyTemplate},
	{Name: "internal/domain/widget.go", Body: fullDomainWidgetTemplate},
	{Name: "internal/app/audit.go", Body: fullAppAuditTemplate},
	{Name: "internal/app/audit_test.go", Body: fullAppAuditTestTemplate},
	{Name: "internal/app/cache.go", Body: fullAppCacheTemplate},
	{Name: "internal/app/cache_test.go", Body: fullAppCacheTestTemplate},
	{Name: "internal/app/api_keys.go", Body: fullAppAPIKeysTemplate},
	{Name: "internal/app/api_keys_test.go", Body: fullAppAPIKeysTestTemplate},
	{Name: "internal/app/async.go", Body: fullAppAsyncTemplate},
	{Name: "internal/app/async_test.go", Body: fullAppAsyncTestTemplate},
	{Name: "internal/app/tenancy.go", Body: fullAppTenancyTemplate},
	{Name: "internal/app/tenancy_test.go", Body: fullAppTenancyTestTemplate},
	{Name: "internal/app/widgets.go", Body: fullAppWidgetsTemplate},
	{Name: "internal/app/webhooks.go", Body: fullAppWebhooksTemplate},
	{Name: "internal/app/webhooks_test.go", Body: fullAppWebhooksTestTemplate},
	{Name: "internal/app/objects.go", Body: fullAppObjectsTemplate},
	{Name: "internal/app/objects_test.go", Body: fullAppObjectsTestTemplate},
	{Name: "internal/adapters/postgres/postgres.go", Body: fullPostgresAdapterTemplate},
	{Name: "internal/adapters/postgres/postgres_test.go", Body: fullPostgresAdapterTestTemplate},
	{Name: "internal/adapters/postgres/tenancy.go", Body: fullPostgresTenancyStoreTemplate},
	{Name: "internal/adapters/postgres/tenancy_test.go", Body: fullPostgresTenancyStoreTestTemplate},
	{Name: "internal/adapters/postgres/widgets.go", Body: fullPostgresWidgetStoreTemplate},
	{Name: "internal/adapters/postgres/widgets_test.go", Body: fullPostgresWidgetStoreTestTemplate},
	{Name: "internal/adapters/postgres/objects.go", Body: fullPostgresObjectStoreTemplate},
	{Name: "internal/adapters/postgres/objects_test.go", Body: fullPostgresObjectStoreTestTemplate},
	{Name: "internal/adapters/postgres/api_keys.go", Body: fullPostgresAPIKeyStoreTemplate},
	{Name: "internal/adapters/postgres/api_keys_test.go", Body: fullPostgresAPIKeyStoreTestTemplate},
	{Name: "internal/adapters/postgres/async.go", Body: fullPostgresAsyncStoreTemplate},
	{Name: "internal/adapters/postgres/async_test.go", Body: fullPostgresAsyncStoreTestTemplate},
	{Name: "internal/adapters/postgres/webhooks.go", Body: fullPostgresWebhookStoreTemplate},
	{Name: "internal/adapters/postgres/webhooks_test.go", Body: fullPostgresWebhookStoreTestTemplate},
	{Name: "internal/adapters/objectstore/s3.go", Body: fullObjectStoreS3AdapterTemplate},
	{Name: "internal/adapters/objectstore/s3_test.go", Body: fullObjectStoreS3AdapterTestTemplate},
	{Name: "internal/adapters/redis/cache.go", Body: fullRedisCacheAdapterTemplate},
	{Name: "internal/adapters/redis/cache_test.go", Body: fullRedisCacheAdapterTestTemplate},
	{Name: "internal/httpapi/openapi.go", Body: fullHTTPAPIOpenAPITemplate},
	{Name: "internal/httpapi/router.go", Body: fullHTTPAPIRouterTemplate},
	{Name: "internal/httpapi/router_test.go", Body: fullHTTPAPIRouterTestTemplate},
	{Name: "migrations/20260517000100_platform.up.sql", Body: fullMigrationTemplate},
	{Name: "scripts/integration_check.sh", Body: fullIntegrationCheckScriptTemplate},
	{Name: "Makefile", Body: fullMakefileTemplate},
	{Name: ".env.example", Body: fullEnvTemplate},
	{Name: ".gitignore", Body: fullGitignoreTemplate},
	{Name: ".dockerignore", Body: fullDockerignoreTemplate},
	{Name: ".github/workflows/ci.yml", Body: fullCIWorkflowTemplate},
	{Name: ".github/workflows/integration.yml", Body: fullIntegrationWorkflowTemplate},
	{Name: "Dockerfile", Body: fullDockerfileTemplate},
	{Name: "docker-compose.yml", Body: fullComposeTemplate},
	{Name: "deploy/kubernetes/configmap.yaml", Body: fullKubernetesConfigMapTemplate},
	{Name: "deploy/kubernetes/secret.example.yaml", Body: fullKubernetesSecretTemplate},
	{Name: "deploy/kubernetes/migration-job.yaml", Body: fullKubernetesMigrationJobTemplate},
	{Name: "deploy/kubernetes/deployment.yaml", Body: fullKubernetesDeploymentTemplate},
	{Name: "deploy/kubernetes/worker-deployment.yaml", Body: fullKubernetesWorkerDeploymentTemplate},
	{Name: "deploy/kubernetes/service.yaml", Body: fullKubernetesServiceTemplate},
	{Name: "deploy/kubernetes/admin-service.yaml", Body: fullKubernetesAdminServiceTemplate},
	{Name: "deploy/kubernetes/pod-disruption-budget.yaml", Body: fullKubernetesPDBTemplate},
	{Name: "deploy/kubernetes/hpa.yaml", Body: fullKubernetesHPATemplate},
	{Name: "deploy/kubernetes/network-policy.yaml", Body: fullKubernetesNetworkPolicyTemplate},
	{Name: "README.md", Body: fullReadmeTemplate},
}

var stripeBillingScaffoldFiles = []scaffoldFile{
	{Name: "internal/providers/stripebilling/billing.go", Body: providerStripeBillingTemplate},
	{Name: "internal/providers/stripebilling/billing_test.go", Body: providerStripeBillingTestTemplate},
	{Name: "docs/providers/stripe-billing.md", Body: providerStripeBillingDocTemplate},
}

var resendEmailScaffoldFiles = []scaffoldFile{
	{Name: "internal/providers/resendemail/invitations.go", Body: providerResendEmailTemplate},
	{Name: "internal/providers/resendemail/invitations_test.go", Body: providerResendEmailTestTemplate},
	{Name: "docs/providers/resend-email.md", Body: providerResendEmailDocTemplate},
}

var clerkWebhooksScaffoldFiles = []scaffoldFile{
	{Name: "internal/providers/clerkwebhooks/webhooks.go", Body: providerClerkWebhooksTemplate},
	{Name: "internal/providers/clerkwebhooks/webhooks_test.go", Body: providerClerkWebhooksTestTemplate},
	{Name: "docs/providers/clerk-webhooks.md", Body: providerClerkWebhooksDocTemplate},
}

const providerStripeBillingTemplate = `package stripebilling

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/contrib/v2/audit"
	compatbilling "github.com/aatuh/api-toolkit/v2/compat/billing"
	"{{ .Module }}/internal/app"
)

type Provider interface {
	CreateCheckoutSession(context.Context, compatbilling.CheckoutSessionRequest) (compatbilling.CheckoutSession, error)
	ParseWebhook(context.Context, []byte, string) (compatbilling.WebhookEvent, error)
}

type Service struct {
	Provider   Provider
	Audit      *app.AuditService
	SuccessURL string
	CancelURL  string
	PriceID    string
}

func NewService(provider Provider, auditLog *app.AuditService) *Service {
	return &Service{Provider: provider, Audit: auditLog}
}

func (s *Service) CreateCheckoutSession(ctx context.Context, actorID, tenantID string) (compatbilling.CheckoutSession, error) {
	if err := ctx.Err(); err != nil {
		return compatbilling.CheckoutSession{}, err
	}
	if s == nil || s.Provider == nil {
		return compatbilling.CheckoutSession{}, app.ErrValidation
	}
	actorID = strings.TrimSpace(actorID)
	tenantID = strings.TrimSpace(tenantID)
	priceID := strings.TrimSpace(s.PriceID)
	if actorID == "" || tenantID == "" || priceID == "" || strings.TrimSpace(s.SuccessURL) == "" || strings.TrimSpace(s.CancelURL) == "" {
		return compatbilling.CheckoutSession{}, app.ErrValidation
	}
	session, err := s.Provider.CreateCheckoutSession(ctx, compatbilling.CheckoutSessionRequest{
		Mode:              "subscription",
		PriceID:           priceID,
		SuccessURL:        strings.TrimSpace(s.SuccessURL),
		CancelURL:         strings.TrimSpace(s.CancelURL),
		ClientReferenceID: tenantID,
		Metadata: map[string]string{
			"tenant_id": tenantID,
			"actor_id":  actorID,
		},
		SubscriptionMetadata: map[string]string{
			"tenant_id": tenantID,
		},
	})
	if err != nil {
		return compatbilling.CheckoutSession{}, err
	}
	s.record(ctx, tenantID, actorID, "stripe.checkout_session.create", "billing_checkout_session", session.ID, audit.ResultSuccess, map[string]string{"price_id": priceID})
	return session, nil
}

func (s *Service) HandleWebhook(ctx context.Context, tenantID string, payload []byte, signature string) (compatbilling.WebhookEvent, error) {
	if err := ctx.Err(); err != nil {
		return compatbilling.WebhookEvent{}, err
	}
	if s == nil || s.Provider == nil {
		return compatbilling.WebhookEvent{}, app.ErrValidation
	}
	event, err := s.Provider.ParseWebhook(ctx, payload, strings.TrimSpace(signature))
	if err != nil {
		return compatbilling.WebhookEvent{}, err
	}
	event.ID = strings.TrimSpace(event.ID)
	event.Type = strings.TrimSpace(event.Type)
	if event.ID == "" || event.Type == "" {
		return compatbilling.WebhookEvent{}, app.ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	payloadTenant := tenantIDFromStripePayload(event.Payload)
	if tenantID != "" && payloadTenant != "" && tenantID != payloadTenant {
		s.record(ctx, tenantID, "stripe", "stripe.webhook.tenant_mismatch", "billing_webhook", event.ID, audit.ResultFailure, map[string]string{"event_type": event.Type})
		return compatbilling.WebhookEvent{}, app.ErrForbidden
	}
	if tenantID == "" {
		tenantID = payloadTenant
	}
	if tenantID == "" {
		return compatbilling.WebhookEvent{}, app.ErrValidation
	}
	s.record(ctx, tenantID, "stripe", "stripe.webhook."+event.Type, "billing_webhook", event.ID, audit.ResultSuccess, map[string]string{"event_type": event.Type})
	return event, nil
}

func (s *Service) ServeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid webhook body", http.StatusBadRequest)
		return
	}
	if _, err := s.HandleWebhook(r.Context(), r.Header.Get("X-Tenant-ID"), body, r.Header.Get("Stripe-Signature")); err != nil {
		if errors.Is(err, app.ErrForbidden) {
			http.Error(w, "webhook rejected", http.StatusForbidden)
			return
		}
		http.Error(w, "webhook rejected", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func tenantIDFromStripePayload(payload []byte) string {
	var event struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
		Object   struct {
			Metadata map[string]string ` + "`json:\"metadata\"`" + `
		} ` + "`json:\"object\"`" + `
		Data struct {
			Object struct {
				Metadata map[string]string ` + "`json:\"metadata\"`" + `
			} ` + "`json:\"object\"`" + `
		} ` + "`json:\"data\"`" + `
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return ""
	}
	for _, metadata := range []map[string]string{event.Metadata, event.Object.Metadata, event.Data.Object.Metadata} {
		if tenantID := strings.TrimSpace(metadata["tenant_id"]); tenantID != "" {
			return tenantID
		}
	}
	return ""
}

func (s *Service) record(ctx context.Context, tenantID, actorID, action, resourceType, resourceID string, result audit.Result, metadata map[string]string) {
	if s == nil || s.Audit == nil {
		return
	}
	_ = s.Audit.Record(ctx, audit.Event{
		TenantID: strings.TrimSpace(tenantID),
		Actor: audit.Actor{Type: "provider", ID: strings.TrimSpace(actorID)},
		Action: action,
		Resource: audit.Resource{Type: resourceType, ID: strings.TrimSpace(resourceID)},
		Result: result,
		Metadata: metadata,
	})
}
`

const providerStripeBillingTestTemplate = `package stripebilling

import (
	"context"
	"errors"
	"strings"
	"testing"

	compatbilling "github.com/aatuh/api-toolkit/v2/compat/billing"
	"{{ .Module }}/internal/app"
)

func TestCreateCheckoutSessionRecordsTenantScopedAudit(t *testing.T) {
	auditLog := app.NewAuditService()
	provider := &fakeBillingProvider{session: compatbilling.CheckoutSession{ID: "cs_test_123", URL: "https://checkout.example.test/session"}}
	service := NewService(provider, auditLog)
	service.PriceID = "price_test"
	service.SuccessURL = "https://app.example.test/success"
	service.CancelURL = "https://app.example.test/cancel"

	session, err := service.CreateCheckoutSession(context.Background(), "user_1", "org_1")
	if err != nil {
		t.Fatalf("CreateCheckoutSession() error = %v", err)
	}
	if session.ID != "cs_test_123" || provider.request.ClientReferenceID != "org_1" || provider.request.Metadata["tenant_id"] != "org_1" {
		t.Fatalf("session=%#v request=%#v", session, provider.request)
	}
	events, err := auditLog.Events(context.Background())
	if err != nil || len(events) != 1 {
		t.Fatalf("audit events=%#v err=%v", events, err)
	}
	if events[0].TenantID != "org_1" || events[0].Action != "stripe.checkout_session.create" {
		t.Fatalf("audit event = %#v", events[0])
	}
}

func TestHandleWebhookRequiresVerifiedTenant(t *testing.T) {
	auditLog := app.NewAuditService()
	provider := &fakeBillingProvider{event: compatbilling.WebhookEvent{
		ID:      "evt_1",
		Type:    "checkout.session.completed",
		Payload: []byte(` + "`" + `{"data":{"object":{"metadata":{"tenant_id":"org_2"}}}}` + "`" + `),
	}}
	service := NewService(provider, auditLog)

	_, err := service.HandleWebhook(context.Background(), "org_1", []byte(` + "`" + `{"id":"evt_1"}` + "`" + `), "sig_test")
	if !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("HandleWebhook() error = %v, want forbidden", err)
	}
	events, err := auditLog.Events(context.Background())
	if err != nil || len(events) != 1 {
		t.Fatalf("audit events=%#v err=%v", events, err)
	}
	if events[0].Action != "stripe.webhook.tenant_mismatch" || strings.Contains(strings.Join(metadataValues(events[0].Metadata), " "), "sig_test") {
		t.Fatalf("unsafe audit event = %#v", events[0])
	}
}

type fakeBillingProvider struct {
	request compatbilling.CheckoutSessionRequest
	session compatbilling.CheckoutSession
	event   compatbilling.WebhookEvent
}

func (p *fakeBillingProvider) CreateCheckoutSession(_ context.Context, req compatbilling.CheckoutSessionRequest) (compatbilling.CheckoutSession, error) {
	p.request = req
	return p.session, nil
}

func (p *fakeBillingProvider) ParseWebhook(_ context.Context, _ []byte, _ string) (compatbilling.WebhookEvent, error) {
	return p.event, nil
}

func metadataValues(metadata map[string]string) []string {
	values := make([]string, 0, len(metadata))
	for _, value := range metadata {
		values = append(values, value)
	}
	return values
}
`

const providerStripeBillingDocTemplate = `# Stripe Billing Starter

This optional scaffold is generated only with ` + "`--with stripe-billing`" + `.

- ` + "`internal/providers/stripebilling`" + ` keeps Stripe-specific checkout and webhook behavior outside core app services.
- Checkout sessions include tenant and actor metadata so billing callbacks can be reconciled without trusting client-supplied state.
- Webhooks must be verified by the configured provider before audit writes or entitlement changes are trusted.
- Audit metadata intentionally excludes raw webhook signatures and provider secrets.

Configure ` + "`STRIPE_SECRET_KEY`" + `, ` + "`STRIPE_WEBHOOK_SECRET`" + `, ` + "`STRIPE_PRICE_ID`" + `, ` + "`STRIPE_SUCCESS_URL`" + `, and ` + "`STRIPE_CANCEL_URL`" + ` before wiring the real adapter.
`

const providerResendEmailTemplate = `package resendemail

import (
	"context"
	"fmt"
	"net/mail"
	"net/url"
	"strings"

	"github.com/aatuh/api-toolkit/contrib/v2/audit"
	"github.com/aatuh/api-toolkit/v2/email"
	"{{ .Module }}/internal/app"
)

type Sender interface {
	Send(context.Context, email.Message) (string, error)
}

type NoopSender struct{}

func (NoopSender) Send(context.Context, email.Message) (string, error) {
	return "noop-email", nil
}

type InvitationMailer struct {
	Sender  Sender
	Audit   *app.AuditService
	From    string
	BaseURL string
}

func NewInvitationMailer(sender Sender, auditLog *app.AuditService) *InvitationMailer {
	if sender == nil {
		sender = NoopSender{}
	}
	return &InvitationMailer{Sender: sender, Audit: auditLog}
}

func (m *InvitationMailer) SendInvitation(ctx context.Context, actorID, tenantID, recipient, invitationID, token string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if m == nil {
		return "", app.ErrValidation
	}
	if m.Sender == nil {
		m.Sender = NoopSender{}
	}
	actorID = strings.TrimSpace(actorID)
	tenantID = strings.TrimSpace(tenantID)
	recipient = strings.TrimSpace(recipient)
	invitationID = strings.TrimSpace(invitationID)
	token = strings.TrimSpace(token)
	if actorID == "" || tenantID == "" || invitationID == "" || token == "" || !validEmail(recipient) {
		return "", app.ErrValidation
	}
	baseURL := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	from := strings.TrimSpace(m.From)
	if from == "" {
		from = "no-reply@example.com"
	}
	acceptURL := baseURL + "/invitations/" + url.PathEscape(invitationID) + "/accept?token=" + url.QueryEscape(token)
	message := email.Message{
		From:    from,
		To:      []string{recipient},
		Subject: "You have been invited",
		Text:    fmt.Sprintf("Accept your invitation: %s", acceptURL),
	}
	emailID, err := m.Sender.Send(ctx, message)
	if err != nil {
		return "", err
	}
	m.record(ctx, tenantID, actorID, "invitation.email.send", invitationID, map[string]string{
		"email_id":         strings.TrimSpace(emailID),
		"recipient_domain": recipientDomain(recipient),
	})
	return emailID, nil
}

func validEmail(value string) bool {
	_, err := mail.ParseAddress(value)
	return err == nil
}

func recipientDomain(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[1]))
}

func (m *InvitationMailer) record(ctx context.Context, tenantID, actorID, action, invitationID string, metadata map[string]string) {
	if m == nil || m.Audit == nil {
		return
	}
	_ = m.Audit.Record(ctx, audit.Event{
		TenantID: strings.TrimSpace(tenantID),
		Actor: audit.Actor{Type: "user", ID: strings.TrimSpace(actorID)},
		Action: action,
		Resource: audit.Resource{Type: "invitation", ID: strings.TrimSpace(invitationID)},
		Result: audit.ResultSuccess,
		Metadata: metadata,
	})
}
`

const providerResendEmailTestTemplate = `package resendemail

import (
	"context"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/email"
	"{{ .Module }}/internal/app"
)

func TestSendInvitationUsesNoopFallbackAndRedactsTokenFromAudit(t *testing.T) {
	auditLog := app.NewAuditService()
	mailer := NewInvitationMailer(nil, auditLog)
	mailer.BaseURL = "https://app.example.test"
	mailer.From = "invites@example.test"

	emailID, err := mailer.SendInvitation(context.Background(), "user_1", "org_1", "member@example.test", "inv_1", "secret-token")
	if err != nil {
		t.Fatalf("SendInvitation() error = %v", err)
	}
	if emailID != "noop-email" {
		t.Fatalf("emailID = %q", emailID)
	}
	events, err := auditLog.Events(context.Background())
	if err != nil || len(events) != 1 {
		t.Fatalf("audit events=%#v err=%v", events, err)
	}
	if events[0].Action != "invitation.email.send" || events[0].Metadata["recipient_domain"] != "example.test" {
		t.Fatalf("audit event = %#v", events[0])
	}
	if strings.Contains(strings.Join(metadataValues(events[0].Metadata), " "), "secret-token") || strings.Contains(strings.Join(metadataValues(events[0].Metadata), " "), "member@example.test") {
		t.Fatalf("audit metadata leaked invite details: %#v", events[0].Metadata)
	}
}

func TestSendInvitationSendsExpectedEmailBodyThroughBoundary(t *testing.T) {
	sender := &recordingSender{id: "email_123"}
	mailer := NewInvitationMailer(sender, app.NewAuditService())
	mailer.BaseURL = "https://app.example.test"
	mailer.From = "invites@example.test"

	if _, err := mailer.SendInvitation(context.Background(), "user_1", "org_1", "member@example.test", "inv_1", "secret-token"); err != nil {
		t.Fatalf("SendInvitation() error = %v", err)
	}
	if sender.message.From != "invites@example.test" || sender.message.To[0] != "member@example.test" || !strings.Contains(sender.message.Text, "secret-token") {
		t.Fatalf("message = %#v", sender.message)
	}
}

type recordingSender struct {
	id      string
	message email.Message
}

func (s *recordingSender) Send(_ context.Context, msg email.Message) (string, error) {
	s.message = msg
	return s.id, nil
}

func metadataValues(metadata map[string]string) []string {
	values := make([]string, 0, len(metadata))
	for _, value := range metadata {
		values = append(values, value)
	}
	return values
}
`

const providerResendEmailDocTemplate = `# Resend Email Starter

This optional scaffold is generated only with ` + "`--with resend-email`" + `.

- ` + "`internal/providers/resendemail`" + ` exposes an invitation mailer behind a small sender interface.
- Local development uses a no-op sender fallback, so tests and setup do not require a provider account.
- Audit metadata records the email provider ID and recipient domain only. Invitation tokens and full recipient addresses are not audit metadata.

Configure ` + "`RESEND_API_KEY`" + `, ` + "`RESEND_FROM`" + `, and ` + "`APP_BASE_URL`" + ` before wiring the real Resend adapter.
`

const providerClerkWebhooksTemplate = `package clerkwebhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/contrib/v2/audit"
	"{{ .Module }}/internal/app"
)

type Verifier interface {
	Verify(payload []byte, signature string) bool
}

type HMACVerifier struct {
	Secret string
}

func (v HMACVerifier) Verify(payload []byte, signature string) bool {
	secret := strings.TrimSpace(v.Secret)
	signature = strings.TrimSpace(signature)
	if secret == "" || signature == "" {
		return false
	}
	signature = strings.TrimPrefix(signature, "sha256=")
	got, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return hmac.Equal(got, mac.Sum(nil))
}

type Handler struct {
	Verifier Verifier
	Audit    *app.AuditService
}

func NewHandler(verifier Verifier, auditLog *app.AuditService) *Handler {
	return &Handler{Verifier: verifier, Audit: auditLog}
}

type Event struct {
	ID     string ` + "`json:\"id\"`" + `
	Type   string ` + "`json:\"type\"`" + `
	Data   struct {
		ID     string ` + "`json:\"id\"`" + `
		OrgID  string ` + "`json:\"org_id\"`" + `
		UserID string ` + "`json:\"user_id\"`" + `
	} ` + "`json:\"data\"`" + `
}

func (h *Handler) Handle(ctx context.Context, tenantID string, payload []byte, signature string) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	if h == nil || h.Verifier == nil {
		return Event{}, app.ErrValidation
	}
	if !h.Verifier.Verify(payload, signature) {
		return Event{}, app.ErrForbidden
	}
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, app.ErrValidation
	}
	event.ID = strings.TrimSpace(event.ID)
	event.Type = strings.TrimSpace(event.Type)
	event.Data.ID = strings.TrimSpace(event.Data.ID)
	event.Data.OrgID = strings.TrimSpace(event.Data.OrgID)
	event.Data.UserID = strings.TrimSpace(event.Data.UserID)
	if event.ID == "" || event.Type == "" {
		return Event{}, app.ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID != "" && event.Data.OrgID != "" && tenantID != event.Data.OrgID {
		h.record(ctx, tenantID, "clerk", "clerk_webhook.tenant_mismatch", event.ID, audit.ResultFailure, map[string]string{"event_type": event.Type})
		return Event{}, app.ErrForbidden
	}
	if tenantID == "" {
		tenantID = event.Data.OrgID
	}
	if tenantID == "" {
		return Event{}, app.ErrValidation
	}
	h.record(ctx, tenantID, "clerk", "clerk_webhook."+event.Type, event.ID, audit.ResultSuccess, map[string]string{"event_type": event.Type})
	return event, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid webhook body", http.StatusBadRequest)
		return
	}
	if _, err := h.Handle(r.Context(), r.Header.Get("X-Tenant-ID"), body, r.Header.Get("X-Clerk-Signature")); err != nil {
		if errors.Is(err, app.ErrForbidden) {
			http.Error(w, "webhook rejected", http.StatusForbidden)
			return
		}
		http.Error(w, "webhook rejected", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) record(ctx context.Context, tenantID, actorID, action, eventID string, result audit.Result, metadata map[string]string) {
	if h == nil || h.Audit == nil {
		return
	}
	_ = h.Audit.Record(ctx, audit.Event{
		TenantID: strings.TrimSpace(tenantID),
		Actor: audit.Actor{Type: "provider", ID: strings.TrimSpace(actorID)},
		Action: action,
		Resource: audit.Resource{Type: "clerk_event", ID: strings.TrimSpace(eventID)},
		Result: result,
		Metadata: metadata,
	})
}
`

const providerClerkWebhooksTestTemplate = `package clerkwebhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"{{ .Module }}/internal/app"
)

func TestHandleVerifiesSignatureAndRecordsAudit(t *testing.T) {
	payload := []byte(` + "`" + `{"id":"evt_1","type":"organization.created","data":{"id":"org_1","org_id":"org_1","user_id":"user_1"}}` + "`" + `)
	auditLog := app.NewAuditService()
	handler := NewHandler(HMACVerifier{Secret: "clerk-secret"}, auditLog)

	event, err := handler.Handle(context.Background(), "org_1", payload, signPayload("clerk-secret", payload))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if event.ID != "evt_1" || event.Data.OrgID != "org_1" {
		t.Fatalf("event = %#v", event)
	}
	events, err := auditLog.Events(context.Background())
	if err != nil || len(events) != 1 {
		t.Fatalf("audit events=%#v err=%v", events, err)
	}
	if events[0].Action != "clerk_webhook.organization.created" || strings.Contains(strings.Join(metadataValues(events[0].Metadata), " "), "clerk-secret") {
		t.Fatalf("unsafe audit event = %#v", events[0])
	}
}

func TestHandleRejectsBadSignatureAndTenantMismatch(t *testing.T) {
	payload := []byte(` + "`" + `{"id":"evt_1","type":"organization.updated","data":{"id":"org_2","org_id":"org_2","user_id":"user_1"}}` + "`" + `)
	handler := NewHandler(HMACVerifier{Secret: "clerk-secret"}, app.NewAuditService())

	if _, err := handler.Handle(context.Background(), "org_2", payload, "bad-signature"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("bad signature error = %v, want forbidden", err)
	}
	if _, err := handler.Handle(context.Background(), "org_1", payload, signPayload("clerk-secret", payload)); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("tenant mismatch error = %v, want forbidden", err)
	}
}

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func metadataValues(metadata map[string]string) []string {
	values := make([]string, 0, len(metadata))
	for _, value := range metadata {
		values = append(values, value)
	}
	return values
}
`

const providerClerkWebhooksDocTemplate = `# Clerk Webhooks Starter

This optional scaffold is generated only with ` + "`--with clerk-webhooks`" + `.

- ` + "`internal/providers/clerkwebhooks`" + ` verifies signed callbacks before sync hooks trust provider payloads.
- Tenant mismatches between the request tenant and provider organization are rejected and audited as failures.
- Audit metadata includes event type only; webhook secrets and signatures are not recorded.

Configure ` + "`CLERK_WEBHOOK_SECRET`" + ` before mounting the handler.
`

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

func formatParamValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return strings.Join(typed, ",")
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
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

func DecodeJSONResponse[T any](resp *http.Response) (*T, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("response body is required")
	}
	defer resp.Body.Close()
	var out T
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response body: %w", err)
	}
	return &out, nil
}

func copyHeaders(dst, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

`

const fullManifestTemplate = `profile: {{ .Profile }}
module: {{ .Module }}
generator_version: {{ .CoreVersion }}
openapi: testdata/openapi.golden.json
client:
  go:
    path: internal/client/apiclient
    package: apiclient
resources:
  # api-toolkit:manifest-resources
providers:
{{ if eq .HasStripeBilling "true" }}  - stripe-billing
{{ end }}{{ if eq .HasResendEmail "true" }}  - resend-email
{{ end }}{{ if eq .HasClerkWebhooks "true" }}  - clerk-webhooks
{{ end }}{{ if eq .HasProviderWorkflows "true" }}
{{ end }}
  # api-toolkit:manifest-providers
`

const fullGoModTemplate = `module {{ .Module }}

go 1.25.0

require (
	github.com/aatuh/api-toolkit/v2 {{ .CoreVersion }}
	github.com/aatuh/api-toolkit/contrib/v2 {{ .ContribVersion }}
)

{{ .CoreReplace }}{{ .ContribReplace }}`

const fullCmdMainTemplate = `package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/auditpostgres"
	webhookdeliverypostgres "github.com/aatuh/api-toolkit/contrib/v2/adapters/webhookdeliverypostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/idempotency"
	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v2/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v2/async"
	"github.com/aatuh/api-toolkit/contrib/v2/bootstrap"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	"github.com/aatuh/api-toolkit/contrib/v2/webhookdelivery"
{{ if eq .AuthMode "clerk" }}	clerkauth "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/clerk"
{{ else if eq .AuthMode "oidc" }}	oidcauth "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/oidc"
{{ else if eq .AuthMode "jwt" }}	jwtauth "github.com/aatuh/api-toolkit/v2/middleware/auth/jwt"
{{ end }}
	"github.com/aatuh/api-toolkit/v2/endpoints/version"
	"github.com/aatuh/api-toolkit/v2/ports"
	objectstorage "{{ .Module }}/internal/adapters/objectstore"
	"{{ .Module }}/internal/adapters/postgres"
	rediscache "{{ .Module }}/internal/adapters/redis"
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
	tenancy := app.NewTenancyService()
	apiKeys := app.NewAPIKeyService(cfg.APIKeyPepper, tenancy)
	auditLog := app.NewAuditService()
	webhookEndpointPolicy := webhookdelivery.EndpointPolicy{AllowInsecureHTTP: !strings.EqualFold(os.Getenv("ENV"), "production")}
	webhooks := app.NewWebhookServiceWithEndpointPolicy(tenancy, webhookEndpointPolicy)
	objects := app.NewObjectService(tenancy)
	cacheService := app.NewCacheService(nil)
	// api-toolkit:main-service-defaults
	var rateLimiter ports.RateLimiter
	idempotencyStore := ports.IdempotencyStore(idempotency.NewMemoryStore())
	var objectMetadata app.ObjectMetadataStore
	var cacheReadiness httpapi.HealthChecker = cacheService
	var readiness httpapi.HealthChecker = httpapi.HealthCheckFunc(func(context.Context) error { return nil })
	var postgresPool ports.DatabasePool
	var webhookStore *postgres.WebhookStore
	if cfg.DatabaseURL != "" {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		pool, err := postgres.Open(dbCtx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			return err
		}
		dbCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		if err := postgres.CheckRequiredTables(dbCtx, pool, nil); err != nil {
			cancel()
			pool.Close()
			return err
		}
		cancel()
		defer pool.Close()
		postgresPool = &pgxpooladapter.Adapter{Pool: pool}
		tenancy = app.NewTenancyServiceWithStore(postgres.NewTenancyStore(pool))
		widgets = app.NewWidgetServiceWithStore(postgres.NewWidgetStore(pool))
		// api-toolkit:main-postgres-stores
		apiKeys = app.NewAPIKeyServiceWithStore(cfg.APIKeyPepper, tenancy, postgres.NewAPIKeyStore(pool))
		auditLog = app.NewAuditServiceWithRecorder(auditpostgres.New(postgresPool, auditpostgres.Options{}))
		webhookStore, err = postgres.NewWebhookStore(postgresPool, cfg.WebhookSecretKey)
		if err != nil {
			pool.Close()
			return err
		}
		webhooks = app.NewWebhookServiceWithStoreAndEndpointPolicy(tenancy, webhookStore, webhookEndpointPolicy)
		objectMetadata = postgres.NewObjectStore(pool)
		objects = app.NewObjectService(tenancy)
		readiness = postgres.HealthChecker{Pool: pool}
	}
	if strings.EqualFold(cfg.ObjectStore, "s3") {
		blobStore, err := objectstorage.OpenS3BlobStore(objectstorage.S3Config{
			Endpoint:        cfg.S3Endpoint,
			Region:          cfg.S3Region,
			Bucket:          cfg.S3Bucket,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
		})
		if err != nil {
			return err
		}
		objects = app.NewObjectServiceWithStores(tenancy, objectMetadata, blobStore)
	}
	if strings.EqualFold(cfg.CacheStore, "redis") {
		redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		redisCache, err := rediscache.OpenCache(redisCtx, cfg.RedisAddr)
		cancel()
		if err != nil {
			return err
		}
		defer redisCache.Close()
		cacheService = app.NewCacheService(redisCache.Store)
		cacheReadiness = redisCache
	}
	if strings.EqualFold(cfg.RateLimitStore, "redis") {
		redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		redisRateLimit, err := rediscache.OpenRateLimiter(redisCtx, cfg.RedisAddr, cfg.RateLimitKeyPrefix, 20, 10)
		cancel()
		if err != nil {
			return err
		}
		defer redisRateLimit.Close()
		rateLimiter = redisRateLimit.Limiter
	}
	if strings.EqualFold(cfg.IdempotencyStore, "redis") {
		redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		redisIdempotency, err := rediscache.OpenIdempotencyStore(redisCtx, cfg.RedisAddr, cfg.IdempotencyKeyPrefix)
		cancel()
		if err != nil {
			return err
		}
		defer redisIdempotency.Close()
		idempotencyStore = redisIdempotency.Store
	}
	rateLimitMiddleware, err := httpapi.NewRateLimitMiddleware(rateLimiter)
	if err != nil {
		return err
	}
	idempotencyMiddleware, err := httpapi.NewIdempotencyMiddleware(idempotencyStore)
	if err != nil {
		return err
	}
	metricsRecorder, err := metricsmw.NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		return err
	}
	metricsMiddleware, err := httpapi.NewMetricsMiddleware(metricsRecorder)
	if err != nil {
		return err
	}
	readiness = httpapi.CombineHealthChecks(readiness, cacheReadiness)
	asyncJobs := app.NewAsyncService(widgets)
	asyncStore := async.Store(asyncJobs)
	if postgresPool != nil {
		operationStore := postgres.NewWidgetImportOperationStore(postgresPool)
		outbox := postgres.NewWidgetImportOutbox(postgresPool, operationStore)
		asyncJobs = app.NewAsyncServiceWithStores(widgets, operationStore, outbox)
		asyncStore = outbox
	}
	asyncHandler, err := async.NewHandlerMux(async.HandlerRoute{Kind: app.WidgetImportJobKind, Handler: asyncJobs})
	if err != nil {
		return err
	}
	if webhookStore != nil {
		webhookDeliverer, err := webhookdelivery.NewDeliverer(webhookdelivery.DelivererConfig{
			EndpointPolicy: webhookdelivery.EndpointPolicy{AllowInsecureHTTP: !strings.EqualFold(os.Getenv("ENV"), "production")},
			Metrics:        metricsRecorder,
			UserAgent:      "saas-api-full-webhooks/1",
		})
		if err != nil {
			return err
		}
		webhookHandler, err := webhookdelivery.NewHandler(webhookdelivery.HandlerConfig{
			Endpoints: webhookStore,
			Deliverer: webhookDeliverer,
			Attempts:  webhookStore,
		})
		if err != nil {
			return err
		}
		if err := asyncHandler.Register(webhookdeliverypostgres.OutboxEventType, webhookHandler); err != nil {
			return err
		}
	}
	asyncRunner, err := async.New(async.Config{
		Store:        asyncStore,
		Handler:      asyncHandler,
		Logger:       ports.NopLogger{},
		BatchSize:    5,
		Concurrency:  2,
		PollInterval: time.Second,
	})
	if err != nil {
		return err
	}
	backgroundTasks := []bootstrap.BackgroundTask{}
	if cfg.AsyncWorkerEnabled {
		backgroundTasks = append(backgroundTasks, bootstrap.BackgroundTask{
			Name: "async-worker",
			Run:  asyncRunner.Run,
		})
	}
{{ if eq .AuthMode "jwt" }}	jwtMiddleware, err := newJWTMiddleware(ctx, cfg)
	if err != nil {
		return err
	}
	defer jwtMiddleware.Close()
{{ else if eq .AuthMode "clerk" }}	clerkMiddleware, err := newClerkMiddleware(ctx, cfg)
	if err != nil {
		return err
	}
	defer clerkMiddleware.Close()
{{ else if eq .AuthMode "oidc" }}	oidcMiddleware, err := newOIDCMiddleware(ctx, cfg)
	if err != nil {
		return err
	}
	defer oidcMiddleware.Close()
{{ end }}
	openAPIValidation, err := httpapi.NewOpenAPIValidationMiddleware(cfg)
	if err != nil {
		return err
	}
	routerConfig := httpapi.RouterConfig{
		Widgets:           widgets,
		Tenancy:           tenancy,
		APIKeys:           apiKeys,
		Async:             asyncJobs,
		Audit:             auditLog,
		Webhooks:          webhooks,
		Objects:           objects,
		Cache:             cacheService,
		Metrics:           metricsMiddleware,
		MetricsHandler:    metricsmw.PrometheusHandler(),
		OpenAPIValidation: openAPIValidation,
		RateLimit:         rateLimitMiddleware,
		Idempotency:       idempotencyMiddleware,
		Readiness:         readiness,
		AdminKey:          cfg.AdminKey,
		// api-toolkit:main-router-config
{{ if eq .AuthMode "jwt" }}		JWT: jwtMiddleware,
{{ else if eq .AuthMode "clerk" }}		Clerk: clerkMiddleware,
{{ else if eq .AuthMode "oidc" }}		OIDC: oidcMiddleware,
{{ else }}		APIKey: cfg.APIKey,
{{ end }}	}
	bootstrapRouterConfig, err := bootstrap.DefaultRouterConfigFromEnv(nil)
	if err != nil {
		return err
	}
	bootstrapRouterConfig.Metrics = metricsRecorder
	if rateLimiter != nil {
		bootstrapRouterConfig.RateLimit.Limiter = rateLimiter
	}
	router, err := bootstrap.NewDefaultRouterWithConfig(ports.NopLogger{}, bootstrapRouterConfig)
	if err != nil {
		return err
	}
	service, err := bootstrap.NewAPIService(bootstrap.APIServiceConfig{
		Addr:                    cfg.Addr,
		AdminAddr:               cfg.AdminAddr,
		Log:                     ports.NopLogger{},
		Router:                  router,
		MiddlewareOrder:         bootstrap.StrictSaaSAPIMiddlewareOrder(),
		RequiredMiddlewareOrder: bootstrap.StrictSaaSAPIMiddlewareOrder(),
		RegisterRoutes: func(r ports.HTTPRouter) error {
			return httpapi.RegisterRoutes(r, routerConfig)
		},
		BackgroundTasks: backgroundTasks,
		SystemEndpoints: bootstrap.SystemEndpoints{
			Health:  httpapi.NewHealthHandler(readiness),
			Version: version.NewHandler(version.Config{Info: ports.VersionInfo{Version: appVersion, Commit: buildCommit, Date: buildDate}}),
			Metrics: metricsmw.PrometheusHandler(),
			Pprof:   http.DefaultServeMux,
		},
		Admin: bootstrap.SystemEndpointAdminOptions{
			RequireAdmin: httpapi.RequireAdmin(cfg.AdminKey),
			EnablePprof:  true,
		},
	})
	if err != nil {
		return err
	}
	return service.Start(ctx)
}

{{ if eq .AuthMode "jwt" }}func newJWTMiddleware(ctx context.Context, cfg httpapi.Config) (*jwtauth.Middleware, error) {
	return jwtauth.NewMiddleware(ctx, jwtauth.Config{
		Enabled:             true,
		Issuer:              cfg.JWTIssuer,
		Audience:            cfg.JWTAudience,
		JWKSURL:             cfg.JWTJWKSURL,
		AllowedAlgorithms:   []string{"RS256"},
		AllowedClockSkew:    30 * time.Second,
		JWKSRefreshTimeout:  5 * time.Second,
		JWKSRefreshInterval: 10 * time.Minute,
	}, ports.NopLogger{})
}

{{ else if eq .AuthMode "clerk" }}func newClerkMiddleware(ctx context.Context, cfg httpapi.Config) (*clerkauth.Middleware, error) {
	return clerkauth.NewMiddleware(ctx, clerkauth.Config{
		Enabled:             true,
		Issuer:              cfg.ClerkIssuer,
		Audience:            cfg.ClerkAudience,
		JWKSURL:             cfg.ClerkJWKSURL,
		AllowedAlgorithms:   []string{"RS256"},
		AllowedClockSkew:    30 * time.Second,
		JWKSRefreshTimeout:  5 * time.Second,
		JWKSRefreshInterval: 10 * time.Minute,
	}, ports.NopLogger{})
}

{{ else if eq .AuthMode "oidc" }}func newOIDCMiddleware(ctx context.Context, cfg httpapi.Config) (*oidcauth.Middleware, error) {
	return oidcauth.NewMiddleware(ctx, oidcauth.Config{
		Enabled:             true,
		Issuer:              cfg.OIDCIssuer,
		Audience:            cfg.OIDCAudience,
		JWKSURL:             cfg.OIDCJWKSURL,
		DiscoveryURL:        cfg.OIDCDiscoveryURL,
		TenantClaim:         cfg.OIDCTenantClaim,
		ScopeClaim:          cfg.OIDCScopeClaim,
		AllowedAlgorithms:   []string{"RS256"},
		AllowedClockSkew:    30 * time.Second,
		JWKSRefreshTimeout:  5 * time.Second,
		JWKSRefreshInterval: 10 * time.Minute,
	}, ports.NopLogger{})
}

{{ end }}
`

const fullCmdWorkerTemplate = `package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	webhookdeliverypostgres "github.com/aatuh/api-toolkit/contrib/v2/adapters/webhookdeliverypostgres"
	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v2/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v2/async"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	"github.com/aatuh/api-toolkit/contrib/v2/webhookdelivery"
	"github.com/aatuh/api-toolkit/v2/ports"
	"{{ .Module }}/internal/adapters/postgres"
	"{{ .Module }}/internal/app"
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
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required for worker")
	}
	webhookSecretKey := strings.TrimSpace(os.Getenv("WEBHOOK_SECRET_KEY"))
	if webhookSecretKey == "" {
		return errors.New("WEBHOOK_SECRET_KEY is required for worker")
	}
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := postgres.Open(dbCtx, databaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	dbCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
	if err := postgres.CheckRequiredTables(dbCtx, pool, nil); err != nil {
		cancel()
		return err
	}
	cancel()

	postgresPool := &pgxpooladapter.Adapter{Pool: pool}
	widgets := app.NewWidgetServiceWithStore(postgres.NewWidgetStore(pool))
	operationStore := postgres.NewWidgetImportOperationStore(postgresPool)
	outbox := postgres.NewWidgetImportOutbox(postgresPool, operationStore)
	asyncJobs := app.NewAsyncServiceWithStores(widgets, operationStore, outbox)
	metricsRecorder, err := metricsmw.NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		return err
	}
	asyncHandler, err := async.NewHandlerMux(async.HandlerRoute{Kind: app.WidgetImportJobKind, Handler: asyncJobs})
	if err != nil {
		return err
	}
	webhookStore, err := postgres.NewWebhookStore(postgresPool, webhookSecretKey)
	if err != nil {
		return err
	}
	webhookDeliverer, err := webhookdelivery.NewDeliverer(webhookdelivery.DelivererConfig{
		EndpointPolicy: webhookdelivery.EndpointPolicy{AllowInsecureHTTP: !strings.EqualFold(os.Getenv("ENV"), "production")},
		Metrics:        metricsRecorder,
		UserAgent:      "saas-api-full-webhooks/1",
	})
	if err != nil {
		return err
	}
	webhookHandler, err := webhookdelivery.NewHandler(webhookdelivery.HandlerConfig{
		Endpoints: webhookStore,
		Deliverer: webhookDeliverer,
		Attempts:  webhookStore,
	})
	if err != nil {
		return err
	}
	if err := asyncHandler.Register(webhookdeliverypostgres.OutboxEventType, webhookHandler); err != nil {
		return err
	}
	asyncRunner, err := async.New(async.Config{
		Store:        outbox,
		Handler:      asyncHandler,
		Logger:       ports.NopLogger{},
		BatchSize:    5,
		Concurrency:  2,
		PollInterval: time.Second,
	})
	if err != nil {
		return err
	}
	return asyncRunner.Run(ctx)
}
`

const fullCmdMigrateTemplate = `package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/logzap"
	"github.com/aatuh/api-toolkit/contrib/v2/bootstrap"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")), "migration directory")
	table := flags.String("table", "schema_migrations", "migration table")
	lock := flags.Int64("lock", 0, "advisory lock key")
	timeout := flags.Duration("timeout", 15*time.Minute, "migration timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("command required: up | status | check")
	}
	if strings.TrimSpace(*dir) == "" {
		*dir = "migrations"
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	migrator, err := bootstrap.NewMigrator(databaseURL, *table, *lock, false, logzap.NewProduction(), []string{*dir}, nil)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer migrator.Close()

	switch strings.ToLower(flags.Arg(0)) {
	case "up":
		if err := bootstrap.RunUp(ctx, migrator, *dir); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "migrations applied")
		return nil
	case "status":
		status, err := bootstrap.Status(ctx, migrator, *dir)
		if err != nil {
			return err
		}
		fmt.Print(status)
		return nil
	case "check":
		status, err := bootstrap.Status(ctx, migrator, *dir)
		if err != nil {
			return err
		}
		if strings.Contains(status, "*") {
			fmt.Print(status)
			return errors.New("pending migrations")
		}
		fmt.Fprintln(os.Stdout, "migrations up-to-date")
		return nil
	default:
		return errors.New("unknown command; expected up, status, or check")
	}
}
`

const fullDomainAPIKeyTemplate = `package domain

import "time"

type APIKey struct {
	ID             string
	OrganizationID string
	Name           string
	Prefix         string
	Scopes         []string
	ExpiresAt      *time.Time
	LastUsedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

func (k APIKey) Public() map[string]any {
	return map[string]any{
		"id":              k.ID,
		"organization_id": k.OrganizationID,
		"name":            k.Name,
		"prefix":          k.Prefix,
		"scopes":          append([]string(nil), k.Scopes...),
		"expires_at":      k.ExpiresAt,
		"last_used_at":    k.LastUsedAt,
		"revoked_at":      k.RevokedAt,
		"created_at":      k.CreatedAt,
	}
}
`

const fullDomainTenancyTemplate = `package domain

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

func (r Role) Allows(required Role) bool {
	return roleRank(r) >= roleRank(required)
}

func roleRank(role Role) int {
	switch role {
	case RoleOwner:
		return 4
	case RoleAdmin:
		return 3
	case RoleMember:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

type Organization struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (o Organization) Public() map[string]any {
	return map[string]any{
		"id":         o.ID,
		"name":       o.Name,
		"created_at": o.CreatedAt,
		"updated_at": o.UpdatedAt,
	}
}

type Membership struct {
	OrganizationID string
	UserID         string
	Role           Role
	CreatedAt      time.Time
}

func (m Membership) Public() map[string]any {
	return map[string]any{
		"organization_id": m.OrganizationID,
		"user_id":         m.UserID,
		"role":            string(m.Role),
		"created_at":      m.CreatedAt,
	}
}

type Invitation struct {
	ID             string
	OrganizationID string
	Email          string
	Role           Role
	TokenPrefix    string
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	CreatedAt      time.Time
}

func (i Invitation) Public() map[string]any {
	return map[string]any{
		"id":              i.ID,
		"organization_id": i.OrganizationID,
		"email":           i.Email,
		"role":            string(i.Role),
		"token_prefix":    i.TokenPrefix,
		"expires_at":      i.ExpiresAt,
		"accepted_at":     i.AcceptedAt,
		"created_at":      i.CreatedAt,
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

const fullAppAuditTemplate = `package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/aatuh/api-toolkit/contrib/v2/audit"
)

type AuditService struct {
	mu       sync.Mutex
	next     int
	now      func() time.Time
	recorder audit.Recorder
	events   []audit.Event
}

func NewAuditService() *AuditService {
	return &AuditService{now: time.Now}
}

func NewAuditServiceWithRecorder(recorder audit.Recorder) *AuditService {
	service := NewAuditService()
	service.recorder = recorder
	return service
}

func (s *AuditService) Record(ctx context.Context, event audit.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	event.ID = fmt.Sprintf("aud_%06d", s.next)
	event.TenantID = strings.TrimSpace(event.TenantID)
	event.Actor.Type = cleanAuditLabel(event.Actor.Type)
	event.Actor.ID = strings.TrimSpace(event.Actor.ID)
	event.Action = cleanAuditLabel(event.Action)
	event.Resource.Type = cleanAuditLabel(event.Resource.Type)
	event.Resource.ID = strings.TrimSpace(event.Resource.ID)
	if event.Result == "" {
		event.Result = audit.ResultSuccess
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.now().UTC()
	}
	event.RequestID = strings.TrimSpace(event.RequestID)
	event.Metadata = safeAuditMetadata(event.Metadata)
	if err := audit.ValidateEvent(event); err != nil {
		return err
	}
	if s.recorder != nil {
		return s.recorder.Record(ctx, event)
	}
	s.events = append(s.events, event)
	return nil
}

func (s *AuditService) Events(ctx context.Context) ([]audit.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]audit.Event(nil), s.events...)
	for i := range out {
		out[i].Metadata = audit.CloneMetadata(out[i].Metadata)
	}
	return out, nil
}

func safeAuditMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		cleanKey := cleanAuditMetadataPart(key)
		if cleanKey == "" || unsafeAuditMetadataPart(cleanKey) {
			continue
		}
		value := cleanAuditMetadataPart(metadata[key])
		if value == "" || unsafeAuditMetadataPart(value) {
			continue
		}
		out[cleanKey] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanAuditLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			out.WriteRune(r)
		}
		if out.Len() >= 80 {
			break
		}
	}
	return out.String()
}

func cleanAuditMetadataPart(value string) string {
	value = strings.TrimSpace(value)
	var out strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			continue
		}
		out.WriteRune(r)
		if out.Len() >= 128 {
			break
		}
	}
	return strings.TrimSpace(out.String())
}

func unsafeAuditMetadataPart(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, token := range []string{"authorization", "bearer ", "cookie", "password", "private_key", "secret", "set-cookie", "token", "api_key", "apikey", "pepper", "idempotency"} {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

`

const fullAppAuditTestTemplate = `package app

import (
	"context"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v2/audit"
)

func TestAuditServiceRecordsAndRedactsMetadata(t *testing.T) {
	service := NewAuditService()
	err := service.Record(context.Background(), audit.Event{
		TenantID: "org_1",
		Actor: audit.Actor{Type: "user", ID: "usr_1"},
		Action: "widget.create",
		Resource: audit.Resource{Type: "widget", ID: "wgt_1"},
		Result: audit.ResultSuccess,
		RequestID: "req_1",
		Metadata: map[string]string{
			"count": "2",
			"api_key": "atk_secret",
			"note": "contains secret token",
		},
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	events, err := service.Events(context.Background())
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].ID == "" || events[0].OccurredAt.IsZero() {
		t.Fatalf("event missing generated fields: %#v", events[0])
	}
	if events[0].Metadata["count"] != "2" {
		t.Fatalf("safe metadata missing: %#v", events[0].Metadata)
	}
	for key, value := range events[0].Metadata {
		if strings.Contains(strings.ToLower(key), "key") || strings.Contains(strings.ToLower(value), "secret") {
			t.Fatalf("unsafe audit metadata survived: %#v", events[0].Metadata)
		}
	}
}

func TestAuditServiceWithRecorderRedactsBeforeDelegating(t *testing.T) {
	recorder := &recordingAuditRecorder{}
	service := NewAuditServiceWithRecorder(recorder)
	err := service.Record(context.Background(), audit.Event{
		TenantID: "org_1",
		Actor: audit.Actor{Type: "user", ID: "usr_1"},
		Action: "api_key.create",
		Resource: audit.Resource{Type: "api_key", ID: "key_1"},
		Result: audit.ResultSuccess,
		Metadata: map[string]string{
			"scope_count": "2",
			"token": "raw-secret-token",
		},
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("delegated events = %#v", recorder.events)
	}
	if recorder.events[0].ID == "" || recorder.events[0].OccurredAt.IsZero() {
		t.Fatalf("delegated event missing generated fields: %#v", recorder.events[0])
	}
	if _, ok := recorder.events[0].Metadata["token"]; ok {
		t.Fatalf("delegated metadata leaked token: %#v", recorder.events[0].Metadata)
	}
	if recorder.events[0].Metadata["scope_count"] != "2" {
		t.Fatalf("delegated metadata = %#v", recorder.events[0].Metadata)
	}
}

type recordingAuditRecorder struct {
	events []audit.Event
}

func (r *recordingAuditRecorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

`

const fullAppCacheTemplate = `package app

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/cache"
)

const webhookEventTypesCacheKey = "catalog:webhook-events"

type CacheService struct {
	store cache.Store
}

func NewCacheService(store cache.Store) *CacheService {
	if store == nil {
		store = NewMemoryCacheStore()
	}
	return &CacheService{store: store}
}

func (s *CacheService) WebhookEventTypes(ctx context.Context, loader func() []string) ([]string, bool, error) {
	var cached []string
	ok, err := s.GetJSON(ctx, webhookEventTypesCacheKey, &cached)
	if err != nil {
		return nil, false, err
	}
	if ok {
		return append([]string(nil), cached...), true, nil
	}
	if loader == nil {
		return nil, false, ErrValidation
	}
	loaded := append([]string(nil), loader()...)
	if err := s.SetJSON(ctx, webhookEventTypesCacheKey, loaded, time.Minute); err != nil {
		return nil, false, err
	}
	return loaded, false, nil
}

func (s *CacheService) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.store == nil || dst == nil {
		return false, ErrValidation
	}
	value, ok, err := s.store.Get(ctx, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(value, dst); err != nil {
		_ = s.store.Delete(ctx, key)
		return false, nil
	}
	return true, nil
}

func (s *CacheService) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.store == nil {
		return ErrValidation
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.store.Set(ctx, key, encoded, ttl)
}

func (s *CacheService) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.store == nil {
		return ErrValidation
	}
	return s.store.Delete(ctx, key)
}

func (s *CacheService) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.store == nil {
		return ErrValidation
	}
	const key = "health:cache"
	if err := s.store.Set(ctx, key, []byte("ok"), time.Second); err != nil {
		return err
	}
	value, ok, err := s.store.Get(ctx, key)
	if err != nil {
		return err
	}
	if !ok || string(value) != "ok" {
		return ErrValidation
	}
	return s.store.Delete(ctx, key)
}

type MemoryCacheStore struct {
	mu     sync.Mutex
	now    func() time.Time
	values map[string]memoryCacheEntry
}

type memoryCacheEntry struct {
	value     []byte
	expiresAt time.Time
}

func NewMemoryCacheStore() *MemoryCacheStore {
	return &MemoryCacheStore{now: time.Now, values: map[string]memoryCacheEntry{}}
}

func (s *MemoryCacheStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if err := cache.ValidateKey(key); err != nil {
		return nil, false, err
	}
	if s == nil {
		return nil, false, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.values[key]
	if !ok {
		return nil, false, nil
	}
	if !entry.expiresAt.IsZero() && !s.now().Before(entry.expiresAt) {
		delete(s.values, key)
		return nil, false, nil
	}
	return cache.CloneBytes(entry.value), true, nil
}

func (s *MemoryCacheStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cache.ValidateKey(key); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = s.now().Add(ttl)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = memoryCacheEntry{value: cache.CloneBytes(value), expiresAt: expiresAt}
	return nil
}

func (s *MemoryCacheStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cache.ValidateKey(key); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
	return nil
}

var _ cache.Store = (*MemoryCacheStore)(nil)

`

const fullAppCacheTestTemplate = `package app

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCacheStoreHonorsTTLAndClonesValues(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	store := NewMemoryCacheStore()
	store.now = func() time.Time { return now }
	value := []byte("alpha")
	if err := store.Set(ctx, "widgets:1", value, time.Second); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value[0] = 'x'
	got, ok, err := store.Get(ctx, "widgets:1")
	if err != nil || !ok || string(got) != "alpha" {
		t.Fatalf("Get() = %q ok=%v err=%v", got, ok, err)
	}
	got[0] = 'z'
	again, ok, err := store.Get(ctx, "widgets:1")
	if err != nil || !ok || string(again) != "alpha" {
		t.Fatalf("Get() after mutation = %q ok=%v err=%v", again, ok, err)
	}
	now = now.Add(time.Second)
	if _, ok, err := store.Get(ctx, "widgets:1"); err != nil || ok {
		t.Fatalf("expired Get() ok=%v err=%v", ok, err)
	}
}

func TestCacheServiceCachesWebhookEventCatalog(t *testing.T) {
	ctx := context.Background()
	service := NewCacheService(NewMemoryCacheStore())
	calls := 0
	events, hit, err := service.WebhookEventTypes(ctx, func() []string {
		calls++
		return []string{"widget.created"}
	})
	if err != nil || hit || calls != 1 || len(events) != 1 {
		t.Fatalf("first WebhookEventTypes() events=%v hit=%v calls=%d err=%v", events, hit, calls, err)
	}
	events, hit, err = service.WebhookEventTypes(ctx, func() []string {
		calls++
		return []string{"widget.deleted"}
	})
	if err != nil || !hit || calls != 1 || events[0] != "widget.created" {
		t.Fatalf("cached WebhookEventTypes() events=%v hit=%v calls=%d err=%v", events, hit, calls, err)
	}
	if err := service.Check(ctx); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
}

`

// #nosec G101 -- generated source uses API key and secret variable names, not hardcoded production credentials.
const fullAppAPIKeysTemplate = `package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"{{ .Module }}/internal/domain"
)

type APIKeyService struct {
	mu        sync.Mutex
	next      int
	keys      map[string]apiKeyRecord
	byHash    map[string]string
	pepper    string
	tenancy   *TenancyService
	now       func() time.Time
	newID     func() (string, error)
	newSecret func() (string, error)
	store     APIKeyStore
}

type apiKeyRecord struct {
	key  domain.APIKey
	hash string
}

type APIKeyStore interface {
	CreateAPIKey(ctx context.Context, key domain.APIKey, hash string) error
	ListAPIKeys(ctx context.Context, organizationID string) ([]domain.APIKey, error)
	GetAPIKeyByHash(ctx context.Context, hash string) (domain.APIKey, bool, error)
	RevokeAPIKey(ctx context.Context, organizationID, keyID string, revokedAt time.Time) (bool, error)
	TouchAPIKey(ctx context.Context, keyID string, lastUsedAt time.Time) error
}

func NewAPIKeyService(pepper string, tenancy *TenancyService) *APIKeyService {
	return &APIKeyService{
		keys:      map[string]apiKeyRecord{},
		byHash:    map[string]string{},
		pepper:    strings.TrimSpace(pepper),
		tenancy:   tenancy,
		now:       time.Now,
		newID:     randomAPIKeyID,
		newSecret: randomAPIKeySecret,
	}
}

func NewAPIKeyServiceWithStore(pepper string, tenancy *TenancyService, store APIKeyStore) *APIKeyService {
	service := NewAPIKeyService(pepper, tenancy)
	service.store = store
	return service
}

func (s *APIKeyService) Create(ctx context.Context, actorID, organizationID, name string, scopes []string, expiresAt *time.Time) (domain.APIKey, string, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIKey{}, "", err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	name = strings.TrimSpace(name)
	cleanScopes, ok := normalizeAPIKeyScopes(scopes)
	if actorID == "" || organizationID == "" || name == "" || !ok || strings.TrimSpace(s.pepper) == "" {
		return domain.APIKey{}, "", ErrValidation
	}
	if err := s.requireRole(ctx, actorID, organizationID, domain.RoleAdmin); err != nil {
		return domain.APIKey{}, "", err
	}
	secret, err := s.newSecret()
	if err != nil {
		return domain.APIKey{}, "", err
	}
	keyID, err := s.newID()
	if err != nil {
		return domain.APIKey{}, "", err
	}
	prefix := apiKeyPrefix(secret)
	recordHash := s.hashSecret(secret)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	now := s.now().UTC()
	key := domain.APIKey{
		ID:             keyID,
		OrganizationID: organizationID,
		Name:           name,
		Prefix:         prefix,
		Scopes:         cleanScopes,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
	}
	if s.store != nil {
		if err := s.store.CreateAPIKey(ctx, key, recordHash); err != nil {
			return domain.APIKey{}, "", err
		}
		return key, secret, nil
	}
	s.keys[key.ID] = apiKeyRecord{key: key, hash: recordHash}
	s.byHash[recordHash] = key.ID
	return key, secret, nil
}

func (s *APIKeyService) List(ctx context.Context, actorID, organizationID string) ([]domain.APIKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	if actorID == "" || organizationID == "" {
		return nil, ErrValidation
	}
	if err := s.requireRole(ctx, actorID, organizationID, domain.RoleAdmin); err != nil {
		return nil, err
	}
	if s.store != nil {
		return s.store.ListAPIKeys(ctx, organizationID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.APIKey, 0, len(s.keys))
	for _, record := range s.keys {
		if record.key.OrganizationID == organizationID {
			out = append(out, record.key)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *APIKeyService) Revoke(ctx context.Context, actorID, organizationID, keyID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	keyID = strings.TrimSpace(keyID)
	if actorID == "" || organizationID == "" || keyID == "" {
		return ErrValidation
	}
	if err := s.requireRole(ctx, actorID, organizationID, domain.RoleAdmin); err != nil {
		return err
	}
	if s.store != nil {
		revokedAt := s.now().UTC()
		ok, err := s.store.RevokeAPIKey(ctx, organizationID, keyID, revokedAt)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.keys[keyID]
	if !ok || record.key.OrganizationID != organizationID {
		return ErrNotFound
	}
	if record.key.RevokedAt == nil {
		now := s.now().UTC()
		record.key.RevokedAt = &now
		s.keys[keyID] = record
	}
	return nil
}

func (s *APIKeyService) Verify(ctx context.Context, secret string) (domain.APIKey, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIKey{}, false, err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" || strings.TrimSpace(s.pepper) == "" {
		return domain.APIKey{}, false, ErrValidation
	}
	hash := s.hashSecret(secret)
	if s.store != nil {
		key, ok, err := s.store.GetAPIKeyByHash(ctx, hash)
		if err != nil || !ok {
			return domain.APIKey{}, ok, err
		}
		now := s.now().UTC()
		if key.RevokedAt != nil || (key.ExpiresAt != nil && !now.Before(*key.ExpiresAt)) {
			return domain.APIKey{}, false, nil
		}
		key.LastUsedAt = &now
		if err := s.store.TouchAPIKey(ctx, key.ID, now); err != nil {
			return domain.APIKey{}, false, err
		}
		return key, true, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	keyID, ok := s.byHash[hash]
	if !ok {
		return domain.APIKey{}, false, nil
	}
	record := s.keys[keyID]
	now := s.now().UTC()
	if record.key.RevokedAt != nil || (record.key.ExpiresAt != nil && !now.Before(*record.key.ExpiresAt)) {
		return domain.APIKey{}, false, nil
	}
	record.key.LastUsedAt = &now
	s.keys[keyID] = record
	return record.key, true, nil
}

func (s *APIKeyService) requireRole(ctx context.Context, actorID, organizationID string, role domain.Role) error {
	if s.tenancy == nil {
		return ErrForbidden
	}
	ok, err := s.tenancy.HasRole(ctx, organizationID, actorID, role)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *APIKeyService) hashSecret(secret string) string {
	mac := hmac.New(sha256.New, []byte(s.pepper))
	_, _ = mac.Write([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomAPIKeyID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "key_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func randomAPIKeySecret() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "atk_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func apiKeyPrefix(secret string) string {
	secret = strings.TrimSpace(secret)
	if len(secret) <= 12 {
		return secret
	}
	return secret[:12]
}

func normalizeAPIKeyScopes(scopes []string) ([]string, bool) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || len(scope) > 80 || !safeAPIKeyScope(scope) {
			return nil, false
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	sort.Strings(out)
	return out, len(out) > 0
}

func safeAPIKeyScope(scope string) bool {
	for _, r := range scope {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case ':', '.', '_', '-':
			continue
		default:
			return false
		}
	}
	return true
}
`

// #nosec G101 -- generated tests use fixed API key secrets to verify hashing behavior.
const fullAppAPIKeysTestTemplate = `package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"{{ .Module }}/internal/domain"
)

func TestAPIKeyServiceHashesSecretAndRevokes(t *testing.T) {
	tenancy := NewTenancyService()
	tenancy.now = fixedAPIKeyTime
	org, _, err := tenancy.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewAPIKeyService("test-pepper", tenancy)
	service.now = fixedAPIKeyTime
	service.newSecret = func() (string, error) { return "atk_raw-secret-value", nil }
	expiresAt := fixedAPIKeyTime().Add(time.Hour)

	key, secret, err := service.Create(context.Background(), "owner_1", org.ID, "CI", []string{"widgets:write", "widgets:read", "widgets:read"}, &expiresAt)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if secret != "atk_raw-secret-value" {
		t.Fatalf("secret = %q", secret)
	}
	record := service.keys[key.ID]
	if record.hash == "" || strings.Contains(record.hash, secret) || record.hash == secret {
		t.Fatalf("stored hash is unsafe: %#v", record)
	}
	if key.Prefix == "" || key.Prefix == secret {
		t.Fatalf("prefix = %q secret=%q", key.Prefix, secret)
	}
	if len(key.Scopes) != 2 {
		t.Fatalf("deduped scopes = %#v", key.Scopes)
	}
	publicJSON, err := json.Marshal(key.Public())
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	if strings.Contains(string(publicJSON), secret) {
		t.Fatalf("public API key leaked secret: %s", publicJSON)
	}
	verified, ok, err := service.Verify(context.Background(), secret)
	if err != nil || !ok {
		t.Fatalf("Verify() key=%#v ok=%v err=%v", verified, ok, err)
	}
	if verified.LastUsedAt == nil {
		t.Fatalf("Verify() did not update last_used_at: %#v", verified)
	}
	if err := service.Revoke(context.Background(), "owner_1", org.ID, key.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, ok, err := service.Verify(context.Background(), secret); err != nil || ok {
		t.Fatalf("Verify() after revoke ok=%v err=%v", ok, err)
	}
}

func TestAPIKeyServiceRequiresAdminAndPepper(t *testing.T) {
	tenancy := NewTenancyService()
	tenancy.now = fixedAPIKeyTime
	tenancy.newToken = func() (string, error) { return "invite-token-value", nil }
	org, _, err := tenancy.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	invitation, token, err := tenancy.InviteMember(context.Background(), "owner_1", org.ID, "viewer@example.com", "viewer")
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if _, err := tenancy.AcceptInvitation(context.Background(), invitation.ID, token, "viewer_1"); err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	service := NewAPIKeyService("test-pepper", tenancy)
	if _, _, err := service.Create(context.Background(), "viewer_1", org.ID, "Viewer", []string{"widgets:read"}, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer create error = %v, want %v", err, ErrForbidden)
	}
	withoutPepper := NewAPIKeyService("", tenancy)
	if _, _, err := withoutPepper.Create(context.Background(), "owner_1", org.ID, "Missing pepper", []string{"widgets:read"}, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing pepper error = %v, want %v", err, ErrValidation)
	}
}

func TestAPIKeyServiceWithStorePersistsHashAndTouchesUsage(t *testing.T) {
	tenancy := NewTenancyService()
	tenancy.now = fixedAPIKeyTime
	org, _, err := tenancy.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	store := newRecordingAPIKeyStore()
	service := NewAPIKeyServiceWithStore("test-pepper", tenancy, store)
	service.now = fixedAPIKeyTime
	service.newID = func() (string, error) { return "key_store_1", nil }
	service.newSecret = func() (string, error) { return "atk_store-secret-value", nil }

	key, secret, err := service.Create(context.Background(), "owner_1", org.ID, "Stored", []string{"widgets:read"}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if key.ID != "key_store_1" {
		t.Fatalf("key ID = %q", key.ID)
	}
	if store.createdHash == "" || strings.Contains(store.createdHash, secret) {
		t.Fatalf("stored hash leaked secret: %q", store.createdHash)
	}
	verified, ok, err := service.Verify(context.Background(), secret)
	if err != nil || !ok {
		t.Fatalf("Verify() key=%#v ok=%v err=%v", verified, ok, err)
	}
	if store.touchedKeyID != key.ID || store.touchedAt.IsZero() || verified.LastUsedAt == nil {
		t.Fatalf("touch tracking key=%q at=%v verified=%#v", store.touchedKeyID, store.touchedAt, verified)
	}
	listed, err := service.List(context.Background(), "owner_1", org.ID)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != key.ID {
		t.Fatalf("List() = %#v", listed)
	}
	if err := service.Revoke(context.Background(), "owner_1", org.ID, key.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if !store.revokedAt.Equal(fixedAPIKeyTime()) {
		t.Fatalf("revokedAt = %v", store.revokedAt)
	}
}

type recordingAPIKeyStore struct {
	keys         map[string]domain.APIKey
	byHash       map[string]string
	createdHash  string
	touchedKeyID string
	touchedAt    time.Time
	revokedAt    time.Time
}

func newRecordingAPIKeyStore() *recordingAPIKeyStore {
	return &recordingAPIKeyStore{
		keys:   map[string]domain.APIKey{},
		byHash: map[string]string{},
	}
}

func (s *recordingAPIKeyStore) CreateAPIKey(_ context.Context, key domain.APIKey, hash string) error {
	s.createdHash = hash
	s.keys[key.ID] = key
	s.byHash[hash] = key.ID
	return nil
}

func (s *recordingAPIKeyStore) ListAPIKeys(_ context.Context, organizationID string) ([]domain.APIKey, error) {
	var out []domain.APIKey
	for _, key := range s.keys {
		if key.OrganizationID == organizationID {
			out = append(out, key)
		}
	}
	return out, nil
}

func (s *recordingAPIKeyStore) GetAPIKeyByHash(_ context.Context, hash string) (domain.APIKey, bool, error) {
	keyID, ok := s.byHash[hash]
	if !ok {
		return domain.APIKey{}, false, nil
	}
	key, ok := s.keys[keyID]
	return key, ok, nil
}

func (s *recordingAPIKeyStore) RevokeAPIKey(_ context.Context, organizationID, keyID string, revokedAt time.Time) (bool, error) {
	key, ok := s.keys[keyID]
	if !ok || key.OrganizationID != organizationID {
		return false, nil
	}
	key.RevokedAt = &revokedAt
	s.keys[keyID] = key
	s.revokedAt = revokedAt
	return true, nil
}

func (s *recordingAPIKeyStore) TouchAPIKey(_ context.Context, keyID string, lastUsedAt time.Time) error {
	key := s.keys[keyID]
	key.LastUsedAt = &lastUsedAt
	s.keys[keyID] = key
	s.touchedKeyID = keyID
	s.touchedAt = lastUsedAt
	return nil
}

func fixedAPIKeyTime() time.Time {
	return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
}
`

const fullAppAsyncTemplate = `package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/async"
	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/operations"
)

const WidgetImportJobKind = "widgets.import"

type WidgetImportItem struct {
	Name string ` + "`json:\"name\"`" + `
}

type WidgetImportResult struct {
	Created   int      ` + "`json:\"created\"`" + `
	WidgetIDs []string ` + "`json:\"widget_ids\"`" + `
}

type AsyncService struct {
	mu               sync.Mutex
	nextOperation    int
	nextEvent        int
	widgets          *WidgetService
	operationStore   WidgetImportOperationStore
	outbox           WidgetImportOutbox
	operations       map[string]operations.Operation[WidgetImportResult]
	operationTenants map[string]string
	replays          map[string]string
	events           map[string]outboxEvent
	queue            []string
}

type WidgetImportOperationStore interface {
	CreateWidgetImportOperation(ctx context.Context, tenantID string, operation operations.Operation[WidgetImportResult]) error
	GetWidgetImportOperation(ctx context.Context, tenantID, id string) (operations.Operation[WidgetImportResult], bool, error)
	UpdateWidgetImportOperation(ctx context.Context, tenantID string, operation operations.Operation[WidgetImportResult]) error
}

type WidgetImportOutbox interface {
	EnqueueWidgetImport(ctx context.Context, event WidgetImportEvent) error
}

type WidgetImportEvent struct {
	ID          string
	TenantID    string
	Kind        string
	OperationID string
	Payload     []byte
}

type outboxEvent struct {
	ID          string
	OperationID string
	TenantID    string
	Kind        string
	Payload     []byte
	State       string
	Attempts    int
}

type widgetImportPayload struct {
	OperationID string             ` + "`json:\"operation_id\"`" + `
	Items       []WidgetImportItem ` + "`json:\"items\"`" + `
}

func NewAsyncService(widgets *WidgetService) *AsyncService {
	if widgets == nil {
		widgets = NewWidgetService()
	}
	return &AsyncService{
		widgets:          widgets,
		operations:       map[string]operations.Operation[WidgetImportResult]{},
		operationTenants: map[string]string{},
		replays:          map[string]string{},
		events:           map[string]outboxEvent{},
	}
}

func NewAsyncServiceWithStores(widgets *WidgetService, operationStore WidgetImportOperationStore, outbox WidgetImportOutbox) *AsyncService {
	service := NewAsyncService(widgets)
	service.operationStore = operationStore
	service.outbox = outbox
	return service
}

func (s *AsyncService) StartWidgetImport(ctx context.Context, tenantID, idempotencyKey string, items []WidgetImportItem) (operations.Operation[WidgetImportResult], bool, error) {
	if err := ctx.Err(); err != nil {
		return operations.Operation[WidgetImportResult]{}, false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	cleaned, err := cleanWidgetImportItems(items)
	if err != nil {
		return operations.Operation[WidgetImportResult]{}, false, err
	}
	if tenantID == "" || idempotencyKey == "" {
		return operations.Operation[WidgetImportResult]{}, false, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	replayKey := tenantID + "\x00widgets.import\x00" + idempotencyKey
	if operationID, ok := s.replays[replayKey]; ok {
		return s.operations[operationID], true, nil
	}
	s.nextOperation++
	operationID := formatGeneratedID("op", s.nextOperation)
	operation := operations.Operation[WidgetImportResult]{ID: operationID, State: operations.StatePending}
	payload, err := json.Marshal(widgetImportPayload{OperationID: operationID, Items: cleaned})
	if err != nil {
		return operations.Operation[WidgetImportResult]{}, false, err
	}
	s.nextEvent++
	eventID := formatGeneratedID("out", s.nextEvent)
	s.operations[operationID] = operation
	s.operationTenants[operationID] = tenantID
	s.replays[replayKey] = operationID
	if s.operationStore != nil {
		if err := s.operationStore.CreateWidgetImportOperation(ctx, tenantID, operation); err != nil {
			return operations.Operation[WidgetImportResult]{}, false, err
		}
	}
	if s.outbox != nil {
		if err := s.outbox.EnqueueWidgetImport(ctx, WidgetImportEvent{ID: eventID, TenantID: tenantID, Kind: WidgetImportJobKind, OperationID: operationID, Payload: payload}); err != nil {
			return operations.Operation[WidgetImportResult]{}, false, err
		}
		return operation, false, nil
	}
	s.events[eventID] = outboxEvent{
		ID:          eventID,
		OperationID: operationID,
		TenantID:    tenantID,
		Kind:        WidgetImportJobKind,
		Payload:     payload,
		State:       "pending",
	}
	s.queue = append(s.queue, eventID)
	return operation, false, nil
}

func (s *AsyncService) GetOperation(ctx context.Context, tenantID, id string) (operations.Operation[WidgetImportResult], bool, error) {
	if err := ctx.Err(); err != nil {
		return operations.Operation[WidgetImportResult]{}, false, err
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	if tenantID == "" || id == "" {
		return operations.Operation[WidgetImportResult]{}, false, ErrValidation
	}
	if s.operationStore != nil {
		return s.operationStore.GetWidgetImportOperation(ctx, tenantID, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.operationTenants[id] != tenantID {
		return operations.Operation[WidgetImportResult]{}, false, nil
	}
	operation, ok := s.operations[id]
	return operation, ok, nil
}

func (s *AsyncService) Lease(ctx context.Context, limit int) ([]async.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]async.Job, 0, limit)
	for _, eventID := range s.queue {
		if len(jobs) >= limit {
			break
		}
		event := s.events[eventID]
		if event.State != "pending" {
			continue
		}
		event.State = "running"
		event.Attempts++
		s.events[eventID] = event
		if operation, ok := s.operations[event.OperationID]; ok && operation.State == operations.StatePending {
			running, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateRunning})
			if err != nil {
				return nil, err
			}
			s.operations[event.OperationID] = running
		}
		jobs = append(jobs, async.Job{
			ID:       event.ID,
			Kind:     event.Kind,
			TenantID: event.TenantID,
			Payload:  append([]byte(nil), event.Payload...),
			Attempts: event.Attempts,
		})
	}
	return jobs, nil
}

func (s *AsyncService) Complete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[id]
	if !ok {
		return ErrNotFound
	}
	event.State = "succeeded"
	s.events[id] = event
	return nil
}

func (s *AsyncService) Fail(ctx context.Context, id string, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[id]
	if !ok {
		return ErrNotFound
	}
	event.State = "failed"
	s.events[id] = event
	if operation, ok := s.operations[event.OperationID]; ok && !operations.IsTerminal(operation.State) {
		failed, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{
			To:      operations.StateFailed,
			Problem: &httpx.Problem{Title: "Async work failed", Detail: "worker failed"},
		})
		if err != nil {
			return err
		}
		s.operations[event.OperationID] = failed
	}
	_ = message
	return nil
}

func (s *AsyncService) Handle(ctx context.Context, job async.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if async.SafeLabel(job.Kind) != WidgetImportJobKind {
		return ErrValidation
	}
	var payload widgetImportPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return ErrValidation
	}
	tenantID := strings.TrimSpace(job.TenantID)
	operationID := strings.TrimSpace(payload.OperationID)
	if tenantID == "" || operationID == "" {
		return ErrValidation
	}
	createdIDs := make([]string, 0, len(payload.Items))
	for i, item := range payload.Items {
		widget, _, err := s.widgets.Create(ctx, tenantID, item.Name, operationID+"-"+formatGeneratedID("item", i+1))
		if err != nil {
			return err
		}
		createdIDs = append(createdIDs, widget.ID)
	}
	result := WidgetImportResult{Created: len(createdIDs), WidgetIDs: createdIDs}
	if s.operationStore != nil {
		return s.completeStoredOperation(ctx, tenantID, operationID, result)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.operations[operationID]
	if !ok {
		return ErrNotFound
	}
	if operations.IsTerminal(operation.State) {
		return nil
	}
	if operation.State == operations.StatePending {
		running, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateRunning})
		if err != nil {
			return err
		}
		operation = running
	}
	succeeded, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateSucceeded, Result: &result})
	if err != nil {
		return err
	}
	s.operations[operationID] = succeeded
	return nil
}

func (s *AsyncService) completeStoredOperation(ctx context.Context, tenantID, operationID string, result WidgetImportResult) error {
	operation, ok, err := s.operationStore.GetWidgetImportOperation(ctx, tenantID, operationID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if operations.IsTerminal(operation.State) {
		return nil
	}
	if operation.State == operations.StatePending {
		running, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateRunning})
		if err != nil {
			return err
		}
		operation = running
	}
	succeeded, err := operations.TransitionOperation(operation, operations.TransitionConfig[WidgetImportResult]{To: operations.StateSucceeded, Result: &result})
	if err != nil {
		return err
	}
	return s.operationStore.UpdateWidgetImportOperation(ctx, tenantID, succeeded)
}

func (s *AsyncService) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.runOnce(ctx); err != nil && ctx.Err() == nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *AsyncService) runOnce(ctx context.Context) error {
	jobs, err := s.Lease(ctx, 1)
	if err != nil || len(jobs) == 0 {
		return err
	}
	job := jobs[0]
	if err := s.Handle(ctx, job); err != nil {
		return s.Fail(ctx, job.ID, async.SafeFailureMessage(err))
	}
	return s.Complete(ctx, job.ID)
}

func cleanWidgetImportItems(items []WidgetImportItem) ([]WidgetImportItem, error) {
	if len(items) == 0 || len(items) > 100 {
		return nil, ErrValidation
	}
	out := make([]WidgetImportItem, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" || len(name) > 120 {
			return nil, ErrValidation
		}
		out = append(out, WidgetImportItem{Name: name})
	}
	return out, nil
}

func formatGeneratedID(prefix string, n int) string {
	return fmt.Sprintf("%s_%06d", prefix, n)
}

`

const fullAppAsyncTestTemplate = `package app

import (
	"context"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v2/async"
	"github.com/aatuh/api-toolkit/v2/operations"
)

func TestAsyncServiceCompletesWidgetImport(t *testing.T) {
	ctx := context.Background()
	widgets := NewWidgetService()
	service := NewAsyncService(widgets)
	operation, replayed, err := service.StartWidgetImport(ctx, "org_1", "idem_1", []WidgetImportItem{ {Name: " alpha "}, {Name: "beta"} })
	if err != nil {
		t.Fatalf("StartWidgetImport() error = %v", err)
	}
	if replayed || operation.State != operations.StatePending {
		t.Fatalf("operation = %#v replayed=%v", operation, replayed)
	}
	replay, replayed, err := service.StartWidgetImport(ctx, "org_1", "idem_1", []WidgetImportItem{ {Name: "alpha"} })
	if err != nil {
		t.Fatalf("StartWidgetImport() replay error = %v", err)
	}
	if !replayed || replay.ID != operation.ID {
		t.Fatalf("replay = %#v replayed=%v, want same operation", replay, replayed)
	}

	jobs, err := service.Lease(ctx, 1)
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].Kind != WidgetImportJobKind || jobs[0].TenantID != "org_1" {
		t.Fatalf("leased jobs = %#v", jobs)
	}
	if err := service.Handle(ctx, jobs[0]); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if err := service.Complete(ctx, jobs[0].ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	got, ok, err := service.GetOperation(ctx, "org_1", operation.ID)
	if err != nil {
		t.Fatalf("GetOperation() error = %v", err)
	}
	if !ok || got.State != operations.StateSucceeded || got.Result == nil || got.Result.Created != 2 {
		t.Fatalf("operation after completion = %#v ok=%v", got, ok)
	}
	if _, ok, err := service.GetOperation(ctx, "org_2", operation.ID); err != nil || ok {
		t.Fatalf("cross-tenant GetOperation() ok=%v err=%v", ok, err)
	}
	list, err := widgets.List(ctx, "org_1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 || list[0].Name != "alpha" || list[1].Name != "beta" {
		t.Fatalf("widgets = %#v", list)
	}
}

func TestAsyncServiceFailureDoesNotExposePayloadOrRawError(t *testing.T) {
	ctx := context.Background()
	service := NewAsyncService(NewWidgetService())
	operation, _, err := service.StartWidgetImport(ctx, "org_1", "idem_1", []WidgetImportItem{ {Name: "secret-widget-name"} })
	if err != nil {
		t.Fatalf("StartWidgetImport() error = %v", err)
	}
	jobs, err := service.Lease(ctx, 1)
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("leased jobs = %#v", jobs)
	}
	if err := service.Fail(ctx, jobs[0].ID, "provider failed with secret-widget-name"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	got, ok, err := service.GetOperation(ctx, "org_1", operation.ID)
	if err != nil {
		t.Fatalf("GetOperation() error = %v", err)
	}
	if !ok || got.State != operations.StateFailed || got.Problem == nil {
		t.Fatalf("failed operation = %#v ok=%v", got, ok)
	}
	if strings.Contains(got.Problem.Detail, "secret-widget-name") {
		t.Fatalf("operation problem leaked payload: %#v", got.Problem)
	}
}

func TestAsyncServiceWithStoresPersistsOperationAndOutbox(t *testing.T) {
	ctx := context.Background()
	operationStore := newRecordingWidgetImportOperationStore()
	outbox := &recordingWidgetImportOutbox{}
	service := NewAsyncServiceWithStores(NewWidgetService(), operationStore, outbox)

	operation, replayed, err := service.StartWidgetImport(ctx, "org_1", "idem_1", []WidgetImportItem{WidgetImportItem{Name: "alpha"}})
	if err != nil {
		t.Fatalf("StartWidgetImport() error = %v", err)
	}
	if replayed || operationStore.createdTenant != "org_1" || operationStore.operations[operation.ID].State != operations.StatePending {
		t.Fatalf("operation=%#v replayed=%v store=%#v", operation, replayed, operationStore)
	}
	if outbox.event.ID == "" || outbox.event.OperationID != operation.ID || outbox.event.TenantID != "org_1" || string(outbox.event.Payload) == "" {
		t.Fatalf("outbox event = %#v", outbox.event)
	}
	got, ok, err := service.GetOperation(ctx, "org_1", operation.ID)
	if err != nil || !ok || got.ID != operation.ID {
		t.Fatalf("GetOperation() operation=%#v ok=%v err=%v", got, ok, err)
	}
	if err := service.Handle(ctx, async.Job{ID: outbox.event.ID, Kind: WidgetImportJobKind, TenantID: "org_1", Payload: outbox.event.Payload}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	updated := operationStore.operations[operation.ID]
	if updated.State != operations.StateSucceeded || updated.Result == nil || updated.Result.Created != 1 {
		t.Fatalf("updated operation = %#v", updated)
	}
}

type recordingWidgetImportOperationStore struct {
	createdTenant string
	updatedTenant string
	operations    map[string]operations.Operation[WidgetImportResult]
}

func newRecordingWidgetImportOperationStore() *recordingWidgetImportOperationStore {
	return &recordingWidgetImportOperationStore{operations: map[string]operations.Operation[WidgetImportResult]{}}
}

func (s *recordingWidgetImportOperationStore) CreateWidgetImportOperation(_ context.Context, tenantID string, operation operations.Operation[WidgetImportResult]) error {
	s.createdTenant = tenantID
	s.operations[operation.ID] = operation
	return nil
}

func (s *recordingWidgetImportOperationStore) GetWidgetImportOperation(_ context.Context, tenantID, id string) (operations.Operation[WidgetImportResult], bool, error) {
	operation, ok := s.operations[id]
	if !ok || tenantID != s.createdTenant {
		return operations.Operation[WidgetImportResult]{}, false, nil
	}
	return operation, true, nil
}

func (s *recordingWidgetImportOperationStore) UpdateWidgetImportOperation(_ context.Context, tenantID string, operation operations.Operation[WidgetImportResult]) error {
	s.updatedTenant = tenantID
	s.operations[operation.ID] = operation
	return nil
}

type recordingWidgetImportOutbox struct {
	event WidgetImportEvent
}

func (s *recordingWidgetImportOutbox) EnqueueWidgetImport(_ context.Context, event WidgetImportEvent) error {
	s.event = event
	return nil
}

`

// #nosec G101 -- generated source uses webhook signing secret variable names, not hardcoded credentials.
const fullAppWebhooksTemplate = `package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/webhookdelivery"

	"{{ .Module }}/internal/domain"
)

var webhookEventTypes = []string{
	"widget.created",
	"widget.updated",
	"widget.deleted",
	"widget.import.completed",
	// api-toolkit:webhook-event-types
}

type WebhookService struct {
	mu           sync.Mutex
	nextEndpoint int
	nextEvent    int
	tenancy      *TenancyService
	now          func() time.Time
	newEndpointID func() (string, error)
	newEventID    func() (string, error)
	newSecret    func() (string, error)
	catalog        webhookdelivery.Catalog
	endpointPolicy webhookdelivery.EndpointPolicy
	endpoints      map[string]endpointRecord
	deliveries     map[string]webhookdelivery.Delivery
	jobs           map[string]webhookdelivery.JobPayload
	store          WebhookStore
}

type endpointRecord struct {
	endpoint   webhookdelivery.Endpoint
	secretHash string
}

type WebhookEndpointCreated struct {
	Endpoint webhookdelivery.Endpoint
	Secret   string
}

type WebhookStore interface {
	CreateEndpoint(ctx context.Context, endpoint webhookdelivery.Endpoint) error
	ListEndpointsForActor(ctx context.Context, tenantID string) ([]webhookdelivery.Endpoint, error)
	ListEndpoints(ctx context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error)
	GetEndpoint(ctx context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error)
	EnqueueDelivery(ctx context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error
	RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error
	ListDeliveries(ctx context.Context, tenantID string) ([]webhookdelivery.Delivery, error)
	GetDelivery(ctx context.Context, tenantID, deliveryID string) (webhookdelivery.Delivery, bool, error)
	ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error
}

func NewWebhookService(tenancy *TenancyService) *WebhookService {
	return NewWebhookServiceWithEndpointPolicy(tenancy, webhookdelivery.EndpointPolicy{})
}

func NewWebhookServiceWithEndpointPolicy(tenancy *TenancyService, endpointPolicy webhookdelivery.EndpointPolicy) *WebhookService {
	catalog, _ := webhookdelivery.NewCatalog(webhookEventTypes...)
	return &WebhookService{
		tenancy:        tenancy,
		now:            time.Now,
		newEndpointID:  func() (string, error) { return randomPrefixedID("whend") },
		newEventID:     func() (string, error) { return randomPrefixedID("evt") },
		newSecret:      randomToken,
		catalog:        catalog,
		endpointPolicy: endpointPolicy,
		endpoints:      map[string]endpointRecord{},
		deliveries:     map[string]webhookdelivery.Delivery{},
		jobs:           map[string]webhookdelivery.JobPayload{},
	}
}

func NewWebhookServiceWithStore(tenancy *TenancyService, store WebhookStore) *WebhookService {
	return NewWebhookServiceWithStoreAndEndpointPolicy(tenancy, store, webhookdelivery.EndpointPolicy{})
}

func NewWebhookServiceWithStoreAndEndpointPolicy(tenancy *TenancyService, store WebhookStore, endpointPolicy webhookdelivery.EndpointPolicy) *WebhookService {
	service := NewWebhookServiceWithEndpointPolicy(tenancy, endpointPolicy)
	service.store = store
	return service
}

func (s *WebhookService) EventTypes() []string {
	out := append([]string(nil), webhookEventTypes...)
	sort.Strings(out)
	return out
}

func (s *WebhookService) CreateEndpoint(ctx context.Context, actorID, tenantID, targetURL string, events []string) (WebhookEndpointCreated, error) {
	if err := ctx.Err(); err != nil {
		return WebhookEndpointCreated{}, err
	}
	if s == nil || s.tenancy == nil {
		return WebhookEndpointCreated{}, ErrValidation
	}
	actorID = strings.TrimSpace(actorID)
	tenantID = strings.TrimSpace(tenantID)
	targetURL = strings.TrimSpace(targetURL)
	if actorID == "" || tenantID == "" || targetURL == "" {
		return WebhookEndpointCreated{}, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleAdmin)
	if err != nil {
		return WebhookEndpointCreated{}, err
	}
	if !ok {
		return WebhookEndpointCreated{}, ErrForbidden
	}
	cleanEvents, err := s.cleanEndpointEvents(events)
	if err != nil {
		return WebhookEndpointCreated{}, err
	}
	secret, err := s.newSecret()
	if err != nil {
		return WebhookEndpointCreated{}, err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return WebhookEndpointCreated{}, ErrValidation
	}
	endpointID, err := s.newEndpointID()
	if err != nil {
		return WebhookEndpointCreated{}, err
	}
	endpointID = strings.TrimSpace(endpointID)
	if endpointID == "" {
		return WebhookEndpointCreated{}, ErrValidation
	}

	now := s.now().UTC()
	endpoint := webhookdelivery.Endpoint{
		ID:            endpointID,
		TenantID:      tenantID,
		URL:           targetURL,
		SigningSecret: []byte(secret),
		Events:        cleanEvents,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := webhookdelivery.ValidateEndpoint(endpoint, s.endpointPolicy); err != nil {
		return WebhookEndpointCreated{}, ErrValidation
	}
	if s.store != nil {
		if err := s.store.CreateEndpoint(ctx, endpoint); err != nil {
			return WebhookEndpointCreated{}, err
		}
		return WebhookEndpointCreated{Endpoint: publicWebhookEndpoint(endpoint), Secret: secret}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints[endpoint.ID] = endpointRecord{endpoint: endpoint, secretHash: hashWebhookSecret(secret)}
	return WebhookEndpointCreated{Endpoint: publicWebhookEndpoint(endpoint), Secret: secret}, nil
}

func (s *WebhookService) ListEndpointsForActor(ctx context.Context, actorID, tenantID string) ([]webhookdelivery.Endpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.tenancy == nil {
		return nil, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if s.store != nil {
		endpoints, err := s.store.ListEndpointsForActor(ctx, strings.TrimSpace(tenantID))
		if err != nil {
			return nil, err
		}
		out := make([]webhookdelivery.Endpoint, 0, len(endpoints))
		for _, endpoint := range endpoints {
			out = append(out, publicWebhookEndpoint(endpoint))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]webhookdelivery.Endpoint, 0)
	for _, record := range s.endpoints {
		if strings.TrimSpace(record.endpoint.TenantID) == strings.TrimSpace(tenantID) {
			out = append(out, publicWebhookEndpoint(record.endpoint))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *WebhookService) ListEndpoints(ctx context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	eventType = strings.TrimSpace(eventType)
	if tenantID == "" || !s.catalog.Allows(eventType) {
		return nil, ErrValidation
	}
	if s.store != nil {
		return s.store.ListEndpoints(ctx, tenantID, eventType)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]webhookdelivery.Endpoint, 0)
	for _, record := range s.endpoints {
		endpoint := record.endpoint
		if strings.TrimSpace(endpoint.TenantID) == tenantID && endpoint.SubscribedTo(eventType) {
			out = append(out, cloneWebhookEndpoint(endpoint))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *WebhookService) GetEndpoint(ctx context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error) {
	if err := ctx.Err(); err != nil {
		return webhookdelivery.Endpoint{}, false, err
	}
	if s == nil {
		return webhookdelivery.Endpoint{}, false, ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	endpointID = strings.TrimSpace(endpointID)
	if tenantID == "" || endpointID == "" {
		return webhookdelivery.Endpoint{}, false, ErrValidation
	}
	if s.store != nil {
		return s.store.GetEndpoint(ctx, tenantID, endpointID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.endpoints[endpointID]
	if !ok || strings.TrimSpace(record.endpoint.TenantID) != tenantID {
		return webhookdelivery.Endpoint{}, false, nil
	}
	return cloneWebhookEndpoint(record.endpoint), true, nil
}

func (s *WebhookService) EnqueueDelivery(ctx context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	if err := webhookdelivery.ValidateDelivery(delivery); err != nil {
		return ErrValidation
	}
	if job.Event.TenantID != delivery.TenantID || job.DeliveryID != delivery.ID || job.EndpointID != delivery.EndpointID {
		return ErrValidation
	}
	if s.store != nil {
		return s.store.EnqueueDelivery(ctx, delivery, job)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.deliveries[delivery.ID]; exists {
		return nil
	}
	s.deliveries[delivery.ID] = delivery
	s.jobs[delivery.ID] = job
	return nil
}

func (s *WebhookService) RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	result.DeliveryID = strings.TrimSpace(result.DeliveryID)
	result.TenantID = strings.TrimSpace(result.TenantID)
	result.EndpointID = strings.TrimSpace(result.EndpointID)
	if result.DeliveryID == "" || result.TenantID == "" || result.EndpointID == "" || result.Attempt <= 0 {
		return ErrValidation
	}
	if s.store != nil {
		return s.store.RecordAttempt(ctx, result)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery, ok := s.deliveries[result.DeliveryID]
	if !ok || strings.TrimSpace(delivery.TenantID) != result.TenantID || strings.TrimSpace(delivery.EndpointID) != result.EndpointID {
		return ErrNotFound
	}
	delivery.Attempt = result.Attempt
	delivery.LastStatusCode = result.StatusCode
	delivery.LastError = safeWebhookAttemptError(result.Error)
	delivery.State = webhookdelivery.StateFailed
	if result.Accepted {
		delivery.State = webhookdelivery.StateSucceeded
		delivery.LastError = ""
	} else if !result.Retryable {
		delivery.State = webhookdelivery.StateDeadLetter
	}
	if result.OccurredAt.IsZero() {
		delivery.UpdatedAt = s.now().UTC()
	} else {
		delivery.UpdatedAt = result.OccurredAt.UTC()
	}
	s.deliveries[result.DeliveryID] = delivery
	return nil
}

func (s *WebhookService) DispatchEvent(ctx context.Context, tenantID, eventType string, payload any) ([]webhookdelivery.Delivery, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	eventType = strings.TrimSpace(eventType)
	if tenantID == "" || !s.catalog.Allows(eventType) {
		return nil, ErrValidation
	}
	encoded, err := json.Marshal(payload)
	if err != nil || !json.Valid(encoded) {
		return nil, ErrValidation
	}
	dispatcher, err := webhookdelivery.NewDispatcher(webhookdelivery.DispatcherConfig{
		Catalog:        s.catalog,
		Endpoints:      s,
		Store:          s,
		Clock:          s.now,
		EndpointPolicy: s.endpointPolicy,
	})
	if err != nil {
		return nil, err
	}
	eventID, err := s.nextWebhookEventID()
	if err != nil {
		return nil, err
	}
	return dispatcher.Dispatch(ctx, webhookdelivery.Event{
		ID:       eventID,
		TenantID: tenantID,
		Type:     eventType,
		Payload:  encoded,
	})
}

func (s *WebhookService) ListDeliveriesForActor(ctx context.Context, actorID, tenantID string) ([]webhookdelivery.Delivery, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.tenancy == nil {
		return nil, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if s.store != nil {
		return s.store.ListDeliveries(ctx, strings.TrimSpace(tenantID))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]webhookdelivery.Delivery, 0)
	for _, delivery := range s.deliveries {
		if strings.TrimSpace(delivery.TenantID) == strings.TrimSpace(tenantID) {
			out = append(out, delivery)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *WebhookService) ReplayDeliveryForActor(ctx context.Context, actorID, tenantID, deliveryID string) (webhookdelivery.Delivery, error) {
	if err := ctx.Err(); err != nil {
		return webhookdelivery.Delivery{}, err
	}
	if s == nil || s.tenancy == nil {
		return webhookdelivery.Delivery{}, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleAdmin)
	if err != nil {
		return webhookdelivery.Delivery{}, err
	}
	if !ok {
		return webhookdelivery.Delivery{}, ErrForbidden
	}
	if err := s.ReplayDelivery(ctx, tenantID, deliveryID, s.now().UTC()); err != nil {
		return webhookdelivery.Delivery{}, err
	}
	if s.store != nil {
		delivery, ok, err := s.store.GetDelivery(ctx, tenantID, deliveryID)
		if err != nil {
			return webhookdelivery.Delivery{}, err
		}
		if !ok {
			return webhookdelivery.Delivery{}, ErrNotFound
		}
		return delivery, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deliveries[strings.TrimSpace(deliveryID)], nil
}

func (s *WebhookService) ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	deliveryID = strings.TrimSpace(deliveryID)
	if tenantID == "" || deliveryID == "" {
		return ErrValidation
	}
	if s.store != nil {
		if err := s.store.ReplayDelivery(ctx, tenantID, deliveryID, nextAt); err != nil {
			if errors.Is(err, webhookdelivery.ErrDeliveryNotFound) {
				return ErrNotFound
			}
			return err
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery, ok := s.deliveries[deliveryID]
	if !ok || strings.TrimSpace(delivery.TenantID) != tenantID {
		return ErrNotFound
	}
	if nextAt.IsZero() {
		nextAt = s.now().UTC()
	}
	delivery.State = webhookdelivery.StatePending
	delivery.NextAt = nextAt.UTC()
	delivery.UpdatedAt = s.now().UTC()
	s.deliveries[deliveryID] = delivery
	return nil
}

func (s *WebhookService) cleanEndpointEvents(events []string) ([]string, error) {
	if len(events) == 0 {
		return nil, ErrValidation
	}
	cleaned := make([]string, 0, len(events))
	seen := map[string]struct{}{}
	for _, eventType := range events {
		eventType = strings.TrimSpace(eventType)
		if eventType == webhookdelivery.AnyEventType {
			if _, ok := seen[eventType]; !ok {
				cleaned = append(cleaned, eventType)
				seen[eventType] = struct{}{}
			}
			continue
		}
		if !s.catalog.Allows(eventType) {
			return nil, ErrValidation
		}
		if _, ok := seen[eventType]; ok {
			continue
		}
		cleaned = append(cleaned, eventType)
		seen[eventType] = struct{}{}
	}
	if len(cleaned) == 0 {
		return nil, ErrValidation
	}
	sort.Strings(cleaned)
	return cleaned, nil
}

func (s *WebhookService) nextWebhookEventID() (string, error) {
	if s != nil && s.newEventID != nil {
		id, err := s.newEventID()
		if err != nil {
			return "", err
		}
		id = strings.TrimSpace(id)
		if id == "" {
			return "", ErrValidation
		}
		return id, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextEvent++
	return fmt.Sprintf("evt_%06d", s.nextEvent), nil
}

func publicWebhookEndpoint(endpoint webhookdelivery.Endpoint) webhookdelivery.Endpoint {
	endpoint = cloneWebhookEndpoint(endpoint)
	endpoint.SigningSecret = nil
	return endpoint
}

func cloneWebhookEndpoint(endpoint webhookdelivery.Endpoint) webhookdelivery.Endpoint {
	endpoint.SigningSecret = append([]byte(nil), endpoint.SigningSecret...)
	endpoint.Events = append([]string(nil), endpoint.Events...)
	if endpoint.Headers != nil {
		endpoint.Headers = endpoint.Headers.Clone()
	}
	return endpoint
}

func hashWebhookSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func safeWebhookAttemptError(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") {
		return "delivery failed"
	}
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

var _ webhookdelivery.EndpointRegistry = (*WebhookService)(nil)
var _ webhookdelivery.EndpointGetter = (*WebhookService)(nil)
var _ webhookdelivery.DeliveryEnqueuer = (*WebhookService)(nil)
var _ webhookdelivery.AttemptRecorder = (*WebhookService)(nil)
var _ webhookdelivery.Replayer = (*WebhookService)(nil)

`

// #nosec G101 -- generated tests use fixed webhook signing secret fixtures.
const fullAppWebhooksTestTemplate = `package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/webhookdelivery"
)

func TestWebhookServiceCreatesEndpointAndDispatchesTenantDelivery(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewWebhookService(tenancy)
	service.newSecret = func() (string, error) { return "webhook-secret-value", nil }

	created, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks/widgets", []string{"widget.created"})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if created.Secret != "webhook-secret-value" || created.Endpoint.ID == "" || len(created.Endpoint.SigningSecret) != 0 {
		t.Fatalf("created endpoint leaked or missed secret data: %#v", created)
	}

	listed, err := service.ListEndpointsForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListEndpointsForActor() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.Endpoint.ID || len(listed[0].SigningSecret) != 0 {
		t.Fatalf("listed endpoints = %#v", listed)
	}

	deliveries, err := service.DispatchEvent(ctx, org.ID, "widget.created", map[string]any{"id": "wgt_1"})
	if err != nil {
		t.Fatalf("DispatchEvent() error = %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].EndpointID != created.Endpoint.ID || deliveries[0].State != webhookdelivery.StatePending {
		t.Fatalf("deliveries = %#v", deliveries)
	}
	deliveryList, err := service.ListDeliveriesForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListDeliveriesForActor() error = %v", err)
	}
	encoded, err := json.Marshal(deliveryList)
	if err != nil {
		t.Fatalf("marshal deliveries: %v", err)
	}
	if strings.Contains(string(encoded), created.Secret) {
		t.Fatalf("delivery list leaked signing secret: %s", encoded)
	}
	if _, ok, err := service.GetEndpoint(ctx, org.ID, created.Endpoint.ID); err != nil || !ok {
		t.Fatalf("GetEndpoint() ok=%v err=%v", ok, err)
	}
	if _, ok, err := service.GetEndpoint(ctx, "org_other", created.Endpoint.ID); err != nil || ok {
		t.Fatalf("cross-tenant GetEndpoint() ok=%v err=%v", ok, err)
	}
}

func TestWebhookServiceRejectsUnsafeEndpointAndReplaysTenantScopedDelivery(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewWebhookService(tenancy)
	service.newSecret = func() (string, error) { return "webhook-secret-value", nil }

	if _, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "http://example.com/insecure", []string{"widget.created"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("unsafe endpoint error = %v, want %v", err, ErrValidation)
	}
	if _, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks", []string{"unsupported.event"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("unsupported event error = %v, want %v", err, ErrValidation)
	}

	created, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks", []string{"widget.created"})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	deliveries, err := service.DispatchEvent(ctx, org.ID, "widget.created", map[string]any{"id": "wgt_1"})
	if err != nil {
		t.Fatalf("DispatchEvent() error = %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("deliveries = %#v", deliveries)
	}
	if _, err := service.ReplayDeliveryForActor(ctx, "owner_1", "org_other", deliveries[0].ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant replay error = %v, want %v", err, ErrForbidden)
	}
	replayed, err := service.ReplayDeliveryForActor(ctx, "owner_1", org.ID, deliveries[0].ID)
	if err != nil {
		t.Fatalf("ReplayDeliveryForActor() error = %v", err)
	}
	if replayed.ID != deliveries[0].ID || replayed.EndpointID != created.Endpoint.ID || replayed.State != webhookdelivery.StatePending {
		t.Fatalf("replayed delivery = %#v", replayed)
	}
}

func TestWebhookServiceAllowsInsecureEndpointOnlyWhenConfigured(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewWebhookServiceWithEndpointPolicy(tenancy, webhookdelivery.EndpointPolicy{AllowInsecureHTTP: true})
	service.newSecret = func() (string, error) { return "webhook-secret-value", nil }
	if _, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "http://127.0.0.1:18081/webhooks", []string{"widget.created"}); err != nil {
		t.Fatalf("CreateEndpoint() with development HTTP policy error = %v", err)
	}
}

func TestWebhookServiceRecordsAttemptsWithoutSecretLeak(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewWebhookService(tenancy)
	service.newSecret = func() (string, error) { return "webhook-secret-value", nil }
	created, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks", []string{"widget.created"})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	deliveries, err := service.DispatchEvent(ctx, org.ID, "widget.created", map[string]any{"id": "wgt_1"})
	if err != nil {
		t.Fatalf("DispatchEvent() error = %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("deliveries = %#v", deliveries)
	}
	result := webhookdelivery.AttemptResult{
		DeliveryID: deliveries[0].ID,
		TenantID:   org.ID,
		EndpointID: created.Endpoint.ID,
		EventID:    deliveries[0].EventID,
		EventType:  "widget.created",
		Attempt:    1,
		Retryable:  true,
		Error:      "dial webhook-secret-value failed",
		OccurredAt: time.Unix(1_700_000_100, 0).UTC(),
	}
	if err := service.RecordAttempt(ctx, result); err != nil {
		t.Fatalf("RecordAttempt() error = %v", err)
	}
	listed, err := service.ListDeliveriesForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListDeliveriesForActor() error = %v", err)
	}
	if len(listed) != 1 || listed[0].State != webhookdelivery.StateFailed || listed[0].LastError != "delivery failed" {
		t.Fatalf("listed delivery after attempt = %#v", listed)
	}
	encoded, err := json.Marshal(listed)
	if err != nil {
		t.Fatalf("marshal deliveries: %v", err)
	}
	if strings.Contains(string(encoded), "webhook-secret-value") {
		t.Fatalf("attempt result leaked signing secret: %s", encoded)
	}
}

func TestWebhookServiceWithStorePersistsEndpointsAndDeliveries(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	store := newRecordingWebhookStore()
	service := NewWebhookServiceWithStore(tenancy, store)
	service.newEndpointID = func() (string, error) { return "whend_store", nil }
	service.newEventID = func() (string, error) { return "evt_store", nil }
	service.newSecret = func() (string, error) { return "stored-webhook-secret", nil }

	created, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks", []string{"widget.created"})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if created.Endpoint.ID != "whend_store" || len(created.Endpoint.SigningSecret) != 0 || created.Secret != "stored-webhook-secret" {
		t.Fatalf("created endpoint = %#v", created)
	}
	if string(store.endpoint.SigningSecret) != "stored-webhook-secret" {
		t.Fatalf("store did not receive signing secret")
	}

	listed, err := service.ListEndpointsForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListEndpointsForActor() error = %v", err)
	}
	if len(listed) != 1 || len(listed[0].SigningSecret) != 0 {
		t.Fatalf("listed endpoints leaked secret: %#v", listed)
	}

	deliveries, err := service.DispatchEvent(ctx, org.ID, "widget.created", map[string]any{"id": "wgt_1"})
	if err != nil {
		t.Fatalf("DispatchEvent() error = %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].ID == "" || store.job.DeliveryID != deliveries[0].ID {
		t.Fatalf("deliveries=%#v job=%#v", deliveries, store.job)
	}
	listedDeliveries, err := service.ListDeliveriesForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListDeliveriesForActor() error = %v", err)
	}
	if len(listedDeliveries) != 1 || listedDeliveries[0].TenantID != org.ID {
		t.Fatalf("listed deliveries = %#v", listedDeliveries)
	}
	replayed, err := service.ReplayDeliveryForActor(ctx, "owner_1", org.ID, deliveries[0].ID)
	if err != nil {
		t.Fatalf("ReplayDeliveryForActor() error = %v", err)
	}
	if replayed.ID != deliveries[0].ID || replayed.State != webhookdelivery.StatePending {
		t.Fatalf("replayed delivery = %#v", replayed)
	}
}

type recordingWebhookStore struct {
	endpoint   webhookdelivery.Endpoint
	delivery   webhookdelivery.Delivery
	job        webhookdelivery.JobPayload
	replayedID string
}

func newRecordingWebhookStore() *recordingWebhookStore {
	return &recordingWebhookStore{}
}

func (s *recordingWebhookStore) CreateEndpoint(_ context.Context, endpoint webhookdelivery.Endpoint) error {
	s.endpoint = cloneWebhookEndpoint(endpoint)
	return nil
}

func (s *recordingWebhookStore) ListEndpointsForActor(_ context.Context, tenantID string) ([]webhookdelivery.Endpoint, error) {
	if strings.TrimSpace(s.endpoint.TenantID) != strings.TrimSpace(tenantID) {
		return nil, nil
	}
	return []webhookdelivery.Endpoint{cloneWebhookEndpoint(s.endpoint)}, nil
}

func (s *recordingWebhookStore) ListEndpoints(_ context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error) {
	if strings.TrimSpace(s.endpoint.TenantID) != strings.TrimSpace(tenantID) || !s.endpoint.SubscribedTo(eventType) {
		return nil, nil
	}
	return []webhookdelivery.Endpoint{cloneWebhookEndpoint(s.endpoint)}, nil
}

func (s *recordingWebhookStore) GetEndpoint(_ context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error) {
	if strings.TrimSpace(s.endpoint.TenantID) != strings.TrimSpace(tenantID) || strings.TrimSpace(s.endpoint.ID) != strings.TrimSpace(endpointID) {
		return webhookdelivery.Endpoint{}, false, nil
	}
	return cloneWebhookEndpoint(s.endpoint), true, nil
}

func (s *recordingWebhookStore) EnqueueDelivery(_ context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error {
	s.delivery = delivery
	s.job = job
	return nil
}

func (s *recordingWebhookStore) RecordAttempt(_ context.Context, result webhookdelivery.AttemptResult) error {
	if strings.TrimSpace(s.delivery.ID) != strings.TrimSpace(result.DeliveryID) {
		return webhookdelivery.ErrDeliveryNotFound
	}
	if result.Accepted {
		s.delivery.State = webhookdelivery.StateSucceeded
	} else if result.Retryable {
		s.delivery.State = webhookdelivery.StateFailed
	} else {
		s.delivery.State = webhookdelivery.StateDeadLetter
	}
	s.delivery.Attempt = result.Attempt
	s.delivery.LastStatusCode = result.StatusCode
	s.delivery.LastError = result.Error
	return nil
}

func (s *recordingWebhookStore) ListDeliveries(_ context.Context, tenantID string) ([]webhookdelivery.Delivery, error) {
	if strings.TrimSpace(s.delivery.TenantID) != strings.TrimSpace(tenantID) {
		return nil, nil
	}
	return []webhookdelivery.Delivery{s.delivery}, nil
}

func (s *recordingWebhookStore) GetDelivery(_ context.Context, tenantID, deliveryID string) (webhookdelivery.Delivery, bool, error) {
	if strings.TrimSpace(s.delivery.TenantID) != strings.TrimSpace(tenantID) || strings.TrimSpace(s.delivery.ID) != strings.TrimSpace(deliveryID) {
		return webhookdelivery.Delivery{}, false, nil
	}
	return s.delivery, true, nil
}

func (s *recordingWebhookStore) ReplayDelivery(_ context.Context, tenantID, deliveryID string, nextAt time.Time) error {
	if strings.TrimSpace(s.delivery.TenantID) != strings.TrimSpace(tenantID) || strings.TrimSpace(s.delivery.ID) != strings.TrimSpace(deliveryID) {
		return webhookdelivery.ErrDeliveryNotFound
	}
	s.replayedID = strings.TrimSpace(deliveryID)
	s.delivery.State = webhookdelivery.StatePending
	s.delivery.NextAt = nextAt.UTC()
	return nil
}

`

const fullAppObjectsTemplate = `package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"{{ .Module }}/internal/domain"
)

const maxObjectBytes = 1024 * 1024

var allowedObjectContentTypes = map[string]struct{}{
	"application/json": {},
	"application/pdf":  {},
	"image/jpeg":       {},
	"image/png":        {},
	"text/plain":       {},
}

type ObjectService struct {
	mu      sync.Mutex
	tenancy *TenancyService
	now     func() time.Time
	objects map[string]Object
	data    map[string][]byte
	store   ObjectMetadataStore
	blobs   ObjectBlobStore
}

type ObjectMetadataStore interface {
	SaveObjectMetadata(ctx context.Context, object Object) error
	GetObjectMetadata(ctx context.Context, tenantID, key string) (Object, bool, error)
	ListObjectMetadata(ctx context.Context, tenantID string) ([]Object, error)
	DeleteObjectMetadata(ctx context.Context, tenantID, key string) (bool, error)
}

type ObjectBlobStore interface {
	PutObject(ctx context.Context, ref ObjectRef, data []byte, contentType string) error
	GetObject(ctx context.Context, ref ObjectRef) (ObjectBlob, bool, error)
	DeleteObject(ctx context.Context, ref ObjectRef) (bool, error)
}

type ObjectRef struct {
	TenantID string
	Key      string
}

type ObjectBlob struct {
	ContentType string
	Size        int64
	Data        []byte
}

type Object struct {
	TenantID    string
	Key         string
	ContentType string
	Size        int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (o Object) Public() map[string]any {
	return map[string]any{
		"tenant_id":    o.TenantID,
		"key":          o.Key,
		"content_type": o.ContentType,
		"size":         o.Size,
		"created_at":   o.CreatedAt,
		"updated_at":   o.UpdatedAt,
	}
}

func NewObjectService(tenancy *TenancyService) *ObjectService {
	return &ObjectService{tenancy: tenancy, now: time.Now, objects: map[string]Object{}, data: map[string][]byte{}}
}

func NewObjectServiceWithBlobStore(tenancy *TenancyService, blobs ObjectBlobStore) *ObjectService {
	return NewObjectServiceWithStores(tenancy, nil, blobs)
}

func NewObjectServiceWithStores(tenancy *TenancyService, store ObjectMetadataStore, blobs ObjectBlobStore) *ObjectService {
	service := NewObjectService(tenancy)
	service.store = store
	service.blobs = blobs
	return service
}

func (s *ObjectService) Put(ctx context.Context, actorID, tenantID, key, contentType string, data []byte) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	if s == nil || s.tenancy == nil {
		return Object{}, ErrValidation
	}
	if err := validateObjectKey(key); err != nil {
		return Object{}, err
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if _, ok := allowedObjectContentTypes[contentType]; !ok {
		return Object{}, ErrValidation
	}
	if len(data) == 0 || len(data) > maxObjectBytes {
		return Object{}, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleMember)
	if err != nil {
		return Object{}, err
	}
	if !ok {
		return Object{}, ErrForbidden
	}
	key = strings.TrimSpace(key)
	tenantID = strings.TrimSpace(tenantID)
	now := s.now().UTC()
	id := objectID(tenantID, key)
	obj := Object{TenantID: tenantID, Key: key, ContentType: contentType, Size: int64(len(data)), CreatedAt: now, UpdatedAt: now}
	if s.store != nil {
		existing, ok, err := s.store.GetObjectMetadata(ctx, tenantID, key)
		if err != nil {
			return Object{}, err
		}
		if ok {
			obj.CreatedAt = existing.CreatedAt
		}
		if s.blobs != nil {
			if err := s.blobs.PutObject(ctx, ObjectRef{TenantID: tenantID, Key: key}, append([]byte(nil), data...), contentType); err != nil {
				return Object{}, err
			}
		}
		if err := s.store.SaveObjectMetadata(ctx, obj); err != nil {
			if s.blobs != nil {
				_, _ = s.blobs.DeleteObject(ctx, ObjectRef{TenantID: tenantID, Key: key})
			}
			return Object{}, err
		}
		if s.blobs == nil {
			s.mu.Lock()
			s.data[id] = append([]byte(nil), data...)
			s.mu.Unlock()
		}
		return obj, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.objects[id]; ok {
		obj.CreatedAt = existing.CreatedAt
	}
	if s.blobs != nil {
		if err := s.blobs.PutObject(ctx, ObjectRef{TenantID: tenantID, Key: key}, append([]byte(nil), data...), contentType); err != nil {
			return Object{}, err
		}
	}
	s.objects[id] = obj
	s.data[id] = append([]byte(nil), data...)
	return obj, nil
}

func (s *ObjectService) Get(ctx context.Context, actorID, tenantID, key string) (Object, []byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, nil, false, err
	}
	if s == nil || s.tenancy == nil {
		return Object{}, nil, false, ErrValidation
	}
	if err := validateObjectKey(key); err != nil {
		return Object{}, nil, false, err
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleViewer)
	if err != nil {
		return Object{}, nil, false, err
	}
	if !ok {
		return Object{}, nil, false, ErrForbidden
	}
	if s.store != nil {
		tenantID = strings.TrimSpace(tenantID)
		key = strings.TrimSpace(key)
		obj, ok, err := s.store.GetObjectMetadata(ctx, tenantID, key)
		if err != nil || !ok {
			return Object{}, nil, ok, err
		}
		if s.blobs != nil {
			blob, ok, err := s.blobs.GetObject(ctx, ObjectRef{TenantID: tenantID, Key: key})
			if err != nil || !ok {
				return Object{}, nil, ok, err
			}
			return obj, append([]byte(nil), blob.Data...), true, nil
		}
		id := objectID(tenantID, key)
		s.mu.Lock()
		defer s.mu.Unlock()
		data, ok := s.data[id]
		if !ok {
			return Object{}, nil, false, nil
		}
		return obj, append([]byte(nil), data...), true, nil
	}
	if s.blobs != nil {
		blob, ok, err := s.blobs.GetObject(ctx, ObjectRef{TenantID: strings.TrimSpace(tenantID), Key: strings.TrimSpace(key)})
		if err != nil || !ok {
			return Object{}, nil, ok, err
		}
		now := s.now().UTC()
		obj := Object{TenantID: strings.TrimSpace(tenantID), Key: strings.TrimSpace(key), ContentType: blob.ContentType, Size: blob.Size, CreatedAt: now, UpdatedAt: now}
		return obj, append([]byte(nil), blob.Data...), true, nil
	}
	id := objectID(tenantID, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, ok := s.objects[id]
	if !ok {
		return Object{}, nil, false, nil
	}
	return obj, append([]byte(nil), s.data[id]...), true, nil
}

func (s *ObjectService) List(ctx context.Context, actorID, tenantID string) ([]Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.tenancy == nil {
		return nil, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	tenantID = strings.TrimSpace(tenantID)
	if s.store != nil {
		return s.store.ListObjectMetadata(ctx, tenantID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Object, 0)
	for _, obj := range s.objects {
		if obj.TenantID == tenantID {
			out = append(out, obj)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *ObjectService) Delete(ctx context.Context, actorID, tenantID, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.tenancy == nil {
		return ErrValidation
	}
	if err := validateObjectKey(key); err != nil {
		return err
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleMember)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if s.store != nil {
		tenantID = strings.TrimSpace(tenantID)
		key = strings.TrimSpace(key)
		if _, ok, err := s.store.GetObjectMetadata(ctx, tenantID, key); err != nil {
			return err
		} else if !ok {
			return ErrNotFound
		}
		if s.blobs != nil {
			if _, err := s.blobs.DeleteObject(ctx, ObjectRef{TenantID: tenantID, Key: key}); err != nil {
				return err
			}
		}
		deleted, err := s.store.DeleteObjectMetadata(ctx, tenantID, key)
		if err != nil {
			return err
		}
		if !deleted {
			return ErrNotFound
		}
		id := objectID(tenantID, key)
		s.mu.Lock()
		delete(s.data, id)
		s.mu.Unlock()
		return nil
	}
	if s.blobs != nil {
		ok, err := s.blobs.DeleteObject(ctx, ObjectRef{TenantID: strings.TrimSpace(tenantID), Key: strings.TrimSpace(key)})
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		id := objectID(tenantID, key)
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.objects, id)
		delete(s.data, id)
		return nil
	}
	id := objectID(tenantID, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[id]; !ok {
		return ErrNotFound
	}
	delete(s.objects, id)
	delete(s.data, id)
	return nil
}

func validateObjectKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 || strings.HasPrefix(key, ".") || strings.Contains(key, "..") || strings.ContainsAny(key, "/\\") {
		return ErrValidation
	}
	for _, r := range key {
		if unicode.IsControl(r) {
			return ErrValidation
		}
	}
	return nil
}

func objectID(tenantID, key string) string {
	return fmt.Sprintf("%s\x00%s", strings.TrimSpace(tenantID), strings.TrimSpace(key))
}

`

const fullAppObjectsTestTemplate = `package app

import (
	"context"
	"errors"
	"sort"
	"testing"
)

func TestObjectServiceEnforcesTenantRolesAndPolicy(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewObjectService(tenancy)
	obj, err := service.Put(ctx, "owner_1", org.ID, "readme.txt", "text/plain", []byte("hello"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if obj.Key != "readme.txt" || obj.Size != 5 {
		t.Fatalf("object = %#v", obj)
	}
	got, data, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt")
	if err != nil || !ok || got.Key != obj.Key || string(data) != "hello" {
		t.Fatalf("Get() object=%#v data=%q ok=%v err=%v", got, data, ok, err)
	}
	data[0] = 'x'
	_, again, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt")
	if err != nil || !ok || string(again) != "hello" {
		t.Fatalf("Get() after data mutation data=%q ok=%v err=%v", again, ok, err)
	}
	if _, err := service.List(ctx, "owner_1", org.ID); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if err := service.Delete(ctx, "owner_1", org.ID, "readme.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt"); err != nil || ok {
		t.Fatalf("Get() after delete ok=%v err=%v", ok, err)
	}
}

func TestObjectServiceRejectsUnsafeInputs(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewObjectService(tenancy)
	for _, key := range []string{"", "../secret", "nested/file", ".hidden"} {
		if _, err := service.Put(ctx, "owner_1", org.ID, key, "text/plain", []byte("hello")); !errors.Is(err, ErrValidation) {
			t.Fatalf("Put(%q) error = %v, want %v", key, err, ErrValidation)
		}
	}
	if _, err := service.Put(ctx, "owner_1", org.ID, "readme.txt", "application/x-secret", []byte("hello")); !errors.Is(err, ErrValidation) {
		t.Fatalf("unsafe content-type error = %v, want %v", err, ErrValidation)
	}
	if _, err := service.Put(ctx, "stranger_1", org.ID, "readme.txt", "text/plain", []byte("hello")); !errors.Is(err, ErrForbidden) {
		t.Fatalf("stranger Put() error = %v, want %v", err, ErrForbidden)
	}
}

func TestObjectServiceWithBlobStoreKeepsTenantPolicy(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	blobs := newRecordingObjectBlobStore()
	service := NewObjectServiceWithBlobStore(tenancy, blobs)
	if _, err := service.Put(ctx, "owner_1", org.ID, "readme.txt", "text/plain", []byte("hello")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if blobs.putRef.TenantID != org.ID || blobs.putRef.Key != "readme.txt" || string(blobs.data) != "hello" {
		t.Fatalf("blob put ref=%#v data=%q", blobs.putRef, blobs.data)
	}
	got, data, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt")
	if err != nil || !ok || got.Size != 5 || string(data) != "hello" {
		t.Fatalf("Get() object=%#v data=%q ok=%v err=%v", got, data, ok, err)
	}
	if _, _, _, err := service.Get(ctx, "other_user", org.ID, "readme.txt"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-actor Get() error = %v, want %v", err, ErrForbidden)
	}
	if err := service.Delete(ctx, "owner_1", org.ID, "readme.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !blobs.deleted {
		t.Fatalf("blob was not deleted")
	}
}

func TestObjectServiceWithMetadataStorePersistsListAndDelete(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	metadata := newRecordingObjectMetadataStore()
	blobs := newRecordingObjectBlobStore()
	service := NewObjectServiceWithStores(tenancy, metadata, blobs)
	obj, err := service.Put(ctx, "owner_1", org.ID, "readme.txt", "text/plain", []byte("hello"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if metadata.saved.Key != "readme.txt" || metadata.saved.Size != 5 {
		t.Fatalf("saved metadata = %#v", metadata.saved)
	}
	listed, err := service.List(ctx, "owner_1", org.ID)
	if err != nil || len(listed) != 1 || listed[0].Key != obj.Key {
		t.Fatalf("List() = %#v err=%v", listed, err)
	}
	got, data, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt")
	if err != nil || !ok || got.CreatedAt != obj.CreatedAt || string(data) != "hello" {
		t.Fatalf("Get() object=%#v data=%q ok=%v err=%v", got, data, ok, err)
	}
	if err := service.Delete(ctx, "owner_1", org.ID, "readme.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !metadata.deleted || !blobs.deleted {
		t.Fatalf("delete did not update metadata/blob: metadata=%v blob=%v", metadata.deleted, blobs.deleted)
	}
}

type recordingObjectMetadataStore struct {
	objects map[string]Object
	saved   Object
	deleted bool
}

func newRecordingObjectMetadataStore() *recordingObjectMetadataStore {
	return &recordingObjectMetadataStore{objects: map[string]Object{}}
}

func (s *recordingObjectMetadataStore) SaveObjectMetadata(_ context.Context, object Object) error {
	s.saved = object
	s.objects[objectID(object.TenantID, object.Key)] = object
	return nil
}

func (s *recordingObjectMetadataStore) GetObjectMetadata(_ context.Context, tenantID, key string) (Object, bool, error) {
	object, ok := s.objects[objectID(tenantID, key)]
	return object, ok, nil
}

func (s *recordingObjectMetadataStore) ListObjectMetadata(_ context.Context, tenantID string) ([]Object, error) {
	out := make([]Object, 0, len(s.objects))
	for _, object := range s.objects {
		if object.TenantID == tenantID {
			out = append(out, object)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *recordingObjectMetadataStore) DeleteObjectMetadata(_ context.Context, tenantID, key string) (bool, error) {
	id := objectID(tenantID, key)
	if _, ok := s.objects[id]; !ok {
		return false, nil
	}
	delete(s.objects, id)
	s.deleted = true
	return true, nil
}

type recordingObjectBlobStore struct {
	putRef      ObjectRef
	contentType string
	data        []byte
	deleted     bool
}

func newRecordingObjectBlobStore() *recordingObjectBlobStore {
	return &recordingObjectBlobStore{}
}

func (s *recordingObjectBlobStore) PutObject(_ context.Context, ref ObjectRef, data []byte, contentType string) error {
	s.putRef = ref
	s.contentType = contentType
	s.data = append([]byte(nil), data...)
	return nil
}

func (s *recordingObjectBlobStore) GetObject(_ context.Context, ref ObjectRef) (ObjectBlob, bool, error) {
	if ref != s.putRef {
		return ObjectBlob{}, false, nil
	}
	return ObjectBlob{ContentType: s.contentType, Size: int64(len(s.data)), Data: append([]byte(nil), s.data...)}, true, nil
}

func (s *recordingObjectBlobStore) DeleteObject(_ context.Context, ref ObjectRef) (bool, error) {
	if ref != s.putRef {
		return false, nil
	}
	s.deleted = true
	return true, nil
}

`

// #nosec G101 -- generated source uses invitation token variables, not hardcoded secrets.
const fullAppTenancyTemplate = `package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"{{ .Module }}/internal/domain"
)

var ErrForbidden = errors.New("forbidden")

type TenancyService struct {
	mu          sync.Mutex
	nextOrg     int
	nextInvite  int
	orgs        map[string]domain.Organization
	memberships map[string]map[string]domain.Membership
	invitations map[string]invitationRecord
	now         func() time.Time
	newToken    func() (string, error)
	newOrgID    func() (string, error)
	newInviteID func() (string, error)
	store       TenancyStore
}

type invitationRecord struct {
	invitation domain.Invitation
	tokenHash  string
}

type TenancyStore interface {
	CreateOrganization(ctx context.Context, org domain.Organization, owner domain.Membership) error
	ListOrganizations(ctx context.Context, actorID string) ([]domain.Organization, error)
	ListMembers(ctx context.Context, organizationID string) ([]domain.Membership, error)
	CreateInvitation(ctx context.Context, invitation domain.Invitation, tokenHash string) error
	GetInvitation(ctx context.Context, invitationID string) (domain.Invitation, string, bool, error)
	AcceptInvitation(ctx context.Context, invitationID, userID string, acceptedAt time.Time) (domain.Membership, bool, error)
	HasRole(ctx context.Context, organizationID, actorID string, required domain.Role) (bool, error)
}

func NewTenancyService() *TenancyService {
	return &TenancyService{
		orgs:        map[string]domain.Organization{},
		memberships: map[string]map[string]domain.Membership{},
		invitations: map[string]invitationRecord{},
		now:         time.Now,
		newToken:    randomToken,
		newOrgID:    func() (string, error) { return randomPrefixedID("org") },
		newInviteID: func() (string, error) { return randomPrefixedID("inv") },
	}
}

func NewTenancyServiceWithStore(store TenancyStore) *TenancyService {
	service := NewTenancyService()
	service.store = store
	return service
}

func (s *TenancyService) CreateOrganization(ctx context.Context, actorID, name string) (domain.Organization, domain.Membership, error) {
	if err := ctx.Err(); err != nil {
		return domain.Organization{}, domain.Membership{}, err
	}
	actorID = strings.TrimSpace(actorID)
	name = strings.TrimSpace(name)
	if actorID == "" || name == "" {
		return domain.Organization{}, domain.Membership{}, ErrValidation
	}
	orgID, err := s.newOrgID()
	if err != nil {
		return domain.Organization{}, domain.Membership{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextOrg++
	now := s.now().UTC()
	org := domain.Organization{
		ID:        orgID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	member := domain.Membership{
		OrganizationID: org.ID,
		UserID:         actorID,
		Role:           domain.RoleOwner,
		CreatedAt:      now,
	}
	if s.store != nil {
		if err := s.store.CreateOrganization(ctx, org, member); err != nil {
			return domain.Organization{}, domain.Membership{}, err
		}
		return org, member, nil
	}
	s.orgs[org.ID] = org
	s.memberships[org.ID] = map[string]domain.Membership{actorID: member}
	return org, member, nil
}

func (s *TenancyService) ListOrganizations(ctx context.Context, actorID string) ([]domain.Organization, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, ErrValidation
	}
	if s.store != nil {
		return s.store.ListOrganizations(ctx, actorID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Organization, 0)
	for orgID, members := range s.memberships {
		if _, ok := members[actorID]; ok {
			if org, exists := s.orgs[orgID]; exists {
				out = append(out, org)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *TenancyService) ListMembers(ctx context.Context, actorID, organizationID string) ([]domain.Membership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	if actorID == "" || organizationID == "" {
		return nil, ErrValidation
	}
	if s.store != nil {
		ok, err := s.store.HasRole(ctx, organizationID, actorID, domain.RoleViewer)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrForbidden
		}
		return s.store.ListMembers(ctx, organizationID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasRoleLocked(organizationID, actorID, domain.RoleViewer) {
		return nil, ErrForbidden
	}
	members := s.memberships[organizationID]
	out := make([]domain.Membership, 0, len(members))
	for _, member := range members {
		out = append(out, member)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UserID < out[j].UserID
	})
	return out, nil
}

func (s *TenancyService) InviteMember(ctx context.Context, actorID, organizationID, email string, role domain.Role) (domain.Invitation, string, error) {
	if err := ctx.Err(); err != nil {
		return domain.Invitation{}, "", err
	}
	actorID = strings.TrimSpace(actorID)
	organizationID = strings.TrimSpace(organizationID)
	email = strings.ToLower(strings.TrimSpace(email))
	if actorID == "" || organizationID == "" || email == "" || !strings.Contains(email, "@") || !role.Valid() {
		return domain.Invitation{}, "", ErrValidation
	}
	if s.store != nil {
		ok, err := s.store.HasRole(ctx, organizationID, actorID, domain.RoleAdmin)
		if err != nil {
			return domain.Invitation{}, "", err
		}
		if !ok {
			return domain.Invitation{}, "", ErrForbidden
		}
		if role == domain.RoleOwner {
			owner, err := s.store.HasRole(ctx, organizationID, actorID, domain.RoleOwner)
			if err != nil {
				return domain.Invitation{}, "", err
			}
			if !owner {
				return domain.Invitation{}, "", ErrForbidden
			}
		}
		token, err := s.newToken()
		if err != nil {
			return domain.Invitation{}, "", err
		}
		invitationID, err := s.newInviteID()
		if err != nil {
			return domain.Invitation{}, "", err
		}
		now := s.now().UTC()
		prefix := token
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		invitation := domain.Invitation{
			ID:             invitationID,
			OrganizationID: organizationID,
			Email:          email,
			Role:           role,
			TokenPrefix:    prefix,
			ExpiresAt:      now.Add(7 * 24 * time.Hour),
			CreatedAt:      now,
		}
		if err := s.store.CreateInvitation(ctx, invitation, hashToken(token)); err != nil {
			return domain.Invitation{}, "", err
		}
		return invitation, token, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasRoleLocked(organizationID, actorID, domain.RoleAdmin) {
		return domain.Invitation{}, "", ErrForbidden
	}
	if role == domain.RoleOwner && !s.hasRoleLocked(organizationID, actorID, domain.RoleOwner) {
		return domain.Invitation{}, "", ErrForbidden
	}
	token, err := s.newToken()
	if err != nil {
		return domain.Invitation{}, "", err
	}
	invitationID, err := s.newInviteID()
	if err != nil {
		return domain.Invitation{}, "", err
	}
	s.nextInvite++
	now := s.now().UTC()
	prefix := token
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	invitation := domain.Invitation{
		ID:             invitationID,
		OrganizationID: organizationID,
		Email:          email,
		Role:           role,
		TokenPrefix:    prefix,
		ExpiresAt:      now.Add(7 * 24 * time.Hour),
		CreatedAt:      now,
	}
	s.invitations[invitation.ID] = invitationRecord{
		invitation: invitation,
		tokenHash:  hashToken(token),
	}
	return invitation, token, nil
}

func (s *TenancyService) AcceptInvitation(ctx context.Context, invitationID, token, userID string) (domain.Membership, error) {
	if err := ctx.Err(); err != nil {
		return domain.Membership{}, err
	}
	invitationID = strings.TrimSpace(invitationID)
	token = strings.TrimSpace(token)
	userID = strings.TrimSpace(userID)
	if invitationID == "" || token == "" || userID == "" {
		return domain.Membership{}, ErrValidation
	}
	if s.store != nil {
		invitation, tokenHash, ok, err := s.store.GetInvitation(ctx, invitationID)
		if err != nil {
			return domain.Membership{}, err
		}
		if !ok {
			return domain.Membership{}, ErrNotFound
		}
		now := s.now().UTC()
		if invitation.AcceptedAt != nil || !now.Before(invitation.ExpiresAt) || hashToken(token) != tokenHash {
			return domain.Membership{}, ErrNotFound
		}
		member, ok, err := s.store.AcceptInvitation(ctx, invitationID, userID, now)
		if err != nil {
			return domain.Membership{}, err
		}
		if !ok {
			return domain.Membership{}, ErrNotFound
		}
		return member, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.invitations[invitationID]
	if !ok {
		return domain.Membership{}, ErrNotFound
	}
	now := s.now().UTC()
	if record.invitation.AcceptedAt != nil || !now.Before(record.invitation.ExpiresAt) {
		return domain.Membership{}, ErrNotFound
	}
	if hashToken(token) != record.tokenHash {
		return domain.Membership{}, ErrNotFound
	}
	member := domain.Membership{
		OrganizationID: record.invitation.OrganizationID,
		UserID:         userID,
		Role:           record.invitation.Role,
		CreatedAt:      now,
	}
	if s.memberships[member.OrganizationID] == nil {
		s.memberships[member.OrganizationID] = map[string]domain.Membership{}
	}
	s.memberships[member.OrganizationID][userID] = member
	record.invitation.AcceptedAt = &now
	s.invitations[invitationID] = record
	return member, nil
}

func (s *TenancyService) HasRole(ctx context.Context, organizationID, actorID string, required domain.Role) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	organizationID = strings.TrimSpace(organizationID)
	actorID = strings.TrimSpace(actorID)
	if organizationID == "" || actorID == "" || !required.Valid() {
		return false, ErrValidation
	}
	if s.store != nil {
		return s.store.HasRole(ctx, organizationID, actorID, required)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasRoleLocked(organizationID, actorID, required), nil
}

func (s *TenancyService) hasRoleLocked(organizationID, actorID string, required domain.Role) bool {
	members := s.memberships[organizationID]
	if members == nil {
		return false
	}
	member, ok := members[actorID]
	return ok && member.Role.Allows(required)
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func randomPrefixedID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return strings.TrimSpace(prefix) + "_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
`

const fullAppTenancyTestTemplate = `package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"{{ .Module }}/internal/domain"
)

func TestTenancyServiceCreatesOrganizationWithOwnerMembership(t *testing.T) {
	service := NewTenancyService()
	service.now = fixedTenancyTime

	org, member, err := service.CreateOrganization(context.Background(), "user_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if org.ID == "" || org.Name != "Acme" {
		t.Fatalf("organization = %#v", org)
	}
	if member.OrganizationID != org.ID || member.UserID != "user_1" || member.Role != domain.RoleOwner {
		t.Fatalf("membership = %#v", member)
	}
	orgs, err := service.ListOrganizations(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("ListOrganizations() error = %v", err)
	}
	if len(orgs) != 1 || orgs[0].ID != org.ID {
		t.Fatalf("organizations = %#v", orgs)
	}
}

func TestTenancyServiceInvitationHashesTokenAndAcceptsOnce(t *testing.T) {
	service := NewTenancyService()
	service.now = fixedTenancyTime
	service.newToken = func() (string, error) { return "invite-token-value", nil }
	org, _, err := service.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}

	invitation, token, err := service.InviteMember(context.Background(), "owner_1", org.ID, "Member@Example.com", domain.RoleMember)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if token != "invite-token-value" {
		t.Fatalf("token = %q", token)
	}
	record := service.invitations[invitation.ID]
	if record.tokenHash == "" || record.tokenHash == token {
		t.Fatalf("invitation token hash was not stored safely: %#v", record)
	}
	if invitation.Email != "member@example.com" || invitation.TokenPrefix == "" {
		t.Fatalf("invitation = %#v", invitation)
	}
	if _, err := service.AcceptInvitation(context.Background(), invitation.ID, "wrong-token", "user_2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong token error = %v, want %v", err, ErrNotFound)
	}
	member, err := service.AcceptInvitation(context.Background(), invitation.ID, token, "user_2")
	if err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	if member.OrganizationID != org.ID || member.UserID != "user_2" || member.Role != domain.RoleMember {
		t.Fatalf("accepted member = %#v", member)
	}
	if _, err := service.AcceptInvitation(context.Background(), invitation.ID, token, "user_3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replay accept error = %v, want %v", err, ErrNotFound)
	}
}

func TestTenancyServiceEnforcesRoleChecks(t *testing.T) {
	service := NewTenancyService()
	service.now = fixedTenancyTime
	service.newToken = func() (string, error) { return "invite-token-value", nil }
	org, _, err := service.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	invitation, token, err := service.InviteMember(context.Background(), "owner_1", org.ID, "viewer@example.com", domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if _, err := service.AcceptInvitation(context.Background(), invitation.ID, token, "viewer_1"); err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	if _, _, err := service.InviteMember(context.Background(), "viewer_1", org.ID, "member@example.com", domain.RoleMember); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer invite error = %v, want %v", err, ErrForbidden)
	}
	ok, err := service.HasRole(context.Background(), org.ID, "viewer_1", domain.RoleViewer)
	if err != nil {
		t.Fatalf("HasRole() error = %v", err)
	}
	if !ok {
		t.Fatal("viewer should satisfy viewer role")
	}
	ok, err = service.HasRole(context.Background(), org.ID, "viewer_1", domain.RoleAdmin)
	if err != nil {
		t.Fatalf("HasRole() admin error = %v", err)
	}
	if ok {
		t.Fatal("viewer should not satisfy admin role")
	}
}

func TestTenancyServiceWithStorePersistsInvitationHash(t *testing.T) {
	store := newRecordingTenancyStore()
	service := NewTenancyServiceWithStore(store)
	service.now = fixedTenancyTime
	service.newOrgID = func() (string, error) { return "org_store_1", nil }
	service.newInviteID = func() (string, error) { return "inv_store_1", nil }
	service.newToken = func() (string, error) { return "stored-invitation-token", nil }

	org, owner, err := service.CreateOrganization(context.Background(), "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if org.ID != "org_store_1" || owner.Role != domain.RoleOwner {
		t.Fatalf("org=%#v owner=%#v", org, owner)
	}
	invitation, token, err := service.InviteMember(context.Background(), "owner_1", org.ID, "Member@Example.com", domain.RoleMember)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if store.invitationHashes[invitation.ID] == "" || strings.Contains(store.invitationHashes[invitation.ID], token) {
		t.Fatalf("stored token hash leaked token: %q", store.invitationHashes[invitation.ID])
	}
	member, err := service.AcceptInvitation(context.Background(), invitation.ID, token, "member_1")
	if err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	if member.OrganizationID != org.ID || member.Role != domain.RoleMember {
		t.Fatalf("accepted member = %#v", member)
	}
	members, err := service.ListMembers(context.Background(), "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListMembers() error = %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %#v", members)
	}
	ok, err := service.HasRole(context.Background(), org.ID, "member_1", domain.RoleMember)
	if err != nil || !ok {
		t.Fatalf("HasRole() ok=%v err=%v", ok, err)
	}
}

type recordingTenancyStore struct {
	orgs             map[string]domain.Organization
	memberships      map[string]map[string]domain.Membership
	invitations      map[string]domain.Invitation
	invitationHashes map[string]string
}

func newRecordingTenancyStore() *recordingTenancyStore {
	return &recordingTenancyStore{
		orgs:             map[string]domain.Organization{},
		memberships:      map[string]map[string]domain.Membership{},
		invitations:      map[string]domain.Invitation{},
		invitationHashes: map[string]string{},
	}
}

func (s *recordingTenancyStore) CreateOrganization(_ context.Context, org domain.Organization, owner domain.Membership) error {
	s.orgs[org.ID] = org
	if s.memberships[org.ID] == nil {
		s.memberships[org.ID] = map[string]domain.Membership{}
	}
	s.memberships[org.ID][owner.UserID] = owner
	return nil
}

func (s *recordingTenancyStore) ListOrganizations(_ context.Context, actorID string) ([]domain.Organization, error) {
	var out []domain.Organization
	for orgID, members := range s.memberships {
		if _, ok := members[actorID]; ok {
			out = append(out, s.orgs[orgID])
		}
	}
	return out, nil
}

func (s *recordingTenancyStore) ListMembers(_ context.Context, organizationID string) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, member := range s.memberships[organizationID] {
		out = append(out, member)
	}
	return out, nil
}

func (s *recordingTenancyStore) CreateInvitation(_ context.Context, invitation domain.Invitation, tokenHash string) error {
	s.invitations[invitation.ID] = invitation
	s.invitationHashes[invitation.ID] = tokenHash
	return nil
}

func (s *recordingTenancyStore) GetInvitation(_ context.Context, invitationID string) (domain.Invitation, string, bool, error) {
	invitation, ok := s.invitations[invitationID]
	return invitation, s.invitationHashes[invitationID], ok, nil
}

func (s *recordingTenancyStore) AcceptInvitation(_ context.Context, invitationID, userID string, acceptedAt time.Time) (domain.Membership, bool, error) {
	invitation, ok := s.invitations[invitationID]
	if !ok || invitation.AcceptedAt != nil {
		return domain.Membership{}, false, nil
	}
	invitation.AcceptedAt = &acceptedAt
	s.invitations[invitationID] = invitation
	member := domain.Membership{OrganizationID: invitation.OrganizationID, UserID: userID, Role: invitation.Role, CreatedAt: acceptedAt}
	if s.memberships[invitation.OrganizationID] == nil {
		s.memberships[invitation.OrganizationID] = map[string]domain.Membership{}
	}
	s.memberships[invitation.OrganizationID][userID] = member
	return member, true, nil
}

func (s *recordingTenancyStore) HasRole(_ context.Context, organizationID, actorID string, required domain.Role) (bool, error) {
	member, ok := s.memberships[organizationID][actorID]
	return ok && member.Role.Allows(required), nil
}

func fixedTenancyTime() time.Time {
	return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
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
	store         WidgetStore
}

type WidgetStore interface {
	List(ctx context.Context, tenantID string) ([]domain.Widget, error)
	Get(ctx context.Context, tenantID, id string) (domain.Widget, bool, error)
	Save(ctx context.Context, widget domain.Widget) error
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

func NewWidgetServiceWithStore(store WidgetStore) *WidgetService {
	service := NewWidgetService()
	service.store = store
	return service
}

func (s *WidgetService) List(ctx context.Context, tenantID string) ([]domain.Widget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrValidation
	}
	if s.store != nil {
		return s.store.List(ctx, tenantID)
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
	if s.store != nil {
		if err := s.store.Save(ctx, widget); err != nil {
			return domain.Widget{}, false, err
		}
	} else {
		s.widgets[widget.ID] = widget
	}
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
	var (
		widget domain.Widget
		ok     bool
		err    error
	)
	if s.store != nil {
		widget, ok, err = s.store.Get(ctx, tenantID, id)
		if err != nil {
			return domain.Widget{}, false, err
		}
	} else {
		widget, ok = s.widgets[id]
	}
	if !ok || widget.Deleted || widget.TenantID != tenantID {
		return domain.Widget{}, false, ErrNotFound
	}
	if widget.ETag() != ifMatch {
		return domain.Widget{}, false, ErrPreconditionFailed
	}
	widget.Name = name
	widget.Version++
	widget.UpdatedAt = s.now().UTC()
	if s.store != nil {
		if err := s.store.Save(ctx, widget); err != nil {
			return domain.Widget{}, false, err
		}
	} else {
		s.widgets[id] = widget
	}
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
	var (
		widget domain.Widget
		ok     bool
		err    error
	)
	if s.store != nil {
		widget, ok, err = s.store.Get(ctx, tenantID, id)
		if err != nil {
			return false, err
		}
	} else {
		widget, ok = s.widgets[id]
	}
	if !ok || widget.Deleted || widget.TenantID != tenantID {
		return false, ErrNotFound
	}
	widget.Deleted = true
	widget.Version++
	widget.UpdatedAt = s.now().UTC()
	if s.store != nil {
		if err := s.store.Save(ctx, widget); err != nil {
			return false, err
		}
	} else {
		s.widgets[id] = widget
	}
	s.deleteReplays[replayKey] = struct{}{}
	return false, nil
}
`

const fullPostgresAdapterTemplate = `package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDatabaseURLRequired = errors.New("DATABASE_URL is required")
	ErrPoolRequired        = errors.New("postgres pool is required")
	ErrMigrationsRequired  = errors.New("postgres migrations are not applied")
)

var RequiredTables = []string{
	"organizations",
	"memberships",
	"invitations",
	"api_keys",
	"widgets",
	"operations",
	"outbox_events",
	"audit_events",
	"objects",
	"webhook_endpoints",
	"webhook_deliveries",
	// api-toolkit:postgres-required-tables
}

type Pinger interface {
	Ping(context.Context) error
}

type TableQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, ErrDatabaseURLRequired
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
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

func CheckRequiredTables(ctx context.Context, db TableQuerier, tables []string) error {
	if db == nil {
		return ErrPoolRequired
	}
	if len(tables) == 0 {
		tables = RequiredTables
	}
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table == "" {
			continue
		}
		qualified := "public." + table
		var exists bool
		if err := db.QueryRow(ctx, "select to_regclass($1) is not null", qualified).Scan(&exists); err != nil {
			return fmt.Errorf("check postgres table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("%w: missing table %s", ErrMigrationsRequired, table)
		}
	}
	return nil
}
`

const fullPostgresAdapterTestTemplate = `package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCheckRequiredTablesPassesWhenTablesExist(t *testing.T) {
	db := fakeTableQuerier{tables: map[string]bool{
		"public.organizations": true,
		"public.widgets":       true,
	}}
	if err := CheckRequiredTables(context.Background(), db, []string{"organizations", "widgets"}); err != nil {
		t.Fatalf("CheckRequiredTables() error = %v", err)
	}
}

func TestCheckRequiredTablesFailsClosedForMissingTables(t *testing.T) {
	db := fakeTableQuerier{tables: map[string]bool{"public.organizations": true}}
	err := CheckRequiredTables(context.Background(), db, []string{"organizations", "widgets"})
	if !errors.Is(err, ErrMigrationsRequired) {
		t.Fatalf("CheckRequiredTables() error = %v, want %v", err, ErrMigrationsRequired)
	}
}

func TestHealthCheckerRequiresPool(t *testing.T) {
	if err := (HealthChecker{}).Check(context.Background()); !errors.Is(err, ErrPoolRequired) {
		t.Fatalf("HealthChecker.Check() error = %v, want %v", err, ErrPoolRequired)
	}
}

type fakeTableQuerier struct {
	tables map[string]bool
	err    error
}

func (f fakeTableQuerier) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	table, _ := args[0].(string)
	return fakeRow{exists: f.tables[table], err: f.err}
}

type fakeRow struct {
	exists bool
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("one destination is required")
	}
	exists, ok := dest[0].(*bool)
	if !ok {
		return errors.New("bool destination is required")
	}
	*exists = r.exists
	return nil
}
`

const fullPostgresTenancyStoreTemplate = `package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"{{ .Module }}/internal/domain"
)

var (
	ErrTenancyStoreRequired = errors.New("postgres tenancy store db is required")
	ErrTenancyInvalid       = errors.New("postgres tenancy record is invalid")
)

type TenancyDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type TenancyStore struct {
	db TenancyDB
}

func NewTenancyStore(db TenancyDB) *TenancyStore {
	return &TenancyStore{db: db}
}

func (s *TenancyStore) CreateOrganization(ctx context.Context, org domain.Organization, owner domain.Membership) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrTenancyStoreRequired
	}
	org.ID = strings.TrimSpace(org.ID)
	org.Name = strings.TrimSpace(org.Name)
	owner.OrganizationID = strings.TrimSpace(owner.OrganizationID)
	owner.UserID = strings.TrimSpace(owner.UserID)
	if org.ID == "" || org.Name == "" || owner.OrganizationID != org.ID || owner.UserID == "" || owner.Role != domain.RoleOwner || org.CreatedAt.IsZero() || org.UpdatedAt.IsZero() || owner.CreatedAt.IsZero() {
		return ErrTenancyInvalid
	}
	_, err := s.db.Exec(ctx,
		"with inserted_org as (insert into organizations (id, name, created_at, updated_at) values ($1, $2, $3, $4) returning id) "+
			"insert into memberships (organization_id, user_id, role, created_at) select id, $5, $6, $7 from inserted_org",
		org.ID,
		org.Name,
		org.CreatedAt.UTC(),
		org.UpdatedAt.UTC(),
		owner.UserID,
		string(owner.Role),
		owner.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("create organization: %w", err)
	}
	return nil
}

func (s *TenancyStore) ListOrganizations(ctx context.Context, actorID string) ([]domain.Organization, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrTenancyStoreRequired
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, ErrTenancyInvalid
	}
	rows, err := s.db.Query(ctx,
		"select o.id, o.name, o.created_at, o.updated_at from organizations o join memberships m on m.organization_id=o.id where m.user_id=$1 order by o.id",
		actorID,
	)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	defer rows.Close()
	var out []domain.Organization
	for rows.Next() {
		org, err := scanOrganization(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list organization rows: %w", err)
	}
	return out, nil
}

func (s *TenancyStore) ListMembers(ctx context.Context, organizationID string) ([]domain.Membership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrTenancyStoreRequired
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil, ErrTenancyInvalid
	}
	rows, err := s.db.Query(ctx,
		"select organization_id, user_id, role, created_at from memberships where organization_id=$1 order by user_id",
		organizationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	var out []domain.Membership
	for rows.Next() {
		member, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list member rows: %w", err)
	}
	return out, nil
}

func (s *TenancyStore) CreateInvitation(ctx context.Context, invitation domain.Invitation, tokenHash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrTenancyStoreRequired
	}
	invitation.ID = strings.TrimSpace(invitation.ID)
	invitation.OrganizationID = strings.TrimSpace(invitation.OrganizationID)
	invitation.Email = strings.ToLower(strings.TrimSpace(invitation.Email))
	hashBytes, err := decodeTokenHash(tokenHash)
	if err != nil {
		return err
	}
	if invitation.ID == "" || invitation.OrganizationID == "" || invitation.Email == "" || !strings.Contains(invitation.Email, "@") || !invitation.Role.Valid() || invitation.ExpiresAt.IsZero() || invitation.CreatedAt.IsZero() {
		return ErrTenancyInvalid
	}
	if _, err := s.db.Exec(ctx,
		"insert into invitations (id, organization_id, email, role, token_hash, expires_at, created_at) values ($1, $2, $3, $4, $5, $6, $7)",
		invitation.ID,
		invitation.OrganizationID,
		invitation.Email,
		string(invitation.Role),
		hashBytes,
		invitation.ExpiresAt.UTC(),
		invitation.CreatedAt.UTC(),
	); err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

func (s *TenancyStore) GetInvitation(ctx context.Context, invitationID string) (domain.Invitation, string, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Invitation{}, "", false, err
	}
	if s == nil || s.db == nil {
		return domain.Invitation{}, "", false, ErrTenancyStoreRequired
	}
	invitationID = strings.TrimSpace(invitationID)
	if invitationID == "" {
		return domain.Invitation{}, "", false, ErrTenancyInvalid
	}
	row := s.db.QueryRow(ctx,
		"select id, organization_id, email, role, token_hash, expires_at, accepted_at, created_at from invitations where id=$1",
		invitationID,
	)
	invitation, tokenHash, err := scanInvitation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Invitation{}, "", false, nil
	}
	if err != nil {
		return domain.Invitation{}, "", false, fmt.Errorf("get invitation: %w", err)
	}
	return invitation, tokenHash, true, nil
}

func (s *TenancyStore) AcceptInvitation(ctx context.Context, invitationID, userID string, acceptedAt time.Time) (domain.Membership, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Membership{}, false, err
	}
	if s == nil || s.db == nil {
		return domain.Membership{}, false, ErrTenancyStoreRequired
	}
	invitationID = strings.TrimSpace(invitationID)
	userID = strings.TrimSpace(userID)
	if invitationID == "" || userID == "" || acceptedAt.IsZero() {
		return domain.Membership{}, false, ErrTenancyInvalid
	}
	row := s.db.QueryRow(ctx,
		"with accepted as (update invitations set accepted_at=$2 where id=$1 and accepted_at is null returning organization_id, role) "+
			"insert into memberships (organization_id, user_id, role, created_at) "+
			"select organization_id, $3, role, $2 from accepted "+
			"on conflict (organization_id, user_id) do update set role=excluded.role "+
			"returning organization_id, user_id, role, created_at",
		invitationID,
		acceptedAt.UTC(),
		userID,
	)
	member, err := scanMembership(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, false, nil
	}
	if err != nil {
		return domain.Membership{}, false, fmt.Errorf("accept invitation: %w", err)
	}
	return member, true, nil
}

func (s *TenancyStore) HasRole(ctx context.Context, organizationID, actorID string, required domain.Role) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.db == nil {
		return false, ErrTenancyStoreRequired
	}
	organizationID = strings.TrimSpace(organizationID)
	actorID = strings.TrimSpace(actorID)
	if organizationID == "" || actorID == "" || !required.Valid() {
		return false, ErrTenancyInvalid
	}
	var role string
	if err := s.db.QueryRow(ctx, "select role from memberships where organization_id=$1 and user_id=$2", organizationID, actorID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check role: %w", err)
	}
	got := domain.Role(role)
	if !got.Valid() {
		return false, ErrTenancyInvalid
	}
	return got.Allows(required), nil
}

type tenancyScanner interface {
	Scan(dest ...any) error
}

func scanOrganization(row tenancyScanner) (domain.Organization, error) {
	var org domain.Organization
	if err := row.Scan(&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt); err != nil {
		return domain.Organization{}, err
	}
	org.CreatedAt = org.CreatedAt.UTC()
	org.UpdatedAt = org.UpdatedAt.UTC()
	return org, nil
}

func scanMembership(row tenancyScanner) (domain.Membership, error) {
	var (
		member domain.Membership
		role   string
	)
	if err := row.Scan(&member.OrganizationID, &member.UserID, &role, &member.CreatedAt); err != nil {
		return domain.Membership{}, err
	}
	member.Role = domain.Role(role)
	if !member.Role.Valid() {
		return domain.Membership{}, ErrTenancyInvalid
	}
	member.CreatedAt = member.CreatedAt.UTC()
	return member, nil
}

func scanInvitation(row tenancyScanner) (domain.Invitation, string, error) {
	var (
		invitation domain.Invitation
		role       string
		tokenHash  []byte
		acceptedAt pgtype.Timestamptz
	)
	if err := row.Scan(&invitation.ID, &invitation.OrganizationID, &invitation.Email, &role, &tokenHash, &invitation.ExpiresAt, &acceptedAt, &invitation.CreatedAt); err != nil {
		return domain.Invitation{}, "", err
	}
	invitation.Role = domain.Role(role)
	if !invitation.Role.Valid() || len(tokenHash) != sha256HashSize {
		return domain.Invitation{}, "", ErrTenancyInvalid
	}
	invitation.AcceptedAt = tenancyNullableTimestamptz(acceptedAt)
	invitation.ExpiresAt = invitation.ExpiresAt.UTC()
	invitation.CreatedAt = invitation.CreatedAt.UTC()
	return invitation, hex.EncodeToString(tokenHash), nil
}

func decodeTokenHash(hash string) ([]byte, error) {
	decoded, err := hex.DecodeString(strings.TrimSpace(hash))
	if err != nil || len(decoded) != sha256HashSize {
		return nil, ErrTenancyInvalid
	}
	return decoded, nil
}

func tenancyNullableTimestamptz(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

const sha256HashSize = 32
`

const fullPostgresTenancyStoreTestTemplate = `package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"{{ .Module }}/internal/domain"
)

func TestTenancyStoreCreateOrganizationUsesSingleStatement(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	db := &fakeTenancyDB{execTag: pgconn.NewCommandTag("INSERT 0 1")}
	store := NewTenancyStore(db)
	err := store.CreateOrganization(context.Background(),
		domain.Organization{ID: "org_1", Name: "Acme", CreatedAt: now, UpdatedAt: now},
		domain.Membership{OrganizationID: "org_1", UserID: "owner_1", Role: domain.RoleOwner, CreatedAt: now},
	)
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if !strings.Contains(db.lastExecSQL, "insert into organizations") || !strings.Contains(db.lastExecSQL, "insert into memberships") {
		t.Fatalf("CreateOrganization() SQL = %q", db.lastExecSQL)
	}
}

func TestTenancyStoreInvitationStoresHashBytesAndAccepts(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	hash := hex.EncodeToString([]byte("12345678901234567890123456789012"))
	db := &fakeTenancyDB{execTag: pgconn.NewCommandTag("INSERT 0 1")}
	store := NewTenancyStore(db)
	err := store.CreateInvitation(context.Background(), domain.Invitation{ID: "inv_1", OrganizationID: "org_1", Email: "member@example.com", Role: domain.RoleMember, ExpiresAt: now.Add(time.Hour), CreatedAt: now}, hash)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	hashArg, ok := db.lastExecArgs[4].([]byte)
	if !ok || string(hashArg) != "12345678901234567890123456789012" {
		t.Fatalf("token hash arg = %#v", db.lastExecArgs[4])
	}
	if strings.Contains(fmt.Sprint(db.lastExecArgs...), "raw-token") {
		t.Fatalf("exec args leaked invitation token: %#v", db.lastExecArgs)
	}

	db.row = fakeTenancyRow{values: []any{"org_1", "member_1", "member", now}}
	member, ok, err := store.AcceptInvitation(context.Background(), "inv_1", "member_1", now)
	if err != nil || !ok || member.Role != domain.RoleMember {
		t.Fatalf("AcceptInvitation() member=%#v ok=%v err=%v", member, ok, err)
	}
	if !strings.Contains(db.lastRowSQL, "accepted_at is null") {
		t.Fatalf("AcceptInvitation() SQL = %q", db.lastRowSQL)
	}
}

func TestTenancyStoreListsAndChecksRoles(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	db := &fakeTenancyDB{
		rows: &fakeTenancyRows{rows: [][]any{[]any{"org_1", "Acme", now, now}}},
		row:  fakeTenancyRow{values: []any{"admin"}},
	}
	store := NewTenancyStore(db)
	orgs, err := store.ListOrganizations(context.Background(), "owner_1")
	if err != nil {
		t.Fatalf("ListOrganizations() error = %v", err)
	}
	if len(orgs) != 1 || orgs[0].ID != "org_1" {
		t.Fatalf("ListOrganizations() = %#v", orgs)
	}
	ok, err := store.HasRole(context.Background(), "org_1", "admin_1", domain.RoleMember)
	if err != nil || !ok {
		t.Fatalf("HasRole() ok=%v err=%v", ok, err)
	}
	ok, err = store.HasRole(context.Background(), "org_1", "admin_1", domain.RoleOwner)
	if err != nil || ok {
		t.Fatalf("HasRole(owner) ok=%v err=%v", ok, err)
	}
}

func TestTenancyStoreGetInvitationScansTokenHash(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	tokenHash := []byte("12345678901234567890123456789012")
	db := &fakeTenancyDB{row: fakeTenancyRow{values: []any{"inv_1", "org_1", "member@example.com", "member", tokenHash, now.Add(time.Hour), pgtype.Timestamptz{}, now}}}
	store := NewTenancyStore(db)
	invitation, hash, ok, err := store.GetInvitation(context.Background(), "inv_1")
	if err != nil || !ok || invitation.ID != "inv_1" || hash != hex.EncodeToString(tokenHash) {
		t.Fatalf("GetInvitation() invitation=%#v hash=%q ok=%v err=%v", invitation, hash, ok, err)
	}
}

func TestTenancyStoreRequiresDBAndValidHash(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	err := (*TenancyStore)(nil).CreateInvitation(context.Background(), domain.Invitation{ID: "inv_1", OrganizationID: "org_1", Email: "member@example.com", Role: domain.RoleMember, ExpiresAt: now.Add(time.Hour), CreatedAt: now}, strings.Repeat("0", 64))
	if !errors.Is(err, ErrTenancyStoreRequired) {
		t.Fatalf("nil CreateInvitation() error = %v, want %v", err, ErrTenancyStoreRequired)
	}
	store := NewTenancyStore(&fakeTenancyDB{})
	err = store.CreateInvitation(context.Background(), domain.Invitation{ID: "inv_1", OrganizationID: "org_1", Email: "member@example.com", Role: domain.RoleMember, ExpiresAt: now.Add(time.Hour), CreatedAt: now}, "not-hex")
	if !errors.Is(err, ErrTenancyInvalid) {
		t.Fatalf("invalid hash CreateInvitation() error = %v, want %v", err, ErrTenancyInvalid)
	}
}

type fakeTenancyDB struct {
	rows          pgx.Rows
	row           pgx.Row
	execTag       pgconn.CommandTag
	execErr       error
	lastQuerySQL  string
	lastQueryArgs []any
	lastRowSQL    string
	lastRowArgs   []any
	lastExecSQL   string
	lastExecArgs  []any
}

func (f *fakeTenancyDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastQuerySQL = sql
	f.lastQueryArgs = append([]any(nil), args...)
	if f.rows == nil {
		return &fakeTenancyRows{}, nil
	}
	return f.rows, nil
}

func (f *fakeTenancyDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastRowSQL = sql
	f.lastRowArgs = append([]any(nil), args...)
	if f.row == nil {
		return fakeTenancyRow{err: pgx.ErrNoRows}
	}
	return f.row
}

func (f *fakeTenancyDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastExecSQL = sql
	f.lastExecArgs = append([]any(nil), args...)
	return f.execTag, f.execErr
}

type fakeTenancyRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeTenancyRows) Close() {}
func (r *fakeTenancyRows) Err() error { return r.err }
func (r *fakeTenancyRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeTenancyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeTenancyRows) Values() ([]any, error) { return r.rows[r.idx-1], nil }
func (r *fakeTenancyRows) RawValues() [][]byte { return nil }
func (r *fakeTenancyRows) Conn() *pgx.Conn { return nil }

func (r *fakeTenancyRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeTenancyRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return scanFakeTenancyValues(r.rows[r.idx-1], dest...)
}

type fakeTenancyRow struct {
	values []any
	err    error
}

func (r fakeTenancyRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanFakeTenancyValues(r.values, dest...)
}

func scanFakeTenancyValues(values []any, dest ...any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("value count %d does not match destination count %d", len(values), len(dest))
	}
	for i := range values {
		switch d := dest[i].(type) {
		case *string:
			value, ok := values[i].(string)
			if !ok {
				return fmt.Errorf("value %d is %T, want string", i, values[i])
			}
			*d = value
		case *[]byte:
			value, ok := values[i].([]byte)
			if !ok {
				return fmt.Errorf("value %d is %T, want []byte", i, values[i])
			}
			*d = append([]byte(nil), value...)
		case *pgtype.Timestamptz:
			value, ok := values[i].(pgtype.Timestamptz)
			if !ok {
				return fmt.Errorf("value %d is %T, want pgtype.Timestamptz", i, values[i])
			}
			*d = value
		case *time.Time:
			value, ok := values[i].(time.Time)
			if !ok {
				return fmt.Errorf("value %d is %T, want time.Time", i, values[i])
			}
			*d = value
		default:
			return fmt.Errorf("unsupported destination %T", dest[i])
		}
	}
	return nil
}
`

const fullPostgresWidgetStoreTemplate = `package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"{{ .Module }}/internal/domain"
)

var (
	ErrWidgetStoreRequired = errors.New("postgres widget store db is required")
	ErrWidgetInvalid       = errors.New("postgres widget is invalid")
)

type WidgetDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type WidgetStore struct {
	db WidgetDB
}

func NewWidgetStore(db WidgetDB) *WidgetStore {
	return &WidgetStore{db: db}
}

func (s *WidgetStore) List(ctx context.Context, tenantID string) ([]domain.Widget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrWidgetStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrWidgetInvalid
	}
	rows, err := s.db.Query(ctx,
		"select id, organization_id, name, version, deleted_at is not null, created_at, updated_at "+
			"from widgets where organization_id=$1 and deleted_at is null order by id",
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list widgets: %w", err)
	}
	defer rows.Close()
	var out []domain.Widget
	for rows.Next() {
		widget, err := scanWidget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, widget)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list widgets rows: %w", err)
	}
	return out, nil
}

func (s *WidgetStore) Get(ctx context.Context, tenantID, id string) (domain.Widget, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Widget{}, false, err
	}
	if s == nil || s.db == nil {
		return domain.Widget{}, false, ErrWidgetStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	if tenantID == "" || id == "" {
		return domain.Widget{}, false, ErrWidgetInvalid
	}
	row := s.db.QueryRow(ctx,
		"select id, organization_id, name, version, deleted_at is not null, created_at, updated_at "+
			"from widgets where organization_id=$1 and id=$2",
		tenantID,
		id,
	)
	widget, err := scanWidget(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Widget{}, false, nil
	}
	if err != nil {
		return domain.Widget{}, false, fmt.Errorf("get widget: %w", err)
	}
	return widget, true, nil
}

func (s *WidgetStore) Save(ctx context.Context, widget domain.Widget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrWidgetStoreRequired
	}
	widget.ID = strings.TrimSpace(widget.ID)
	widget.TenantID = strings.TrimSpace(widget.TenantID)
	widget.Name = strings.TrimSpace(widget.Name)
	if widget.ID == "" || widget.TenantID == "" || widget.Name == "" || widget.Version <= 0 || widget.CreatedAt.IsZero() || widget.UpdatedAt.IsZero() {
		return ErrWidgetInvalid
	}
	var deletedAt any
	if widget.Deleted {
		deletedAt = widget.UpdatedAt.UTC()
	}
	var savedID string
	if err := s.db.QueryRow(ctx,
		"insert into widgets (id, organization_id, name, version, deleted_at, created_at, updated_at) "+
			"values ($1, $2, $3, $4, $5, $6, $7) "+
			"on conflict (id) do update set name=excluded.name, version=excluded.version, deleted_at=excluded.deleted_at, updated_at=excluded.updated_at "+
			"returning id",
		widget.ID,
		widget.TenantID,
		widget.Name,
		widget.Version,
		deletedAt,
		widget.CreatedAt.UTC(),
		widget.UpdatedAt.UTC(),
	).Scan(&savedID); err != nil {
		return fmt.Errorf("save widget: %w", err)
	}
	return nil
}

type widgetScanner interface {
	Scan(dest ...any) error
}

func scanWidget(row widgetScanner) (domain.Widget, error) {
	var widget domain.Widget
	if err := row.Scan(&widget.ID, &widget.TenantID, &widget.Name, &widget.Version, &widget.Deleted, &widget.CreatedAt, &widget.UpdatedAt); err != nil {
		return domain.Widget{}, err
	}
	widget.CreatedAt = widget.CreatedAt.UTC()
	widget.UpdatedAt = widget.UpdatedAt.UTC()
	return widget, nil
}
`

const fullPostgresWidgetStoreTestTemplate = `package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"{{ .Module }}/internal/domain"
)

func TestWidgetStoreSaveUsesUpsert(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	db := &fakeWidgetDB{row: fakeWidgetRow{values: []any{"wgt_1"}}}
	store := NewWidgetStore(db)
	err := store.Save(context.Background(), domain.Widget{ID: "wgt_1", TenantID: "org_1", Name: "First", Version: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !strings.Contains(db.lastRowSQL, "insert into widgets") || !strings.Contains(db.lastRowSQL, "on conflict") {
		t.Fatalf("Save() SQL = %q", db.lastRowSQL)
	}
	if got := db.lastRowArgs[4]; got != nil {
		t.Fatalf("Save() active deleted_at arg = %#v, want nil", got)
	}
}

func TestWidgetStoreListAndGetScanWidgets(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	db := &fakeWidgetDB{
		rows: &fakeWidgetRows{rows: [][]any{[]any{"wgt_1", "org_1", "First", int64(2), false, now, now}}},
		row:  fakeWidgetRow{values: []any{"wgt_1", "org_1", "First", int64(2), false, now, now}},
	}
	store := NewWidgetStore(db)
	widgets, err := store.List(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(widgets) != 1 || widgets[0].ID != "wgt_1" || widgets[0].Version != 2 {
		t.Fatalf("List() = %#v", widgets)
	}
	got, ok, err := store.Get(context.Background(), "org_1", "wgt_1")
	if err != nil || !ok || got.ID != "wgt_1" {
		t.Fatalf("Get() widget=%#v ok=%v err=%v", got, ok, err)
	}
}

func TestWidgetStoreGetNotFound(t *testing.T) {
	store := NewWidgetStore(&fakeWidgetDB{row: fakeWidgetRow{err: pgx.ErrNoRows}})
	got, ok, err := store.Get(context.Background(), "org_1", "missing")
	if err != nil || ok || got.ID != "" {
		t.Fatalf("Get() widget=%#v ok=%v err=%v", got, ok, err)
	}
}

func TestWidgetStoreRequiresDBAndValidWidget(t *testing.T) {
	if err := (*WidgetStore)(nil).Save(context.Background(), domain.Widget{}); !errors.Is(err, ErrWidgetStoreRequired) {
		t.Fatalf("nil Save() error = %v, want %v", err, ErrWidgetStoreRequired)
	}
	if err := NewWidgetStore(&fakeWidgetDB{}).Save(context.Background(), domain.Widget{}); !errors.Is(err, ErrWidgetInvalid) {
		t.Fatalf("invalid Save() error = %v, want %v", err, ErrWidgetInvalid)
	}
}

type fakeWidgetDB struct {
	rows         pgx.Rows
	row          pgx.Row
	queryErr     error
	lastQuerySQL string
	lastQueryArgs []any
	lastRowSQL   string
	lastRowArgs  []any
}

func (f *fakeWidgetDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastQuerySQL = sql
	f.lastQueryArgs = append([]any(nil), args...)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.rows == nil {
		return &fakeWidgetRows{}, nil
	}
	return f.rows, nil
}

func (f *fakeWidgetDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastRowSQL = sql
	f.lastRowArgs = append([]any(nil), args...)
	if f.row == nil {
		return fakeWidgetRow{err: pgx.ErrNoRows}
	}
	return f.row
}

type fakeWidgetRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeWidgetRows) Close() {}
func (r *fakeWidgetRows) Err() error { return r.err }
func (r *fakeWidgetRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeWidgetRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeWidgetRows) Values() ([]any, error) { return r.rows[r.idx-1], nil }
func (r *fakeWidgetRows) RawValues() [][]byte { return nil }
func (r *fakeWidgetRows) Conn() *pgx.Conn { return nil }

func (r *fakeWidgetRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeWidgetRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return scanFakeWidgetValues(r.rows[r.idx-1], dest...)
}

type fakeWidgetRow struct {
	values []any
	err    error
}

func (r fakeWidgetRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanFakeWidgetValues(r.values, dest...)
}

func scanFakeWidgetValues(values []any, dest ...any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("value count %d does not match destination count %d", len(values), len(dest))
	}
	for i := range values {
		switch d := dest[i].(type) {
		case *string:
			value, ok := values[i].(string)
			if !ok {
				return fmt.Errorf("value %d is %T, want string", i, values[i])
			}
			*d = value
		case *int64:
			value, ok := values[i].(int64)
			if !ok {
				return fmt.Errorf("value %d is %T, want int64", i, values[i])
			}
			*d = value
		case *bool:
			value, ok := values[i].(bool)
			if !ok {
				return fmt.Errorf("value %d is %T, want bool", i, values[i])
			}
			*d = value
		case *time.Time:
			value, ok := values[i].(time.Time)
			if !ok {
				return fmt.Errorf("value %d is %T, want time.Time", i, values[i])
			}
			*d = value
		default:
			return fmt.Errorf("unsupported destination %T", dest[i])
		}
	}
	return nil
}
`

const fullPostgresObjectStoreTemplate = `package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"{{ .Module }}/internal/app"
)

var (
	ErrObjectStoreRequired = errors.New("postgres object store db is required")
	ErrObjectInvalid       = errors.New("postgres object metadata is invalid")
)

type ObjectDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type ObjectStore struct {
	db ObjectDB
}

func NewObjectStore(db ObjectDB) *ObjectStore {
	return &ObjectStore{db: db}
}

func (s *ObjectStore) SaveObjectMetadata(ctx context.Context, object app.Object) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrObjectStoreRequired
	}
	object.TenantID = strings.TrimSpace(object.TenantID)
	object.Key = strings.TrimSpace(object.Key)
	object.ContentType = strings.TrimSpace(object.ContentType)
	if object.TenantID == "" || object.Key == "" || object.ContentType == "" || object.Size < 0 || object.CreatedAt.IsZero() || object.UpdatedAt.IsZero() {
		return ErrObjectInvalid
	}
	var savedKey string
	if err := s.db.QueryRow(ctx,
		"insert into objects (organization_id, key, content_type, size, created_at, updated_at) "+
			"values ($1, $2, $3, $4, $5, $6) "+
			"on conflict (organization_id, key) do update set content_type=excluded.content_type, size=excluded.size, updated_at=excluded.updated_at "+
			"returning key",
		object.TenantID,
		object.Key,
		object.ContentType,
		object.Size,
		object.CreatedAt.UTC(),
		object.UpdatedAt.UTC(),
	).Scan(&savedKey); err != nil {
		return fmt.Errorf("save object metadata: %w", err)
	}
	return nil
}

func (s *ObjectStore) GetObjectMetadata(ctx context.Context, tenantID, key string) (app.Object, bool, error) {
	if err := ctx.Err(); err != nil {
		return app.Object{}, false, err
	}
	if s == nil || s.db == nil {
		return app.Object{}, false, ErrObjectStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	key = strings.TrimSpace(key)
	if tenantID == "" || key == "" {
		return app.Object{}, false, ErrObjectInvalid
	}
	row := s.db.QueryRow(ctx,
		"select organization_id, key, content_type, size, created_at, updated_at from objects where organization_id=$1 and key=$2",
		tenantID,
		key,
	)
	object, err := scanObjectMetadata(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Object{}, false, nil
	}
	if err != nil {
		return app.Object{}, false, fmt.Errorf("get object metadata: %w", err)
	}
	return object, true, nil
}

func (s *ObjectStore) ListObjectMetadata(ctx context.Context, tenantID string) ([]app.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrObjectStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrObjectInvalid
	}
	rows, err := s.db.Query(ctx,
		"select organization_id, key, content_type, size, created_at, updated_at from objects where organization_id=$1 order by key",
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list object metadata: %w", err)
	}
	defer rows.Close()
	var out []app.Object
	for rows.Next() {
		object, err := scanObjectMetadata(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list object metadata rows: %w", err)
	}
	return out, nil
}

func (s *ObjectStore) DeleteObjectMetadata(ctx context.Context, tenantID, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.db == nil {
		return false, ErrObjectStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	key = strings.TrimSpace(key)
	if tenantID == "" || key == "" {
		return false, ErrObjectInvalid
	}
	tag, err := s.db.Exec(ctx, "delete from objects where organization_id=$1 and key=$2", tenantID, key)
	if err != nil {
		return false, fmt.Errorf("delete object metadata: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

type objectScanner interface {
	Scan(dest ...any) error
}

func scanObjectMetadata(row objectScanner) (app.Object, error) {
	var object app.Object
	if err := row.Scan(&object.TenantID, &object.Key, &object.ContentType, &object.Size, &object.CreatedAt, &object.UpdatedAt); err != nil {
		return app.Object{}, err
	}
	object.CreatedAt = object.CreatedAt.UTC()
	object.UpdatedAt = object.UpdatedAt.UTC()
	return object, nil
}
`

const fullPostgresObjectStoreTestTemplate = `package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"{{ .Module }}/internal/app"
)

func TestObjectStoreSaveUsesUpsertWithoutPayload(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	db := &fakeObjectDB{row: fakeObjectRow{values: []any{"readme.txt"}}}
	store := NewObjectStore(db)
	err := store.SaveObjectMetadata(context.Background(), app.Object{TenantID: "org_1", Key: "readme.txt", ContentType: "text/plain", Size: 5, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("SaveObjectMetadata() error = %v", err)
	}
	if !strings.Contains(db.lastRowSQL, "insert into objects") || !strings.Contains(db.lastRowSQL, "on conflict") {
		t.Fatalf("SaveObjectMetadata() SQL = %q", db.lastRowSQL)
	}
	if strings.Contains(fmt.Sprint(db.lastRowArgs...), "hello") {
		t.Fatalf("metadata args leaked object payload: %#v", db.lastRowArgs)
	}
}

func TestObjectStoreListGetAndDelete(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	values := []any{"org_1", "readme.txt", "text/plain", int64(5), now, now}
	db := &fakeObjectDB{
		rows:    &fakeObjectRows{rows: [][]any{values}},
		row:     fakeObjectRow{values: values},
		execTag: pgconn.NewCommandTag("DELETE 1"),
	}
	store := NewObjectStore(db)
	objects, err := store.ListObjectMetadata(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("ListObjectMetadata() error = %v", err)
	}
	if len(objects) != 1 || objects[0].Key != "readme.txt" || objects[0].Size != 5 {
		t.Fatalf("ListObjectMetadata() = %#v", objects)
	}
	got, ok, err := store.GetObjectMetadata(context.Background(), "org_1", "readme.txt")
	if err != nil || !ok || got.ContentType != "text/plain" {
		t.Fatalf("GetObjectMetadata() object=%#v ok=%v err=%v", got, ok, err)
	}
	deleted, err := store.DeleteObjectMetadata(context.Background(), "org_1", "readme.txt")
	if err != nil || !deleted {
		t.Fatalf("DeleteObjectMetadata() deleted=%v err=%v", deleted, err)
	}
	if strings.Contains(fmt.Sprint(db.lastExecArgs...), "hello") {
		t.Fatalf("delete args leaked object payload: %#v", db.lastExecArgs)
	}
}

func TestObjectStoreGetNotFound(t *testing.T) {
	store := NewObjectStore(&fakeObjectDB{row: fakeObjectRow{err: pgx.ErrNoRows}})
	got, ok, err := store.GetObjectMetadata(context.Background(), "org_1", "missing.txt")
	if err != nil || ok || got.Key != "" {
		t.Fatalf("GetObjectMetadata() object=%#v ok=%v err=%v", got, ok, err)
	}
}

func TestObjectStoreRequiresDBAndValidObject(t *testing.T) {
	if err := (*ObjectStore)(nil).SaveObjectMetadata(context.Background(), app.Object{}); !errors.Is(err, ErrObjectStoreRequired) {
		t.Fatalf("nil SaveObjectMetadata() error = %v, want %v", err, ErrObjectStoreRequired)
	}
	if err := NewObjectStore(&fakeObjectDB{}).SaveObjectMetadata(context.Background(), app.Object{}); !errors.Is(err, ErrObjectInvalid) {
		t.Fatalf("invalid SaveObjectMetadata() error = %v, want %v", err, ErrObjectInvalid)
	}
}

type fakeObjectDB struct {
	rows          pgx.Rows
	row           pgx.Row
	queryErr      error
	execTag       pgconn.CommandTag
	execErr       error
	lastQuerySQL  string
	lastQueryArgs []any
	lastRowSQL    string
	lastRowArgs   []any
	lastExecSQL   string
	lastExecArgs  []any
}

func (f *fakeObjectDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastQuerySQL = sql
	f.lastQueryArgs = append([]any(nil), args...)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.rows == nil {
		return &fakeObjectRows{}, nil
	}
	return f.rows, nil
}

func (f *fakeObjectDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastRowSQL = sql
	f.lastRowArgs = append([]any(nil), args...)
	if f.row == nil {
		return fakeObjectRow{err: pgx.ErrNoRows}
	}
	return f.row
}

func (f *fakeObjectDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastExecSQL = sql
	f.lastExecArgs = append([]any(nil), args...)
	return f.execTag, f.execErr
}

type fakeObjectRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeObjectRows) Close() {}
func (r *fakeObjectRows) Err() error { return r.err }
func (r *fakeObjectRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeObjectRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeObjectRows) Values() ([]any, error) { return r.rows[r.idx-1], nil }
func (r *fakeObjectRows) RawValues() [][]byte { return nil }
func (r *fakeObjectRows) Conn() *pgx.Conn { return nil }

func (r *fakeObjectRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeObjectRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return scanFakeObjectValues(r.rows[r.idx-1], dest...)
}

type fakeObjectRow struct {
	values []any
	err    error
}

func (r fakeObjectRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanFakeObjectValues(r.values, dest...)
}

func scanFakeObjectValues(values []any, dest ...any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("value count %d does not match destination count %d", len(values), len(dest))
	}
	for i := range values {
		switch d := dest[i].(type) {
		case *string:
			value, ok := values[i].(string)
			if !ok {
				return fmt.Errorf("value %d is %T, want string", i, values[i])
			}
			*d = value
		case *int64:
			value, ok := values[i].(int64)
			if !ok {
				return fmt.Errorf("value %d is %T, want int64", i, values[i])
			}
			*d = value
		case *time.Time:
			value, ok := values[i].(time.Time)
			if !ok {
				return fmt.Errorf("value %d is %T, want time.Time", i, values[i])
			}
			*d = value
		default:
			return fmt.Errorf("unsupported destination %T", dest[i])
		}
	}
	return nil
}
`

const fullPostgresAPIKeyStoreTemplate = `package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"{{ .Module }}/internal/domain"
)

var (
	ErrAPIKeyStoreRequired = errors.New("postgres api key store db is required")
	ErrAPIKeyInvalid       = errors.New("postgres api key is invalid")
)

type APIKeyDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type APIKeyStore struct {
	db APIKeyDB
}

func NewAPIKeyStore(db APIKeyDB) *APIKeyStore {
	return &APIKeyStore{db: db}
}

func (s *APIKeyStore) CreateAPIKey(ctx context.Context, key domain.APIKey, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrAPIKeyStoreRequired
	}
	key.ID = strings.TrimSpace(key.ID)
	key.OrganizationID = strings.TrimSpace(key.OrganizationID)
	key.Name = strings.TrimSpace(key.Name)
	key.Prefix = strings.TrimSpace(key.Prefix)
	hashBytes, err := decodeAPIKeyHash(hash)
	if err != nil {
		return err
	}
	if key.ID == "" || key.OrganizationID == "" || key.Name == "" || key.Prefix == "" || len(key.Scopes) == 0 || key.CreatedAt.IsZero() {
		return ErrAPIKeyInvalid
	}
	if _, err := s.db.Exec(ctx,
		"insert into api_keys (id, organization_id, name, prefix, key_hash, scopes, expires_at, created_at) values ($1, $2, $3, $4, $5, $6, $7, $8)",
		key.ID,
		key.OrganizationID,
		key.Name,
		key.Prefix,
		hashBytes,
		append([]string(nil), key.Scopes...),
		nullableTime(key.ExpiresAt),
		key.CreatedAt.UTC(),
	); err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

func (s *APIKeyStore) ListAPIKeys(ctx context.Context, organizationID string) ([]domain.APIKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrAPIKeyStoreRequired
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil, ErrAPIKeyInvalid
	}
	rows, err := s.db.Query(ctx,
		"select id, organization_id, name, prefix, scopes, expires_at, last_used_at, revoked_at, created_at from api_keys where organization_id=$1 order by id",
		organizationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var out []domain.APIKey
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list api key rows: %w", err)
	}
	return out, nil
}

func (s *APIKeyStore) GetAPIKeyByHash(ctx context.Context, hash string) (domain.APIKey, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIKey{}, false, err
	}
	if s == nil || s.db == nil {
		return domain.APIKey{}, false, ErrAPIKeyStoreRequired
	}
	hashBytes, err := decodeAPIKeyHash(hash)
	if err != nil {
		return domain.APIKey{}, false, err
	}
	row := s.db.QueryRow(ctx,
		"select id, organization_id, name, prefix, scopes, expires_at, last_used_at, revoked_at, created_at from api_keys where key_hash=$1",
		hashBytes,
	)
	key, err := scanAPIKey(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.APIKey{}, false, nil
	}
	if err != nil {
		return domain.APIKey{}, false, fmt.Errorf("get api key by hash: %w", err)
	}
	return key, true, nil
}

func (s *APIKeyStore) RevokeAPIKey(ctx context.Context, organizationID, keyID string, revokedAt time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.db == nil {
		return false, ErrAPIKeyStoreRequired
	}
	organizationID = strings.TrimSpace(organizationID)
	keyID = strings.TrimSpace(keyID)
	if organizationID == "" || keyID == "" || revokedAt.IsZero() {
		return false, ErrAPIKeyInvalid
	}
	tag, err := s.db.Exec(ctx,
		"update api_keys set revoked_at=$1 where organization_id=$2 and id=$3 and revoked_at is null",
		revokedAt.UTC(),
		organizationID,
		keyID,
	)
	if err != nil {
		return false, fmt.Errorf("revoke api key: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *APIKeyStore) TouchAPIKey(ctx context.Context, keyID string, lastUsedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrAPIKeyStoreRequired
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" || lastUsedAt.IsZero() {
		return ErrAPIKeyInvalid
	}
	if _, err := s.db.Exec(ctx, "update api_keys set last_used_at=$1 where id=$2", lastUsedAt.UTC(), keyID); err != nil {
		return fmt.Errorf("touch api key: %w", err)
	}
	return nil
}

type apiKeyScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(row apiKeyScanner) (domain.APIKey, error) {
	var (
		key        domain.APIKey
		expiresAt  pgtype.Timestamptz
		lastUsedAt pgtype.Timestamptz
		revokedAt  pgtype.Timestamptz
	)
	if err := row.Scan(&key.ID, &key.OrganizationID, &key.Name, &key.Prefix, &key.Scopes, &expiresAt, &lastUsedAt, &revokedAt, &key.CreatedAt); err != nil {
		return domain.APIKey{}, err
	}
	key.ExpiresAt = nullableTimestamptz(expiresAt)
	key.LastUsedAt = nullableTimestamptz(lastUsedAt)
	key.RevokedAt = nullableTimestamptz(revokedAt)
	key.CreatedAt = key.CreatedAt.UTC()
	return key, nil
}

func decodeAPIKeyHash(hash string) ([]byte, error) {
	hash = strings.TrimSpace(hash)
	decoded, err := hex.DecodeString(hash)
	if err != nil || len(decoded) != sha256Size {
		return nil, ErrAPIKeyInvalid
	}
	return decoded, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableTimestamptz(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

const sha256Size = 32
`

const fullPostgresAPIKeyStoreTestTemplate = `package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"{{ .Module }}/internal/domain"
)

func TestAPIKeyStoreCreateStoresDecodedHash(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	hash := hex.EncodeToString([]byte("12345678901234567890123456789012"))
	db := &fakeAPIKeyDB{execTag: pgconn.NewCommandTag("INSERT 0 1")}
	store := NewAPIKeyStore(db)
	err := store.CreateAPIKey(context.Background(), domain.APIKey{ID: "key_1", OrganizationID: "org_1", Name: "CI", Prefix: "atk_prefix", Scopes: []string{"widgets:read"}, CreatedAt: now}, hash)
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if !strings.Contains(db.lastExecSQL, "insert into api_keys") {
		t.Fatalf("CreateAPIKey() SQL = %q", db.lastExecSQL)
	}
	hashArg, ok := db.lastExecArgs[4].([]byte)
	if !ok || string(hashArg) != "12345678901234567890123456789012" {
		t.Fatalf("hash arg = %#v", db.lastExecArgs[4])
	}
	if strings.Contains(fmt.Sprint(db.lastExecArgs...), "raw-secret") {
		t.Fatalf("exec args leaked raw secret: %#v", db.lastExecArgs)
	}
}

func TestAPIKeyStoreListAndGetByHashScanKeys(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	values := []any{"key_1", "org_1", "CI", "atk_prefix", []string{"widgets:read"}, pgtype.Timestamptz{}, pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, now}
	db := &fakeAPIKeyDB{
		rows: &fakeAPIKeyRows{rows: [][]any{values}},
		row:  fakeAPIKeyRow{values: values},
	}
	store := NewAPIKeyStore(db)
	keys, err := store.ListAPIKeys(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("ListAPIKeys() error = %v", err)
	}
	if len(keys) != 1 || keys[0].ID != "key_1" || keys[0].LastUsedAt == nil {
		t.Fatalf("ListAPIKeys() = %#v", keys)
	}
	hash := hex.EncodeToString([]byte("12345678901234567890123456789012"))
	key, ok, err := store.GetAPIKeyByHash(context.Background(), hash)
	if err != nil || !ok || key.ID != "key_1" {
		t.Fatalf("GetAPIKeyByHash() key=%#v ok=%v err=%v", key, ok, err)
	}
}

func TestAPIKeyStoreRevokeAndTouchUseSafeUpdates(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	db := &fakeAPIKeyDB{execTag: pgconn.NewCommandTag("UPDATE 1")}
	store := NewAPIKeyStore(db)
	ok, err := store.RevokeAPIKey(context.Background(), "org_1", "key_1", now)
	if err != nil || !ok {
		t.Fatalf("RevokeAPIKey() ok=%v err=%v", ok, err)
	}
	if !strings.Contains(db.lastExecSQL, "revoked_at") {
		t.Fatalf("RevokeAPIKey() SQL = %q", db.lastExecSQL)
	}
	if err := store.TouchAPIKey(context.Background(), "key_1", now); err != nil {
		t.Fatalf("TouchAPIKey() error = %v", err)
	}
	if !strings.Contains(db.lastExecSQL, "last_used_at") {
		t.Fatalf("TouchAPIKey() SQL = %q", db.lastExecSQL)
	}
}

func TestAPIKeyStoreRequiresDBAndValidHash(t *testing.T) {
	if err := (*APIKeyStore)(nil).CreateAPIKey(context.Background(), domain.APIKey{}, strings.Repeat("0", 64)); !errors.Is(err, ErrAPIKeyStoreRequired) {
		t.Fatalf("nil CreateAPIKey() error = %v, want %v", err, ErrAPIKeyStoreRequired)
	}
	store := NewAPIKeyStore(&fakeAPIKeyDB{})
	if err := store.CreateAPIKey(context.Background(), domain.APIKey{ID: "key_1", OrganizationID: "org_1", Name: "CI", Prefix: "atk", Scopes: []string{"widgets:read"}, CreatedAt: time.Now()}, "not-hex"); !errors.Is(err, ErrAPIKeyInvalid) {
		t.Fatalf("invalid hash CreateAPIKey() error = %v, want %v", err, ErrAPIKeyInvalid)
	}
}

type fakeAPIKeyDB struct {
	rows          pgx.Rows
	row           pgx.Row
	execTag       pgconn.CommandTag
	execErr       error
	lastQuerySQL  string
	lastQueryArgs []any
	lastRowSQL    string
	lastRowArgs   []any
	lastExecSQL   string
	lastExecArgs  []any
}

func (f *fakeAPIKeyDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastQuerySQL = sql
	f.lastQueryArgs = append([]any(nil), args...)
	if f.rows == nil {
		return &fakeAPIKeyRows{}, nil
	}
	return f.rows, nil
}

func (f *fakeAPIKeyDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastRowSQL = sql
	f.lastRowArgs = append([]any(nil), args...)
	if f.row == nil {
		return fakeAPIKeyRow{err: pgx.ErrNoRows}
	}
	return f.row
}

func (f *fakeAPIKeyDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastExecSQL = sql
	f.lastExecArgs = append([]any(nil), args...)
	return f.execTag, f.execErr
}

type fakeAPIKeyRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeAPIKeyRows) Close() {}
func (r *fakeAPIKeyRows) Err() error { return r.err }
func (r *fakeAPIKeyRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeAPIKeyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeAPIKeyRows) Values() ([]any, error) { return r.rows[r.idx-1], nil }
func (r *fakeAPIKeyRows) RawValues() [][]byte { return nil }
func (r *fakeAPIKeyRows) Conn() *pgx.Conn { return nil }

func (r *fakeAPIKeyRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeAPIKeyRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("Scan called without current row")
	}
	return scanFakeAPIKeyValues(r.rows[r.idx-1], dest...)
}

type fakeAPIKeyRow struct {
	values []any
	err    error
}

func (r fakeAPIKeyRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanFakeAPIKeyValues(r.values, dest...)
}

func scanFakeAPIKeyValues(values []any, dest ...any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("value count %d does not match destination count %d", len(values), len(dest))
	}
	for i := range values {
		switch d := dest[i].(type) {
		case *string:
			value, ok := values[i].(string)
			if !ok {
				return fmt.Errorf("value %d is %T, want string", i, values[i])
			}
			*d = value
		case *[]string:
			value, ok := values[i].([]string)
			if !ok {
				return fmt.Errorf("value %d is %T, want []string", i, values[i])
			}
			*d = append([]string(nil), value...)
		case *pgtype.Timestamptz:
			value, ok := values[i].(pgtype.Timestamptz)
			if !ok {
				return fmt.Errorf("value %d is %T, want pgtype.Timestamptz", i, values[i])
			}
			*d = value
		case *time.Time:
			value, ok := values[i].(time.Time)
			if !ok {
				return fmt.Errorf("value %d is %T, want time.Time", i, values[i])
			}
			*d = value
		default:
			return fmt.Errorf("unsupported destination %T", dest[i])
		}
	}
	return nil
}
`

const fullPostgresAsyncStoreTemplate = `package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/operationpostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/outboxpostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/async"
	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/operations"
	"github.com/aatuh/api-toolkit/v2/ports"

	"{{ .Module }}/internal/app"
)

type WidgetImportOperationStore struct {
	store *operationpostgres.Store[app.WidgetImportResult]
}

func NewWidgetImportOperationStore(pool ports.DatabasePool) *WidgetImportOperationStore {
	return &WidgetImportOperationStore{store: operationpostgres.New[app.WidgetImportResult](pool, operationpostgres.Options{})}
}

func (s *WidgetImportOperationStore) CreateWidgetImportOperation(ctx context.Context, tenantID string, operation operations.Operation[app.WidgetImportResult]) error {
	if s == nil || s.store == nil {
		return operationpostgres.ErrStoreNotConfigured
	}
	return s.store.CreateOperation(operationpostgres.WithTenantID(ctx, tenantID), operation)
}

func (s *WidgetImportOperationStore) GetWidgetImportOperation(ctx context.Context, tenantID, id string) (operations.Operation[app.WidgetImportResult], bool, error) {
	if s == nil || s.store == nil {
		return operations.Operation[app.WidgetImportResult]{}, false, operationpostgres.ErrStoreNotConfigured
	}
	return s.store.GetOperation(operationpostgres.WithTenantID(ctx, tenantID), id)
}

func (s *WidgetImportOperationStore) UpdateWidgetImportOperation(ctx context.Context, tenantID string, operation operations.Operation[app.WidgetImportResult]) error {
	if s == nil || s.store == nil {
		return operationpostgres.ErrStoreNotConfigured
	}
	return s.store.UpdateOperation(operationpostgres.WithTenantID(ctx, tenantID), operation)
}

type WidgetImportOutbox struct {
	store      *outboxpostgres.Store
	operations *WidgetImportOperationStore
	mu         sync.Mutex
	leased     map[string]leasedWidgetImport
}

type leasedWidgetImport struct {
	TenantID    string
	OperationID string
}

func NewWidgetImportOutbox(pool ports.DatabasePool, operations *WidgetImportOperationStore) *WidgetImportOutbox {
	return &WidgetImportOutbox{
		store:      outboxpostgres.New(pool, outboxpostgres.Options{}),
		operations: operations,
		leased:     map[string]leasedWidgetImport{},
	}
}

func (s *WidgetImportOutbox) EnqueueWidgetImport(ctx context.Context, event app.WidgetImportEvent) error {
	if s == nil || s.store == nil {
		return outboxpostgres.ErrStoreNotConfigured
	}
	return s.store.Enqueue(ctx, outboxpostgres.Event{
		ID:       strings.TrimSpace(event.ID),
		TenantID: strings.TrimSpace(event.TenantID),
		Type:     strings.TrimSpace(event.Kind),
		Payload:  append([]byte(nil), event.Payload...),
	})
}

func (s *WidgetImportOutbox) Lease(ctx context.Context, limit int) ([]async.Job, error) {
	if s == nil || s.store == nil {
		return nil, outboxpostgres.ErrStoreNotConfigured
	}
	jobs, err := s.store.Lease(ctx, limit)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		operationID := widgetImportOperationID(job.Payload)
		if operationID == "" {
			continue
		}
		s.remember(job.ID, leasedWidgetImport{TenantID: job.TenantID, OperationID: operationID})
		if s.operations == nil {
			continue
		}
		operation, ok, err := s.operations.GetWidgetImportOperation(ctx, job.TenantID, operationID)
		if err != nil || !ok || operation.State != operations.StatePending {
			if err != nil {
				return nil, err
			}
			continue
		}
		running, err := operations.TransitionOperation(operation, operations.TransitionConfig[app.WidgetImportResult]{To: operations.StateRunning})
		if err != nil {
			return nil, err
		}
		if err := s.operations.UpdateWidgetImportOperation(ctx, job.TenantID, running); err != nil {
			return nil, err
		}
	}
	return jobs, nil
}

func (s *WidgetImportOutbox) Complete(ctx context.Context, id string) error {
	if s == nil || s.store == nil {
		return outboxpostgres.ErrStoreNotConfigured
	}
	err := s.store.Complete(ctx, id)
	s.forget(id)
	return err
}

func (s *WidgetImportOutbox) Fail(ctx context.Context, id string, message string) error {
	if s == nil || s.store == nil {
		return outboxpostgres.ErrStoreNotConfigured
	}
	if err := s.store.Fail(ctx, id, message); err != nil {
		return err
	}
	leased := s.forget(id)
	if s.operations == nil || leased.OperationID == "" || leased.TenantID == "" {
		return nil
	}
	operation, ok, err := s.operations.GetWidgetImportOperation(ctx, leased.TenantID, leased.OperationID)
	if err != nil || !ok || operations.IsTerminal(operation.State) {
		return err
	}
	failed, err := operations.TransitionOperation(operation, operations.TransitionConfig[app.WidgetImportResult]{
		To:      operations.StateFailed,
		Problem: &httpx.Problem{Title: "Async work failed", Detail: "worker failed"},
	})
	if err != nil {
		return err
	}
	return s.operations.UpdateWidgetImportOperation(ctx, leased.TenantID, failed)
}

func (s *WidgetImportOutbox) remember(id string, leased leasedWidgetImport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leased[strings.TrimSpace(id)] = leased
}

func (s *WidgetImportOutbox) forget(id string) leasedWidgetImport {
	s.mu.Lock()
	defer s.mu.Unlock()
	leased := s.leased[strings.TrimSpace(id)]
	delete(s.leased, strings.TrimSpace(id))
	return leased
}

func widgetImportOperationID(payload []byte) string {
	var body struct {
		OperationID string ` + "`json:\"operation_id\"`" + `
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.OperationID)
}
`

const fullPostgresAsyncStoreTestTemplate = `package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/operationpostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/outboxpostgres"
	"github.com/aatuh/api-toolkit/v2/operations"

	"{{ .Module }}/internal/app"
)

func TestWidgetImportOperationIDParsesPayloadSafely(t *testing.T) {
	if got := widgetImportOperationID([]byte(` + "`" + `{"operation_id":" op_1 "}` + "`" + `)); got != "op_1" {
		t.Fatalf("widgetImportOperationID() = %q", got)
	}
	if got := widgetImportOperationID([]byte(` + "`" + `{"operation_id":""}` + "`" + `)); got != "" {
		t.Fatalf("empty widgetImportOperationID() = %q", got)
	}
	if got := widgetImportOperationID([]byte(` + "`" + `not-json` + "`" + `)); got != "" {
		t.Fatalf("invalid widgetImportOperationID() = %q", got)
	}
}

func TestWidgetImportOperationStoreRequiresPool(t *testing.T) {
	store := NewWidgetImportOperationStore(nil)
	if err := store.CreateWidgetImportOperation(context.Background(), "org_1", operationsOperationForTest()); !errors.Is(err, operationpostgres.ErrStoreNotConfigured) {
		t.Fatalf("CreateWidgetImportOperation() error = %v, want %v", err, operationpostgres.ErrStoreNotConfigured)
	}
}

func TestWidgetImportOutboxRequiresPool(t *testing.T) {
	outbox := NewWidgetImportOutbox(nil, nil)
	if _, err := outbox.Lease(context.Background(), 1); !errors.Is(err, outboxpostgres.ErrStoreNotConfigured) {
		t.Fatalf("Lease() error = %v, want %v", err, outboxpostgres.ErrStoreNotConfigured)
	}
}

func operationsOperationForTest() operations.Operation[app.WidgetImportResult] {
	return operations.Operation[app.WidgetImportResult]{ID: "op_1", State: operations.StatePending}
}
`

const fullPostgresWebhookStoreTemplate = `package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/webhookdeliverypostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/webhookdelivery"
	"github.com/aatuh/api-toolkit/v2/ports"
)

var (
	ErrWebhookStoreRequired    = errors.New("postgres webhook store db is required")
	ErrWebhookInvalid          = errors.New("postgres webhook record is invalid")
	ErrWebhookSecretKeyInvalid = errors.New("WEBHOOK_SECRET_KEY must decode to 32 bytes")
)

type WebhookStore struct {
	pool      ports.DatabasePool
	base      *webhookdeliverypostgres.Store
	secretKey []byte
	now       func() time.Time
	tx        ports.TxManager
}

func NewWebhookStore(pool ports.DatabasePool, secretKey string) (*WebhookStore, error) {
	key, err := decodeWebhookSecretKey(secretKey)
	if err != nil {
		return nil, err
	}
	store := &WebhookStore{
		pool:      pool,
		secretKey: key,
		now:       time.Now,
		tx:        txpostgres.New(pool),
	}
	store.base = webhookdeliverypostgres.New(pool, webhookdeliverypostgres.Options{
		SecretResolver: webhookdeliverypostgres.SecretResolverFunc(store.resolveSigningSecret),
	})
	return store, nil
}

func (s *WebhookStore) CreateEndpoint(ctx context.Context, endpoint webhookdelivery.Endpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.pool == nil {
		return ErrWebhookStoreRequired
	}
	if endpoint.CreatedAt.IsZero() {
		endpoint.CreatedAt = s.now().UTC()
	}
	if endpoint.UpdatedAt.IsZero() {
		endpoint.UpdatedAt = endpoint.CreatedAt
	}
	if err := webhookdelivery.ValidateEndpoint(endpoint, webhookdelivery.EndpointPolicy{}); err != nil {
		return ErrWebhookInvalid
	}
	ciphertext, nonce, err := s.encryptWebhookSecret(endpoint.SigningSecret)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(endpoint.SigningSecret)
	db := txpostgres.FromCtx(ctx, s.pool)
	_, err = db.Exec(ctx,
		"insert into webhook_endpoints (id, organization_id, url, event_types, secret_hash, secret_ciphertext, secret_nonce, created_at) values ($1, $2, $3, $4, $5, $6, $7, $8)",
		strings.TrimSpace(endpoint.ID),
		strings.TrimSpace(endpoint.TenantID),
		strings.TrimSpace(endpoint.URL),
		append([]string(nil), endpoint.Events...),
		hash[:],
		ciphertext,
		nonce,
		endpoint.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("create webhook endpoint: %w", err)
	}
	return nil
}

func (s *WebhookStore) ListEndpointsForActor(ctx context.Context, tenantID string) ([]webhookdelivery.Endpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.pool == nil {
		return nil, ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrWebhookInvalid
	}
	db := txpostgres.FromCtx(ctx, s.pool)
	rows, err := db.Query(ctx, "select id, organization_id, url, event_types, disabled_at is not null, created_at from webhook_endpoints where organization_id=$1 order by id", tenantID)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	defer rows.Close()
	var out []webhookdelivery.Endpoint
	for rows.Next() {
		endpoint, err := scanPublicWebhookEndpoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, endpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list webhook endpoint rows: %w", err)
	}
	return out, nil
}

func (s *WebhookStore) ListEndpoints(ctx context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error) {
	if s == nil || s.base == nil {
		return nil, ErrWebhookStoreRequired
	}
	return s.base.ListEndpoints(ctx, tenantID, eventType)
}

func (s *WebhookStore) GetEndpoint(ctx context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error) {
	if s == nil || s.base == nil {
		return webhookdelivery.Endpoint{}, false, ErrWebhookStoreRequired
	}
	return s.base.GetEndpoint(ctx, tenantID, endpointID)
}

func (s *WebhookStore) EnqueueDelivery(ctx context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error {
	if s == nil || s.base == nil {
		return ErrWebhookStoreRequired
	}
	return s.base.EnqueueDelivery(ctx, delivery, job)
}

func (s *WebhookStore) RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error {
	if s == nil || s.base == nil {
		return ErrWebhookStoreRequired
	}
	return s.base.RecordAttempt(ctx, result)
}

func (s *WebhookStore) ListDeliveries(ctx context.Context, tenantID string) ([]webhookdelivery.Delivery, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.pool == nil {
		return nil, ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrWebhookInvalid
	}
	db := txpostgres.FromCtx(ctx, s.pool)
	rows, err := db.Query(ctx, "select d.id, d.organization_id, d.endpoint_id, d.event_id, d.event_type, e.url, d.state, d.attempts, d.next_at, coalesce(d.last_status_code, 0), coalesce(d.last_error, ''), d.created_at, coalesce(d.delivered_at, d.created_at) from webhook_deliveries d join webhook_endpoints e on e.id=d.endpoint_id and e.organization_id=d.organization_id where d.organization_id=$1 order by d.id", tenantID)
	if err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	defer rows.Close()
	var out []webhookdelivery.Delivery
	for rows.Next() {
		delivery, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, delivery)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list webhook delivery rows: %w", err)
	}
	return out, nil
}

func (s *WebhookStore) GetDelivery(ctx context.Context, tenantID, deliveryID string) (webhookdelivery.Delivery, bool, error) {
	if err := ctx.Err(); err != nil {
		return webhookdelivery.Delivery{}, false, err
	}
	if s == nil || s.pool == nil {
		return webhookdelivery.Delivery{}, false, ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	deliveryID = strings.TrimSpace(deliveryID)
	if tenantID == "" || deliveryID == "" {
		return webhookdelivery.Delivery{}, false, ErrWebhookInvalid
	}
	db := txpostgres.FromCtx(ctx, s.pool)
	row := db.QueryRow(ctx, "select d.id, d.organization_id, d.endpoint_id, d.event_id, d.event_type, e.url, d.state, d.attempts, d.next_at, coalesce(d.last_status_code, 0), coalesce(d.last_error, ''), d.created_at, coalesce(d.delivered_at, d.created_at) from webhook_deliveries d join webhook_endpoints e on e.id=d.endpoint_id and e.organization_id=d.organization_id where d.organization_id=$1 and d.id=$2", tenantID, deliveryID)
	delivery, err := scanWebhookDelivery(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhookdelivery.Delivery{}, false, nil
	}
	if err != nil {
		return webhookdelivery.Delivery{}, false, fmt.Errorf("get webhook delivery: %w", err)
	}
	return delivery, true, nil
}

func (s *WebhookStore) ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.pool == nil || s.tx == nil {
		return ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	deliveryID = strings.TrimSpace(deliveryID)
	if tenantID == "" || deliveryID == "" {
		return webhookdelivery.ErrInvalidDelivery
	}
	if nextAt.IsZero() {
		nextAt = s.now().UTC()
	}

	var (
		endpointID string
		eventID    string
		eventType  string
		payload    []byte
	)
	db := txpostgres.FromCtx(ctx, s.pool)
	err := db.QueryRow(ctx, "select endpoint_id, event_id, event_type, payload from webhook_deliveries where organization_id=$1 and id=$2", tenantID, deliveryID).Scan(&endpointID, &eventID, &eventType, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhookdelivery.ErrDeliveryNotFound
	}
	if err != nil {
		return fmt.Errorf("load webhook delivery replay payload: %w", err)
	}
	jobPayload, err := webhookdelivery.EncodeJobPayload(webhookdelivery.JobPayload{
		DeliveryID: deliveryID,
		EndpointID: strings.TrimSpace(endpointID),
		Event: webhookdelivery.Event{
			ID:       strings.TrimSpace(eventID),
			TenantID: tenantID,
			Type:     strings.TrimSpace(eventType),
			Payload:  append([]byte(nil), payload...),
		},
	})
	if err != nil {
		return err
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		db := txpostgres.FromCtx(ctx, s.pool)
		res, err := db.Exec(ctx, "update webhook_deliveries set state='pending', next_at=$3, last_error=null where organization_id=$1 and id=$2", tenantID, deliveryID, nextAt.UTC())
		if err != nil {
			return fmt.Errorf("replay webhook delivery: %w", err)
		}
		if res == nil || res.RowsAffected() == 0 {
			return webhookdelivery.ErrDeliveryNotFound
		}
		_, err = db.Exec(ctx,
			"insert into outbox_events (id, organization_id, event_type, payload, state, next_at, created_at) values ($1, $2, $3, $4, 'pending', $5, $6) on conflict (id) do update set event_type=excluded.event_type, payload=excluded.payload, state='pending', lease_owner=null, lease_expires_at=null, retry_count=0, next_at=excluded.next_at",
			deliveryID,
			tenantID,
			webhookdeliverypostgres.OutboxEventType,
			jobPayload,
			nextAt.UTC(),
			s.now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("requeue webhook delivery: %w", err)
		}
		return nil
	})
}

func (s *WebhookStore) resolveSigningSecret(ctx context.Context, tenantID, endpointID string) ([]byte, bool, error) {
	if s == nil || s.pool == nil {
		return nil, false, ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	endpointID = strings.TrimSpace(endpointID)
	if tenantID == "" || endpointID == "" {
		return nil, false, ErrWebhookInvalid
	}
	var ciphertext, nonce []byte
	db := txpostgres.FromCtx(ctx, s.pool)
	err := db.QueryRow(ctx, "select secret_ciphertext, secret_nonce from webhook_endpoints where organization_id=$1 and id=$2 and disabled_at is null", tenantID, endpointID).Scan(&ciphertext, &nonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load webhook signing secret: %w", err)
	}
	secret, err := s.decryptWebhookSecret(ciphertext, nonce)
	if err != nil {
		return nil, false, err
	}
	return secret, true, nil
}

func (s *WebhookStore) encryptWebhookSecret(secret []byte) ([]byte, []byte, error) {
	if len(secret) == 0 || len(s.secretKey) != 32 {
		return nil, nil, ErrWebhookSecretKeyInvalid
	}
	block, err := aes.NewCipher(s.secretKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, secret, nil), nonce, nil
}

func (s *WebhookStore) decryptWebhookSecret(ciphertext, nonce []byte) ([]byte, error) {
	if len(ciphertext) == 0 || len(nonce) == 0 || len(s.secretKey) != 32 {
		return nil, ErrWebhookSecretKeyInvalid
	}
	block, err := aes.NewCipher(s.secretKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, ErrWebhookSecretKeyInvalid
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func decodeWebhookSecretKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ErrWebhookSecretKeyInvalid
	}
	for _, encoding := range []*base64.Encoding{base64.RawStdEncoding, base64.StdEncoding, base64.RawURLEncoding, base64.URLEncoding} {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) == 32 {
			return append([]byte(nil), decoded...), nil
		}
	}
	if len(value) == 32 {
		return []byte(value), nil
	}
	return nil, ErrWebhookSecretKeyInvalid
}

type webhookScanner interface {
	Scan(dest ...any) error
}

func scanPublicWebhookEndpoint(row webhookScanner) (webhookdelivery.Endpoint, error) {
	var endpoint webhookdelivery.Endpoint
	if err := row.Scan(&endpoint.ID, &endpoint.TenantID, &endpoint.URL, &endpoint.Events, &endpoint.Disabled, &endpoint.CreatedAt); err != nil {
		return webhookdelivery.Endpoint{}, err
	}
	endpoint.ID = strings.TrimSpace(endpoint.ID)
	endpoint.TenantID = strings.TrimSpace(endpoint.TenantID)
	endpoint.URL = strings.TrimSpace(endpoint.URL)
	endpoint.Events = append([]string(nil), endpoint.Events...)
	endpoint.CreatedAt = endpoint.CreatedAt.UTC()
	if endpoint.ID == "" || endpoint.TenantID == "" || endpoint.URL == "" || len(endpoint.Events) == 0 {
		return webhookdelivery.Endpoint{}, ErrWebhookInvalid
	}
	return endpoint, nil
}

func scanWebhookDelivery(row webhookScanner) (webhookdelivery.Delivery, error) {
	var (
		delivery webhookdelivery.Delivery
		state    string
	)
	if err := row.Scan(
		&delivery.ID,
		&delivery.TenantID,
		&delivery.EndpointID,
		&delivery.EventID,
		&delivery.EventType,
		&delivery.URL,
		&state,
		&delivery.Attempt,
		&delivery.NextAt,
		&delivery.LastStatusCode,
		&delivery.LastError,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	); err != nil {
		return webhookdelivery.Delivery{}, err
	}
	delivery.State = webhookdelivery.DeliveryState(strings.TrimSpace(state))
	delivery.NextAt = delivery.NextAt.UTC()
	delivery.CreatedAt = delivery.CreatedAt.UTC()
	delivery.UpdatedAt = delivery.UpdatedAt.UTC()
	if err := webhookdelivery.ValidateDelivery(delivery); err != nil {
		return webhookdelivery.Delivery{}, ErrWebhookInvalid
	}
	return delivery, nil
}
`

const fullPostgresWebhookStoreTestTemplate = `package postgres

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/webhookdeliverypostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/webhookdelivery"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestNewWebhookStoreRequiresSecretKey(t *testing.T) {
	if _, err := NewWebhookStore(&fakeWebhookPool{}, "short"); !errors.Is(err, ErrWebhookSecretKeyInvalid) {
		t.Fatalf("NewWebhookStore() error = %v, want %v", err, ErrWebhookSecretKeyInvalid)
	}
}

func TestWebhookStoreCreateEndpointEncryptsSecret(t *testing.T) {
	conn := &fakeWebhookConn{}
	store, err := NewWebhookStore(&fakeWebhookPool{conn: conn}, strings.Repeat("a", 32))
	if err != nil {
		t.Fatalf("NewWebhookStore() error = %v", err)
	}
	endpoint := webhookdelivery.Endpoint{
		ID:            "whend_1",
		TenantID:      "org_1",
		URL:           "https://example.com/webhooks",
		SigningSecret: []byte("webhook-secret-value"),
		Events:        []string{"widget.created"},
		CreatedAt:     time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := store.CreateEndpoint(context.Background(), endpoint); err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if len(conn.execCalls) != 1 {
		t.Fatalf("Exec calls = %d, want 1", len(conn.execCalls))
	}
	args := conn.execCalls[0].args
	if got, _ := args[0].(string); got != "whend_1" {
		t.Fatalf("endpoint id arg = %#v", args[0])
	}
	for _, arg := range args {
		if bytes.Contains([]byte(strings.TrimSpace(toString(arg))), endpoint.SigningSecret) {
			t.Fatalf("raw webhook secret leaked into exec args: %#v", arg)
		}
	}
}

func TestWebhookStoreListDeliveriesScansTenantRows(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	conn := &fakeWebhookConn{rows: &fakeWebhookRows{values: [][]any{[]any{
		"whdel_1", "org_1", "whend_1", "evt_1", "widget.created", "https://example.com/webhooks", "pending", 0, now, 0, "", now, now,
	}}}}
	store, err := NewWebhookStore(&fakeWebhookPool{conn: conn}, strings.Repeat("a", 32))
	if err != nil {
		t.Fatalf("NewWebhookStore() error = %v", err)
	}
	deliveries, err := store.ListDeliveries(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].ID != "whdel_1" || deliveries[0].URL == "" {
		t.Fatalf("deliveries = %#v", deliveries)
	}
}

func TestWebhookStoreReplayRequeuesOutbox(t *testing.T) {
	now := time.Unix(1_700_000_100, 0).UTC()
	conn := &fakeWebhookConn{row: fakeWebhookRowFunc(func(dest ...any) error {
		*(dest[0].(*string)) = "whend_1"
		*(dest[1].(*string)) = "evt_1"
		*(dest[2].(*string)) = "widget.created"
		*(dest[3].(*[]byte)) = []byte(` + "`" + `{"id":"wgt_1"}` + "`" + `)
		return nil
	})}
	tx := &fakeWebhookTx{rowsAffected: 1}
	conn.tx = tx
	store, err := NewWebhookStore(&fakeWebhookPool{conn: conn}, strings.Repeat("a", 32))
	if err != nil {
		t.Fatalf("NewWebhookStore() error = %v", err)
	}
	store.now = func() time.Time { return now }
	if err := store.ReplayDelivery(context.Background(), "org_1", "whdel_1", now); err != nil {
		t.Fatalf("ReplayDelivery() error = %v", err)
	}
	if len(tx.execCalls) != 2 {
		t.Fatalf("tx exec calls = %d, want 2", len(tx.execCalls))
	}
	if !strings.Contains(tx.execCalls[1].sql, "on conflict (id) do update") || tx.execCalls[1].args[2] != webhookdeliverypostgres.OutboxEventType {
		t.Fatalf("outbox replay call = %#v", tx.execCalls[1])
	}
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

type fakeWebhookPool struct {
	conn *fakeWebhookConn
}

func (p *fakeWebhookPool) Ping(context.Context) error { return nil }
func (p *fakeWebhookPool) Close()                     {}
func (p *fakeWebhookPool) Stat() ports.DatabaseStats  { return nil }

func (p *fakeWebhookPool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	if p.conn == nil {
		p.conn = &fakeWebhookConn{}
	}
	return p.conn, nil
}

type fakeWebhookExecCall struct {
	sql  string
	args []any
}

type fakeWebhookConn struct {
	execCalls    []fakeWebhookExecCall
	row          ports.DatabaseRow
	rows         ports.DatabaseRows
	tx           ports.DatabaseTransaction
	rowsAffected int64
}

func (c *fakeWebhookConn) Query(_ context.Context, _ string, _ ...any) (ports.DatabaseRows, error) {
	if c.rows != nil {
		return c.rows, nil
	}
	return &fakeWebhookRows{}, nil
}

func (c *fakeWebhookConn) QueryRow(_ context.Context, _ string, _ ...any) ports.DatabaseRow {
	if c.row != nil {
		return c.row
	}
	return fakeWebhookRowFunc(func(...any) error { return nil })
}

func (c *fakeWebhookConn) Exec(_ context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
	c.execCalls = append(c.execCalls, fakeWebhookExecCall{sql: sql, args: append([]any(nil), args...)})
	return fakeWebhookResult(c.rowsAffected), nil
}

func (c *fakeWebhookConn) Begin(context.Context) (ports.DatabaseTransaction, error) {
	if c.tx == nil {
		return nil, errors.New("transaction not configured")
	}
	return c.tx, nil
}

func (c *fakeWebhookConn) Release() {}

type fakeWebhookTx struct {
	execCalls    []fakeWebhookExecCall
	rowsAffected int64
}

func (t *fakeWebhookTx) Query(context.Context, string, ...any) (ports.DatabaseRows, error) {
	return &fakeWebhookRows{}, nil
}

func (t *fakeWebhookTx) QueryRow(context.Context, string, ...any) ports.DatabaseRow {
	return fakeWebhookRowFunc(func(...any) error { return nil })
}

func (t *fakeWebhookTx) Exec(_ context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
	t.execCalls = append(t.execCalls, fakeWebhookExecCall{sql: sql, args: append([]any(nil), args...)})
	return fakeWebhookResult(t.rowsAffected), nil
}

func (t *fakeWebhookTx) Commit(context.Context) error   { return nil }
func (t *fakeWebhookTx) Rollback(context.Context) error { return nil }

type fakeWebhookRows struct {
	values [][]any
	index  int
}

func (r *fakeWebhookRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeWebhookRows) Scan(dest ...any) error {
	row := r.values[r.index]
	r.index++
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *[]string:
			*d = append([]string(nil), row[i].([]string)...)
		case *bool:
			*d = row[i].(bool)
		case *int:
			*d = row[i].(int)
		case *time.Time:
			*d = row[i].(time.Time)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func (r *fakeWebhookRows) Close() {}
func (r *fakeWebhookRows) Err() error { return nil }

type fakeWebhookResult int64

func (r fakeWebhookResult) RowsAffected() int64 {
	if r == 0 {
		return 1
	}
	return int64(r)
}

type fakeWebhookRowFunc func(...any) error

func (r fakeWebhookRowFunc) Scan(dest ...any) error { return r(dest...) }
`

const fullObjectStoreS3AdapterTemplate = `package objectstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/objectstores3"
	toolkitobjectstore "github.com/aatuh/api-toolkit/contrib/v2/objectstore"

	"{{ .Module }}/internal/app"
)

const maxS3ObjectBytes = 1024 * 1024

var ErrS3ConfigRequired = errors.New("s3 object store configuration is required")

type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type S3BlobStore struct {
	store toolkitobjectstore.Store
}

func OpenS3BlobStore(cfg S3Config) (*S3BlobStore, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" ||
		strings.TrimSpace(cfg.Region) == "" ||
		strings.TrimSpace(cfg.Bucket) == "" ||
		strings.TrimSpace(cfg.AccessKeyID) == "" ||
		strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, ErrS3ConfigRequired
	}
	store, err := objectstores3.New(objectstores3.Options{
		Endpoint:            cfg.Endpoint,
		Region:              cfg.Region,
		Bucket:              cfg.Bucket,
		AccessKeyID:         cfg.AccessKeyID,
		SecretAccessKey:     cfg.SecretAccessKey,
		MaxObjectSize:       maxS3ObjectBytes,
		AllowedContentTypes: []string{"application/json", "application/pdf", "image/jpeg", "image/png", "text/plain"},
	})
	if err != nil {
		return nil, err
	}
	return &S3BlobStore{store: store}, nil
}

func NewS3BlobStore(store toolkitobjectstore.Store) *S3BlobStore {
	return &S3BlobStore{store: store}
}

func (s *S3BlobStore) PutObject(ctx context.Context, ref app.ObjectRef, data []byte, contentType string) error {
	if s == nil || s.store == nil {
		return ErrS3ConfigRequired
	}
	objectRef, err := toObjectRef(ref)
	if err != nil {
		return err
	}
	return s.store.Put(ctx, objectRef, bytes.NewReader(data), toolkitobjectstore.PutOptions{
		Size:        int64(len(data)),
		ContentType: strings.TrimSpace(contentType),
		Metadata:    map[string]string{"tenant_id": strings.TrimSpace(ref.TenantID)},
	})
}

func (s *S3BlobStore) GetObject(ctx context.Context, ref app.ObjectRef) (app.ObjectBlob, bool, error) {
	if s == nil || s.store == nil {
		return app.ObjectBlob{}, false, ErrS3ConfigRequired
	}
	objectRef, err := toObjectRef(ref)
	if err != nil {
		return app.ObjectBlob{}, false, err
	}
	result, err := s.store.Get(ctx, objectRef)
	if errors.Is(err, toolkitobjectstore.ErrObjectNotFound) {
		return app.ObjectBlob{}, false, nil
	}
	if err != nil {
		return app.ObjectBlob{}, false, err
	}
	defer result.Body.Close()
	data, err := io.ReadAll(io.LimitReader(result.Body, maxS3ObjectBytes+1))
	if err != nil {
		return app.ObjectBlob{}, false, err
	}
	if len(data) > maxS3ObjectBytes {
		return app.ObjectBlob{}, false, toolkitobjectstore.ErrObjectTooLarge
	}
	return app.ObjectBlob{ContentType: result.ContentType, Size: int64(len(data)), Data: data}, true, nil
}

func (s *S3BlobStore) DeleteObject(ctx context.Context, ref app.ObjectRef) (bool, error) {
	if s == nil || s.store == nil {
		return false, ErrS3ConfigRequired
	}
	objectRef, err := toObjectRef(ref)
	if err != nil {
		return false, err
	}
	if err := s.store.Delete(ctx, objectRef); errors.Is(err, toolkitobjectstore.ErrObjectNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func toObjectRef(ref app.ObjectRef) (toolkitobjectstore.Ref, error) {
	tenantID := strings.TrimSpace(ref.TenantID)
	key := strings.TrimSpace(ref.Key)
	if tenantID == "" || key == "" {
		return toolkitobjectstore.Ref{}, toolkitobjectstore.ErrInvalidRef
	}
	objectRef := toolkitobjectstore.Ref{Key: fmt.Sprintf("%s/%s", tenantID, key)}
	if err := toolkitobjectstore.ValidateRef(toolkitobjectstore.Ref{Bucket: "api-objects", Key: objectRef.Key}); err != nil {
		return toolkitobjectstore.Ref{}, err
	}
	return objectRef, nil
}
`

const fullObjectStoreS3AdapterTestTemplate = `package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	toolkitobjectstore "github.com/aatuh/api-toolkit/contrib/v2/objectstore"

	"{{ .Module }}/internal/app"
)

func TestOpenS3BlobStoreRequiresConfig(t *testing.T) {
	if _, err := OpenS3BlobStore(S3Config{}); !errors.Is(err, ErrS3ConfigRequired) {
		t.Fatalf("OpenS3BlobStore() error = %v, want %v", err, ErrS3ConfigRequired)
	}
}

func TestS3BlobStorePutGetDelete(t *testing.T) {
	store := &recordingObjectStore{}
	blobs := NewS3BlobStore(store)
	ref := app.ObjectRef{TenantID: "org_1", Key: "readme.txt"}
	if err := blobs.PutObject(context.Background(), ref, []byte("hello"), "text/plain"); err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if store.ref.Key != "org_1/readme.txt" || store.contentType != "text/plain" || string(store.data) != "hello" {
		t.Fatalf("put ref=%#v contentType=%q data=%q", store.ref, store.contentType, store.data)
	}
	got, ok, err := blobs.GetObject(context.Background(), ref)
	if err != nil || !ok || got.ContentType != "text/plain" || string(got.Data) != "hello" {
		t.Fatalf("GetObject() blob=%#v ok=%v err=%v", got, ok, err)
	}
	ok, err = blobs.DeleteObject(context.Background(), ref)
	if err != nil || !ok || !store.deleted {
		t.Fatalf("DeleteObject() ok=%v deleted=%v err=%v", ok, store.deleted, err)
	}
}

func TestS3BlobStoreRejectsUnsafeRef(t *testing.T) {
	blobs := NewS3BlobStore(&recordingObjectStore{})
	if err := blobs.PutObject(context.Background(), app.ObjectRef{TenantID: "org_1", Key: "../secret"}, []byte("hello"), "text/plain"); !errors.Is(err, toolkitobjectstore.ErrInvalidRef) {
		t.Fatalf("PutObject() error = %v, want %v", err, toolkitobjectstore.ErrInvalidRef)
	}
}

type recordingObjectStore struct {
	ref         toolkitobjectstore.Ref
	contentType string
	data        []byte
	deleted     bool
}

func (s *recordingObjectStore) Put(_ context.Context, ref toolkitobjectstore.Ref, body io.Reader, opts toolkitobjectstore.PutOptions) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.ref = ref
	s.contentType = opts.ContentType
	s.data = data
	return nil
}

func (s *recordingObjectStore) Get(_ context.Context, ref toolkitobjectstore.Ref) (toolkitobjectstore.GetResult, error) {
	if ref != s.ref {
		return toolkitobjectstore.GetResult{}, toolkitobjectstore.ErrObjectNotFound
	}
	return toolkitobjectstore.GetResult{
		Body:        io.NopCloser(strings.NewReader(string(s.data))),
		ContentType: s.contentType,
		Size:        int64(len(s.data)),
	}, nil
}

func (s *recordingObjectStore) Delete(_ context.Context, ref toolkitobjectstore.Ref) error {
	if ref != s.ref {
		return toolkitobjectstore.ErrObjectNotFound
	}
	s.deleted = true
	return nil
}
`

const fullRedisCacheAdapterTemplate = `package redis

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/idempotencyredis"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/cacheredis"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/ratelimitredis"
	"github.com/aatuh/api-toolkit/contrib/v2/cache"
	"github.com/aatuh/api-toolkit/v2/ports"
)

var (
	ErrRedisAddrRequired  = errors.New("REDIS_ADDR is required")
	ErrRedisClientMissing = errors.New("redis cache client is required")
)

type Cache struct {
	Store  cache.Store
	client redis.UniversalClient
}

type Idempotency struct {
	Store  ports.IdempotencyStore
	client redis.UniversalClient
}

type RateLimiter struct {
	Limiter ports.RateLimiter
	client  redis.UniversalClient
}

func OpenCache(ctx context.Context, addr string) (*Cache, error) {
	addrs := parseRedisAddrs(addr)
	if len(addrs) == 0 {
		return nil, ErrRedisAddrRequired
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Cache{
		Store:  cacheredis.New(client, cacheredis.Options{KeyPrefix: "api:", DefaultTTL: 5 * time.Minute}),
		client: client,
	}, nil
}

func OpenRateLimiter(ctx context.Context, addr, keyPrefix string, capacity, refillRate float64) (*RateLimiter, error) {
	addrs := parseRedisAddrs(addr)
	if len(addrs) == 0 {
		return nil, ErrRedisAddrRequired
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RateLimiter{
		Limiter: ratelimitredis.New(client, ratelimitredis.Options{Capacity: capacity, RefillRate: refillRate, KeyPrefix: strings.TrimSpace(keyPrefix)}),
		client:  client,
	}, nil
}

func OpenIdempotencyStore(ctx context.Context, addr, keyPrefix string) (*Idempotency, error) {
	addrs := parseRedisAddrs(addr)
	if len(addrs) == 0 {
		return nil, ErrRedisAddrRequired
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: addrs})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Idempotency{
		Store:  idempotencyredis.New(client, idempotencyredis.Options{KeyPrefix: strings.TrimSpace(keyPrefix)}),
		client: client,
	}, nil
}

func (c *Cache) Check(ctx context.Context) error {
	if c == nil || c.client == nil {
		return ErrRedisClientMissing
	}
	return c.client.Ping(ctx).Err()
}

func (c *Cache) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (r *RateLimiter) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

func (i *Idempotency) Close() error {
	if i == nil || i.client == nil {
		return nil
	}
	return i.client.Close()
}

func parseRedisAddrs(addr string) []string {
	parts := strings.FieldsFunc(addr, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

`

const fullRedisCacheAdapterTestTemplate = `package redis

import (
	"context"
	"errors"
	"testing"
)

func TestParseRedisAddrsRejectsEmptyAndSplits(t *testing.T) {
	if got := parseRedisAddrs(" "); len(got) != 0 {
		t.Fatalf("empty parseRedisAddrs() = %#v", got)
	}
	got := parseRedisAddrs("localhost:6379, redis-2:6379 ;redis-3:6379")
	if len(got) != 3 || got[0] != "localhost:6379" || got[2] != "redis-3:6379" {
		t.Fatalf("parseRedisAddrs() = %#v", got)
	}
}

func TestCacheCheckRequiresClient(t *testing.T) {
	if err := (*Cache)(nil).Check(context.Background()); !errors.Is(err, ErrRedisClientMissing) {
		t.Fatalf("nil Check() error = %v, want %v", err, ErrRedisClientMissing)
	}
	if err := (&Cache{}).Check(context.Background()); !errors.Is(err, ErrRedisClientMissing) {
		t.Fatalf("missing client Check() error = %v, want %v", err, ErrRedisClientMissing)
	}
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
	registry := specs.NewRegistryWithOptions(specs.Info{
		Title:       "Full SaaS API",
		Description: "Generated api-toolkit full SaaS/API profile.",
		Version:     "dev",
	}, specs.RegistryOptions{
		OpenAPIVersion: specs.OpenAPIVersion31,
	})
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	registry.RegisterSecurityScheme("BearerAuth", specs.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"})
	registry.SetSecurity([]specs.SecurityRequirement{ {Name: "BearerAuth"} })
{{ else }}	registry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	registry.SetSecurity([]specs.SecurityRequirement{ {Name: "ApiKeyAuth"} })
{{ end }}
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
	registry.RegisterSchema("WidgetImportItem", map[string]any{
		"type":                 "object",
		"required":             []string{"name"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		},
	})
	registry.RegisterSchema("WidgetImportRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"items"},
		"additionalProperties": false,
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WidgetImportItem"}, "minItems": 1, "maxItems": 100},
		},
	})
	registry.RegisterSchema("WidgetImportResult", map[string]any{
		"type":     "object",
		"required": []string{"created", "widget_ids"},
		"properties": map[string]any{
			"created":    map[string]any{"type": "integer", "minimum": 0},
			"widget_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	})
	registry.RegisterSchema("OperationAccepted", map[string]any{
		"type":     "object",
		"required": []string{"state"},
		"properties": map[string]any{
			"id":       map[string]any{"type": "string"},
			"state":    map[string]any{"type": "string", "enum": []string{"pending"}},
			"location": map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("WidgetImportOperation", map[string]any{
		"type":     "object",
		"required": []string{"id", "state"},
		"properties": map[string]any{
			"id":      map[string]any{"type": "string"},
			"state":   map[string]any{"type": "string", "enum": []string{"pending", "running", "succeeded", "failed", "canceled"}},
			"result":  map[string]any{"$ref": "#/components/schemas/WidgetImportResult", "nullable": true},
			"problem": map[string]any{"$ref": "#/components/schemas/Problem", "nullable": true},
		},
	})
	registry.RegisterSchema("Organization", map[string]any{
		"type":     "object",
		"required": []string{"id", "name", "created_at", "updated_at"},
		"properties": map[string]any{
			"id":         map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
			"updated_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("OrganizationCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"name"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 160},
		},
	})
	registry.RegisterSchema("OrganizationList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Organization"}},
		},
	})
	registry.RegisterSchema("Membership", map[string]any{
		"type":     "object",
		"required": []string{"organization_id", "user_id", "role", "created_at"},
		"properties": map[string]any{
			"organization_id": map[string]any{"type": "string"},
			"user_id":         map[string]any{"type": "string"},
			"role":            map[string]any{"type": "string", "enum": []string{"owner", "admin", "member", "viewer"}},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("MembershipList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Membership"}},
		},
	})
	registry.RegisterSchema("Invitation", map[string]any{
		"type":     "object",
		"required": []string{"id", "organization_id", "email", "role", "token_prefix", "expires_at", "created_at"},
		"properties": map[string]any{
			"id":              map[string]any{"type": "string"},
			"organization_id": map[string]any{"type": "string"},
			"email":           map[string]any{"type": "string", "format": "email"},
			"role":            map[string]any{"type": "string", "enum": []string{"owner", "admin", "member", "viewer"}},
			"token_prefix":    map[string]any{"type": "string"},
			"expires_at":      map[string]any{"type": "string", "format": "date-time"},
			"accepted_at":     map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("InvitationCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"email", "role"},
		"additionalProperties": false,
		"properties": map[string]any{
			"email": map[string]any{"type": "string", "format": "email"},
			"role":  map[string]any{"type": "string", "enum": []string{"admin", "member", "viewer"}},
		},
	})
	registry.RegisterSchema("InvitationCreated", map[string]any{
		"type":     "object",
		"required": []string{"invitation", "token"},
		"properties": map[string]any{
			"invitation": map[string]any{"$ref": "#/components/schemas/Invitation"},
			"token":      map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("InvitationAcceptRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"token"},
		"additionalProperties": false,
		"properties": map[string]any{
			"token": map[string]any{"type": "string", "minLength": 1},
		},
	})
	registry.RegisterSchema("APIKey", map[string]any{
		"type":     "object",
		"required": []string{"id", "organization_id", "name", "prefix", "scopes", "created_at"},
		"properties": map[string]any{
			"id":              map[string]any{"type": "string"},
			"organization_id": map[string]any{"type": "string"},
			"name":            map[string]any{"type": "string"},
			"prefix":          map[string]any{"type": "string"},
			"scopes":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"expires_at":      map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"last_used_at":    map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"revoked_at":      map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("APIKeyCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"name", "scopes"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name":       map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
			"scopes":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
			"expires_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("APIKeyCreated", map[string]any{
		"type":     "object",
		"required": []string{"api_key", "secret"},
		"properties": map[string]any{
			"api_key": map[string]any{"$ref": "#/components/schemas/APIKey"},
			"secret":  map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("APIKeyList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/APIKey"}},
		},
	})
	registry.RegisterSchema("WebhookEventCatalog", map[string]any{
		"type":     "object",
		"required": []string{"event_types"},
		"properties": map[string]any{
			"event_types": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{
				"widget.created",
				"widget.updated",
				"widget.deleted",
				"widget.import.completed",
				// api-toolkit:openapi-webhook-event-types
			}}},
		},
	})
	registry.RegisterSchema("Object", map[string]any{
		"type":     "object",
		"required": []string{"tenant_id", "key", "content_type", "size", "created_at", "updated_at"},
		"properties": map[string]any{
			"tenant_id":    map[string]any{"type": "string"},
			"key":          map[string]any{"type": "string"},
			"content_type": map[string]any{"type": "string"},
			"size":         map[string]any{"type": "integer", "minimum": 0},
			"created_at":   map[string]any{"type": "string", "format": "date-time"},
			"updated_at":   map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("ObjectPutRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"key", "content_type", "content_base64"},
		"additionalProperties": false,
		"properties": map[string]any{
			"key":            map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
			"content_type":   map[string]any{"type": "string", "enum": []string{"application/json", "application/pdf", "image/jpeg", "image/png", "text/plain"}},
			"content_base64": map[string]any{"type": "string", "format": "byte"},
		},
	})
	registry.RegisterSchema("ObjectRead", map[string]any{
		"type":     "object",
		"required": []string{"object", "content_base64"},
		"properties": map[string]any{
			"object":         map[string]any{"$ref": "#/components/schemas/Object"},
			"content_base64": map[string]any{"type": "string", "format": "byte"},
		},
	})
	registry.RegisterSchema("ObjectList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Object"}},
		},
	})
	registry.RegisterSchema("WebhookEndpoint", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "url", "events", "created_at"},
		"properties": map[string]any{
			"id":         map[string]any{"type": "string"},
			"tenant_id":  map[string]any{"type": "string"},
			"url":        map[string]any{"type": "string", "format": "uri"},
			"events":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"disabled":   map[string]any{"type": "boolean"},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
			"updated_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("WebhookEndpointCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"url", "events"},
		"additionalProperties": false,
		"properties": map[string]any{
			"url":    map[string]any{"type": "string", "format": "uri"},
			"events": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
		},
	})
	registry.RegisterSchema("WebhookEndpointCreated", map[string]any{
		"type":     "object",
		"required": []string{"endpoint", "secret"},
		"properties": map[string]any{
			"endpoint": map[string]any{"$ref": "#/components/schemas/WebhookEndpoint"},
			"secret":   map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("WebhookEndpointList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WebhookEndpoint"}},
		},
	})
	registry.RegisterSchema("WebhookDelivery", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "endpoint_id", "event_id", "event_type", "url", "state", "attempt", "next_at", "created_at", "updated_at"},
		"properties": map[string]any{
			"id":               map[string]any{"type": "string"},
			"tenant_id":        map[string]any{"type": "string"},
			"endpoint_id":      map[string]any{"type": "string"},
			"event_id":         map[string]any{"type": "string"},
			"event_type":       map[string]any{"type": "string"},
			"url":              map[string]any{"type": "string", "format": "uri"},
			"state":            map[string]any{"type": "string", "enum": []string{"pending", "leased", "succeeded", "failed", "dead_letter"}},
			"attempt":          map[string]any{"type": "integer", "minimum": 0},
			"next_at":          map[string]any{"type": "string", "format": "date-time"},
			"last_status_code": map[string]any{"type": "integer", "nullable": true},
			"last_error":       map[string]any{"type": "string", "nullable": true},
			"created_at":       map[string]any{"type": "string", "format": "date-time"},
			"updated_at":       map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("WebhookDeliveryList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WebhookDelivery"}},
		},
	})
	registry.RegisterSchema("WebhookReplayRequest", map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           map[string]any{},
	})
	// api-toolkit:openapi-schemas
}

func operations() []specs.Operation {
	auth := func(scopes ...string) []specs.SecurityRequirement {
		return []specs.SecurityRequirement{ {Name: "{{ .AuthSchemeName }}", Scopes: scopes} }
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
	widgetImportBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetImportRequest"},
		},
	}
	operationAcceptedResponse := specs.Response{
		Description: "Operation accepted",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/OperationAccepted"},
		},
	}
	operationResponse := specs.Response{
		Description: "Operation",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetImportOperation"},
		},
	}
	organizationCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/OrganizationCreateRequest"},
		},
	}
	invitationCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationCreateRequest"},
		},
	}
	invitationAcceptBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationAcceptRequest"},
		},
	}
	organizationResponse := specs.Response{
		Description: "Organization",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Organization"},
		},
	}
	membershipResponse := specs.Response{
		Description: "Membership",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Membership"},
		},
	}
	invitationCreatedResponse := specs.Response{
		Description: "Invitation created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationCreated"},
		},
	}
	apiKeyCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/APIKeyCreateRequest"},
		},
	}
	apiKeyCreatedResponse := specs.Response{
		Description: "API key created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/APIKeyCreated"},
		},
	}
	webhookEventCatalogResponse := specs.Response{
		Description: "Webhook event catalog",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEventCatalog"},
		},
	}
	webhookEndpointCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointCreateRequest"},
		},
	}
	webhookEndpointCreatedResponse := specs.Response{
		Description: "Webhook endpoint created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointCreated"},
		},
	}
	webhookEndpointListResponse := specs.Response{
		Description: "Webhook endpoint list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointList"},
		},
	}
	webhookDeliveryListResponse := specs.Response{
		Description: "Webhook delivery list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookDeliveryList"},
		},
	}
	webhookDeliveryResponse := specs.Response{
		Description: "Webhook delivery",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookDelivery"},
		},
	}
	webhookReplayBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookReplayRequest"},
		},
	}
	objectPutBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectPutRequest"},
		},
	}
	objectResponse := specs.Response{
		Description: "Object",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Object"},
		},
	}
	objectReadResponse := specs.Response{
		Description: "Object content",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectRead"},
		},
	}
	objectListResponse := specs.Response{
		Description: "Object list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectList"},
		},
	}
	// api-toolkit:openapi-operation-variables
	return []specs.Operation{
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getLiveness",
			Method:      http.MethodGet,
			Path:        "/livez",
			Summary:     "Liveness",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Live"}},
		}),
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
			OperationID: "listOrganizations",
			Method:      http.MethodGet,
			Path:        "/organizations",
			Summary:     "List organizations",
			Security:    auth("organizations:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Organization list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/OrganizationList"},
					},
				},
			},
		}, routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganization",
			Method:      http.MethodPost,
			Path:        "/organizations",
			Summary:     "Create organization",
			Parameters: []specs.Parameter{
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("organizations:write"),
			RequestBody: organizationCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: organizationResponse},
		}, routepolicy.WithTenantRequired("actor"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationMembers",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/members",
			Summary:     "List organization members",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security: auth("members:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Membership list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/MembershipList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationInvitation",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/invitations",
			Summary:     "Create organization invitation",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("invitations:write"),
			RequestBody: invitationCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: invitationCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationAPIKeys",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/api-keys",
			Summary:     "List organization API keys",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security: auth("api-keys:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "API key list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/APIKeyList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationAPIKey",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/api-keys",
			Summary:     "Create organization API key",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("api-keys:write"),
			RequestBody: apiKeyCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: apiKeyCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "revokeOrganizationAPIKey",
			Method:      http.MethodDelete,
			Path:        "/organizations/{organization_id}/api-keys/{api_key_id}",
			Summary:     "Revoke organization API key",
			Parameters: []specs.Parameter{
				{Name: "api_key_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("api-keys:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Revoked"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listWebhookEvents",
			Method:      http.MethodGet,
			Path:        "/webhook-events",
			Summary:     "List webhook event types",
			Security:    auth("webhooks:read"),
			Responses:   map[int]specs.Response{http.StatusOK: webhookEventCatalogResponse},
		}, routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationWebhookEndpoints",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/webhook-endpoints",
			Summary:     "List organization webhook endpoints",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("webhooks:read"),
			Responses: map[int]specs.Response{http.StatusOK: webhookEndpointListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationWebhookEndpoint",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/webhook-endpoints",
			Summary:     "Create organization webhook endpoint",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("webhooks:write"),
			RequestBody: webhookEndpointCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: webhookEndpointCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationWebhookDeliveries",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/webhook-deliveries",
			Summary:     "List organization webhook deliveries",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("webhooks:read"),
			Responses: map[int]specs.Response{http.StatusOK: webhookDeliveryListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "replayOrganizationWebhookDelivery",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/webhook-deliveries/{delivery_id}/replay",
			Summary:     "Replay organization webhook delivery",
			Parameters: []specs.Parameter{
				{Name: "delivery_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("webhooks:write"),
			RequestBody: webhookReplayBody,
			Responses:   map[int]specs.Response{http.StatusAccepted: webhookDeliveryResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationObjects",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/objects",
			Summary:     "List organization objects",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:read"),
			Responses: map[int]specs.Response{http.StatusOK: objectListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "putOrganizationObject",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/objects",
			Summary:     "Put organization object",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("objects:write"),
			RequestBody: objectPutBody,
			Responses:   map[int]specs.Response{http.StatusCreated: objectResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOrganizationObject",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/objects/{object_key}",
			Summary:     "Get organization object",
			Parameters: []specs.Parameter{
				{Name: "object_key", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:read"),
			Responses: map[int]specs.Response{http.StatusOK: objectReadResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "deleteOrganizationObject",
			Method:      http.MethodDelete,
			Path:        "/organizations/{organization_id}/objects/{object_key}",
			Summary:     "Delete organization object",
			Parameters: []specs.Parameter{
				{Name: "object_key", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Deleted"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "acceptInvitation",
			Method:      http.MethodPost,
			Path:        "/invitations/{id}/accept",
			Summary:     "Accept invitation",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("invitations:accept"),
			RequestBody: invitationAcceptBody,
			Responses:   map[int]specs.Response{http.StatusOK: membershipResponse},
		}, routepolicy.WithTenantRequired("invitation"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOperation",
			Method:      http.MethodGet,
			Path:        "/operations/{id}",
			Summary:     "Get operation",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("operations:read"),
			Responses: map[int]specs.Response{http.StatusOK: operationResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
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
			OperationID: "createWidgetImport",
			Method:      http.MethodPost,
			Path:        "/widgets/imports",
			Summary:     "Create widget import",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: widgetImportBody,
			Responses:   map[int]specs.Response{http.StatusAccepted: operationAcceptedResponse},
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
		// api-toolkit:openapi-operations
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
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/idempotency"
	"github.com/aatuh/api-toolkit/contrib/v2/audit"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	openapimw "github.com/aatuh/api-toolkit/contrib/v2/middleware/openapi"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
{{ if eq .AuthMode "clerk" }}	clerkauth "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/clerk"
{{ else if eq .AuthMode "oidc" }}	oidcauth "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/oidc"
{{ else if eq .AuthMode "jwt" }}	jwtauth "github.com/aatuh/api-toolkit/v2/middleware/auth/jwt"
{{ end }}
	"github.com/aatuh/api-toolkit/v2/endpoints/health"
	"github.com/aatuh/api-toolkit/v2/httpx"
	corepprof "github.com/aatuh/api-toolkit/v2/endpoints/pprof"
	idempotencymw "github.com/aatuh/api-toolkit/v2/middleware/idempotency"
	ratelimitmw "github.com/aatuh/api-toolkit/v2/middleware/ratelimit"
	apitkops "github.com/aatuh/api-toolkit/v2/operations"
	"github.com/aatuh/api-toolkit/v2/ports"

	"{{ .Module }}/internal/app"
	"{{ .Module }}/internal/domain"
)

type Config struct {
	Addr         string
	AdminAddr    string
	APIKey       string
	AdminKey     string
	DatabaseURL  string
	RedisAddr    string
	CacheStore   string
	RateLimitStore     string
	RateLimitKeyPrefix string
	IdempotencyStore     string
	IdempotencyKeyPrefix string
	APIKeyPepper string
	WebhookSecretKey string
	ObjectStore       string
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	OpenAPIRequestValidation  bool
	OpenAPIResponseValidation bool
	AsyncWorkerEnabled        bool
{{ if eq .AuthMode "jwt" }}	JWTJWKSURL   string
	JWTIssuer    string
	JWTAudience  string
{{ else if eq .AuthMode "clerk" }}	ClerkJWKSURL  string
	ClerkIssuer   string
	ClerkAudience string
{{ else if eq .AuthMode "oidc" }}	OIDCIssuer       string
	OIDCAudience     string
	OIDCJWKSURL      string
	OIDCDiscoveryURL string
	OIDCTenantClaim  string
	OIDCScopeClaim   string
{{ end }}
}

func ConfigFromEnv() (Config, error) {
	cacheStore := envDefault("CACHE_STORE", "memory")
	if strings.EqualFold(os.Getenv("ENV"), "production") && strings.TrimSpace(os.Getenv("CACHE_STORE")) == "" {
		cacheStore = "redis"
	}
	rateLimitStore := envDefault("RATE_LIMIT_STORE", "memory")
	if strings.EqualFold(os.Getenv("ENV"), "production") && strings.TrimSpace(os.Getenv("RATE_LIMIT_STORE")) == "" {
		rateLimitStore = "redis"
	}
	idempotencyStore := envDefault("IDEMPOTENCY_STORE", "memory")
	if strings.EqualFold(os.Getenv("ENV"), "production") && strings.TrimSpace(os.Getenv("IDEMPOTENCY_STORE")) == "" {
		idempotencyStore = "redis"
	}
	cfg := Config{
		Addr:         envDefault("API_ADDR", ":8080"),
		AdminAddr:    strings.TrimSpace(os.Getenv("ADMIN_ADDR")),
		APIKey:       envDefault("API_KEY", "local-dev-key"),
		AdminKey:     envDefault("ADMIN_KEY", "local-admin-key"),
		DatabaseURL:  strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RedisAddr:    strings.TrimSpace(os.Getenv("REDIS_ADDR")),
		CacheStore:   strings.ToLower(strings.TrimSpace(cacheStore)),
		RateLimitStore:     strings.ToLower(strings.TrimSpace(rateLimitStore)),
		RateLimitKeyPrefix: envDefault("RATE_LIMIT_KEY_PREFIX", "ratelimit:"),
		IdempotencyStore: strings.ToLower(strings.TrimSpace(idempotencyStore)),
		IdempotencyKeyPrefix: envDefault("IDEMPOTENCY_KEY_PREFIX", "idempotency:"),
		APIKeyPepper: strings.TrimSpace(os.Getenv("API_KEY_PEPPER")),
		WebhookSecretKey: strings.TrimSpace(os.Getenv("WEBHOOK_SECRET_KEY")),
		ObjectStore:       strings.ToLower(envDefault("OBJECT_STORE", "memory")),
		S3Endpoint:        strings.TrimSpace(os.Getenv("S3_ENDPOINT")),
		S3Region:          envDefault("S3_REGION", "us-east-1"),
		S3Bucket:          strings.TrimSpace(os.Getenv("S3_BUCKET")),
		S3AccessKeyID:     strings.TrimSpace(os.Getenv("S3_ACCESS_KEY_ID")),
		S3SecretAccessKey: strings.TrimSpace(os.Getenv("S3_SECRET_ACCESS_KEY")),
		OpenAPIRequestValidation: envBoolDefault("OPENAPI_REQUEST_VALIDATION", true),
		OpenAPIResponseValidation: envBoolDefault("OPENAPI_RESPONSE_VALIDATION", defaultOpenAPIResponseValidation()),
		AsyncWorkerEnabled:        envBoolDefault("ASYNC_WORKER_ENABLED", true),
{{ if eq .AuthMode "jwt" }}		JWTJWKSURL:   strings.TrimSpace(os.Getenv("JWT_JWKS_URL")),
		JWTIssuer:    strings.TrimSpace(os.Getenv("JWT_ISSUER")),
		JWTAudience:  envDefault("JWT_AUDIENCE", "saas-api-full"),
{{ else if eq .AuthMode "clerk" }}		ClerkJWKSURL:  strings.TrimSpace(os.Getenv("CLERK_JWKS_URL")),
		ClerkIssuer:   strings.TrimSpace(os.Getenv("CLERK_ISSUER")),
		ClerkAudience: envDefault("CLERK_AUDIENCE", "saas-api-full"),
{{ else if eq .AuthMode "oidc" }}		OIDCIssuer:       strings.TrimSpace(os.Getenv("OIDC_ISSUER")),
		OIDCAudience:     strings.TrimSpace(os.Getenv("OIDC_AUDIENCE")),
		OIDCJWKSURL:      strings.TrimSpace(os.Getenv("OIDC_JWKS_URL")),
		OIDCDiscoveryURL: strings.TrimSpace(os.Getenv("OIDC_DISCOVERY_URL")),
		OIDCTenantClaim:  envDefault("OIDC_TENANT_CLAIM", "tenant_id"),
		OIDCScopeClaim:   envDefault("OIDC_SCOPE_CLAIM", "scope"),
{{ end }}
	}
	if cfg.CacheStore != "memory" && cfg.CacheStore != "redis" {
		return Config{}, errors.New("CACHE_STORE must be memory or redis")
	}
	if cfg.RateLimitStore != "memory" && cfg.RateLimitStore != "redis" {
		return Config{}, errors.New("RATE_LIMIT_STORE must be memory or redis")
	}
	if cfg.IdempotencyStore != "memory" && cfg.IdempotencyStore != "redis" {
		return Config{}, errors.New("IDEMPOTENCY_STORE must be memory or redis")
	}
	if cfg.ObjectStore != "memory" && cfg.ObjectStore != "s3" {
		return Config{}, errors.New("OBJECT_STORE must be memory or s3")
	}
	if cfg.ObjectStore == "s3" && (cfg.S3Endpoint == "" || cfg.S3Bucket == "" || cfg.S3AccessKeyID == "" || cfg.S3SecretAccessKey == "") {
		return Config{}, errors.New("S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY_ID, and S3_SECRET_ACCESS_KEY are required when OBJECT_STORE=s3")
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
		if cfg.WebhookSecretKey == "" {
			missing = append(missing, "WEBHOOK_SECRET_KEY")
		}
		if cfg.CacheStore != "redis" {
			missing = append(missing, "CACHE_STORE=redis")
		}
		if cfg.RateLimitStore != "redis" {
			missing = append(missing, "RATE_LIMIT_STORE=redis")
		}
		if cfg.IdempotencyStore != "redis" {
			missing = append(missing, "IDEMPOTENCY_STORE=redis")
		}
{{ if eq .AuthMode "jwt" }}		if cfg.JWTJWKSURL == "" {
			missing = append(missing, "JWT_JWKS_URL")
		}
		if cfg.JWTIssuer == "" {
			missing = append(missing, "JWT_ISSUER")
		}
		if cfg.JWTAudience == "" {
			missing = append(missing, "JWT_AUDIENCE")
		}
{{ else if eq .AuthMode "clerk" }}		if cfg.ClerkJWKSURL == "" {
			missing = append(missing, "CLERK_JWKS_URL")
		}
		if cfg.ClerkIssuer == "" {
			missing = append(missing, "CLERK_ISSUER")
		}
		if cfg.ClerkAudience == "" {
			missing = append(missing, "CLERK_AUDIENCE")
		}
{{ else if eq .AuthMode "oidc" }}		if cfg.OIDCIssuer == "" {
			missing = append(missing, "OIDC_ISSUER")
		}
		if cfg.OIDCAudience == "" {
			missing = append(missing, "OIDC_AUDIENCE")
		}
		if cfg.OIDCJWKSURL == "" && cfg.OIDCDiscoveryURL == "" {
			missing = append(missing, "OIDC_JWKS_URL or OIDC_DISCOVERY_URL")
		}
{{ else }}		if cfg.APIKey == "" || cfg.APIKey == "local-dev-key" {
			missing = append(missing, "API_KEY")
		}
{{ end }}
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

func envBoolDefault(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch value {
	case "":
		return fallback
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func defaultOpenAPIResponseValidation() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ENV"))) {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

type RouterConfig struct {
	Widgets  *app.WidgetService
	Tenancy  *app.TenancyService
	APIKeys  *app.APIKeyService
	Async    *app.AsyncService
	Audit    *app.AuditService
	Webhooks *app.WebhookService
	Objects  *app.ObjectService
	Cache    *app.CacheService
	// api-toolkit:router-config-fields
	Metrics  *metricsmw.Middleware
	MetricsHandler http.Handler
	OpenAPIValidation *openapimw.Middleware
	RateLimit *ratelimitmw.Middleware
	Idempotency *idempotencymw.Middleware
	Readiness HealthChecker
	APIKey   string
	AdminKey string
{{ if eq .AuthMode "jwt" }}	JWT      *jwtauth.Middleware
{{ else if eq .AuthMode "clerk" }}	Clerk    *clerkauth.Middleware
{{ else if eq .AuthMode "oidc" }}	OIDC     *oidcauth.Middleware
{{ end }}
}

type HealthChecker interface {
	Check(context.Context) error
}

type HealthCheckFunc func(context.Context) error

func (f HealthCheckFunc) Check(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

func CombineHealthChecks(checkers ...HealthChecker) HealthChecker {
	return HealthCheckFunc(func(ctx context.Context) error {
		for _, checker := range checkers {
			if checker == nil {
				continue
			}
			if err := checker.Check(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

type apiKeyPrincipal struct {
	Key domain.APIKey
}

type apiKeyPrincipalContextKey struct{}

func withAPIKeyPrincipal(ctx context.Context, key domain.APIKey) context.Context {
	return context.WithValue(ctx, apiKeyPrincipalContextKey{}, apiKeyPrincipal{Key: key})
}

func apiKeyPrincipalFromContext(ctx context.Context) (apiKeyPrincipal, bool) {
	if ctx == nil {
		return apiKeyPrincipal{}, false
	}
	principal, ok := ctx.Value(apiKeyPrincipalContextKey{}).(apiKeyPrincipal)
	if !ok || strings.TrimSpace(principal.Key.ID) == "" {
		return apiKeyPrincipal{}, false
	}
	return principal, true
}

func (p apiKeyPrincipal) ActorID() string {
	return strings.TrimSpace(p.Key.ID)
}

func (p apiKeyPrincipal) TenantID() string {
	return strings.TrimSpace(p.Key.OrganizationID)
}

func (p apiKeyPrincipal) HasScope(required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return true
	}
	for _, scope := range p.Key.Scopes {
		scope = strings.TrimSpace(scope)
		if scope == "*" || strings.EqualFold(scope, required) {
			return true
		}
	}
	return false
}

func NewRouter(cfg RouterConfig) http.Handler {
	cfg = cfg.withDefaults()
	router := &serveMuxRouter{mux: http.NewServeMux()}
	router.Get("/livez", handleLive)
	router.Get("/readyz", cfg.handleReady)
	if err := RegisterRoutes(router, cfg); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "router registration failed"})
		})
	}
	return cfg.metrics(router)
}

func RegisterRoutes(r ports.HTTPRouter, cfg RouterConfig) error {
	if r == nil {
		return errors.New("router is required")
	}
	cfg = cfg.withDefaults()
	if cfg.OpenAPIValidation != nil {
		r.Use(cfg.OpenAPIValidation.Middleware())
	}
	r.Get("/docs/openapi.json", handleOpenAPI)
	r.Get("/organizations", cfg.protect("organizations:read", http.HandlerFunc(cfg.handleListOrganizations)).ServeHTTP)
	r.Post("/organizations", cfg.protect("organizations:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateOrganization))).ServeHTTP)
	r.Get("/organizations/{organization_id}/members", cfg.protect("members:read", http.HandlerFunc(cfg.handleListMembers)).ServeHTTP)
	r.Post("/organizations/{organization_id}/invitations", cfg.protect("invitations:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateInvitation))).ServeHTTP)
	r.Get("/organizations/{organization_id}/api-keys", cfg.protect("api-keys:read", http.HandlerFunc(cfg.handleListAPIKeys)).ServeHTTP)
	r.Post("/organizations/{organization_id}/api-keys", cfg.protect("api-keys:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateAPIKey))).ServeHTTP)
	r.Delete("/organizations/{organization_id}/api-keys/{api_key_id}", cfg.protect("api-keys:write", cfg.idempotent(http.HandlerFunc(cfg.handleRevokeAPIKey))).ServeHTTP)
	r.Get("/webhook-events", cfg.protect("webhooks:read", http.HandlerFunc(cfg.handleListWebhookEvents)).ServeHTTP)
	r.Get("/organizations/{organization_id}/webhook-endpoints", cfg.protect("webhooks:read", http.HandlerFunc(cfg.handleListWebhookEndpoints)).ServeHTTP)
	r.Post("/organizations/{organization_id}/webhook-endpoints", cfg.protect("webhooks:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateWebhookEndpoint))).ServeHTTP)
	r.Get("/organizations/{organization_id}/webhook-deliveries", cfg.protect("webhooks:read", http.HandlerFunc(cfg.handleListWebhookDeliveries)).ServeHTTP)
	r.Post("/organizations/{organization_id}/webhook-deliveries/{delivery_id}/replay", cfg.protect("webhooks:write", cfg.idempotent(http.HandlerFunc(cfg.handleReplayWebhookDelivery))).ServeHTTP)
	r.Get("/organizations/{organization_id}/objects", cfg.protect("objects:read", http.HandlerFunc(cfg.handleListObjects)).ServeHTTP)
	r.Post("/organizations/{organization_id}/objects", cfg.protect("objects:write", cfg.idempotent(http.HandlerFunc(cfg.handlePutObject))).ServeHTTP)
	r.Get("/organizations/{organization_id}/objects/{object_key}", cfg.protect("objects:read", http.HandlerFunc(cfg.handleGetObject)).ServeHTTP)
	r.Delete("/organizations/{organization_id}/objects/{object_key}", cfg.protect("objects:write", cfg.idempotent(http.HandlerFunc(cfg.handleDeleteObject))).ServeHTTP)
	r.Post("/invitations/{id}/accept", cfg.protect("invitations:accept", cfg.idempotent(http.HandlerFunc(cfg.handleAcceptInvitation))).ServeHTTP)
	r.Get("/operations/{id}", cfg.protect("operations:read", http.HandlerFunc(cfg.handleGetOperation)).ServeHTTP)
	r.Get("/widgets", cfg.protect("", http.HandlerFunc(cfg.handleListWidgets)).ServeHTTP)
	r.Post("/widgets", cfg.protect("widgets:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateWidget))).ServeHTTP)
	r.Post("/widgets/imports", cfg.protect("widgets:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateWidgetImport))).ServeHTTP)
	registerPatch(r, "/widgets/{id}", cfg.protect("widgets:write", cfg.idempotent(http.HandlerFunc(cfg.handleUpdateWidget))).ServeHTTP)
	r.Delete("/widgets/{id}", cfg.protect("widgets:write", cfg.idempotent(http.HandlerFunc(cfg.handleDeleteWidget))).ServeHTTP)
	// api-toolkit:router-register-routes
	return nil
}

func NewAdminRouter(cfg RouterConfig) http.Handler {
	cfg = cfg.withDefaults()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/detailed", cfg.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		if err := cfg.checkReady(r); err != nil {
			httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.Problem{Title: http.StatusText(http.StatusServiceUnavailable), Detail: "service is not ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))
	mux.Handle("GET /metrics", http.HandlerFunc(cfg.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		cfg.MetricsHandler.ServeHTTP(w, r)
	})))
	_ = corepprof.RegisterAdminRoutes(serveMuxGetRouter{mux: mux}, cfg.requireAdminHandler)
	return mux
}

type serveMuxGetRouter struct {
	mux *http.ServeMux
}

func (r serveMuxGetRouter) Get(pattern string, h http.HandlerFunc) {
	if r.mux == nil {
		return
	}
	r.mux.HandleFunc("GET "+pattern, h)
}

type serveMuxRouter struct {
	mux         *http.ServeMux
	middlewares []func(http.Handler) http.Handler
}

func (r *serveMuxRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r == nil || r.mux == nil {
		http.NotFound(w, req)
		return
	}
	var handler http.Handler = r.mux
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		if r.middlewares[i] != nil {
			handler = r.middlewares[i](handler)
		}
	}
	handler.ServeHTTP(w, req)
}

func (r *serveMuxRouter) Use(middlewares ...func(http.Handler) http.Handler) {
	r.middlewares = append(r.middlewares, middlewares...)
}

func (r *serveMuxRouter) Get(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodGet, pattern, h)
}

func (r *serveMuxRouter) Post(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodPost, pattern, h)
}

func (r *serveMuxRouter) Put(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodPut, pattern, h)
}

func (r *serveMuxRouter) Patch(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodPatch, pattern, h)
}

func (r *serveMuxRouter) Delete(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodDelete, pattern, h)
}

func (r *serveMuxRouter) Mount(pattern string, h http.Handler) {
	if r == nil || r.mux == nil || h == nil {
		return
	}
	r.mux.Handle(pattern, h)
}

func (r *serveMuxRouter) handle(method, pattern string, h http.HandlerFunc) {
	if r == nil || r.mux == nil || h == nil {
		return
	}
	r.mux.HandleFunc(method+" "+pattern, h)
}

type patchRouter interface {
	Patch(pattern string, h http.HandlerFunc)
}

func registerPatch(r ports.HTTPRouter, pattern string, h http.HandlerFunc) {
	if pr, ok := r.(patchRouter); ok {
		pr.Patch(pattern, h)
		return
	}
	r.Mount(pattern, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch {
			http.NotFound(w, req)
			return
		}
		h(w, req)
	}))
}

func (cfg RouterConfig) withDefaults() RouterConfig {
	if cfg.Widgets == nil {
		cfg.Widgets = app.NewWidgetService()
	}
	if cfg.Tenancy == nil {
		cfg.Tenancy = app.NewTenancyService()
	}
	if cfg.APIKeys == nil {
		cfg.APIKeys = app.NewAPIKeyService(os.Getenv("API_KEY_PEPPER"), cfg.Tenancy)
	}
	if cfg.Async == nil {
		cfg.Async = app.NewAsyncService(cfg.Widgets)
	}
	if cfg.Audit == nil {
		cfg.Audit = app.NewAuditService()
	}
	if cfg.Webhooks == nil {
		cfg.Webhooks = app.NewWebhookService(cfg.Tenancy)
	}
	if cfg.Objects == nil {
		cfg.Objects = app.NewObjectService(cfg.Tenancy)
	}
	if cfg.Cache == nil {
		cfg.Cache = app.NewCacheService(nil)
	}
	// api-toolkit:router-default-services
	if cfg.MetricsHandler == nil {
		cfg.MetricsHandler = metricsmw.PrometheusHandler()
	}
	if cfg.RateLimit == nil {
		middleware, err := NewRateLimitMiddleware(nil)
		if err == nil {
			cfg.RateLimit = middleware
		}
	}
	if cfg.Idempotency == nil {
		middleware, err := NewIdempotencyMiddleware(idempotency.NewMemoryStore())
		if err == nil {
			cfg.Idempotency = middleware
		}
	}
	if cfg.APIKey == "" {
		cfg.APIKey = "local-dev-key"
	}
	if cfg.AdminKey == "" {
		cfg.AdminKey = "local-admin-key"
	}
	return cfg
}

func NewMetricsMiddleware(recorder metricsmw.MetricsRecorder) (*metricsmw.Middleware, error) {
	return metricsmw.New(metricsmw.Options{Recorder: recorder})
}

func NewRateLimitMiddleware(limiter ports.RateLimiter) (*ratelimitmw.Middleware, error) {
	return ratelimitmw.New(ratelimitmw.Options{
		Capacity:     20,
		RefillRate:   10,
		RetryAfter:   time.Second,
		Limiter:      limiter,
		Key:          fullRateLimitKey,
		HeaderConfig: ratelimitmw.DefaultHeaderConfig(),
	})
}

func NewIdempotencyMiddleware(store ports.IdempotencyStore) (*idempotencymw.Middleware, error) {
	return idempotencymw.New(idempotencymw.Options{
		Store:          store,
		StorageKeyFunc: fullIdempotencyStorageKey,
		HashFunc:       fullIdempotencyRequestHash,
		RequireKey:     true,
		ShouldStore: func(status int) bool {
			return status >= http.StatusOK && status < http.StatusBadRequest
		},
	})
}

func (cfg RouterConfig) metrics(next http.Handler) http.Handler {
	if cfg.Metrics == nil {
		return next
	}
	return cfg.Metrics.Handler(next)
}

func (cfg RouterConfig) rateLimited(next http.Handler) http.Handler {
	if cfg.RateLimit == nil {
		return next
	}
	return cfg.RateLimit.Handler(next)
}

func (cfg RouterConfig) idempotent(next http.Handler) http.Handler {
	if cfg.Idempotency == nil {
		return next
	}
	return cfg.Idempotency.Handler(next)
}

func fullRateLimitKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	actorID, tenantID := idempotencyScope(r)
	h := sha256.New()
	h.Write([]byte("saas-api-full:rate-limit-key:v1"))
	h.Write([]byte{0})
	h.Write([]byte(tenantID))
	h.Write([]byte{0})
	h.Write([]byte(actorID))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToUpper(r.Method)))
	h.Write([]byte{0})
	if r.URL != nil {
		h.Write([]byte(r.URL.Path))
	}
	return "atk:v1:" + hex.EncodeToString(h.Sum(nil))
}

func fullIdempotencyStorageKey(r *http.Request, clientKey string) string {
	clientKey = strings.TrimSpace(clientKey)
	if r == nil || clientKey == "" {
		return ""
	}
	actorID, tenantID := idempotencyScope(r)
	h := sha256.New()
	h.Write([]byte("saas-api-full:idempotency-storage-key:v1"))
	h.Write([]byte{0})
	h.Write([]byte(tenantID))
	h.Write([]byte{0})
	h.Write([]byte(actorID))
	h.Write([]byte{0})
	h.Write([]byte(clientKey))
	return "atk:v1:" + hex.EncodeToString(h.Sum(nil))
}

func fullIdempotencyRequestHash(r *http.Request, body []byte) (string, error) {
	if r == nil {
		return "", errors.New("request is nil")
	}
	actorID, tenantID := idempotencyScope(r)
	h := sha256.New()
	h.Write([]byte(actorID))
	h.Write([]byte{0})
	h.Write([]byte(tenantID))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToUpper(r.Method)))
	h.Write([]byte{0})
	if r.URL != nil {
		h.Write([]byte(r.URL.Path))
		h.Write([]byte{0})
		h.Write([]byte(r.URL.Query().Encode()))
	}
	h.Write([]byte{0})
	h.Write([]byte(r.Header.Get("Content-Type")))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func idempotencyScope(r *http.Request) (string, string) {
	if r == nil {
		return "", ""
	}
	actorID := strings.TrimSpace(os.Getenv("API_ACTOR_ID"))
	if actorID == "" && !strings.EqualFold(os.Getenv("ENV"), "production") {
		actorID = strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if organizationID := strings.TrimSpace(r.PathValue("organization_id")); organizationID != "" {
		tenantID = organizationID
	}
	if tenantID == "" {
		tenantID = strings.TrimSpace(r.PathValue("id"))
	}
	if principal, ok := apiKeyPrincipalFromContext(r.Context()); ok {
		if actorID = principal.ActorID(); actorID == "" {
			actorID = strings.TrimSpace(os.Getenv("API_ACTOR_ID"))
		}
		if principalTenant := principal.TenantID(); principalTenant != "" && tenantID == "" {
			tenantID = principalTenant
		}
	}
{{ if eq .AuthMode "jwt" }}	if subj, ok := jwtauth.SubjectFromContext(r.Context()); ok {
		if strings.TrimSpace(subj.UserID) != "" {
			actorID = strings.TrimSpace(subj.UserID)
		}
		if claimTenant := jwtSubjectTenantID(subj); claimTenant != "" && tenantID == "" {
			tenantID = claimTenant
		}
	}
{{ else if eq .AuthMode "clerk" }}	if subj, ok := clerkauth.SubjectFromContext(r.Context()); ok {
		if strings.TrimSpace(subj.UserID) != "" {
			actorID = strings.TrimSpace(subj.UserID)
		}
		if strings.TrimSpace(subj.TenantID) != "" && tenantID == "" {
			tenantID = strings.TrimSpace(subj.TenantID)
		}
	}
{{ else if eq .AuthMode "oidc" }}	if subj, ok := oidcauth.SubjectFromContext(r.Context()); ok {
		if strings.TrimSpace(subj.UserID) != "" {
			actorID = strings.TrimSpace(subj.UserID)
		}
		if strings.TrimSpace(subj.TenantID) != "" && tenantID == "" {
			tenantID = strings.TrimSpace(subj.TenantID)
		}
	}
{{ end }}
	return actorID, tenantID
}

func NewHealthHandler(readiness HealthChecker) *health.Handler {
	manager := health.NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         5 * time.Second,
		CacheDuration:   5 * time.Second,
		EnableCaching:   true,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic", "dependencies"},
	})
	manager.RegisterChecker(health.NewBasicChecker())
	manager.RegisterChecker(health.NewCustomChecker("dependencies", func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
		if readiness == nil {
			return ports.HealthStatusHealthy, "dependencies ready", nil
		}
		if err := readiness.Check(ctx); err != nil {
			return ports.HealthStatusUnhealthy, "dependencies unavailable", nil
		}
		return ports.HealthStatusHealthy, "dependencies ready", nil
	}))
	return health.NewHandler(manager)
}

func NewOpenAPIValidationMiddleware(cfg Config) (*openapimw.Middleware, error) {
	if !cfg.OpenAPIRequestValidation {
		return nil, nil
	}
	doc, err := OpenAPIDocument()
	if err != nil {
		return nil, err
	}
	spec, err := openapi3.NewLoader().LoadFromData(doc)
	if err != nil {
		return nil, err
	}
	opts := []openapimw.Option{
		openapimw.WithIgnoreNotFound(true),
		openapimw.WithFilterOptions(openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		}),
	}
	if cfg.OpenAPIResponseValidation {
		opts = append(opts, openapimw.WithResponseValidation(openapimw.ResponseValidationOptions{
			Enabled:      true,
			MaxBodyBytes: 1 << 20,
			ShouldValidate: func(r *http.Request) bool {
				if r == nil || r.URL == nil {
					return false
				}
				path := r.URL.Path
				return !strings.HasPrefix(path, "/docs") &&
					path != "/livez" &&
					path != "/readyz" &&
					path != "/health" &&
					path != "/healthz" &&
					path != "/health/detailed" &&
					path != "/metrics" &&
					!strings.HasPrefix(path, "/debug/pprof/")
			},
		}))
	}
	return openapimw.New(spec, opts...)
}

func handleLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (cfg RouterConfig) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := cfg.checkReady(r); err != nil {
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.Problem{Title: http.StatusText(http.StatusServiceUnavailable), Detail: "service is not ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (cfg RouterConfig) checkReady(r *http.Request) error {
	if cfg.Readiness == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	return cfg.Readiness.Check(ctx)
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

func (cfg RouterConfig) recordAudit(r *http.Request, tenantID, actorID, action, resourceType, resourceID string, metadata map[string]string) {
	if cfg.Audit == nil {
		return
	}
	_ = cfg.Audit.Record(r.Context(), audit.Event{
		TenantID: strings.TrimSpace(tenantID),
		Actor: audit.Actor{
			Type: "user",
			ID:   strings.TrimSpace(actorID),
		},
		Action: strings.TrimSpace(action),
		Resource: audit.Resource{
			Type: strings.TrimSpace(resourceType),
			ID:   strings.TrimSpace(resourceID),
		},
		Result:    audit.ResultSuccess,
		RequestID: requestID(r),
		Metadata:  metadata,
	})
}

func requestID(r *http.Request) string {
	for _, name := range []string{"X-Request-ID", "X-Correlation-ID"} {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func (cfg RouterConfig) handleListOrganizations(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	orgs, err := cfg.Tenancy.ListOrganizations(r.Context(), actorID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, org.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (cfg RouterConfig) handleCreateOrganization(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeOrganizationRequest(w, r)
	if !ok {
		return
	}
	org, _, err := cfg.Tenancy.CreateOrganization(r.Context(), actorID, req.Name)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, org.ID, actorID, "organization.create", "organization", org.ID, nil)
	writeJSON(w, http.StatusCreated, org.Public())
}

func (cfg RouterConfig) handleListMembers(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	members, err := cfg.Tenancy.ListMembers(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(members))
	for _, member := range members {
		items = append(items, member.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (cfg RouterConfig) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeInvitationRequest(w, r)
	if !ok {
		return
	}
	invitation, token, err := cfg.Tenancy.InviteMember(r.Context(), actorID, organizationID, req.Email, req.Role)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "invitation.create", "invitation", invitation.ID, map[string]string{"role": string(invitation.Role)})
	writeJSON(w, http.StatusCreated, map[string]any{"invitation": invitation.Public(), "token": token})
}

func (cfg RouterConfig) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	keys, err := cfg.APIKeys.List(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		items = append(items, key.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (cfg RouterConfig) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeAPIKeyCreateRequest(w, r)
	if !ok {
		return
	}
	key, secret, err := cfg.APIKeys.Create(r.Context(), actorID, organizationID, req.Name, req.Scopes, req.ExpiresAt)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "api_key.create", "api_key", key.ID, map[string]string{
		"scope_count": strconv.Itoa(len(key.Scopes)),
		"expires":     strconv.FormatBool(key.ExpiresAt != nil),
	})
	writeJSON(w, http.StatusCreated, map[string]any{"api_key": key.Public(), "secret": secret})
}

func (cfg RouterConfig) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	keyID := strings.TrimSpace(r.PathValue("api_key_id"))
	if keyID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "api key id is required"})
		return
	}
	if err := cfg.APIKeys.Revoke(r.Context(), actorID, organizationID, keyID); err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "api_key.revoke", "api_key", keyID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (cfg RouterConfig) handleListWebhookEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := cfg.authenticateActor(w, r); !ok {
		return
	}
	eventTypes, hit, err := cfg.Cache.WebhookEventTypes(r.Context(), cfg.Webhooks.EventTypes)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if hit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	writeJSON(w, http.StatusOK, map[string]any{"event_types": eventTypes})
}

func (cfg RouterConfig) handleListWebhookEndpoints(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	endpoints, err := cfg.Webhooks.ListEndpointsForActor(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": endpoints})
}

func (cfg RouterConfig) handleCreateWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeWebhookEndpointRequest(w, r)
	if !ok {
		return
	}
	created, err := cfg.Webhooks.CreateEndpoint(r.Context(), actorID, organizationID, req.URL, req.Events)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "webhook_endpoint.create", "webhook_endpoint", created.Endpoint.ID, map[string]string{"event_count": strconv.Itoa(len(created.Endpoint.Events))})
	writeJSON(w, http.StatusCreated, map[string]any{"endpoint": created.Endpoint, "secret": created.Secret})
}

func (cfg RouterConfig) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	deliveries, err := cfg.Webhooks.ListDeliveriesForActor(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": deliveries})
}

func (cfg RouterConfig) handleReplayWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	if !decodeWebhookReplayRequest(w, r) {
		return
	}
	deliveryID := strings.TrimSpace(r.PathValue("delivery_id"))
	if deliveryID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "delivery id is required"})
		return
	}
	delivery, err := cfg.Webhooks.ReplayDeliveryForActor(r.Context(), actorID, organizationID, deliveryID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "webhook_delivery.replay", "webhook_delivery", delivery.ID, map[string]string{"event_type": delivery.EventType})
	writeJSON(w, http.StatusAccepted, delivery)
}

func (cfg RouterConfig) handleListObjects(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	objects, err := cfg.Objects.List(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(objects))
	for _, object := range objects {
		items = append(items, object.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (cfg RouterConfig) handlePutObject(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeObjectPutRequest(w, r)
	if !ok {
		return
	}
	object, err := cfg.Objects.Put(r.Context(), actorID, organizationID, req.Key, req.ContentType, req.Data)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "object.put", "object", object.Key, map[string]string{
		"content_type": object.ContentType,
		"size":         strconv.FormatInt(object.Size, 10),
	})
	writeJSON(w, http.StatusCreated, object.Public())
}

func (cfg RouterConfig) handleGetObject(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.PathValue("object_key"))
	object, data, found, err := cfg.Objects.Get(r.Context(), actorID, organizationID, key)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !found {
		writeAppError(w, app.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": object.Public(), "content_base64": base64.StdEncoding.EncodeToString(data)})
}

func (cfg RouterConfig) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	key := strings.TrimSpace(r.PathValue("object_key"))
	if err := cfg.Objects.Delete(r.Context(), actorID, organizationID, key); err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "object.delete", "object", key, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (cfg RouterConfig) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	invitationID := strings.TrimSpace(r.PathValue("id"))
	if invitationID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invitation id is required"})
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeAcceptInvitationRequest(w, r)
	if !ok {
		return
	}
	member, err := cfg.Tenancy.AcceptInvitation(r.Context(), invitationID, req.Token, actorID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, member.OrganizationID, actorID, "invitation.accept", "membership", member.UserID, nil)
	writeJSON(w, http.StatusOK, member.Public())
}

func (cfg RouterConfig) handleGetOperation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	operationID := strings.TrimSpace(r.PathValue("id"))
	if operationID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "operation id is required"})
		return
	}
	operation, found, err := cfg.Async.GetOperation(r.Context(), tenantID, operationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !found {
		writeAppError(w, app.ErrNotFound)
		return
	}
	apitkops.WriteOperation(w, http.StatusOK, operation)
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
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
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
	if !replayed {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "widget.created", map[string]any{"id": widget.ID, "tenant_id": tenantID, "version": widget.Version}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "widget.create", "widget", widget.ID, map[string]string{"replayed": strconv.FormatBool(replayed)})
	writeJSON(w, status, widget.Public())
}

func (cfg RouterConfig) handleCreateWidgetImport(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	req, ok := decodeWidgetImportRequest(w, r)
	if !ok {
		return
	}
	operation, replayed, err := cfg.Async.StartWidgetImport(r.Context(), tenantID, idempotencyKey, req.Items)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	cfg.recordAudit(r, tenantID, actorID, "widget_import.create", "operation", operation.ID, map[string]string{
		"item_count": strconv.Itoa(len(req.Items)),
		"replayed":   strconv.FormatBool(replayed),
	})
	apitkops.WriteAccepted(w, apitkops.AcceptedConfig{
		ID:         operation.ID,
		Location:   "/operations/" + operation.ID,
		RetryAfter: time.Second,
	})
}

func (cfg RouterConfig) handleUpdateWidget(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
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
	widget, replayed, err := cfg.Widgets.Update(r.Context(), tenantID, r.PathValue("id"), req.Name, ifMatch, idempotencyKey)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("ETag", widget.ETag())
	if !replayed {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "widget.updated", map[string]any{"id": widget.ID, "tenant_id": tenantID, "version": widget.Version}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "widget.update", "widget", widget.ID, nil)
	writeJSON(w, http.StatusOK, widget.Public())
}

func (cfg RouterConfig) handleDeleteWidget(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	replayed, err := cfg.Widgets.Delete(r.Context(), tenantID, r.PathValue("id"), idempotencyKey)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !replayed {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "widget.deleted", map[string]any{"id": strings.TrimSpace(r.PathValue("id")), "tenant_id": tenantID}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "widget.delete", "widget", r.PathValue("id"), nil)
	w.WriteHeader(http.StatusNoContent)
}

type widgetRequest struct {
	Name string
}

type widgetImportRequest struct {
	Items []app.WidgetImportItem
}

type organizationRequest struct {
	Name string
}

type invitationRequest struct {
	Email string
	Role  domain.Role
}

type acceptInvitationRequest struct {
	Token string
}

type apiKeyCreateRequest struct {
	Name      string
	Scopes    []string
	ExpiresAt *time.Time
}

type webhookEndpointRequest struct {
	URL    string
	Events []string
}

type objectPutRequest struct {
	Key         string
	ContentType string
	Data        []byte
}

func decodeOrganizationRequest(w http.ResponseWriter, r *http.Request) (organizationRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]string
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return organizationRequest{}, false
	}
	name := strings.TrimSpace(raw["name"])
	if name == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "name is required"})
		return organizationRequest{}, false
	}
	return organizationRequest{Name: name}, true
}

func decodeInvitationRequest(w http.ResponseWriter, r *http.Request) (invitationRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]string
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return invitationRequest{}, false
	}
	email := strings.TrimSpace(raw["email"])
	role := domain.Role(strings.TrimSpace(raw["role"]))
	if email == "" || !strings.Contains(email, "@") || !role.Valid() || role == domain.RoleOwner {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "valid email and non-owner role are required"})
		return invitationRequest{}, false
	}
	return invitationRequest{Email: email, Role: role}, true
}

func decodeAcceptInvitationRequest(w http.ResponseWriter, r *http.Request) (acceptInvitationRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]string
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return acceptInvitationRequest{}, false
	}
	token := strings.TrimSpace(raw["token"])
	if token == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "token is required"})
		return acceptInvitationRequest{}, false
	}
	return acceptInvitationRequest{Token: token}, true
}

func decodeAPIKeyCreateRequest(w http.ResponseWriter, r *http.Request) (apiKeyCreateRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw struct {
		Name      string   ` + "`json:\"name\"`" + `
		Scopes    []string ` + "`json:\"scopes\"`" + `
		ExpiresAt string   ` + "`json:\"expires_at\"`" + `
	}
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return apiKeyCreateRequest{}, false
	}
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "name is required"})
		return apiKeyCreateRequest{}, false
	}
	scopes := make([]string, 0, len(raw.Scopes))
	for _, scope := range raw.Scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	if len(scopes) == 0 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "at least one scope is required"})
		return apiKeyCreateRequest{}, false
	}
	var expiresAt *time.Time
	if strings.TrimSpace(raw.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw.ExpiresAt))
		if err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "expires_at must be RFC3339"})
			return apiKeyCreateRequest{}, false
		}
		parsed = parsed.UTC()
		expiresAt = &parsed
	}
	return apiKeyCreateRequest{Name: name, Scopes: scopes, ExpiresAt: expiresAt}, true
}

func decodeWebhookEndpointRequest(w http.ResponseWriter, r *http.Request) (webhookEndpointRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw struct {
		URL    string   ` + "`json:\"url\"`" + `
		Events []string ` + "`json:\"events\"`" + `
	}
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return webhookEndpointRequest{}, false
	}
	targetURL := strings.TrimSpace(raw.URL)
	if targetURL == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "url is required"})
		return webhookEndpointRequest{}, false
	}
	events := make([]string, 0, len(raw.Events))
	for _, eventType := range raw.Events {
		eventType = strings.TrimSpace(eventType)
		if eventType != "" {
			events = append(events, eventType)
		}
	}
	if len(events) == 0 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "at least one event is required"})
		return webhookEndpointRequest{}, false
	}
	return webhookEndpointRequest{URL: targetURL, Events: events}, true
}

func decodeWebhookReplayRequest(w http.ResponseWriter, r *http.Request) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return false
	}
	if len(raw) != 0 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "replay body must be empty"})
		return false
	}
	return true
}

func decodeObjectPutRequest(w http.ResponseWriter, r *http.Request) (objectPutRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	var raw struct {
		Key           string ` + "`json:\"key\"`" + `
		ContentType   string ` + "`json:\"content_type\"`" + `
		ContentBase64 string ` + "`json:\"content_base64\"`" + `
	}
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return objectPutRequest{}, false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw.ContentBase64))
	if err != nil || len(data) == 0 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "content_base64 is required"})
		return objectPutRequest{}, false
	}
	return objectPutRequest{Key: strings.TrimSpace(raw.Key), ContentType: strings.TrimSpace(raw.ContentType), Data: data}, true
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

func decodeWidgetImportRequest(w http.ResponseWriter, r *http.Request) (widgetImportRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw struct {
		Items []app.WidgetImportItem ` + "`json:\"items\"`" + `
	}
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return widgetImportRequest{}, false
	}
	if len(raw.Items) == 0 || len(raw.Items) > 100 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "items must contain 1 to 100 widgets"})
		return widgetImportRequest{}, false
	}
	for _, item := range raw.Items {
		name := strings.TrimSpace(item.Name)
		if name == "" || len(name) > 120 {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "item names are required"})
			return widgetImportRequest{}, false
		}
	}
	return widgetImportRequest{Items: raw.Items}, true
}

func (cfg RouterConfig) authenticateManagedAPIKey(w http.ResponseWriter, r *http.Request) (apiKeyPrincipal, bool) {
	if cfg.APIKeys == nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "API key service is not configured"})
		return apiKeyPrincipal{}, false
	}
	key, ok, err := cfg.APIKeys.Verify(r.Context(), r.Header.Get("X-API-Key"))
	if err != nil || !ok {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid API key required"})
		return apiKeyPrincipal{}, false
	}
	return apiKeyPrincipal{Key: key}, true
}

func (cfg RouterConfig) authenticateActor(w http.ResponseWriter, r *http.Request) (string, bool) {
{{ if eq .AuthMode "jwt" }}	subj, ok := jwtauth.SubjectFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
		return "", false
	}
	actorID := strings.TrimSpace(subj.UserID)
	if actorID == "" {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "actor subject required"})
		return "", false
	}
	return actorID, true
{{ else if eq .AuthMode "clerk" }}	subj, ok := clerkauth.SubjectFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
		return "", false
	}
	actorID := strings.TrimSpace(subj.UserID)
	if actorID == "" {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "actor subject required"})
		return "", false
	}
	return actorID, true
{{ else if eq .AuthMode "oidc" }}	subj, ok := oidcauth.SubjectFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
		return "", false
	}
	actorID := strings.TrimSpace(subj.UserID)
	if actorID == "" {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "actor subject required"})
		return "", false
	}
	return actorID, true
{{ else }}
	if principal, ok := apiKeyPrincipalFromContext(r.Context()); ok {
		if actorID := principal.ActorID(); actorID != "" {
			return actorID, true
		}
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "API key actor required"})
		return "", false
	}
	if !sameSecret(r.Header.Get("X-API-Key"), cfg.APIKey) {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid API key required"})
		return "", false
	}
	actorID := strings.TrimSpace(os.Getenv("API_ACTOR_ID"))
	if actorID == "" && !strings.EqualFold(os.Getenv("ENV"), "production") {
		actorID = strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	}
	if actorID == "" {
		actorID = "local-api-key"
	}
	return actorID, true
{{ end }}
}

func (cfg RouterConfig) authenticateOrganizationTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	organizationID := strings.TrimSpace(r.PathValue("organization_id"))
	if organizationID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "organization id is required"})
		return "", false
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return "", false
	}
	if tenantID != organizationID {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "tenant path mismatch"})
		return "", false
	}
	return organizationID, true
}

func (cfg RouterConfig) authenticateTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
{{ if eq .AuthMode "jwt" }}	subj, ok := jwtauth.SubjectFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
		return "", false
	}
	tenantID, ok := requireHeader(w, r, "X-Tenant-ID")
	if !ok {
		return "", false
	}
	if jwtSubjectTenantID(subj) != tenantID {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "tenant claim mismatch"})
		return "", false
	}
	return tenantID, true
{{ else if eq .AuthMode "clerk" }}	subj, ok := clerkauth.SubjectFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
		return "", false
	}
	tenantID, ok := requireHeader(w, r, "X-Tenant-ID")
	if !ok {
		return "", false
	}
	if strings.TrimSpace(subj.TenantID) == "" || strings.TrimSpace(subj.TenantID) != tenantID {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "tenant claim mismatch"})
		return "", false
	}
	return tenantID, true
{{ else if eq .AuthMode "oidc" }}	subj, ok := oidcauth.SubjectFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
		return "", false
	}
	tenantID, ok := requireHeader(w, r, "X-Tenant-ID")
	if !ok {
		return "", false
	}
	if strings.TrimSpace(subj.TenantID) == "" || strings.TrimSpace(subj.TenantID) != tenantID {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "tenant claim mismatch"})
		return "", false
	}
	return tenantID, true
{{ else }}
	tenantID, ok := requireHeader(w, r, "X-Tenant-ID")
	if !ok {
		return "", false
	}
	if principal, ok := apiKeyPrincipalFromContext(r.Context()); ok {
		if principal.TenantID() != tenantID {
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "tenant credential mismatch"})
			return "", false
		}
		return tenantID, true
	}
	if !sameSecret(r.Header.Get("X-API-Key"), cfg.APIKey) {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid API key required"})
		return "", false
	}
	return tenantID, true
{{ end }}
}

func (cfg RouterConfig) protect(requiredScope string, next http.Handler) http.Handler {
{{ if eq .AuthMode "jwt" }}	if cfg.JWT == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "JWT middleware not configured"})
		})
	}
	return cfg.JWT.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requiredScope != "" {
			subj, ok := jwtauth.SubjectFromContext(r.Context())
			if !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
				return
			}
			if !jwtSubjectHasScope(subj, requiredScope) {
				httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "required JWT scope missing"})
				return
			}
		}
		cfg.rateLimited(next).ServeHTTP(w, r)
	}))
{{ else if eq .AuthMode "clerk" }}	if cfg.Clerk == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "Clerk middleware not configured"})
		})
	}
	return cfg.Clerk.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requiredScope != "" {
			subj, ok := clerkauth.SubjectFromContext(r.Context())
			if !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
				return
			}
			if !scopeStringContains(subj.Scope, requiredScope) {
				httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "required Clerk scope missing"})
				return
			}
		}
		cfg.rateLimited(next).ServeHTTP(w, r)
	}))
{{ else if eq .AuthMode "oidc" }}	if cfg.OIDC == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "OIDC middleware not configured"})
		})
	}
	return cfg.OIDC.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requiredScope != "" {
			subj, ok := oidcauth.SubjectFromContext(r.Context())
			if !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid bearer token required"})
				return
			}
			if !oidcSubjectHasScope(subj, requiredScope) {
				httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "required OIDC scope missing"})
				return
			}
		}
		cfg.rateLimited(next).ServeHTTP(w, r)
	}))
{{ else }}	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sameSecret(r.Header.Get("X-API-Key"), cfg.APIKey) {
			cfg.rateLimited(next).ServeHTTP(w, r)
			return
		}
		principal, ok := cfg.authenticateManagedAPIKey(w, r)
		if !ok {
			return
		}
		if !principal.HasScope(requiredScope) {
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "required API key scope missing"})
			return
		}
		cfg.rateLimited(next).ServeHTTP(w, r.WithContext(withAPIKeyPrincipal(r.Context(), principal.Key)))
	})
{{ end }}
}

{{ if eq .AuthMode "jwt" }}func jwtSubjectTenantID(subj jwtauth.Subject) string {
	for _, key := range []string{"tenant_id", "tid", "org_id"} {
		if value := stringClaim(subj.Claims, key); value != "" {
			return value
		}
	}
	return ""
}

func jwtSubjectHasScope(subj jwtauth.Subject, required string) bool {
	if required == "" {
		return true
	}
	for _, claim := range []string{"scope", "scp", "permissions"} {
		for _, scope := range scopeValues(subj.Claims[claim]) {
			if strings.EqualFold(scope, required) {
				return true
			}
		}
	}
	return false
}

func stringClaim(claims map[string]any, key string) string {
	if claims == nil {
		return ""
	}
	value, ok := claims[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func scopeValues(value any) []string {
	switch v := value.(type) {
	case string:
		return splitAuthScopes(v)
	case []string:
		return cleanScopes(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if scope, ok := item.(string); ok {
				out = append(out, scope)
			}
		}
		return cleanScopes(out)
	default:
		return nil
	}
}

{{ else if eq .AuthMode "clerk" }}func scopeStringContains(value, required string) bool {
	if required == "" {
		return true
	}
	for _, scope := range splitAuthScopes(value) {
		if strings.EqualFold(scope, required) {
			return true
		}
	}
	return false
}

{{ else if eq .AuthMode "oidc" }}func oidcSubjectHasScope(subj oidcauth.Subject, required string) bool {
	return scopeStringContains(subj.Scope, required)
}

func scopeStringContains(value, required string) bool {
	if required == "" {
		return true
	}
	for _, scope := range splitAuthScopes(value) {
		if strings.EqualFold(scope, required) {
			return true
		}
	}
	return false
}

{{ end }}
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}func splitAuthScopes(value string) []string {
	return cleanScopes(strings.FieldsFunc(value, func(r rune) bool { return r == ' ' || r == ',' }))
}

func cleanScopes(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

{{ end }}

func (cfg RouterConfig) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameSecret(r.Header.Get("X-Admin-Key"), cfg.AdminKey) {
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "admin authentication required"})
			return
		}
		next(w, r)
	}
}

func (cfg RouterConfig) requireAdminHandler(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(cfg.requireAdmin(next.ServeHTTP))
}

func RequireAdmin(adminKey string) func(http.Handler) http.Handler {
	cfg := RouterConfig{AdminKey: adminKey}.withDefaults()
	return cfg.requireAdminHandler
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
	case errors.Is(err, app.ErrForbidden):
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "permission denied"})
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
	"context"
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
{{ end }}
	"encoding/json"
	"errors"
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	"math/big"
{{ end }}
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	"time"

	"github.com/golang-jwt/jwt/v5"
{{ if eq .AuthMode "jwt" }}	jwtauth "github.com/aatuh/api-toolkit/v2/middleware/auth/jwt"
{{ else if eq .AuthMode "clerk" }}	clerkauth "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/clerk"
{{ else if eq .AuthMode "oidc" }}	oidcauth "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/oidc"
{{ end }}
	"github.com/aatuh/api-toolkit/v2/ports"
{{ end }}
	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	"{{ .Module }}/internal/app"
)

func TestReadinessAndOpenAPI(t *testing.T) {
	handler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("live status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
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

func TestReadinessReportsDependencyFailure(t *testing.T) {
	handler := NewRouter(RouterConfig{
		Readiness: HealthCheckFunc(func(context.Context) error { return errors.New("postgres unavailable") }),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready failure status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "postgres unavailable") {
		t.Fatalf("ready failure leaked dependency error: %s", rec.Body.String())
	}
}

func TestOpenAPIValidationRejectsInvalidRequests(t *testing.T) {
	configureTestAuthEnv(t)
	validator, err := NewOpenAPIValidationMiddleware(Config{OpenAPIRequestValidation: true})
	if err != nil {
		t.Fatalf("new openapi validator: %v", err)
	}
	tenancy := app.NewTenancyService()
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), Tenancy: tenancy, APIKeys: app.NewAPIKeyService("test-pepper", tenancy), OpenAPIValidation: validator{{ if eq .AuthMode "jwt" }}, JWT: newTestJWT(t){{ else if eq .AuthMode "clerk" }}, Clerk: newTestClerk(t){{ else if eq .AuthMode "oidc" }}, OIDC: newTestOIDC(t){{ else }}, APIKey: "test-key"{{ end }}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"validated"}` + "`" + `))
	req.Header.Set("Content-Type", "text/plain")
	authorizeTestRequest(t, req, "org_validation")
	req.Header.Set("Idempotency-Key", "openapi-validation")
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusCreated {
		t.Fatalf("expected OpenAPI request validation failure, got status %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported content type") {
		t.Fatalf("validation response body = %s", rec.Body.String())
	}
}

func TestAdminMetricsRecordsHTTPRequestsWithoutSecrets(t *testing.T) {
	handler, adminHandler := newTestRouterWithMetrics(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"metric-widget\"}"))
	authorizeTestRequestAs(t, req, "org_metric_secret", "actor_metric_secret", "widgets:write")
	req.Header.Set("Idempotency-Key", "idem_metric_secret")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create widget status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("X-Admin-Key", "test-admin-key")
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"http_requests_total", "http_request_duration_seconds", ` + "`" + `route="POST /widgets"` + "`" + `, ` + "`" + `status="201"` + "`" + `} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
	for _, secret := range []string{"org_metric_secret", "actor_metric_secret", "idem_metric_secret", "test-key", "test-admin-key"} {
		if strings.Contains(body, secret) {
			t.Fatalf("metrics body leaked secret %q:\n%s", secret, body)
		}
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminPprofRequiresAdminAndServesProfiles(t *testing.T) {
	publicHandler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	publicHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("public pprof status = %d body=%s", rec.Code, rec.Body.String())
	}

	adminHandler := NewAdminRouter(RouterConfig{AdminKey: "test-admin-key"})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated admin pprof status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.Header.Set("X-Admin-Key", "test-admin-key")
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Types of profiles available") {
		t.Fatalf("admin pprof index status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	req.Header.Set("X-Admin-Key", "test-admin-key")
	adminHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin pprof cmdline status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateWidgetRequiresAuth(t *testing.T) {
	handler := newTestRouter(t)
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

func TestProtectedRoutesAreRateLimited(t *testing.T) {
	handler := newTestRouter(t)
	limited := false
	for i := 0; i < 30; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/organizations", nil)
		authorizeTestRequestAs(t, req, "", "owner_1", "organizations:read")
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			if rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), "rate limit exceeded") {
				t.Fatalf("rate limit response headers/body missing: headers=%v body=%s", rec.Header(), rec.Body.String())
			}
			break
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("protected request status = %d body=%s", rec.Code, rec.Body.String())
		}
	}
	if !limited {
		t.Fatal("expected protected route to return 429 after repeated requests")
	}
}

func TestCreateWidgetValidatesBody(t *testing.T) {
	handler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader("{\"name\":\"\"}"))
	authorizeTestRequest(t, req, "org_1")
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", "idem_1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("validation status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateWidgetReplaysIdempotencyKey(t *testing.T) {
	handler := newTestRouter(t)
	first := createWidget(t, handler, "idem_1")
	second := createWidget(t, handler, "idem_1")
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d body=%s", first.Code, first.Body.String())
	}
	if second.Code != http.StatusCreated {
		t.Fatalf("second replay status = %d body=%s", second.Code, second.Body.String())
	}
	if second.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("second replay header = %q", second.Header().Get("Idempotency-Replayed"))
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
	handler := newTestRouter(t)
	created := createWidget(t, handler, "idem_create")
	var body map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode created body: %v", err)
	}
	id, _ := body["id"].(string)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/widgets/"+id, strings.NewReader("{\"name\":\"beta\"}"))
	authorizeTestRequest(t, req, "org_1")
	req.Header.Set("X-Tenant-ID", "org_1")
	req.Header.Set("Idempotency-Key", "idem_update")
	req.Header.Set("If-Match", "\"999\"")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("conflict status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWidgetImportReturnsPollableOperation(t *testing.T) {
	handler := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets/imports", strings.NewReader(` + "`" + `{"items":[{"name":"bulk-a"},{"name":"bulk-b"}]}` + "`" + `))
	authorizeTestRequestAs(t, req, "org_1", "user_123", "widgets:write")
	req.Header.Set("Idempotency-Key", "import-1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("import status = %d body=%s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/operations/") || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("operation headers Location=%q Retry-After=%q", location, rec.Header().Get("Retry-After"))
	}
	var accepted map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode accepted body: %v", err)
	}
	operationID, _ := accepted["id"].(string)
	if operationID == "" || accepted["state"] != "pending" {
		t.Fatalf("accepted body = %#v", accepted)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, location, nil)
	authorizeTestRequestAs(t, req, "org_1", "user_123", "operations:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), operationID) || !strings.Contains(rec.Body.String(), "pending") {
		t.Fatalf("operation poll status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOrganizationInvitationFlow(t *testing.T) {
	handler := newTestRouter(t)
	orgID := createOrganization(t, handler, "owner_1", "Acme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/organizations", nil)
	authorizeTestRequestAs(t, req, "", "owner_1", "organizations:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), orgID) {
		t.Fatalf("list organizations status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/members", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "members:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "owner_1") {
		t.Fatalf("list members status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/invitations", strings.NewReader(` + "`" + `{"email":"other@example.com","role":"member"}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "stranger_1", "invitations:write")
	req.Header.Set("Idempotency-Key", "invite-stranger")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("stranger invite status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/invitations", strings.NewReader(` + "`" + `{"email":"Member@Example.com","role":"member"}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "invitations:write")
	req.Header.Set("Idempotency-Key", "invite-member")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite status = %d body=%s", rec.Code, rec.Body.String())
	}
	var inviteBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &inviteBody); err != nil {
		t.Fatalf("decode invite body: %v", err)
	}
	token, _ := inviteBody["token"].(string)
	if token == "" {
		t.Fatalf("invite body missing token: %#v", inviteBody)
	}
	invitation, _ := inviteBody["invitation"].(map[string]any)
	invitationID, _ := invitation["id"].(string)
	if invitationID == "" || invitation["email"] != "member@example.com" {
		t.Fatalf("invitation body = %#v", inviteBody)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations/"+invitationID+"/accept", strings.NewReader(` + "`" + `{"token":"wrong-token"}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "member_1", "invitations:accept")
	req.Header.Set("Idempotency-Key", "accept-wrong")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("wrong token accept status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), token) {
		t.Fatalf("wrong-token problem leaked invitation token: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations/"+invitationID+"/accept", strings.NewReader("{\"token\":\""+token+"\"}"))
	authorizeTestRequestAs(t, req, orgID, "member_1", "invitations:accept")
	req.Header.Set("Idempotency-Key", "accept-member")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "member_1") {
		t.Fatalf("accept status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations/"+invitationID+"/accept", strings.NewReader("{\"token\":\""+token+"\"}"))
	authorizeTestRequestAs(t, req, orgID, "member_2", "invitations:accept")
	req.Header.Set("Idempotency-Key", "accept-replay")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("replay accept status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(` + "`" + `{"name":"CI","scopes":["widgets:read","widgets:write"],"expires_at":"2030-01-01T00:00:00Z"}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "create-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var apiKeyBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &apiKeyBody); err != nil {
		t.Fatalf("decode api key body: %v", err)
	}
	secret, _ := apiKeyBody["secret"].(string)
	apiKey, _ := apiKeyBody["api_key"].(map[string]any)
	apiKeyID, _ := apiKey["id"].(string)
	if secret == "" || apiKeyID == "" || apiKey["prefix"] == "" {
		t.Fatalf("api key body = %#v", apiKeyBody)
	}

{{ if eq .AuthMode "api-key" }}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"managed-key-widget"}` + "`" + `))
	req.Header.Set("X-API-Key", secret)
	req.Header.Set("X-Tenant-ID", orgID)
	req.Header.Set("Idempotency-Key", "managed-key-widget")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("managed api key widget create status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"wrong-tenant"}` + "`" + `))
	req.Header.Set("X-API-Key", secret)
	req.Header.Set("X-Tenant-ID", orgID+"-other")
	req.Header.Set("Idempotency-Key", "managed-key-wrong-tenant")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("managed api key tenant mismatch status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("tenant mismatch problem leaked api key secret: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(` + "`" + `{"name":"Read Only","scopes":["widgets:read"]}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "create-read-only-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create read-only api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var readOnlyBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &readOnlyBody); err != nil {
		t.Fatalf("decode read-only api key body: %v", err)
	}
	readOnlySecret, _ := readOnlyBody["secret"].(string)
	if readOnlySecret == "" {
		t.Fatalf("read-only api key response missing secret: %#v", readOnlyBody)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"missing-scope"}` + "`" + `))
	req.Header.Set("X-API-Key", readOnlySecret)
	req.Header.Set("X-Tenant-ID", orgID)
	req.Header.Set("Idempotency-Key", "managed-key-missing-scope")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("managed api key missing scope status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), readOnlySecret) {
		t.Fatalf("scope failure problem leaked api key secret: %s", rec.Body.String())
	}

{{ end }}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/api-keys", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), apiKeyID) {
		t.Fatalf("list api keys status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("list api keys leaked secret: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/organizations/"+orgID+"/api-keys/"+apiKeyID, nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "revoke-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke api key status = %d body=%s", rec.Code, rec.Body.String())
	}

{{ if eq .AuthMode "api-key" }}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/widgets", nil)
	req.Header.Set("X-API-Key", secret)
	req.Header.Set("X-Tenant-ID", orgID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked managed api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("revoked api key problem leaked secret: %s", rec.Body.String())
	}

{{ end }}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(` + "`" + `{"name":"Member","scopes":["widgets:read"]}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "member_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "member-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member api key create status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/invitations", strings.NewReader(` + "`" + `{"email":"second@example.com","role":"viewer"}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "member_1", "invitations:write")
	req.Header.Set("Idempotency-Key", "invite-second")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member invite status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWriteRoutesRecordAuditWithoutSecrets(t *testing.T) {
	handler, auditLog := newTestRouterWithAudit(t)
	orgID := createOrganization(t, handler, "owner_1", "Acme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/api-keys", strings.NewReader(` + "`" + `{"name":"CI","scopes":["widgets:read","widgets:write"]}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "api-keys:write")
	req.Header.Set("Idempotency-Key", "audit-api-key")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode api key body: %v", err)
	}
	secret, _ := body["secret"].(string)
	if secret == "" {
		t.Fatalf("api key response missing secret: %#v", body)
	}

	events, err := auditLog.Events(context.Background())
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal audit events: %v", err)
	}
	if !strings.Contains(string(encoded), "organization.create") || !strings.Contains(string(encoded), "api_key.create") {
		t.Fatalf("audit events missing expected actions: %s", encoded)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("audit events leaked api key secret: %s", encoded)
	}
}

func TestWebhookEndpointDeliveryAndReplayFlow(t *testing.T) {
	handler := newTestRouter(t)
	orgID := createOrganization(t, handler, "owner_1", "Acme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook-events", nil)
	authorizeTestRequestAs(t, req, "", "owner_1", "webhooks:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "widget.created") {
		t.Fatalf("webhook event catalog status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("first webhook event catalog cache header = %q", rec.Header().Get("X-Cache"))
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/webhook-events", nil)
	authorizeTestRequestAs(t, req, "", "owner_1", "webhooks:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("cached webhook event catalog status = %d cache=%q body=%s", rec.Code, rec.Header().Get("X-Cache"), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/webhook-endpoints", strings.NewReader(` + "`" + `{"url":"https://example.com/webhooks/widgets","events":["widget.created"]}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "webhooks:write")
	req.Header.Set("Idempotency-Key", "create-webhook")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create webhook endpoint status = %d body=%s", rec.Code, rec.Body.String())
	}
	var endpointBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &endpointBody); err != nil {
		t.Fatalf("decode endpoint body: %v", err)
	}
	secret, _ := endpointBody["secret"].(string)
	endpoint, _ := endpointBody["endpoint"].(map[string]any)
	endpointID, _ := endpoint["id"].(string)
	if secret == "" || endpointID == "" {
		t.Fatalf("endpoint response = %#v", endpointBody)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/webhook-endpoints", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "webhooks:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), endpointID) {
		t.Fatalf("list webhook endpoints status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("list webhook endpoints leaked signing secret: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/widgets", strings.NewReader(` + "`" + `{"name":"emits-webhook"}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "widgets:write")
	req.Header.Set("Idempotency-Key", "webhook-widget")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create widget for webhook status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/webhook-deliveries", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "webhooks:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "widget.created") || !strings.Contains(rec.Body.String(), endpointID) {
		t.Fatalf("list webhook deliveries status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("list webhook deliveries leaked signing secret: %s", rec.Body.String())
	}
	var deliveryBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &deliveryBody); err != nil {
		t.Fatalf("decode delivery list: %v", err)
	}
	items, _ := deliveryBody["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("delivery items = %#v", deliveryBody)
	}
	first, _ := items[0].(map[string]any)
	deliveryID, _ := first["id"].(string)
	if deliveryID == "" {
		t.Fatalf("delivery item missing id: %#v", first)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/webhook-deliveries/"+deliveryID+"/replay", strings.NewReader(` + "`" + `{}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "webhooks:write")
	req.Header.Set("Idempotency-Key", "replay-webhook")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), deliveryID) {
		t.Fatalf("replay webhook delivery status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("replay webhook delivery leaked signing secret: %s", rec.Body.String())
	}
}

func TestObjectStorageFlowRejectsUnsafeInputsAndDoesNotLeakPayload(t *testing.T) {
	handler := newTestRouter(t)
	orgID := createOrganization(t, handler, "owner_1", "Acme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/objects", strings.NewReader(` + "`" + `{"key":"readme.txt","content_type":"text/plain","content_base64":"aGVsbG8="}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:write")
	req.Header.Set("Idempotency-Key", "put-object")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "readme.txt") {
		t.Fatalf("put object status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "aGVsbG8=") {
		t.Fatalf("put object response leaked payload: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/objects", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "readme.txt") {
		t.Fatalf("list objects status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "aGVsbG8=") {
		t.Fatalf("list objects leaked payload: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/objects/readme.txt", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:read")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "\"content_base64\":\"aGVsbG8=\"") {
		t.Fatalf("get object status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/organizations/"+orgID+"/objects", strings.NewReader(` + "`" + `{"key":"../secret","content_type":"text/plain","content_base64":"ZG8tbm90LWxlYWs="}` + "`" + `))
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:write")
	req.Header.Set("Idempotency-Key", "put-unsafe-object")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsafe object status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "ZG8tbm90LWxlYWs=") || strings.Contains(rec.Body.String(), "do-not-leak") {
		t.Fatalf("unsafe object problem leaked payload: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/organizations/"+orgID+"/objects/readme.txt", nil)
	authorizeTestRequestAs(t, req, orgID, "owner_1", "objects:write")
	req.Header.Set("Idempotency-Key", "delete-object")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete object status = %d body=%s", rec.Code, rec.Body.String())
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
	authorizeTestRequest(t, req, "org_1")
	req.Header.Set("Idempotency-Key", idem)
	handler.ServeHTTP(rec, req)
	return rec
}

func createOrganization(t *testing.T, handler http.Handler, actorID, name string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/organizations", strings.NewReader("{\"name\":\""+name+"\"}"))
	authorizeTestRequestAs(t, req, "", actorID, "organizations:write")
	req.Header.Set("Idempotency-Key", "create-org-"+actorID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create organization status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode organization body: %v", err)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("organization body missing id: %#v", body)
	}
	return id
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	handler, _ := newTestRouterWithAudit(t)
	return handler
}

func newTestRouterWithAudit(t *testing.T) (http.Handler, *app.AuditService) {
	t.Helper()
	configureTestAuthEnv(t)
	tenancy := app.NewTenancyService()
	auditLog := app.NewAuditService()
	handler := NewRouter(RouterConfig{Widgets: app.NewWidgetService(), Tenancy: tenancy, APIKeys: app.NewAPIKeyService("test-pepper", tenancy), Audit: auditLog{{ if eq .AuthMode "jwt" }}, JWT: newTestJWT(t){{ else if eq .AuthMode "clerk" }}, Clerk: newTestClerk(t){{ else if eq .AuthMode "oidc" }}, OIDC: newTestOIDC(t){{ else }}, APIKey: "test-key"{{ end }}})
	return handler, auditLog
}

func newTestRouterWithMetrics(t *testing.T) (http.Handler, http.Handler) {
	t.Helper()
	configureTestAuthEnv(t)
	recorder, err := metricsmw.NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		t.Fatalf("new metrics recorder: %v", err)
	}
	middleware, err := NewMetricsMiddleware(recorder)
	if err != nil {
		t.Fatalf("new metrics middleware: %v", err)
	}
	tenancy := app.NewTenancyService()
	cfg := RouterConfig{
		Widgets:        app.NewWidgetService(),
		Tenancy:        tenancy,
		APIKeys:        app.NewAPIKeyService("test-pepper", tenancy),
		Audit:          app.NewAuditService(),
		Metrics:        middleware,
		MetricsHandler: metricsmw.PrometheusHandler(),
		AdminKey:       "test-admin-key",
{{ if eq .AuthMode "jwt" }}		JWT: newTestJWT(t),
{{ else if eq .AuthMode "clerk" }}		Clerk: newTestClerk(t),
{{ else if eq .AuthMode "oidc" }}		OIDC: newTestOIDC(t),
{{ else }}		APIKey: "test-key",
{{ end }}	}
	return NewRouter(cfg), NewAdminRouter(cfg)
}

func configureTestAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENV", "test")
	t.Setenv("API_ACTOR_ID", "")
}

func authorizeTestRequest(t *testing.T, req *http.Request, tenantID string) {
	t.Helper()
	authorizeTestRequestAs(t, req, tenantID, "user_123", "widgets:write")
}

func authorizeTestRequestAs(t *testing.T, req *http.Request, tenantID, actorID string, scopes ...string) {
	t.Helper()
	if len(scopes) == 0 {
		scopes = []string{"widgets:write"}
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if actorID != "" {
		req.Header.Set("X-Actor-ID", actorID)
	}
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	req.Header.Set("Authorization", "Bearer "+testBearerJWTForActor(t, actorID, tenantID, scopes...))
{{ else }}	req.Header.Set("X-API-Key", "test-key")
{{ end }}}

{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}const (
	testBearerKeyID = "test-kid"
{{ if eq .AuthMode "jwt" }}	testBearerIssuer   = "https://jwt.example.test"
	testBearerAudience = "saas-api-full"
{{ else if eq .AuthMode "clerk" }}	testBearerIssuer   = "https://clerk.example.test"
	testBearerAudience = "saas-api-full"
{{ else if eq .AuthMode "oidc" }}	testBearerIssuer   = "https://oidc.example.test"
	testBearerAudience = "saas-api-full"
{{ end }}
)

var testBearerPrivateKey *rsa.PrivateKey

{{ if eq .AuthMode "jwt" }}func newTestJWT(t *testing.T) *jwtauth.Middleware {
	t.Helper()
	server := newTestJWKSServer(t)
	mw, err := jwtauth.NewMiddleware(context.Background(), jwtauth.Config{
		Enabled:           true,
		JWKSURL:           server.URL,
		Issuer:            testBearerIssuer,
		Audience:          testBearerAudience,
		AllowedAlgorithms: []string{"RS256"},
	}, ports.NopLogger{})
	if err != nil {
		t.Fatalf("new JWT middleware: %v", err)
	}
	t.Cleanup(mw.Close)
	return mw
}

{{ else if eq .AuthMode "clerk" }}func newTestClerk(t *testing.T) *clerkauth.Middleware {
	t.Helper()
	server := newTestJWKSServer(t)
	mw, err := clerkauth.NewMiddleware(context.Background(), clerkauth.Config{
		Enabled:           true,
		JWKSURL:           server.URL,
		Issuer:            testBearerIssuer,
		Audience:          testBearerAudience,
		AllowedAlgorithms: []string{"RS256"},
	}, ports.NopLogger{})
	if err != nil {
		t.Fatalf("new Clerk middleware: %v", err)
	}
	t.Cleanup(mw.Close)
	return mw
}

{{ else if eq .AuthMode "oidc" }}
func newTestOIDC(t *testing.T) *oidcauth.Middleware {
	t.Helper()
	server := newTestJWKSServer(t)
	mw, err := oidcauth.NewMiddleware(context.Background(), oidcauth.Config{
		Enabled:           true,
		JWKSURL:           server.URL,
		Issuer:            testBearerIssuer,
		Audience:          testBearerAudience,
		TenantClaim:       "tenant_id",
		ScopeClaim:        "scope",
		AllowedAlgorithms: []string{"RS256"},
	}, ports.NopLogger{})
	if err != nil {
		t.Fatalf("new OIDC middleware: %v", err)
	}
	t.Cleanup(mw.Close)
	return mw
}

{{ end }}func newTestJWKSServer(t *testing.T) *httptest.Server {
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
	return server
}

func testBearerJWTForActor(t *testing.T, actorID, tenantID string, scopes ...string) string {
	t.Helper()
	if testBearerPrivateKey == nil {
		t.Fatal("bearer test key is not configured")
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":       actorID,
		"tenant_id": tenantID,
		"scope":     strings.Join(scopes, " "),
		"iss":       testBearerIssuer,
		"aud":       testBearerAudience,
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

CREATE INDEX memberships_user_id_idx ON memberships(user_id);

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

CREATE INDEX invitations_organization_id_idx ON invitations(organization_id);

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

CREATE UNIQUE INDEX api_keys_key_hash_idx ON api_keys(key_hash);
CREATE INDEX api_keys_organization_id_idx ON api_keys(organization_id);

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

CREATE TABLE objects (
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	key TEXT NOT NULL,
	content_type TEXT NOT NULL,
	size BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (organization_id, key)
);

CREATE TABLE webhook_endpoints (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	url TEXT NOT NULL,
	event_types TEXT[] NOT NULL,
	secret_hash BYTEA NOT NULL,
	secret_ciphertext BYTEA NOT NULL,
	secret_nonce BYTEA NOT NULL,
	disabled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_deliveries (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	endpoint_id TEXT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
	event_id TEXT NOT NULL,
	event_type TEXT NOT NULL,
	payload JSONB NOT NULL,
	state TEXT NOT NULL DEFAULT 'pending',
	attempts INTEGER NOT NULL DEFAULT 0,
	next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_status_code INTEGER,
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

.PHONY: deps test fmt build openapi-check openapi-update contracts-lint contracts-diff client-check resource-check migrate-up migrate-status migrate-check integration-check clean finalize

deps:
	$(GO) mod tidy

test: deps
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

build: deps
	$(GO) build -trimpath -o bin/api ./cmd/api
	$(GO) build -trimpath -o bin/worker ./cmd/worker
	$(GO) build -trimpath -o bin/migrate ./cmd/migrate

openapi-check: deps
	$(GO) test ./internal/httpapi -run TestOpenAPIGolden

openapi-update:
	UPDATE_OPENAPI=1 $(GO) test ./internal/httpapi -run TestOpenAPIGolden

contracts-lint:
	$(API_TOOLKIT) contracts lint --openapi $(OPENAPI)

contracts-diff:
	$(API_TOOLKIT) contracts diff --base $(OPENAPI_BASE) --head $(OPENAPI)

client-check: deps
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	cp internal/client/apiclient/client.go "$$tmp/client.go"; \
	$(API_TOOLKIT) clients go --openapi $(OPENAPI) --out internal/client/apiclient --package apiclient --style typed; \
	cmp -s "$$tmp/client.go" internal/client/apiclient/client.go || { echo "generated Go client is out of date"; diff -u "$$tmp/client.go" internal/client/apiclient/client.go; exit 1; }
	$(GO) test ./internal/client/apiclient

resource-check: test openapi-check contracts-lint contracts-diff client-check

migrate-up:
	$(GO) run ./cmd/migrate up

migrate-status:
	$(GO) run ./cmd/migrate status

migrate-check:
	$(GO) run ./cmd/migrate check

integration-check:
	bash scripts/integration_check.sh

clean:
	$(GO) clean -testcache

finalize: fmt test build openapi-check contracts-lint contracts-diff clean
`

const fullIntegrationCheckScriptTemplate = `#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for integration-check" >&2
  exit 2
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for integration-check" >&2
  exit 2
fi

read -r -a compose_cmd <<< "${COMPOSE:-docker compose}"

compose() {
  "${compose_cmd[@]}" "$@"
}

tmp_dir="$(mktemp -d)"
api_pid=""
worker_pid=""
receiver_pid=""

cleanup() {
  if [ -n "${api_pid}" ] && kill -0 "${api_pid}" 2>/dev/null; then
    kill "${api_pid}" 2>/dev/null || true
    wait "${api_pid}" 2>/dev/null || true
  fi
  if [ -n "${worker_pid}" ] && kill -0 "${worker_pid}" 2>/dev/null; then
    kill "${worker_pid}" 2>/dev/null || true
    wait "${worker_pid}" 2>/dev/null || true
  fi
  if [ -n "${receiver_pid}" ] && kill -0 "${receiver_pid}" 2>/dev/null; then
    kill "${receiver_pid}" 2>/dev/null || true
    wait "${receiver_pid}" 2>/dev/null || true
  fi
  # Default cleanup command: docker compose --profile objectstore down -v.
  compose --profile objectstore down -v
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

wait_for_postgres() {
  for _ in $(seq 1 60); do
    if compose exec -T postgres pg_isready -U api -d api >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "postgres did not become ready" >&2
  return 1
}

wait_for_redis() {
  for _ in $(seq 1 60); do
    if [ "$(compose exec -T redis redis-cli ping 2>/dev/null | tr -d '\r')" = "PONG" ]; then
      return 0
    fi
    sleep 1
  done
  echo "redis did not become ready" >&2
  return 1
}

wait_for_minio() {
  for _ in $(seq 1 60); do
    if curl -fsS "${S3_ENDPOINT}/minio/health/ready" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "minio did not become ready" >&2
  return 1
}

wait_for_http() {
  local url="$1"
  for _ in $(seq 1 90); do
    if curl -fsS "${url}/readyz" >/dev/null 2>&1; then
      return 0
    fi
    if [ -n "${api_pid}" ] && ! kill -0 "${api_pid}" 2>/dev/null; then
      echo "api process exited before readiness" >&2
      sed -n '1,120p' "${tmp_dir}/api.log" >&2 || true
      return 1
    fi
    sleep 1
  done
  echo "api did not become ready" >&2
  sed -n '1,120p' "${tmp_dir}/api.log" >&2 || true
  return 1
}

wait_for_receiver() {
  local url="$1"
  for _ in $(seq 1 30); do
    if curl -fsS "${url}/readyz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "webhook receiver did not become ready" >&2
  return 1
}

receiver_count() {
  local file="$1"
  if [ ! -f "${file}" ]; then
    printf '0\n'
    return 0
  fi
  wc -l <"${file}" | tr -d '[:space:]'
}

json_field() {
  local path="$1"
  local file="$2"
  python3 - "${path}" "${file}" <<'PY'
import json
import sys

path = sys.argv[1].split(".")
with open(sys.argv[2], "r", encoding="utf-8") as handle:
    data = json.load(handle)
for item in path:
    if isinstance(data, dict):
        data = data[item]
    elif isinstance(data, list):
        data = data[int(item)]
    else:
        raise KeyError(item)
if data is None:
    raise SystemExit(1)
print(data)
PY
}

header_value() {
  local name="$1"
  local file="$2"
  awk -v want="$(printf '%s' "${name}" | tr '[:upper:]' '[:lower:]')" '
    BEGIN { FS = ":" }
    {
      key = tolower($1)
      if (key == want) {
        sub(/^[^:]*:[ \t]*/, "", $0)
        sub(/\r$/, "", $0)
        print $0
        exit
      }
    }
  ' "${file}"
}

psql_exec() {
  local sql="$1"
  shift
  compose exec -T postgres psql -v ON_ERROR_STOP=1 -U api -d api "$@" <<<"${sql}"
}

psql_scalar() {
  local sql="$1"
  shift
  compose exec -T postgres psql -v ON_ERROR_STOP=1 -tA -U api -d api "$@" <<<"${sql}" | tr -d '[:space:]'
}

export ENV=integration
export API_ADDR="${INTEGRATION_API_ADDR:-127.0.0.1:18080}"
export ADMIN_ADDR="${INTEGRATION_ADMIN_ADDR:-127.0.0.1:19090}"
default_db_user="${POSTGRES_USER:-api}"
default_db_password="${POSTGRES_PASSWORD:-api}"
export DATABASE_URL="${DATABASE_URL:-postgres://${default_db_user}:${default_db_password}@localhost:5432/api?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
export CACHE_STORE="${CACHE_STORE:-redis}"
export RATE_LIMIT_STORE="${RATE_LIMIT_STORE:-redis}"
export IDEMPOTENCY_STORE="${IDEMPOTENCY_STORE:-redis}"
export API_KEY="${API_KEY:-local-dev-key}"
export ADMIN_KEY="${ADMIN_KEY:-local-admin-key}"
export API_ACTOR_ID="${API_ACTOR_ID:-integration-actor}"
export API_KEY_PEPPER="${API_KEY_PEPPER:-integration-pepper-change-me}"
export WEBHOOK_SECRET_KEY="${WEBHOOK_SECRET_KEY:-local-webhook-secret-key-1234567}"
export OBJECT_STORE="${INTEGRATION_OBJECT_STORE:-${OBJECT_STORE:-memory}}"
export S3_ENDPOINT="${S3_ENDPOINT:-http://localhost:9000}"
export S3_REGION="${S3_REGION:-us-east-1}"
export S3_BUCKET="${S3_BUCKET:-api-objects}"
export S3_ACCESS_KEY_ID="${S3_ACCESS_KEY_ID:-minio}"
export S3_SECRET_ACCESS_KEY="${S3_SECRET_ACCESS_KEY:-minio123}"
export WEBHOOK_RECEIVER_ADDR="${WEBHOOK_RECEIVER_ADDR:-127.0.0.1:18081}"

api_url="http://${API_ADDR}"
admin_url="http://${ADMIN_ADDR}"
webhook_receiver_url="http://${WEBHOOK_RECEIVER_ADDR}"
webhook_receiver_log="${tmp_dir}/webhook-receiver.ndjson"

if [ ! -f .env ]; then
  cp .env.example .env
fi

if [ "${OBJECT_STORE}" = "s3" ]; then
  compose --profile objectstore up -d postgres redis minio
  wait_for_minio
  compose --profile objectstore run --rm minio-init
else
  compose up -d postgres redis
fi
wait_for_postgres
wait_for_redis

go mod tidy
go run ./cmd/migrate up
go run ./cmd/migrate check
go test ./...

python3 - "${WEBHOOK_RECEIVER_ADDR}" "${webhook_receiver_log}" <<'PY' >"${tmp_dir}/webhook-receiver.log" 2>&1 &
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

addr = sys.argv[1]
log_path = sys.argv[2]
host, port = addr.rsplit(":", 1)

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/readyz":
            self.send_response(204)
            self.end_headers()
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length).decode("utf-8", "replace")
        record = {
            "path": self.path,
            "headers": {key: value for key, value in self.headers.items()},
            "body": body,
        }
        with open(log_path, "a", encoding="utf-8") as handle:
            handle.write(json.dumps(record, sort_keys=True) + "\n")
        if self.path.startswith("/fail"):
            self.send_response(500)
            self.end_headers()
            self.wfile.write(b"retry")
            return
        self.send_response(204)
        self.end_headers()

    def log_message(self, *_):
        return

ThreadingHTTPServer((host, int(port)), Handler).serve_forever()
PY
receiver_pid="$!"
wait_for_receiver "${webhook_receiver_url}"

export ASYNC_WORKER_ENABLED=false
go run ./cmd/worker >"${tmp_dir}/worker.log" 2>&1 &
worker_pid="$!"
sleep 1
if ! kill -0 "${worker_pid}" 2>/dev/null; then
  echo "worker process exited before smoke checks" >&2
  sed -n '1,120p' "${tmp_dir}/worker.log" >&2 || true
  exit 1
fi

go run ./cmd/api >"${tmp_dir}/api.log" 2>&1 &
api_pid="$!"
wait_for_http "${api_url}"

curl -fsS "${api_url}/livez" >/dev/null
curl -fsS "${api_url}/docs/openapi.json" >/dev/null

auth_status="$(curl -sS -o "${tmp_dir}/auth.json" -w '%{http_code}' "${api_url}/organizations")"
if [ "${auth_status}" != "401" ]; then
  echo "expected unauthenticated organization request to return 401, got ${auth_status}" >&2
  sed -n '1,80p' "${tmp_dir}/auth.json" >&2 || true
  exit 1
fi

org_json="$(curl -fsS -X POST "${api_url}/organizations" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "Idempotency-Key: integration-create-organization" \
  --data '{"name":"Integration"}')"
printf '%s' "${org_json}" >"${tmp_dir}/organization.json"
org_id="$(json_field id "${tmp_dir}/organization.json")"
if [ -z "${org_id}" ]; then
  echo "create organization response did not include id" >&2
  exit 1
fi

poison_outbox_id="integration-poison-outbox"
psql_exec "insert into outbox_events (id, organization_id, event_type, payload, state, next_at, created_at) values (:'outbox_id', :'organization_id', 'integration.poison', '{}'::jsonb, 'pending', now(), now()) on conflict (id) do update set state='pending', lease_owner=null, lease_expires_at=null, retry_count=0, next_at=now();" \
  -v organization_id="${org_id}" \
  -v outbox_id="${poison_outbox_id}" >/dev/null
poison_deadletter_outbox_id="integration-poison-outbox-deadletter"
psql_exec "insert into outbox_events (id, organization_id, event_type, payload, state, retry_count, next_at, created_at) values (:'outbox_id', :'organization_id', 'integration.poison.deadletter', '{}'::jsonb, 'pending', 9, now(), now()) on conflict (id) do update set state='pending', lease_owner=null, lease_expires_at=null, retry_count=9, next_at=now();" \
  -v organization_id="${org_id}" \
  -v outbox_id="${poison_deadletter_outbox_id}" >/dev/null

curl -fsS "${api_url}/organizations/${org_id}/members" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" >/dev/null

api_key_json="$(curl -fsS -X POST "${api_url}/organizations/${org_id}/api-keys" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-create-managed-api-key" \
  --data '{"name":"Integration","scopes":["widgets:read","widgets:write","operations:read"]}')"
printf '%s' "${api_key_json}" >"${tmp_dir}/api-key.json"
managed_api_key="$(json_field secret "${tmp_dir}/api-key.json")"
if [ -z "${managed_api_key}" ]; then
  echo "create API key response did not include secret" >&2
  exit 1
fi

webhook_endpoint_body="${tmp_dir}/webhook-endpoint.json"
curl -fsS -X POST "${api_url}/organizations/${org_id}/webhook-endpoints" \
  -o "${webhook_endpoint_body}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-create-webhook-endpoint" \
  --data '{"url":"'"${webhook_receiver_url}"'/webhooks/widgets","events":["widget.created"]}'
webhook_secret="$(json_field secret "${webhook_endpoint_body}")"
if [ -z "${webhook_secret}" ]; then
  echo "create webhook endpoint response did not include signing secret" >&2
  exit 1
fi

curl -fsS "${api_url}/organizations/${org_id}/webhook-endpoints" \
  -o "${tmp_dir}/webhook-endpoints.json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" >/dev/null
if grep -F -q -- "${webhook_secret}" "${tmp_dir}/webhook-endpoints.json"; then
  echo "webhook endpoint list leaked signing secret" >&2
  exit 1
fi

webhook_failure_endpoint_body="${tmp_dir}/webhook-failure-endpoint.json"
curl -fsS -X POST "${api_url}/organizations/${org_id}/webhook-endpoints" \
  -o "${webhook_failure_endpoint_body}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-create-failing-webhook-endpoint" \
  --data '{"url":"'"${webhook_receiver_url}"'/fail/widgets","events":["widget.updated"]}'
failure_endpoint_id="$(json_field endpoint.id "${webhook_failure_endpoint_body}")"
if [ -z "${failure_endpoint_id}" ]; then
  echo "create failing webhook endpoint response did not include id" >&2
  exit 1
fi

widget_headers="${tmp_dir}/widget-create.headers"
widget_body="${tmp_dir}/widget-create.json"
curl -fsS -X POST "${api_url}/widgets" \
  -D "${widget_headers}" \
  -o "${widget_body}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-managed-key-widget" \
  --data '{"name":"managed-key-widget"}'
widget_id="$(json_field id "${widget_body}")"
widget_etag="$(header_value ETag "${widget_headers}")"
if [ -z "${widget_id}" ] || [ -z "${widget_etag}" ]; then
  echo "create widget response did not include id and ETag" >&2
  exit 1
fi

replay_headers="${tmp_dir}/widget-replay.headers"
replay_body="${tmp_dir}/widget-replay.json"
replay_status="$(curl -sS -D "${replay_headers}" -o "${replay_body}" -w '%{http_code}' -X POST "${api_url}/widgets" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-managed-key-widget" \
  --data '{"name":"managed-key-widget"}')"
replay_id="$(json_field id "${replay_body}")"
replay_header="$(header_value Idempotency-Replayed "${replay_headers}")"
if [ "${replay_status}" != "201" ] || [ "${replay_id}" != "${widget_id}" ] || [ "${replay_header}" != "true" ]; then
  echo "expected idempotent widget replay, got status ${replay_status}, id ${replay_id}, replay header ${replay_header}" >&2
  exit 1
fi

delivery_id=""
delivery_state=""
for _ in $(seq 1 30); do
  curl -fsS "${api_url}/organizations/${org_id}/webhook-deliveries" \
    -o "${tmp_dir}/webhook-deliveries.json" \
    -H "X-API-Key: ${API_KEY}" \
    -H "X-Actor-ID: ${API_ACTOR_ID}" \
    -H "X-Tenant-ID: ${org_id}" >/dev/null
  delivery_id="$(json_field items.0.id "${tmp_dir}/webhook-deliveries.json" 2>/dev/null || true)"
  delivery_state="$(psql_scalar "select state from webhook_deliveries where organization_id = :'organization_id' and id = :'delivery_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v delivery_id="${delivery_id}")"
  if [ -n "${delivery_id}" ] && [ "${delivery_state}" = "succeeded" ] && [ "$(receiver_count "${webhook_receiver_log}")" -ge 1 ]; then
    break
  fi
  sleep 1
done
if [ -z "${delivery_id}" ]; then
  echo "widget create did not enqueue webhook delivery" >&2
  exit 1
fi
if [ "${delivery_state}" != "succeeded" ]; then
  echo "webhook delivery did not succeed, last state ${delivery_state}" >&2
  sed -n '1,80p' "${tmp_dir}/webhook-deliveries.json" >&2 || true
  sed -n '1,80p' "${tmp_dir}/worker.log" >&2 || true
  exit 1
fi
if grep -F -q -- "${webhook_secret}" "${tmp_dir}/webhook-deliveries.json"; then
  echo "webhook delivery list leaked signing secret" >&2
  exit 1
fi
if grep -F -q -- "${webhook_secret}" "${webhook_receiver_log}"; then
  echo "webhook receiver log leaked signing secret" >&2
  exit 1
fi

curl -fsS -X POST "${api_url}/organizations/${org_id}/webhook-deliveries/${delivery_id}/replay" \
  -o "${tmp_dir}/webhook-replay.json" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-replay-webhook-delivery" \
  --data '{}'
replayed_delivery_id="$(json_field id "${tmp_dir}/webhook-replay.json")"
if [ "${replayed_delivery_id}" != "${delivery_id}" ]; then
  echo "replay webhook delivery returned ${replayed_delivery_id}, want ${delivery_id}" >&2
  exit 1
fi
for _ in $(seq 1 30); do
  if [ "$(receiver_count "${webhook_receiver_log}")" -ge 2 ]; then
    break
  fi
  sleep 1
done
if [ "$(receiver_count "${webhook_receiver_log}")" -lt 2 ]; then
  echo "replay webhook delivery did not reach local receiver" >&2
  sed -n '1,80p' "${webhook_receiver_log}" >&2 || true
  exit 1
fi

precondition_status="$(curl -sS -o "${tmp_dir}/widget-stale.json" -w '%{http_code}' -X PATCH "${api_url}/widgets/${widget_id}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "If-Match: stale-etag" \
  -H "Idempotency-Key: integration-widget-stale-etag" \
  --data '{"name":"stale-update"}')"
if [ "${precondition_status}" != "412" ]; then
  echo "expected stale widget update to return 412, got ${precondition_status}" >&2
  sed -n '1,80p' "${tmp_dir}/widget-stale.json" >&2 || true
  exit 1
fi

curl -fsS -X PATCH "${api_url}/widgets/${widget_id}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "If-Match: ${widget_etag}" \
  -H "Idempotency-Key: integration-widget-update" \
  --data '{"name":"managed-key-widget-updated"}' >/dev/null

failure_delivery_state=""
failure_retry_count=""
for _ in $(seq 1 30); do
  failure_delivery_state="$(psql_scalar "select state from webhook_deliveries where organization_id = :'organization_id' and endpoint_id = :'endpoint_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v endpoint_id="${failure_endpoint_id}")"
  failure_retry_count="$(psql_scalar "select retry_count from outbox_events where organization_id = :'organization_id' and event_type = 'webhook.delivery' and payload->>'endpoint_id' = :'endpoint_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v endpoint_id="${failure_endpoint_id}")"
  if [ "${failure_delivery_state}" = "failed" ] && [ -n "${failure_retry_count}" ] && [ "${failure_retry_count}" -ge 1 ]; then
    break
  fi
  sleep 1
done
if [ "${failure_delivery_state}" != "failed" ] || [ -z "${failure_retry_count}" ] || [ "${failure_retry_count}" -lt 1 ]; then
  echo "failing webhook did not record retryable failure; state=${failure_delivery_state} retry_count=${failure_retry_count}" >&2
  sed -n '1,120p' "${tmp_dir}/worker.log" >&2 || true
  exit 1
fi

import_body="${tmp_dir}/widget-import.json"
curl -fsS -X POST "${api_url}/widgets/imports" \
  -o "${import_body}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${managed_api_key}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-widget-import" \
  --data '{"items":[{"name":"imported-widget"}]}'
operation_id="$(json_field id "${import_body}")"
if [ -z "${operation_id}" ]; then
  echo "widget import response did not include operation id" >&2
  exit 1
fi
operation_state=""
for _ in $(seq 1 30); do
  curl -fsS "${api_url}/operations/${operation_id}" \
    -o "${tmp_dir}/operation.json" \
    -H "X-API-Key: ${managed_api_key}" \
    -H "X-Tenant-ID: ${org_id}"
  operation_state="$(json_field state "${tmp_dir}/operation.json")"
  if [ "${operation_state}" = "succeeded" ]; then
    break
  fi
  sleep 1
done
if [ "${operation_state}" != "succeeded" ]; then
  echo "operation did not succeed, last state ${operation_state}" >&2
  sed -n '1,80p' "${tmp_dir}/operation.json" >&2 || true
  exit 1
fi

operation_outbox_state=""
for _ in $(seq 1 30); do
  operation_outbox_state="$(psql_scalar "select state from outbox_events where organization_id = :'organization_id' and event_type = 'widgets.import' and payload->>'operation_id' = :'operation_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v operation_id="${operation_id}")"
  if [ "${operation_outbox_state}" = "succeeded" ]; then
    break
  fi
  sleep 1
done
if [ "${operation_outbox_state}" != "succeeded" ]; then
  echo "operation outbox did not complete, last state ${operation_outbox_state}" >&2
  exit 1
fi

poison_retry_count=""
for _ in $(seq 1 30); do
  poison_retry_count="$(psql_scalar "select retry_count from outbox_events where organization_id = :'organization_id' and id = :'outbox_id' and retry_count >= 1 order by retry_count desc limit 1;" \
    -v organization_id="${org_id}" \
    -v outbox_id="${poison_outbox_id}")"
  if [ -n "${poison_retry_count}" ]; then
    break
  fi
  sleep 1
done
case "${poison_retry_count}" in
  ""|*[!0-9]*)
    echo "outbox retry was not recorded" >&2
    exit 1
    ;;
esac

poison_deadletter_state=""
for _ in $(seq 1 30); do
  poison_deadletter_state="$(psql_scalar "select state from outbox_events where organization_id = :'organization_id' and id = :'outbox_id' order by created_at desc limit 1;" \
    -v organization_id="${org_id}" \
    -v outbox_id="${poison_deadletter_outbox_id}")"
  if [ "${poison_deadletter_state}" = "dead_letter" ]; then
    break
  fi
  sleep 1
done
if [ "${poison_deadletter_state}" != "dead_letter" ]; then
  echo "outbox dead-letter was not recorded, last state ${poison_deadletter_state}" >&2
  exit 1
fi

curl -fsS -X POST "${api_url}/organizations/${org_id}/objects" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" \
  -H "Idempotency-Key: integration-put-object" \
  --data '{"key":"integration.txt","content_type":"text/plain","content_base64":"aGVsbG8="}' >/dev/null

curl -fsS "${api_url}/organizations/${org_id}/objects/integration.txt" \
  -o "${tmp_dir}/object-get.json" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Actor-ID: ${API_ACTOR_ID}" \
  -H "X-Tenant-ID: ${org_id}" >/dev/null
object_content="$(json_field content_base64 "${tmp_dir}/object-get.json")"
if [ "${object_content}" != "aGVsbG8=" ]; then
  echo "object get did not return stored content" >&2
  exit 1
fi

audit_count="$(psql_scalar "select count(*) from audit_events where organization_id = :'organization_id';" \
  -v organization_id="${org_id}")"
case "${audit_count}" in
  ""|*[!0-9]*)
    echo "audit event count query returned ${audit_count}" >&2
    exit 1
    ;;
esac
if [ "${audit_count}" -lt 1 ]; then
  echo "audit events were not recorded" >&2
  exit 1
fi

curl -fsS "${admin_url}/health/detailed" \
  -H "X-Admin-Key: ${ADMIN_KEY}" >/dev/null

curl -fsS "${admin_url}/metrics" \
  -H "X-Admin-Key: ${ADMIN_KEY}" | grep -q "http_requests_total"

curl -fsS "${admin_url}/debug/pprof/" \
  -H "X-Admin-Key: ${ADMIN_KEY}" | grep -q "Types of profiles available"

metrics_status="$(curl -sS -o "${tmp_dir}/metrics-unauthorized.txt" -w '%{http_code}' "${admin_url}/metrics")"
if [ "${metrics_status}" != "401" ]; then
  echo "expected unauthenticated admin metrics request to return 401, got ${metrics_status}" >&2
  exit 1
fi

public_pprof_status="$(curl -sS -o "${tmp_dir}/public-pprof.html" -w '%{http_code}' "${api_url}/debug/pprof/")"
if [ "${public_pprof_status}" != "404" ]; then
  echo "public pprof endpoint should be isolated; got ${public_pprof_status}" >&2
  exit 1
fi

echo "integration-check passed"
`

const fullEnvTemplate = `ENV=development
API_ADDR=:8080
ADMIN_ADDR=:9090
DATABASE_URL=
REDIS_ADDR=localhost:6379
CACHE_STORE=memory
RATE_LIMIT_STORE=memory
RATE_LIMIT_KEY_PREFIX=ratelimit:
IDEMPOTENCY_STORE=memory
IDEMPOTENCY_KEY_PREFIX=idempotency:
OPENAPI_REQUEST_VALIDATION=true
OPENAPI_RESPONSE_VALIDATION=true
ASYNC_WORKER_ENABLED=true
OBJECT_STORE=memory
S3_ENDPOINT=http://localhost:9000
S3_REGION=us-east-1
S3_BUCKET=api-objects
S3_ACCESS_KEY_ID=
S3_SECRET_ACCESS_KEY=
API_KEY=local-dev-key
API_ACTOR_ID=
API_KEY_PEPPER=
WEBHOOK_SECRET_KEY=
ADMIN_KEY=local-admin-key
{{ if eq .HasStripeBilling "true" }}STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
STRIPE_PRICE_ID=
STRIPE_SUCCESS_URL=http://localhost:8080/billing/success
STRIPE_CANCEL_URL=http://localhost:8080/billing/cancel
{{ end }}{{ if eq .HasResendEmail "true" }}RESEND_API_KEY=
RESEND_FROM=
APP_BASE_URL=http://localhost:8080
{{ end }}{{ if eq .HasClerkWebhooks "true" }}CLERK_WEBHOOK_SECRET=
{{ end }}
{{ if eq .AuthMode "jwt" }}JWT_JWKS_URL=
JWT_ISSUER=
JWT_AUDIENCE=saas-api-full
{{ else if eq .AuthMode "clerk" }}CLERK_JWKS_URL=
CLERK_ISSUER=
CLERK_AUDIENCE=saas-api-full
{{ else if eq .AuthMode "oidc" }}
OIDC_ISSUER=
OIDC_AUDIENCE=saas-api-full
OIDC_JWKS_URL=
OIDC_DISCOVERY_URL=
OIDC_TENANT_CLAIM=tenant_id
OIDC_SCOPE_CLAIM=scope
{{ end }}
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

const fullIntegrationWorkflowTemplate = `name: integration

on:
  workflow_dispatch:
  schedule:
    - cron: "17 3 * * 1"

permissions:
  contents: read

jobs:
  docker-integration:
    runs-on: ubuntu-latest
    env:
      GOTOOLCHAIN: local
      COMPOSE: docker compose
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # actions/checkout v4
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # actions/setup-go v5
        with:
          go-version: 1.25.x
          check-latest: true
      - name: Verify Docker Compose
        run: docker compose version
      - name: Integration Check
        run: make integration-check
`

const fullDockerfileTemplate = `FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/api /api
COPY --from=build /out/worker /worker
COPY --from=build /out/migrate /migrate
COPY migrations /migrations
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
      RATE_LIMIT_STORE: redis
      IDEMPOTENCY_STORE: redis
      ADMIN_ADDR: :9090
      ASYNC_WORKER_ENABLED: "false"
      WEBHOOK_SECRET_KEY: local-webhook-secret-key-1234567
    depends_on:
      migrate:
        condition: service_completed_successfully
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
  migrate:
    build: .
    entrypoint: ["/migrate"]
    command: ["-dir", "/migrations", "up"]
    env_file:
      - .env
    environment:
      DATABASE_URL: postgres://api:api@postgres:5432/api?sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy
  worker:
    build: .
    entrypoint: ["/worker"]
    env_file:
      - .env
    environment:
      DATABASE_URL: postgres://api:api@postgres:5432/api?sslmode=disable
      REDIS_ADDR: redis:6379
      WEBHOOK_SECRET_KEY: local-webhook-secret-key-1234567
    depends_on:
      migrate:
        condition: service_completed_successfully
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
      - postgres-data:/var/lib/postgresql
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
  minio-init:
    image: minio/mc:latest
    profiles: [objectstore]
    depends_on:
      - minio
    entrypoint: >
      /bin/sh -c "mc alias set local http://minio:9000 minio minio123 &&
      mc mb --ignore-existing local/api-objects"

volumes:
  postgres-data:
  redis-data:
  minio-data:
`

const fullKubernetesConfigMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: api-config
data:
  ENV: "production"
  API_ADDR: ":8080"
  ADMIN_ADDR: ":9090"
  REDIS_ADDR: "redis.example.internal:6379"
  CACHE_STORE: "redis"
  RATE_LIMIT_STORE: "redis"
  RATE_LIMIT_KEY_PREFIX: "ratelimit:"
  IDEMPOTENCY_STORE: "redis"
  IDEMPOTENCY_KEY_PREFIX: "idempotency:"
  OPENAPI_REQUEST_VALIDATION: "true"
  OPENAPI_RESPONSE_VALIDATION: "false"
  ASYNC_WORKER_ENABLED: "true"
  OBJECT_STORE: "memory"
`

// #nosec G101 -- generated Kubernetes Secret values are non-production placeholders that users must replace.
const fullKubernetesSecretTemplate = `apiVersion: v1
kind: Secret
metadata:
  name: api-secrets
type: Opaque
stringData:
  database-url: "postgres://user:password@postgres.example.internal:5432/api?sslmode=require"
  redis-addr: "redis.example.internal:6379"
  api-key: "replace-with-bootstrap-api-key"
  admin-key: "replace-with-admin-api-key"
  api-key-pepper: "replace-with-random-32-byte-secret"
  webhook-secret-key: "replace-with-random-32-byte-secret"
`

const fullKubernetesMigrationJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: api-migrate
spec:
  backoffLimit: 3
  activeDeadlineSeconds: 900
  template:
    metadata:
      labels:
        app: api-migrate
    spec:
      restartPolicy: OnFailure
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: migrate
          image: example/api:dev
          command: ["/migrate"]
          args: ["-dir", "/migrations", "up"]
          envFrom:
            - configMapRef:
                name: api-config
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: database-url
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
`

const fullKubernetesDeploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  labels:
    app: api
spec:
  replicas: 2
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: api
          image: example/api:dev
          imagePullPolicy: IfNotPresent
          ports:
            - name: public
              containerPort: 8080
            - name: admin
              containerPort: 9090
          envFrom:
            - configMapRef:
                name: api-config
          env:
            - name: ASYNC_WORKER_ENABLED
              value: "false"
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
            - name: API_KEY
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: api-key
            - name: ADMIN_KEY
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: admin-key
            - name: API_KEY_PEPPER
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: api-key-pepper
            - name: WEBHOOK_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: webhook-secret-key
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 1000m
              memory: 512Mi
          readinessProbe:
            httpGet:
              path: /readyz
              port: public
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 2
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /livez
              port: public
            initialDelaySeconds: 10
            periodSeconds: 20
            timeoutSeconds: 2
            failureThreshold: 3
`

const fullKubernetesWorkerDeploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-worker
  labels:
    app: api-worker
spec:
  replicas: 1
  selector:
    matchLabels:
      app: api-worker
  template:
    metadata:
      labels:
        app: api-worker
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: worker
          image: example/api:dev
          command: ["/worker"]
          envFrom:
            - configMapRef:
                name: api-config
          env:
            - name: ASYNC_WORKER_ENABLED
              value: "true"
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: database-url
            - name: WEBHOOK_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: api-secrets
                  key: webhook-secret-key
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 1000m
              memory: 512Mi
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
  annotations:
    api-toolkit.dev/internal-only: "true"
spec:
  type: ClusterIP
  selector:
    app: api
  ports:
    - name: admin
      port: 9090
      targetPort: admin
`

const fullKubernetesPDBTemplate = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: api
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: api
`

const fullKubernetesHPATemplate = `apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api
spec:
  minReplicas: 2
  maxReplicas: 10
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
`

const fullKubernetesNetworkPolicyTemplate = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: api
spec:
  podSelector:
    matchExpressions:
      - key: app
        operator: In
        values: ["api", "api-worker", "api-migrate"]
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector: {}
      ports:
        - protocol: TCP
          port: 8080
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: api-admin
      ports:
        - protocol: TCP
          port: 9090
  egress:
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - protocol: TCP
          port: 443
        - protocol: TCP
          port: 5432
        - protocol: TCP
          port: 6379
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
`

const fullReadmeTemplate = `# Generated api-toolkit Full SaaS API

Generated profile: ` + "`{{ .Profile }}`" + `.
Generated auth mode: ` + "`{{ .AuthMode }}`" + `.

Run locally:

` + "```sh" + `
make test
go run ./cmd/api
` + "```" + `

Postgres stores tenants, API keys, widgets, operations, outbox, audit, webhook delivery state, and object metadata.
When ` + "`DATABASE_URL`" + ` is set, ` + "`WEBHOOK_SECRET_KEY`" + ` must be a 32-byte raw or base64-encoded key used to encrypt webhook endpoint signing secrets at rest.
Redis is used for shared idempotency, rate limiting, and cache state in production. Local development uses ` + "`CACHE_STORE=memory`" + `, ` + "`RATE_LIMIT_STORE=memory`" + `, and ` + "`IDEMPOTENCY_STORE=memory`" + ` unless you opt into Redis.
When ` + "`DATABASE_URL`" + ` is set, startup opens a pgx pool, checks required platform tables, and readiness reflects database health.
The generated binary uses ` + "`bootstrap.NewAPIService`" + ` for public/admin listeners, graceful shutdown, background workers, strict middleware order validation, and safe system endpoint mounting.
` + "`cmd/worker`" + ` runs background jobs without serving public HTTP traffic. Set ` + "`ASYNC_WORKER_ENABLED=false`" + ` on API deployments when you run dedicated worker replicas.
` + "`cmd/migrate`" + ` applies and checks contrib migrator-compatible SQL files under ` + "`migrations/`" + `. Docker Compose and Kubernetes assets run it before API/worker startup.
` + "`/livez`" + ` is a process liveness probe and never checks Postgres, Redis, or S3; ` + "`/readyz`" + ` reflects configured dependencies.
Runtime OpenAPI request validation is enabled by default. Response validation is enabled in development/test or when ` + "`OPENAPI_RESPONSE_VALIDATION=true`" + `.
The public router emits bounded Prometheus HTTP request metrics, and ` + "`/metrics`" + ` is served only from the admin router.
The admin router mounts real Go pprof handlers behind ` + "`X-Admin-Key`" + `; the public router does not mount pprof when ` + "`ADMIN_ADDR`" + ` is set.
Write routes record audit events with redaction-safe metadata; raw API-key secrets, invitation tokens, webhook signing secrets, and idempotency keys are not audit metadata.
The generated HTTP layer starts with organization creation/listing, member listing, invitation creation/acceptance, tenant isolation, tenant-scoped idempotent widget writes, async widget imports with pollable operation state, outbound webhook endpoint/delivery/replay routes, and strict tenant-scoped object storage routes. API-key, JWT, Clerk, and OIDC modes are wired with fail-closed startup validation.
Unsafe write routes require ` + "`Idempotency-Key`" + `. Organization-scoped routes require ` + "`X-Tenant-ID`" + ` to match the organization path parameter.
API-key mode keeps ` + "`API_KEY`" + ` as a bootstrap setup credential and verifies generated scoped API keys through the API-key service after setup. Bootstrap requests use ` + "`API_ACTOR_ID`" + ` for production actor identity; in non-production only, tests and local tools may send ` + "`X-Actor-ID`" + ` before a generated API key exists.
{{ if eq .HasProviderWorkflows "true" }}
Optional provider workflows generated: {{ .ProviderWorkflows }}.
{{ if eq .HasStripeBilling "true" }}` + "`internal/providers/stripebilling`" + ` creates tenant-scoped checkout sessions and verifies Stripe webhooks before audit writes.
{{ end }}{{ if eq .HasResendEmail "true" }}` + "`internal/providers/resendemail`" + ` sends invitation emails through a sender boundary with a no-op local fallback.
{{ end }}{{ if eq .HasClerkWebhooks "true" }}` + "`internal/providers/clerkwebhooks`" + ` verifies signed Clerk callbacks before user or organization sync hooks run.
{{ end }}Provider SDKs and provider-specific imports stay in the generated app module; the toolkit root module remains provider-neutral.
{{ end }}

Useful checks:

` + "```sh" + `
make openapi-check
make contracts-lint
make contracts-diff
make integration-check
` + "```" + `

` + "`make integration-check`" + ` is opt-in and starts Postgres and Redis through Docker Compose, applies the generated migration, hydrates module sums with ` + "`go mod tidy`" + `, runs ` + "`go test ./...`" + `, starts the worker and API on localhost, and performs HTTP smoke checks for liveness, readiness, OpenAPI, auth failure, tenant routes, managed API-key auth, idempotent widget writes, ETag conflict handling, async operation polling, outbox completion/retry behavior, webhook delivery/replay, object write/readback, audit writes, admin health, admin metrics, admin pprof, and public admin-route isolation. Set ` + "`INTEGRATION_OBJECT_STORE=s3`" + ` to include MinIO-backed S3 object storage in the same script. The default finalize target stays local and deterministic.

Admin routes are intended for a separate listener when ` + "`ADMIN_ADDR`" + ` is set. Keep ` + "`/health/detailed`" + `, ` + "`/metrics`" + `, and ` + "`/debug/pprof/`" + ` behind admin authentication and network isolation.
`

const goModTemplate = `module {{ .Module }}

go 1.25.0

require (
	github.com/aatuh/api-toolkit/v2 {{ .CoreVersion }}
	github.com/aatuh/api-toolkit/contrib/v2 {{ .ContribVersion }}
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	github.com/golang-jwt/jwt/v5 v5.3.0
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
{{ end }}{{ if eq .AuthMode "oidc" }}	oidcauth "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/oidc"
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
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	specRegistry.RegisterSecurityScheme("BearerAuth", specs.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"})
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
		ReadinessChecks: []string{"basic"{{ if eq .AuthMode "jwt" }}, "jwt"{{ else if eq .AuthMode "clerk" }}, "clerk"{{ else if eq .AuthMode "oidc" }}, "oidc"{{ end }}},
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
{{ else if eq .AuthMode "oidc" }}	oidcMiddleware, oidcConfig, err := newOIDCMiddleware(context.Background())
	if err != nil {
		return nil, err
	}
	healthManager.RegisterChecker(oidcauth.HealthChecker(oidcConfig, nil))
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
{{ else if eq .AuthMode "oidc" }}	shutdownHooks = append(shutdownHooks, bootstrap.ShutdownHook{Name: "oidc", Hook: func(context.Context) error {
		oidcMiddleware.Close()
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
{{ else if eq .AuthMode "oidc" }}			widgetHandler = requireOIDCScope("widgets:write")(widgetHandler)
			widgetHandler = tenantMiddleware.Handler(widgetHandler)
			widgetHandler = withOIDCAuthorizationScope(widgetHandler)
			widgetHandler = oidcMiddleware.Handler(widgetHandler)
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

{{ else if eq .AuthMode "oidc" }}func newOIDCMiddleware(ctx context.Context) (*oidcauth.Middleware, oidcauth.Config, error) {
	issuer, err := requiredEnv("OIDC_ISSUER")
	if err != nil {
		return nil, oidcauth.Config{}, err
	}
	audience, err := requiredEnv("OIDC_AUDIENCE")
	if err != nil {
		return nil, oidcauth.Config{}, err
	}
	jwksURL := strings.TrimSpace(os.Getenv("OIDC_JWKS_URL"))
	discoveryURL := strings.TrimSpace(os.Getenv("OIDC_DISCOVERY_URL"))
	if jwksURL == "" && discoveryURL == "" {
		return nil, oidcauth.Config{}, errors.New("OIDC_JWKS_URL or OIDC_DISCOVERY_URL is required")
	}
	cfg := oidcauth.Config{
		Enabled:             true,
		JWKSURL:             jwksURL,
		DiscoveryURL:        discoveryURL,
		Issuer:              issuer,
		Audience:            audience,
		TenantClaim:         env("OIDC_TENANT_CLAIM", "tenant_id"),
		ScopeClaim:          env("OIDC_SCOPE_CLAIM", "scope"),
		AllowedAlgorithms:   splitCSV(env("OIDC_ALLOWED_ALGORITHMS", "RS256")),
		AllowedClockSkew:    30 * time.Second,
		JWKSRefreshTimeout:  5 * time.Second,
		JWKSRefreshInterval: 10 * time.Minute,
	}
	mw, err := oidcauth.NewMiddleware(ctx, cfg, ports.NopLogger{})
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

{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") (eq .AuthMode "dev-headers") }}{{ if eq .AuthMode "jwt" }}func withJWTAuthorizationScope(next http.Handler) http.Handler {
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

{{ else if eq .AuthMode "oidc" }}func withOIDCAuthorizationScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subj, ok := oidcauth.SubjectFromContext(r.Context())
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

func requireOIDCScope(required string) func(http.Handler) http.Handler {
	required = strings.TrimSpace(required)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subj, ok := oidcauth.SubjectFromContext(r.Context())
			if !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "authentication token required"})
				return
			}
			if !oidcSubjectHasScope(subj, required) {
				httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "required OIDC scope missing"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func oidcSubjectHasScope(subj oidcauth.Subject, required string) bool {
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
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
{{ end }}	"encoding/json"
	"flag"
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	"math/big"
{{ end }}	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}	"time"

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

{{ else if eq .AuthMode "oidc" }}func TestGeneratedServiceRejectsProductionMissingOIDCConfig(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("RATE_LIMIT_REDIS_ADDR", "localhost:6379")
	t.Setenv("OIDC_JWKS_URL", "")
	t.Setenv("OIDC_DISCOVERY_URL", "")
	t.Setenv("OIDC_ISSUER", "https://issuer.example.com")
	t.Setenv("OIDC_AUDIENCE", "saas-api")
	if _, err := newService(); err == nil {
		t.Fatal("expected production service startup to require OIDC config")
	} else if !strings.Contains(err.Error(), "OIDC_JWKS_URL or OIDC_DISCOVERY_URL") {
		t.Fatalf("startup error = %v, want OIDC JWKS/discovery requirement", err)
	}
}

func TestGeneratedServiceRejectsProductionMissingAdminKey(t *testing.T) {
	t.Setenv("ENV", "production")
	setOIDCAuthEnv(t)
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
{{ else if eq .AuthMode "oidc" }}	setOIDCAuthEnv(t)
{{ else }}	t.Setenv("API_KEY", "prod-api-key")
	t.Setenv("API_TENANT_ID", "tenant_1")
{{ end }}}

{{ end }}
func setLocalTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENV", "development")
{{ if eq .AuthMode "jwt" }}	setJWTAuthEnv(t)
{{ else if eq .AuthMode "clerk" }}	setClerkAuthEnv(t)
{{ else if eq .AuthMode "oidc" }}	setOIDCAuthEnv(t)
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
{{ else if eq .AuthMode "oidc" }}	req.Header.Set("Authorization", "Bearer "+testOIDCJWT(t, tenantID, "widgets:write"))
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
{{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}const (
	testBearerKeyID = "test-kid"
{{ if eq .AuthMode "jwt" }}	testJWTIssuer   = "https://issuer.example.test"
	testJWTAudience = "saas-api"
{{ else if eq .AuthMode "clerk" }}	testClerkIssuer   = "https://clerk.example.test"
	testClerkAudience = "saas-api"
{{ else if eq .AuthMode "oidc" }}	testOIDCIssuer   = "https://oidc.example.test"
	testOIDCAudience = "saas-api"
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

{{ else if eq .AuthMode "oidc" }}func setOIDCAuthEnv(t *testing.T) {
	t.Helper()
	setBearerAuthEnv(t, "OIDC", testOIDCIssuer, testOIDCAudience)
	t.Setenv("OIDC_TENANT_CLAIM", "tenant_id")
	t.Setenv("OIDC_SCOPE_CLAIM", "scope")
}

func testOIDCJWT(t *testing.T, tenantID string, scopes ...string) string {
	t.Helper()
	return testBearerJWT(t, testOIDCIssuer, testOIDCAudience, tenantID, scopes...)
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
{{ else if eq .AuthMode "oidc" }}OIDC_JWKS_URL=
OIDC_DISCOVERY_URL=
OIDC_ISSUER=
OIDC_AUDIENCE=saas-api
OIDC_TENANT_CLAIM=tenant_id
OIDC_SCOPE_CLAIM=scope
OIDC_ALLOWED_ALGORITHMS=RS256
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
- ` + "`POST /widgets`" + ` with {{ if or (eq .AuthMode "jwt") (eq .AuthMode "clerk") (eq .AuthMode "oidc") }}` + "`Authorization: Bearer <token>`" + `{{ else if eq .AuthMode "dev-headers" }}` + "`X-Debug-User`" + `, ` + "`X-Debug-Tenant-ID`" + `, ` + "`X-Debug-Scopes`" + `{{ else }}` + "`X-API-Key`" + `{{ end }}, ` + "`X-Tenant-ID`" + `, and ` + "`Idempotency-Key`" + `
- ` + "`POST /widgets/imports`" + ` with tenant auth and ` + "`Idempotency-Key`" + ` returns ` + "`202 Accepted`" + `
- ` + "`GET /operations/{id}`" + ` with tenant auth returns pollable async state
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
{{ else if eq .AuthMode "oidc" }}OIDC mode validates bearer tokens with ` + "`OIDC_ISSUER`" + `, ` + "`OIDC_AUDIENCE`" + `, and either ` + "`OIDC_JWKS_URL`" + ` or ` + "`OIDC_DISCOVERY_URL`" + `. The configured ` + "`OIDC_TENANT_CLAIM`" + ` claim must match ` + "`X-Tenant-ID`" + `, and write requests require the ` + "`widgets:write`" + ` scope.
When ` + "`ENV=production`" + `, startup requires explicit OIDC configuration and ` + "`ADMIN_KEY`" + `.
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
