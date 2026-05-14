package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var out strings.Builder
	code := run(context.Background(), []string{"version"}, &out, &out)
	if code != 0 {
		t.Fatalf("version exit code = %d output=%s", code, out.String())
	}
	if !strings.Contains(out.String(), "api-toolkit") {
		t.Fatalf("version output = %q", out.String())
	}
	for _, want := range []string{
		"go go",
		"main ",
		"core github.com/aatuh/api-toolkit/v2 ",
		"contrib github.com/aatuh/api-toolkit/contrib/v2 ",
		"build_commit ",
		"build_date ",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("version output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunVersionJSON(t *testing.T) {
	var out strings.Builder
	code := run(context.Background(), []string{"version", "--json"}, &out, &out)
	if code != 0 {
		t.Fatalf("version --json exit code = %d output=%s", code, out.String())
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("decode version json: %v\n%s", err, out.String())
	}
	for _, key := range []string{
		"tool_version",
		"go_version",
		"main_path",
		"main_version",
		"core_version",
		"contrib_version",
		"build_commit",
		"build_date",
	} {
		if strings.TrimSpace(got[key]) == "" {
			t.Fatalf("version json missing %q: %#v", key, got)
		}
	}
}

func TestClientsGoGeneratesBuildableClient(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": true,
					"content": {"application/json": {"schema": {"type": "object"}}}
				},
				"responses": {
					"201": {"description": "created", "content": {"application/json": {"schema": {"type": "object"}}}},
					"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"type": "object"}}}}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		},
		"/widgets/{id}": {
			"get": {
				"operationId": "getWidget",
				"parameters": [
					{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
					{"name": "X-Tenant-ID", "in": "header", "required": true, "schema": {"type": "string"}}
				],
				"responses": {
					"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}},
					"404": {"description": "not found", "content": {"application/problem+json": {"schema": {"type": "object"}}}}
				},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		}
	}`)
	clientDir := filepath.Join(tmp, "client")
	var out strings.Builder
	code := run(context.Background(), []string{
		"clients", "go",
		"--openapi", specPath,
		"--out", clientDir,
		"--package", "apiclient",
	}, &out, &out)
	if code != 0 {
		t.Fatalf("clients go failed: %s", out.String())
	}
	generated, err := os.ReadFile(filepath.Join(clientDir, "client.go"))
	if err != nil {
		t.Fatalf("read generated client.go: %v", err)
	}
	for _, want := range []string{
		"package apiclient",
		"type Client struct",
		"func WithAPIKey",
		"func WithBearerToken",
		"func (c *Client) CreateWidget",
		"func (c *Client) GetWidget",
		"type Problem struct",
		"type Error struct",
		"PathParam(\"id\",",
	} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated client.go missing %q:\n%s", want, generated)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/clienttest\n\ngo 1.25.0\n"), 0o600); err != nil {
		t.Fatalf("write temp go.mod: %v", err)
	}
	cmd := exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = tmp
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated client package should build:\n%s\nerror: %v", output, err)
	}
}

func TestClientsGoRejectsUnsafeInputs(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{}`)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "traversal output",
			args: []string{"clients", "go", "--openapi", specPath, "--out", "../escape", "--package", "apiclient"},
			want: "output directory must stay under the current working directory",
		},
		{
			name: "invalid package",
			args: []string{"clients", "go", "--openapi", specPath, "--out", filepath.Join(tmp, "client"), "--package", "api-client"},
			want: "invalid Go package name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errOut strings.Builder
			code := run(context.Background(), tt.args, &strings.Builder{}, &errOut)
			if code == 0 {
				t.Fatal("expected clients go to fail")
			}
			if !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", errOut.String(), tt.want)
			}
		})
	}
}

func TestNewServiceRejectsTraversalOutput(t *testing.T) {
	var errOut strings.Builder
	code := run(context.Background(), []string{"new", "service", "--module", "example.com/my-api", "--dir", "../escape"}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatalf("expected traversal output to fail")
	}
	if !strings.Contains(errOut.String(), "output directory must stay under the current working directory") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestNewServiceRejectsUnsupportedAuthModes(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		auth    string
		want    string
	}{
		{name: "dev headers require development profile", profile: "saas-api", auth: "dev-headers", want: "auth mode \"dev-headers\" requires an explicit development profile"},
		{name: "production modes stay on saas profile", profile: "dev-api", auth: "api-key", want: "auth mode \"api-key\" is not supported for profile \"dev-api\""},
		{name: "unknown", profile: "saas-api", auth: "session", want: "unsupported auth mode \"session\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errOut strings.Builder
			code := run(context.Background(), []string{
				"new", "service",
				"--module", "example.com/my-api",
				"--profile", tt.profile,
				"--auth", tt.auth,
				"--dir", filepath.Join(t.TempDir(), "service"),
			}, &strings.Builder{}, &errOut)
			if code == 0 {
				t.Fatal("expected unsupported auth mode to fail")
			}
			if !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", errOut.String(), tt.want)
			}
		})
	}
}

