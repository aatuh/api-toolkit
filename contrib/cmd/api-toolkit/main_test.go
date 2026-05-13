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
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `dev-headers`.") {
		t.Fatalf("generated README missing dev-header auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
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
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `clerk`.") {
		t.Fatalf("generated README missing Clerk auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
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
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `jwt`.") {
		t.Fatalf("generated README missing JWT auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
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
	for _, name := range []string{"go.mod", "main.go", "main_test.go", "testdata/openapi.golden.json", "Makefile", ".env.example", ".gitignore", ".dockerignore", "Dockerfile", "docker-compose.yml", "README.md"} {
		if _, err := os.Stat(filepath.Join(serviceDir, name)); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}
	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		"MiddlewareOrder:         bootstrap.StrictSaaSAPIMiddlewareOrder()",
		"RequiredMiddlewareOrder: bootstrap.StrictSaaSAPIMiddlewareOrder()",
		"NewPrometheusRecorderChecked",
		"DefaultRouterConfig{Metrics:",
		"HealthStatusChangeHook",
		"IdempotencyOutcomeHook",
		"IdempotencyOutcomeLogHook",
		"BackgroundTasks:",
		"newIdempotencyStore()",
		"StorageKeyFunc: idempotencymw.TenantScopedStorageKeyFunc()",
		"idempotencyredis.New",
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
	generatedDockerfile, err := os.ReadFile(filepath.Join(serviceDir, "Dockerfile"))
	if err != nil {
		t.Fatalf("read generated Dockerfile: %v", err)
	}
	for _, want := range []string{
		"FROM golang:1.25 AS build",
		"CGO_ENABLED=0 GOOS=linux go build",
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
	for _, want := range []string{".env", ".git"} {
		if !strings.Contains(string(generatedDockerignore), want) {
			t.Fatalf("generated .dockerignore missing %q", want)
		}
	}
	generatedGitignore, err := os.ReadFile(filepath.Join(serviceDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read generated .gitignore: %v", err)
	}
	for _, want := range []string{".env", "!.env.example", "coverage.out", ".ci-result/"} {
		if !strings.Contains(string(generatedGitignore), want) {
			t.Fatalf("generated .gitignore missing %q", want)
		}
	}
	generatedMakefile, err := os.ReadFile(filepath.Join(serviceDir, "Makefile"))
	if err != nil {
		t.Fatalf("read generated Makefile: %v", err)
	}
	for _, want := range []string{"contracts-lint:", "contracts-diff:", "API_TOOLKIT ?=", "OPENAPI_BASE ?="} {
		if !strings.Contains(string(generatedMakefile), want) {
			t.Fatalf("generated Makefile missing %q", want)
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
