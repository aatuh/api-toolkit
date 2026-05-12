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
	for _, name := range []string{"go.mod", "main.go", "main_test.go", "testdata/openapi.golden.json", "Makefile", ".env.example", "Dockerfile", "docker-compose.yml", "README.md"} {
		if _, err := os.Stat(filepath.Join(serviceDir, name)); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
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