func TestNewServiceGeneratesBuildableDevAPIWithDevHeaders(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "dev-api",
		"--auth", "dev-headers",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		`RegisterSecurityScheme("DevHeaderAuth"`,
		"newDevHeadersMiddleware",
		"withDevHeaderAuthorizationScope",
		"requireDevHeaderScope",
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated dev-header main.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated .env.example: %v", err)
	}
	for _, want := range []string{
		"DEV_AUTH_FALLBACK_ENABLED=true",
		"DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES=true",
		"DEV_AUTH_TENANT_HEADER=X-Debug-Tenant-ID",
		"DEV_AUTH_SCOPE_HEADER=X-Debug-Scopes",
	} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated dev-header .env.example missing %q", want)
		}
	}
	assertGeneratedEnvDocumentsRateLimitConfig(t, string(generatedEnv))
	assertGeneratedEnvDocumentsTelemetryConfig(t, string(generatedEnv))
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `dev-headers`.") {
		t.Fatalf("generated README missing dev-header auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsIdempotencyRequirement(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsRateLimitStore(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsTelemetry(t, string(generatedREADME))
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "DevHeaderAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dev-header service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dev-header service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dev-header service contracts lint failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIWithClerk(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api",
		"--auth", "clerk",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		`RegisterSecurityScheme("BearerAuth"`,
		"newClerkMiddleware",
		"ShutdownHooks:",
		"withClerkAuthorizationScope",
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated Clerk main.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated .env.example: %v", err)
	}
	for _, want := range []string{"CLERK_JWKS_URL=", "CLERK_ISSUER=", "CLERK_AUDIENCE=saas-api"} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated Clerk .env.example missing %q", want)
		}
	}
	assertGeneratedEnvDocumentsRateLimitConfig(t, string(generatedEnv))
	assertGeneratedEnvDocumentsTelemetryConfig(t, string(generatedEnv))
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `clerk`.") {
		t.Fatalf("generated README missing Clerk auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsIdempotencyRequirement(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsRateLimitStore(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsTelemetry(t, string(generatedREADME))
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "BearerAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Clerk service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Clerk service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Clerk service contracts lint failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIWithJWT(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api",
		"--auth", "jwt",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		`RegisterSecurityScheme("BearerAuth"`,
		"newJWTMiddleware",
		"ShutdownHooks:",
		"tenantIDFromJWTSubject",
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated JWT main.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated .env.example: %v", err)
	}
	for _, want := range []string{"JWT_JWKS_URL=", "JWT_ISSUER=", "JWT_AUDIENCE=saas-api"} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated JWT .env.example missing %q", want)
		}
	}
	assertGeneratedEnvDocumentsRateLimitConfig(t, string(generatedEnv))
	assertGeneratedEnvDocumentsTelemetryConfig(t, string(generatedEnv))
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `jwt`.") {
		t.Fatalf("generated README missing JWT auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsIdempotencyRequirement(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsRateLimitStore(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsTelemetry(t, string(generatedREADME))
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "BearerAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated JWT service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated JWT service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated JWT service contracts lint failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPI(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}
	for _, name := range []string{"go.mod", "main.go", "main_test.go", "testdata/openapi.golden.json", "Makefile", ".env.example", ".gitignore", ".dockerignore", ".github/workflows/ci.yml", "Dockerfile", "docker-compose.yml", "README.md"} {
		if _, err := os.Stat(filepath.Join(serviceDir, name)); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}
	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		`appVersion  = "dev"`,
		`buildCommit = "unknown"`,
		`buildDate   = "unknown"`,
		"MiddlewareOrder:         bootstrap.StrictSaaSAPIMiddlewareOrder()",
		"RequiredMiddlewareOrder: bootstrap.StrictSaaSAPIMiddlewareOrder()",
		"NewPrometheusRecorderChecked",
		"bootstrap.DefaultRouterConfigFromEnv(nil)",
		"routerConfig.Metrics = metricsRecorder",
		"HealthStatusChangeHook",
		"IdempotencyOutcomeHook",
		"IdempotencyOutcomeLogHook",
		"BackgroundTasks:",
		"newTracingShutdown(",
		"telemetry.InitTracing",
		`bootstrap.ShutdownHook{Name: "otel-tracing"`,
		"newRateLimitLimiter(",
		"ratelimitredis.New",
		`bootstrap.ShutdownHook{Name: "rate-limit-redis"`,
		"newIdempotencyStore()",
		`bootstrap.ShutdownHook{Name: "idempotency-redis"`,
		"client.Close()",
		"StorageKeyFunc: idempotencymw.TenantScopedStorageKeyFunc()",
		"idempotencyredis.New",
		`ports.VersionInfo{Version: appVersion, Commit: buildCommit, Date: buildDate}`,
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated main.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated .env.example: %v", err)
	}
	for _, want := range []string{"IDEMPOTENCY_STORE=memory", "REDIS_ADDR=localhost:6379"} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated .env.example missing %q", want)
		}
	}
	assertGeneratedEnvDocumentsRateLimitConfig(t, string(generatedEnv))
	assertGeneratedEnvDocumentsTelemetryConfig(t, string(generatedEnv))
	generatedDockerfile, err := os.ReadFile(filepath.Join(serviceDir, "Dockerfile"))
	if err != nil {
		t.Fatalf("read generated Dockerfile: %v", err)
	}
	for _, want := range []string{
		"FROM golang:1.25 AS build",
		"ARG VERSION=dev",
		"ARG BUILD_COMMIT=unknown",
		"ARG BUILD_DATE=unknown",
		"CGO_ENABLED=0 GOOS=linux go build",
		"-X main.appVersion=${VERSION}",
		"-X main.buildCommit=${BUILD_COMMIT}",
		"-X main.buildDate=${BUILD_DATE}",
		"USER nonroot:nonroot",
		"ENTRYPOINT [\"/api\"]",
	} {
		if !strings.Contains(string(generatedDockerfile), want) {
			t.Fatalf("generated Dockerfile missing %q", want)
		}
	}
	if strings.Contains(string(generatedDockerfile), "go run") {
		t.Fatalf("generated Dockerfile should run a built binary, got:\n%s", generatedDockerfile)
	}
	generatedDockerignore, err := os.ReadFile(filepath.Join(serviceDir, ".dockerignore"))
	if err != nil {
		t.Fatalf("read generated .dockerignore: %v", err)
	}
	for _, want := range []string{".env", ".git", ".tools"} {
		if !strings.Contains(string(generatedDockerignore), want) {
			t.Fatalf("generated .dockerignore missing %q", want)
		}
	}
	generatedGitignore, err := os.ReadFile(filepath.Join(serviceDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read generated .gitignore: %v", err)
	}
	for _, want := range []string{".env", "!.env.example", "coverage.out", ".ci-result/", ".tools/"} {
		if !strings.Contains(string(generatedGitignore), want) {
			t.Fatalf("generated .gitignore missing %q", want)
		}
	}
	generatedMakefile, err := os.ReadFile(filepath.Join(serviceDir, "Makefile"))
	if err != nil {
		t.Fatalf("read generated Makefile: %v", err)
	}
	for _, want := range []string{
		"VERSION ?= dev",
		"BUILD_COMMIT ?=",
		"BUILD_DATE ?=",
		"LDFLAGS ?=",
		"OUTPUT_DIR ?=",
		"TOOLS_DIR ?=",
		"SYFT ?=",
		"COVERAGE_MIN ?=",
		"GOVULNCHECK ?=",
		"GOVULNCHECK_VERSION ?=",
		"tools:",
		"GOBIN=\"$(CURDIR)/$(TOOLS_DIR)/bin\" $(GO) install",
		"build:",
		"-X main.appVersion=$(VERSION)",
		"coverage-check:",
		"tool cover -func=coverage.out",
		"test-race:",
		"vuln:",
		"\"$(GOVULNCHECK)\" ./...",
		"contracts-lint:",
		"contracts-diff:",
		"fast-check:",
		"audit-check:",
		"sbom-local:",
		"clean:",
		"finalize: fmt audit-check clean",
		"API_TOOLKIT ?=",
		"OPENAPI_BASE ?=",
	} {
		if !strings.Contains(string(generatedMakefile), want) {
			t.Fatalf("generated Makefile missing %q", want)
		}
	}
	generatedCI, err := os.ReadFile(filepath.Join(serviceDir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read generated CI workflow: %v", err)
	}
	for _, want := range []string{
		"name: ci",
		"pull_request:",
		"permissions:",
		"contents: read",
		"GOTOOLCHAIN: local",
		"actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5",
		"actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff",
		"go-version: 1.25.x",
		"make finalize",
	} {
		if !strings.Contains(string(generatedCI), want) {
			t.Fatalf("generated CI workflow missing %q:\n%s", want, generatedCI)
		}
	}
	generatedCompose, err := os.ReadFile(filepath.Join(serviceDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("read generated docker-compose.yml: %v", err)
	}
	for _, want := range []string{
		"redis:",
		"image: redis:7-alpine",
		"REDIS_ADDR: redis:6379",
		"RATE_LIMIT_REDIS_ADDR: redis:6379",
		"redis-data:",
		`test: ["CMD", "redis-cli", "ping"]`,
	} {
		if !strings.Contains(string(generatedCompose), want) {
			t.Fatalf("generated docker-compose.yml missing %q:\n%s", want, generatedCompose)
		}
	}
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `api-key`.") {
		t.Fatalf("generated README missing auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsIdempotencyRequirement(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsRateLimitStore(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsTelemetry(t, string(generatedREADME))
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "ApiKeyAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "openapi-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service openapi check failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service contracts lint failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-diff")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service contracts diff failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "build")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service build failed:\n%s\nerror: %v", output, err)
	}
	if _, err := os.Stat(filepath.Join(serviceDir, "bin", "api")); err != nil {
		t.Fatalf("expected generated build artifact: %v", err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "coverage-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service coverage check failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "coverage-check")
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "COVERAGE_MIN=100.0")
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("generated service coverage check should fail below an explicit 100%% floor:\n%s", output)
	}
	if !strings.Contains(string(output), "coverage") || !strings.Contains(string(output), "below required") {
		t.Fatalf("generated service coverage failure should explain the floor:\n%s", output)
	}
	cmd = exec.CommandContext(context.Background(), "make", "vuln", "GOVULNCHECK=/bin/true")
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service vuln target should use overridable local tool path:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "test-race")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service race tests failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIFull(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--auth", "api-key",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new full service failed: %s", out.String())
	}

	for _, name := range []string{
		"go.mod",
		"cmd/api/main.go",
		"internal/domain/api_key.go",
		"internal/domain/tenancy.go",
		"internal/domain/widget.go",
		"internal/app/audit.go",
		"internal/app/audit_test.go",
		"internal/app/cache.go",
		"internal/app/cache_test.go",
		"internal/app/api_keys.go",
		"internal/app/api_keys_test.go",
		"internal/app/async.go",
		"internal/app/async_test.go",
		"internal/app/tenancy.go",
		"internal/app/tenancy_test.go",
		"internal/app/widgets.go",
		"internal/app/webhooks.go",
		"internal/app/webhooks_test.go",
		"internal/app/objects.go",
		"internal/app/objects_test.go",
		"internal/adapters/postgres/postgres.go",
		"internal/adapters/postgres/postgres_test.go",
		"internal/adapters/postgres/widgets.go",
		"internal/adapters/postgres/widgets_test.go",
		"internal/adapters/postgres/objects.go",
		"internal/adapters/postgres/objects_test.go",
		"internal/adapters/postgres/webhooks.go",
		"internal/adapters/postgres/webhooks_test.go",
		"internal/adapters/redis/cache.go",
		"internal/adapters/redis/cache_test.go",
		"internal/httpapi/router.go",
		"internal/httpapi/router_test.go",
		"internal/httpapi/openapi.go",
		"internal/client/apiclient/client.go",
		"migrations/0001_platform.sql",
		"scripts/integration_check.sh",
		"testdata/openapi.golden.json",
		"Makefile",
		".env.example",
		".gitignore",
		".dockerignore",
		".github/workflows/ci.yml",
		"Dockerfile",
		"docker-compose.yml",
		"deploy/kubernetes/deployment.yaml",
		"deploy/kubernetes/service.yaml",
		"deploy/kubernetes/admin-service.yaml",
		"README.md",
	} {
		if _, err := os.Stat(filepath.Join(serviceDir, name)); err != nil {
			t.Fatalf("expected generated full-profile %s: %v", name, err)
		}
	}

	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated full-profile .env.example: %v", err)
	}
	for _, want := range []string{
		"DATABASE_URL=",
		"REDIS_ADDR=localhost:6379",
		"CACHE_STORE=memory",
		"RATE_LIMIT_STORE=memory",
		"RATE_LIMIT_KEY_PREFIX=ratelimit:",
		"IDEMPOTENCY_STORE=memory",
		"IDEMPOTENCY_KEY_PREFIX=idempotency:",
		"API_ACTOR_ID=",
		"API_KEY_PEPPER=",
		"WEBHOOK_SECRET_KEY=",
		"ADMIN_ADDR=:9090",
	} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated full-profile .env.example missing %q", want)
		}
	}

	generatedMigration, err := os.ReadFile(filepath.Join(serviceDir, "migrations", "0001_platform.sql"))
	if err != nil {
		t.Fatalf("read generated migration: %v", err)
	}
	for _, want := range []string{
		"CREATE TABLE organizations",
		"CREATE TABLE memberships",
		"CREATE TABLE invitations",
		"CREATE TABLE api_keys",
		"CREATE TABLE widgets",
		"CREATE TABLE operations",
		"CREATE TABLE outbox_events",
		"CREATE TABLE audit_events",
		"actor_type TEXT NOT NULL",
		"CREATE TABLE objects",
		"content_type TEXT NOT NULL",
		"CREATE TABLE webhook_endpoints",
		"secret_ciphertext BYTEA NOT NULL",
		"secret_nonce BYTEA NOT NULL",
		"CREATE TABLE webhook_deliveries",
		"event_id TEXT NOT NULL",
		"last_status_code INTEGER",
	} {
		if !strings.Contains(string(generatedMigration), want) {
			t.Fatalf("generated migration missing %q", want)
		}
	}

	generatedRouter, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "router.go"))
	if err != nil {
		t.Fatalf("read generated full-profile router: %v", err)
	}
	for _, want := range []string{"If-Match", "ETag", "Idempotency-Key", "X-Tenant-ID", "WriteProblem", "NewRateLimitMiddleware", "rateLimited", "handleCreateOrganization", "handleCreateInvitation", "handleCreateAPIKey", "handleRevokeAPIKey", "handleCreateWidgetImport", "handleGetOperation", "handleCreateWebhookEndpoint", "handleReplayWebhookDelivery", "handlePutObject", "handleGetObject", "recordAudit", "Readiness"} {
		if !strings.Contains(string(generatedRouter), want) {
			t.Fatalf("generated full-profile router missing %q", want)
		}
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "api", "main.go"))
	if err != nil {
		t.Fatalf("read generated full-profile main.go: %v", err)
	}
	for _, want := range []string{"postgres.Open", "postgres.CheckRequiredTables", "postgres.NewWidgetStore", "app.NewWidgetServiceWithStore", "postgres.NewTenancyStore", "app.NewTenancyServiceWithStore", "postgres.NewAPIKeyStore", "app.NewAPIKeyServiceWithStore", "postgres.NewWidgetImportOperationStore", "postgres.NewWidgetImportOutbox", "app.NewAsyncServiceWithStores", "postgres.NewWebhookStore", "app.NewWebhookServiceWithStore", "cfg.WebhookSecretKey", "postgres.NewObjectStore", "app.NewObjectServiceWithStores", "objectstorage.OpenS3BlobStore", "auditpostgres.New", "pgxpooladapter.Adapter", "app.NewAuditServiceWithRecorder", "rediscache.OpenCache", "rediscache.OpenRateLimiter", "rediscache.OpenIdempotencyStore", "httpapi.NewRateLimitMiddleware", "httpapi.NewIdempotencyMiddleware", "app.NewAuditService", "app.NewWebhookService", "app.NewObjectService", "app.NewCacheService", "Audit: auditLog", "Webhooks: webhooks", "Objects: objects", "Cache: cacheService", "RateLimit: rateLimitMiddleware", "Idempotency: idempotencyMiddleware", "Readiness: readiness"} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated full-profile main.go missing %q", want)
		}
	}

	generatedClient, err := os.ReadFile(filepath.Join(serviceDir, "internal", "client", "apiclient", "client.go"))
	if err != nil {
		t.Fatalf("read generated full-profile client: %v", err)
	}
	for _, want := range []string{"package apiclient", "func (c *Client) CreateWidget", "func (c *Client) CreateWidgetImport", "func (c *Client) GetOperation", "func (c *Client) UpdateWidget", "func (c *Client) CreateOrganization", "func (c *Client) CreateOrganizationInvitation", "func (c *Client) CreateOrganizationAPIKey", "func (c *Client) RevokeOrganizationAPIKey", "func (c *Client) CreateOrganizationWebhookEndpoint", "func (c *Client) ReplayOrganizationWebhookDelivery", "func (c *Client) PutOrganizationObject", "func (c *Client) GetOrganizationObject", "func (c *Client) DeleteOrganizationObject", "PathParam(\"id\",", "PathParam(\"organization_id\",", "PathParam(\"api_key_id\",", "PathParam(\"delivery_id\",", "PathParam(\"object_key\","} {
		if !strings.Contains(string(generatedClient), want) {
			t.Fatalf("generated full-profile client missing %q", want)
		}
	}

	generatedMakefile, err := os.ReadFile(filepath.Join(serviceDir, "Makefile"))
	if err != nil {
		t.Fatalf("read generated full-profile Makefile: %v", err)
	}
	for _, want := range []string{"openapi-check:", "contracts-lint:", "contracts-diff:", "integration-check:", "client-check:", "bash scripts/integration_check.sh"} {
		if !strings.Contains(string(generatedMakefile), want) {
			t.Fatalf("generated full-profile Makefile missing %q", want)
		}
	}
	generatedIntegration, err := os.ReadFile(filepath.Join(serviceDir, "scripts", "integration_check.sh"))
	if err != nil {
		t.Fatalf("read generated full-profile integration script: %v", err)
	}
	for _, want := range []string{
		"compose up -d postgres redis",
		"psql -v ON_ERROR_STOP=1 -U api -d api",
		"go test ./...",
		"go run ./cmd/api",
		"/docs/openapi.json",
		"/health/detailed",
		"X-API-Key: ${API_KEY}",
		"X-Actor-ID: ${API_ACTOR_ID}",
		"Idempotency-Key:",
		"docker compose down -v",
	} {
		if !strings.Contains(string(generatedIntegration), want) {
			t.Fatalf("generated full-profile integration script missing %q", want)
		}
	}
	cmd := exec.CommandContext(context.Background(), "bash", "-n", "scripts/integration_check.sh")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile integration script syntax failed:\n%s\nerror: %v", output, err)
	}

	generatedCompose, err := os.ReadFile(filepath.Join(serviceDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("read generated full-profile compose: %v", err)
	}
	for _, want := range []string{
		"postgres:",
		"image: postgres:18-alpine",
		"redis:",
		"image: redis:7-alpine",
		"minio:",
		"profiles: [objectstore]",
	} {
		if !strings.Contains(string(generatedCompose), want) {
			t.Fatalf("generated full-profile docker-compose.yml missing %q:\n%s", want, generatedCompose)
		}
	}

	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated full-profile README.md: %v", err)
	}
	for _, want := range []string{
		"Generated profile: `saas-api-full`.",
		"Postgres stores tenants, API keys, widgets, operations, outbox, audit, webhook delivery state, and object metadata.",
		"`make integration-check`",
	} {
		if !strings.Contains(string(generatedREADME), want) {
			t.Fatalf("generated full-profile README.md missing %q", want)
		}
	}
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "ApiKeyAuth")

	cmd = exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "openapi-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service openapi check failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service contracts lint failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-diff")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service contracts diff failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "client-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service client check failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIFullWithOIDC(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--auth", "oidc",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new OIDC full service failed: %s", out.String())
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "api", "main.go"))
	if err != nil {
		t.Fatalf("read generated full OIDC main.go: %v", err)
	}
	for _, want := range []string{"newOIDCMiddleware", "oidcauth.NewMiddleware", "OIDCJWKSURL", "OIDCDiscoveryURL"} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated full OIDC main.go missing %q", want)
		}
	}
	generatedRouter, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "router.go"))
	if err != nil {
		t.Fatalf("read generated full OIDC router.go: %v", err)
	}
	for _, want := range []string{"oidcauth.SubjectFromContext", "required OIDC scope missing", "tenant claim mismatch"} {
		if !strings.Contains(string(generatedRouter), want) {
			t.Fatalf("generated full OIDC router.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated full OIDC .env.example: %v", err)
	}
	for _, want := range []string{"OIDC_ISSUER=", "OIDC_AUDIENCE=saas-api-full", "OIDC_JWKS_URL=", "OIDC_DISCOVERY_URL=", "OIDC_TENANT_CLAIM=tenant_id", "OIDC_SCOPE_CLAIM=scope"} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated full OIDC .env.example missing %q", want)
		}
	}
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "BearerAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full OIDC service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full OIDC service tests failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIFullWithJWT(t *testing.T) {
	assertFullScaffoldBearerAuthMode(t, "jwt", []string{"newJWTMiddleware", "jwtauth.NewMiddleware", "JWTJWKSURL", "JWTIssuer", "JWTAudience"}, []string{"jwtauth.SubjectFromContext", "required JWT scope missing", "tenant claim mismatch"}, "BearerAuth")
}

