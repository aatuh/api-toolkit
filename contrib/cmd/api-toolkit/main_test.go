package main

import (
	"context"
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
	for _, name := range []string{"go.mod", "main.go", "main_test.go", "testdata/openapi.golden.json", "Makefile", ".env.example", ".dockerignore", "Dockerfile", "docker-compose.yml", "README.md"} {
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
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated main.go missing %q", want)
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
	generatedMakefile, err := os.ReadFile(filepath.Join(serviceDir, "Makefile"))
	if err != nil {
		t.Fatalf("read generated Makefile: %v", err)
	}
	for _, want := range []string{"contracts-lint:", "contracts-diff:", "API_TOOLKIT ?=", "OPENAPI_BASE ?="} {
		if !strings.Contains(string(generatedMakefile), want) {
			t.Fatalf("generated Makefile missing %q", want)
		}
	}

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
