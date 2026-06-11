package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePackageListAcceptsCommaAndWhitespace(t *testing.T) {
	got, err := parsePackageList("./binding, ./queryparams\n./webhooks\t./...")
	if err != nil {
		t.Fatalf("parse package list: %v", err)
	}
	want := []string{"./binding", "./queryparams", "./webhooks", "./..."}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("packages = %#v, want %#v", got, want)
	}
}

func TestParsePackageListRejectsUnsafePatterns(t *testing.T) {
	for _, raw := range []string{
		"../binding",
		"/tmp/pkg",
		"-run",
		"./binding;rm",
		"./../outside",
		"./binding/../queryparams",
		`./binding\..\outside`,
		"./binding//queryparams",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := parsePackageList(raw); err == nil {
				t.Fatalf("parsePackageList(%q) succeeded, want error", raw)
			}
		})
	}
}

func TestResolveOutputPathRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	for _, raw := range []string{"../report.tsv", filepath.Join(root, "..", "report.tsv")} {
		t.Run(raw, func(t *testing.T) {
			if _, err := resolveOutputPath(root, raw); err == nil {
				t.Fatalf("resolveOutputPath(%q) succeeded, want error", raw)
			}
		})
	}

	got, err := resolveOutputPath(root, ".ci-result/mutation/report.tsv")
	if err != nil {
		t.Fatalf("resolveOutputPath inside root: %v", err)
	}
	if !strings.HasPrefix(got, root+string(filepath.Separator)) {
		t.Fatalf("resolved output %q is outside root %q", got, root)
	}
}

func TestDiscoverMutationsFindsBoolAndOperatorCandidates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "pkg", "widget")
	writeFile(t, filepath.Join(dir, "widget.go"), `package widget

func enabled(a, b int) bool {
	if a == b && true {
		return false
	}
	return a < b
}
`)
	writeFile(t, filepath.Join(dir, "widget_test.go"), `package widget

func TestIgnored(t *testing.T) {}
`)

	candidates, err := discoverMutations(root, []packageTarget{{
		Import: "example.com/toolkit/pkg/widget",
		Dir:    dir,
	}})
	if err != nil {
		t.Fatalf("discoverMutations: %v", err)
	}

	seen := map[string]bool{}
	for _, candidate := range candidates {
		seen[candidate.Rule+"\t"+candidate.Original+"\t"+candidate.Replacement] = true
		if strings.HasSuffix(candidate.File, "_test.go") {
			t.Fatalf("discovered mutation in test file: %#v", candidate)
		}
	}
	for _, want := range []string{
		"operator\t==\t!=",
		"operator\t&&\t||",
		"bool-literal\ttrue\tfalse",
		"bool-literal\tfalse\ttrue",
		"operator\t<\t<=",
	} {
		if !seen[want] {
			t.Fatalf("missing mutation candidate %q in %#v", want, candidates)
		}
	}
}

func TestApplyMutationRejectsEscapesAndReplacesExpectedBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pkg", "widget.go")
	source := "package widget\n\nconst enabled = true\n"
	writeFile(t, path, source)

	start := strings.Index(source, "true")
	if start < 0 {
		t.Fatal("fixture missing true literal")
	}
	candidate := mutation{
		File:        "pkg/widget.go",
		Start:       start,
		End:         start + len("true"),
		Original:    "true",
		Replacement: "false",
	}
	if err := applyMutation(root, candidate); err != nil {
		t.Fatalf("applyMutation: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mutated file: %v", err)
	}
	if !strings.Contains(string(got), "enabled = false") {
		t.Fatalf("mutation was not applied:\n%s", got)
	}

	candidate.File = "../outside.go"
	if err := applyMutation(root, candidate); err == nil {
		t.Fatal("applyMutation accepted an escaping path")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
