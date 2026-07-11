package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPassesWhenNewSymbolHasRequiredEvidence(t *testing.T) {
	root := newAPIGateFixture(t)
	writeFile(t, filepath.Join(root, "pkg", "widget.go"), `package pkg

// Widget is a stable API value.
type Widget struct{}
`)
	writeFile(t, filepath.Join(root, "pkg", "widget_test.go"), `package pkg

func ExampleWidget() {
	_ = Widget{}
}
`)
	writeFile(t, filepath.Join(root, "docs", "release-notes.md"), "- `pkg.Widget` adds a stable widget value.\n")

	err := run(testConfig(root))
	if err != nil {
		t.Fatalf("run API additions gate: %v", err)
	}
}

func TestRunAllowsExactExampleExceptionRows(t *testing.T) {
	root := newAPIGateFixture(t)
	writeFile(t, filepath.Join(root, "pkg", "widget.go"), `package pkg

// Widget is documented but intentionally not example-worthy by itself.
type Widget struct{}
`)
	writeFile(t, filepath.Join(root, "docs", "release-notes.md"), "- `pkg.Widget` adds a stable widget value.\n")
	writeFile(t, filepath.Join(root, "docs", "api-addition-exceptions.tsv"), "example.com/toolkit/pkg\tWidget\tNo standalone usage beyond package example.\tmaintainers\t2026-06-05\n")

	err := run(testConfig(root))
	if err != nil {
		t.Fatalf("run API additions gate with exception: %v", err)
	}
}

func TestRunFailsWhenNewSymbolEvidenceIsMissing(t *testing.T) {
	root := newAPIGateFixture(t)
	writeFile(t, filepath.Join(root, "pkg", "widget.go"), `package pkg

type Widget struct{}
`)

	err := run(testConfig(root))
	if err == nil {
		t.Fatal("run API additions gate succeeded without required evidence")
	}
	message := err.Error()
	for _, required := range []string{
		"missing source doc comment",
		"missing compile-checked example",
		"missing package-tied release note",
	} {
		if !strings.Contains(message, required) {
			t.Fatalf("error missing %q:\n%s", required, message)
		}
	}
}

func TestRunRequiresAcceptedDesignExceptionForNewRootPortsExport(t *testing.T) {
	root := newAPIGateFixture(t)
	mustMkdir(t, filepath.Join(root, "ports"))
	writeFile(t, filepath.Join(root, "ports", "limiter.go"), `package ports

// Limiter decides whether a request may proceed.
type Limiter interface {
	Allow() bool
}
`)
	writeFile(t, filepath.Join(root, "ports", "limiter_test.go"), `package ports

func ExampleLimiter() {
	var limiter Limiter
	_ = limiter
}
`)
	writeFile(t, filepath.Join(root, "base.md"), inventoryMarkdownFor("example.com/toolkit/ports", nil))
	writeFile(t, filepath.Join(root, "docs", "api-inventory.md"), inventoryMarkdownFor("example.com/toolkit/ports", []string{"Limiter"}))
	writeFile(t, filepath.Join(root, "docs", "release-notes.md"), "- `ports.Limiter` adds a stable limiter contract.\n")

	err := run(testConfig(root))
	if err == nil || !strings.Contains(err.Error(), "missing exact accepted design exception") {
		t.Fatalf("run API additions gate error = %v, want missing accepted root ports exception", err)
	}
}

func TestRunAllowsNewRootPortsExportWithAcceptedDesignException(t *testing.T) {
	root := newAPIGateFixture(t)
	mustMkdir(t, filepath.Join(root, "ports"))
	mustMkdir(t, filepath.Join(root, "docs", "adr"))
	writeFile(t, filepath.Join(root, "ports", "limiter.go"), `package ports

// Limiter decides whether a request may proceed.
type Limiter interface {
	Allow() bool
}
`)
	writeFile(t, filepath.Join(root, "ports", "limiter_test.go"), `package ports

func ExampleLimiter() {
	var limiter Limiter
	_ = limiter
}
`)
	writeFile(t, filepath.Join(root, "base.md"), inventoryMarkdownFor("example.com/toolkit/ports", nil))
	writeFile(t, filepath.Join(root, "docs", "api-inventory.md"), inventoryMarkdownFor("example.com/toolkit/ports", []string{"Limiter"}))
	writeFile(t, filepath.Join(root, "docs", "release-notes.md"), "- `ports.Limiter` adds a stable limiter contract.\n")
	writeFile(t, filepath.Join(root, "docs", "adr", "0002-limiter.md"), `# ADR 0002: Limiter

Status: Accepted

The limiter interface is adapter-neutral, has at least two real implementations,
and remains in root because the application should not own this cross-package contract.
`)
	writeFile(t, filepath.Join(root, "docs", "ports-export-exceptions.tsv"), "import_path\tsymbol\tdesign_note\tmaintainer_owner\treviewed_on\nexample.com/toolkit/ports\tLimiter\tdocs/adr/0002-limiter.md\tcore-maintainers\t2026-07-11\n")

	if err := run(testConfig(root)); err != nil {
		t.Fatalf("run API additions gate: %v", err)
	}
}

func newAPIGateFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "docs"))
	mustMkdir(t, filepath.Join(root, "pkg"))
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/toolkit\n\ngo 1.25\n")
	writeFile(t, filepath.Join(root, "base.md"), inventoryMarkdown(nil))
	writeFile(t, filepath.Join(root, "docs", "api-inventory.md"), inventoryMarkdown([]string{"Widget"}))
	writeFile(t, filepath.Join(root, "docs", "release-notes.md"), "# Release Notes\n")
	writeFile(t, filepath.Join(root, "docs", "api-addition-exceptions.tsv"), "# import_path\tsymbol\trationale\towner\treviewed_on\n")
	writeFile(t, filepath.Join(root, "docs", "ports-export-exceptions.tsv"), "# Exact accepted design records for new root ports exports.\nimport_path\tsymbol\tdesign_note\tmaintainer_owner\treviewed_on\n")
	return root
}

func testConfig(root string) config {
	return config{
		Root:             root,
		BaseInventory:    filepath.Join(root, "base.md"),
		CurrentInventory: "docs/api-inventory.md",
		ReleaseNotes:     "docs/release-notes.md",
		Exceptions:       "docs/api-addition-exceptions.tsv",
	}
}

func inventoryMarkdown(symbols []string) string {
	return inventoryMarkdownFor("example.com/toolkit/pkg", symbols)
}

func inventoryMarkdownFor(importPath string, symbols []string) string {
	var b strings.Builder
	b.WriteString("# Public API Inventory\n\n")
	b.WriteString("## `" + importPath + "`\n\n")
	b.WriteString("Stability tier: `stable`\n\n")
	if len(symbols) == 0 {
		b.WriteString("No exported symbols detected.\n")
		return b.String()
	}
	b.WriteString("| Symbol | Kind | Added version | Deprecation status |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, symbol := range symbols {
		b.WriteString("| `" + symbol + "` | type | v3 compatibility surface | active |\n")
	}
	return b.String()
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	//nolint:gosec // Test fixture source must stay readable by the parser under test.
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