func TestNewServiceGeneratesBuildableSaaSAPIFullWithClerk(t *testing.T) {
	assertFullScaffoldBearerAuthMode(t, "clerk", []string{"newClerkMiddleware", "clerkauth.NewMiddleware", "ClerkJWKSURL", "ClerkIssuer", "ClerkAudience"}, []string{"clerkauth.SubjectFromContext", "required Clerk scope missing", "tenant claim mismatch"}, "BearerAuth")
}

func assertFullScaffoldBearerAuthMode(t *testing.T, authMode string, mainWant, routerWant []string, securityScheme string) {
	t.Helper()
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--auth", authMode,
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new full service auth=%s failed: %s", authMode, out.String())
	}
	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "api", "main.go"))
	if err != nil {
		t.Fatalf("read generated full %s main.go: %v", authMode, err)
	}
	for _, want := range mainWant {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated full %s main.go missing %q", authMode, want)
		}
	}
	generatedRouter, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "router.go"))
	if err != nil {
		t.Fatalf("read generated full %s router.go: %v", authMode, err)
	}
	for _, want := range routerWant {
		if !strings.Contains(string(generatedRouter), want) {
			t.Fatalf("generated full %s router.go missing %q", authMode, want)
		}
	}
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, securityScheme)
	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full %s service tidy failed:\n%s\nerror: %v", authMode, output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full %s service tests failed:\n%s\nerror: %v", authMode, output, err)
	}
}

