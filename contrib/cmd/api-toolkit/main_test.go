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
	for _, name := range []string{"go.mod", "main.go", "main_test.go", "Makefile", ".env.example", "Dockerfile", "docker-compose.yml", "README.md"} {
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
	if !strings.Contains(errOut.String(), "operation_id_required") {
		t.Fatalf("stderr = %q", errOut.String())
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