func TestContractsLintFailsForMissingPolicy(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{"/widgets":{"post":{"responses":{"201":{"description":"created"}}}}}}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{
		"operation_id_required",
		"security_required",
		"unsafe_write_tenant_required",
		"unsafe_write_rate_limit_required",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintFailsForPrivateReadWithoutSecurity(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	if !strings.Contains(errOut.String(), "security_required") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsLintFailsForPrivateReadWithoutProblemResponse(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	if !strings.Contains(errOut.String(), "problem_response_required") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsLintFailsForUndefinedSecurityScheme(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {
					"200": {"description": "ok"},
					"400": {
						"description": "bad request",
						"content": {"application/problem+json": {"schema": {"type": "object"}}}
					}
				},
				"security": [{"MissingAuth": ["widgets:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	if !strings.Contains(errOut.String(), "security_scheme_undefined") || !strings.Contains(errOut.String(), "MissingAuth") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsLintFailsForUndefinedGlobalSecurityScheme(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"security": [{"MissingGlobalAuth": []}],
		"paths": {}
	}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{"security_scheme_undefined", "GLOBAL", "MissingGlobalAuth"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintUsesGlobalSecurityRequirements(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"security": [{"ApiKeyAuth": ["widgets:read"]}],
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {
						"200": {"description": "ok"},
						"400": {
							"description": "bad request",
							"content": {"application/problem+json": {"schema": {"type": "object"}}}
						}
					}
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var out strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &out, &out)
	if code != 0 {
		t.Fatalf("expected lint to pass with inherited global security: %s", out.String())
	}
}

func TestContractsLintFailsForUndocumentedUnsafeWrite(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"responses": {
					"400": {
						"description": "bad request",
						"content": {"application/problem+json": {"schema": {"type": "object"}}}
					}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}],
				"x-tenant": {"required": true, "source": "header"},
				"x-idempotency-key": {"required": true},
				"x-rate-limit": "write-standard"
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{"unsafe_write_request_body_required", "unsafe_write_success_response_required"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintFailsForNonRequiredUnsafeWritePolicies(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
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
				"x-tenant": {"required": false, "source": "header"},
				"x-idempotency-key": {"required": false, "header": "Idempotency-Key"},
				"x-rate-limit": "write-standard"
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{"unsafe_write_tenant_required", "unsafe_write_idempotency_required"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintAllowsPublicReadinessWithoutSecurity(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/readyz": {
			"get": {
				"operationId": "getReadiness",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)

	var out strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &out, &out)
	if code != 0 {
		t.Fatalf("expected public readiness lint to pass: %s", out.String())
	}
	if !strings.Contains(out.String(), "contracts lint passed") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestContractsLintFailsForDuplicateOperationIDs(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"get": {
				"operationId": "getWidget",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		},
		"/widget-exports": {
			"get": {
				"operationId": "getWidget",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["exports:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	if !strings.Contains(errOut.String(), "operation_id_duplicate") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsLintAllowsAdditionalPublicPath(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/status": {
			"get": {
				"operationId": "getStatus",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail without custom public path")
	}
	if !strings.Contains(errOut.String(), "security_required") {
		t.Fatalf("stderr = %q", errOut.String())
	}

	var out strings.Builder
	code = run(context.Background(), []string{"contracts", "lint", "--openapi", specPath, "--public-path", "/status"}, &out, &out)
	if code != 0 {
		t.Fatalf("expected custom public path lint to pass: %s", out.String())
	}
	if !strings.Contains(out.String(), "contracts lint passed") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestContractsLintChecksAdditionalAdminPath(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/internal/debug": {
			"get": {
				"operationId": "getInternalDebug",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["admin:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath, "--admin-path", "/internal/debug"}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail for custom admin path without policy")
	}
	if !strings.Contains(errOut.String(), "admin_policy_required") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsDiffAllowsAdditiveOperations(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}}
			}
		},
		"/widget-exports": {
			"get": {
				"operationId": "getWidget",
				"responses": {"200": {"description": "ok"}, "404": {"description": "not found"}}
			}
		}
	}`)

	var out strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &out, &out)
	if code != 0 {
		t.Fatalf("expected additive diff to pass: %s", out.String())
	}
	if !strings.Contains(out.String(), "contracts diff passed") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestContractsDiffFailsForBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}, "401": {"description": "unauthorized"}},
				"security": [{"ApiKeyAuth": []}]
			},
			"post": {
				"operationId": "createWidget",
				"responses": {"201": {"description": "created"}, "400": {"description": "bad request"}}
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgetsRenamed",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected breaking diff to fail")
	}
	for _, want := range []string{
		"operation_removed POST /widgets",
		"operation_id_changed GET /widgets",
		"response_removed GET /widgets",
		"security_changed GET /widgets",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForGlobalSecurityDrift(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	baseSpec := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"security": [{"ApiKeyAuth": ["widgets:read"]}],
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`
	headSpec := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
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
	}`
	if err := os.WriteFile(base, []byte(baseSpec), 0o600); err != nil {
		t.Fatalf("write base spec: %v", err)
	}
	if err := os.WriteFile(head, []byte(headSpec), 0o600); err != nil {
		t.Fatalf("write head spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected global security diff to fail")
	}
	for _, want := range []string{
		"global_security_changed",
		"security_changed GET /widgets",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForRoutePolicyDrift(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
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
	}`)
	writeTestOpenAPI(t, head, `{
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
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected policy drift diff to fail")
	}
	for _, want := range []string{
		"tenant_policy_changed POST /widgets",
		"idempotency_policy_changed POST /widgets",
		"rate_limit_policy_changed POST /widgets",
		"deprecation_policy_changed POST /widgets",
		"admin_policy_changed GET /metrics",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForRequestBodyBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
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
	}`)
	writeTestOpenAPI(t, head, `{
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
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected request body breaking diff to fail")
	}
	for _, want := range []string{
		"request_body_required_added POST /widgets",
		"request_body_content_removed POST /widgets application/vnd.widgets+json",
		"request_body_removed POST /widget-imports",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForResponseContentBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
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
	}`)
	writeTestOpenAPI(t, head, `{
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
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected response content breaking diff to fail")
	}
	for _, want := range []string{
		"response_content_removed GET /widgets 200 application/vnd.widgets+json",
		"response_content_removed GET /widgets 400 application/problem+json",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForParameterBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
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
	}`)
	writeTestOpenAPI(t, head, `{
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
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected parameter breaking diff to fail")
	}
	for _, want := range []string{
		"parameter_removed GET /widgets query:cursor",
		"parameter_required_added GET /widgets query:filter",
		"required_parameter_added GET /widgets header:X-Client-Version",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForComponentSchemaBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	if err := os.WriteFile(base, []byte(`{
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
			},
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
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
	}`), 0o600); err != nil {
		t.Fatalf("write base spec: %v", err)
	}
	if err := os.WriteFile(head, []byte(`{
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
			},
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
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
	}`), 0o600); err != nil {
		t.Fatalf("write head spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected schema breaking diff to fail")
	}
	for _, want := range []string{
		"schema_removed Legacy",
		"schema_required_property_added Widget status",
		"schema_property_removed Widget tenant_id",
		"schema_type_changed Widget.name",
		"schema_enum_value_removed Widget.status \"disabled\"",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForSecuritySchemeBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	if err := os.WriteFile(base, []byte(`{
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
	}`), 0o600); err != nil {
		t.Fatalf("write base spec: %v", err)
	}
	if err := os.WriteFile(head, []byte(`{
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
	}`), 0o600); err != nil {
		t.Fatalf("write head spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected security scheme breaking diff to fail")
	}
	for _, want := range []string{
		"security_scheme_changed ApiKeyAuth",
		"security_scheme_removed WebhookAuth",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForInlineSchemaBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
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
	}`)
	writeTestOpenAPI(t, head, `{
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
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected schema breaking diff to fail")
	}
	for _, want := range []string{
		"schema_required_property_added POST /widgets requestBody application/json status",
		"schema_type_changed POST /widgets requestBody application/json.name",
		"schema_enum_value_removed POST /widgets requestBody application/json.status \"disabled\"",
		"schema_type_changed POST /widgets response 201 application/json.id",
		"schema_property_removed POST /widgets response 201 application/json status",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func writeTestOpenAPI(t *testing.T, path, pathsJSON string) {
	t.Helper()
	spec := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"paths": ` + pathsJSON + `
	}`
	if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

func assertGeneratedGoldenHasGlobalSecurity(t *testing.T, serviceDir, scheme string) {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join(serviceDir, "testdata", "openapi.golden.json"))
	if err != nil {
		t.Fatalf("read generated OpenAPI golden: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(golden, &doc); err != nil {
		t.Fatalf("decode generated OpenAPI golden: %v", err)
	}
	security, ok := doc["security"].([]any)
	if !ok || len(security) == 0 {
		t.Fatalf("generated OpenAPI golden has no top-level security: %s", golden)
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
	t.Fatalf("generated OpenAPI golden missing top-level security scheme %q: %s", scheme, golden)
}

func assertGeneratedREADMEListsOperatorRoutes(t *testing.T, readme string) {
	t.Helper()
	for _, want := range []string{"`GET /health/detailed` with `X-Admin-Key`", "`GET /metrics` with `X-Admin-Key`", "`GET /debug/pprof/` with `X-Admin-Key`"} {
		if !strings.Contains(readme, want) {
			t.Fatalf("generated README missing operator route %q:\n%s", want, readme)
		}
	}
}

func assertGeneratedREADMEDocumentsIdempotencyRequirement(t *testing.T, readme string) {
	t.Helper()
	for _, want := range []string{
		"Unsafe writes without `Idempotency-Key` fail with Problem Details 400",
		"Idempotency storage keys are tenant and actor scoped",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("generated README missing idempotency guidance %q:\n%s", want, readme)
		}
	}
}

func assertGeneratedREADMEDocumentsRateLimitStore(t *testing.T, readme string) {
	t.Helper()
	for _, want := range []string{
		"Local development uses `RATE_LIMIT_STORE=memory`",
		"In production, the generated service defaults to `RATE_LIMIT_STORE=redis`",
		"requires `RATE_LIMIT_REDIS_ADDR` or `REDIS_ADDR`",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("generated README missing rate-limit store guidance %q:\n%s", want, readme)
		}
	}
}

func assertGeneratedREADMEDocumentsTelemetry(t *testing.T, readme string) {
	t.Helper()
	for _, want := range []string{
		"OpenTelemetry tracing is disabled by default with `OTEL_TRACING_ENABLED=false`",
		"`OTEL_EXPORTER_OTLP_ENDPOINT` is required when tracing is enabled",
		"tracer provider is closed through the service shutdown hooks",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("generated README missing telemetry guidance %q:\n%s", want, readme)
		}
	}
}

func assertGeneratedEnvDocumentsRateLimitConfig(t *testing.T, env string) {
	t.Helper()
	for _, want := range []string{
		"TRUSTED_PROXIES=",
		"RATE_LIMIT_SKIP_ENABLED=false",
		"RATE_LIMIT_SKIP_HEADER=",
		"RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES=false",
		"RATE_LIMIT_STORE=memory",
		"RATE_LIMIT_REDIS_ADDR=",
		"RATE_LIMIT_KEY_PREFIX=ratelimit:",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("generated .env.example missing rate-limit config %q:\n%s", want, env)
		}
	}
}

func assertGeneratedEnvDocumentsTelemetryConfig(t *testing.T, env string) {
	t.Helper()
	for _, want := range []string{
		"OTEL_TRACING_ENABLED=false",
		"OTEL_SERVICE_NAME=api",
		"OTEL_EXPORTER_OTLP_ENDPOINT=",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
		"OTEL_TRACES_SAMPLER=parentbased_traceidratio",
		"OTEL_SAMPLE_RATIO=1",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("generated .env.example missing telemetry config %q:\n%s", want, env)
		}
	}
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root from %s: %v", wd, err)
	}
	return root
}
