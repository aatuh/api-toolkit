package docscheck

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var (
	rootModulePath    = strings.Join([]string{"github.com", "aatuh", "api-toolkit"}, "/")
	contribModulePath = rootModulePath + "/contrib/v2"
)

func TestPublicMarkdownUsesV2ModulePaths(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	files := publicMarkdownFiles(t, repoRoot)

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		tokens := tokenPattern.FindAllString(string(content), -1)
		for _, token := range tokens {
			if forbiddenModuleToken(token) {
				rel, err := filepath.Rel(repoRoot, path)
				if err != nil {
					rel = path
				}
				t.Fatalf("%s contains stale module path %q", rel, token)
			}
		}
	}
}

func TestGettingStartedGuideBuilds(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	docPath := filepath.Join(repoRoot, "docs", "getting-started.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}

	mainSrc := extractSectionCodeBlock(t, string(content), "## 2) Create `main.go`")
	testSrc := extractSectionCodeBlock(t, string(content), "## 4) Add a tiny test")

	tmpDir := t.TempDir()
	tmpRoot, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("open temp root: %v", err)
	}
	defer func() {
		_ = tmpRoot.Close()
	}()
	writeFile(t, tmpRoot, "main.go", mainSrc)
	writeFile(t, tmpRoot, "main_test.go", testSrc)

	goMod := strings.Join([]string{
		"module example.com/my-api",
		"",
		"go 1.24.0",
		"",
		"require (",
		"\t" + rootModulePath + "/v2 v2.0.0",
		"\t" + contribModulePath + " v2.0.0",
		")",
		"",
		"replace " + rootModulePath + "/v2 => " + repoRoot,
		"replace " + contribModulePath + " => " + filepath.Join(repoRoot, "contrib"),
		"",
	}, "\n")
	writeFile(t, tmpRoot, "go.mod", goMod)

	out, err := runGoCmd(tmpDir, "go", "mod", "tidy")
	if err != nil {
		t.Fatalf("getting-started guide dependencies do not resolve:\n%s\nerror: %v", out, err)
	}

	out, err = runGoCmd(tmpDir, "go", "test", "./...")
	if err != nil {
		t.Fatalf("getting-started guide does not build:\n%s\nerror: %v", out, err)
	}
}

func TestRootProductionCodeDoesNotImportContrib(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	fset := token.NewFileSet()

	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".ci-result", "audit", "contrib":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if importPath == contribModulePath || strings.HasPrefix(importPath, contribModulePath+"/") {
				rel, relErr := filepath.Rel(repoRoot, path)
				if relErr != nil {
					rel = path
				}
				t.Fatalf("%s imports contrib module %q", rel, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan root imports: %v", err)
	}
}

func TestCompatibilitySensitivePortsManifestIsCurrent(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	manifestPath := filepath.Join(repoRoot, "docs", "ports-surface.md")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read ports surface docs: %v", err)
	}
	manifestText := string(manifest)
	for _, symbol := range compatibilitySensitivePortsSymbols(t, repoRoot) {
		if !strings.Contains(manifestText, "`"+symbol+"`") {
			t.Fatalf("docs/ports-surface.md missing compatibility-sensitive symbol %s", symbol)
		}
	}

	versioningPath := filepath.Join(repoRoot, "VERSIONING.md")
	versioning, err := os.ReadFile(versioningPath)
	if err != nil {
		t.Fatalf("read versioning docs: %v", err)
	}
	versioningText := string(versioning)
	for _, required := range []string{
		"ports/billing.go",
		"DatabasePool.Stat",
		"DatabaseStats",
		"response_writer",
	} {
		if !strings.Contains(versioningText, required) {
			t.Fatalf("VERSIONING.md missing compatibility-sensitive surface %q", required)
		}
	}
}

func TestStableAPISurfaceMatchesAPICheckPackages(t *testing.T) {
	repoRoot := mustRepoRoot(t)

	versioningPackages := stablePackagesFromVersioning(t, filepath.Join(repoRoot, "VERSIONING.md"))
	apiCheckPackages := stablePackagesFromAPICheck(t, filepath.Join(repoRoot, "scripts", "apicheck.sh"))

	assertStringSlicesEqual(t, "stable API surface", versioningPackages, apiCheckPackages)
}

func TestREADMEStabilitySummaryUsesVersioningSourceOfTruth(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	section := markdownSection(t, string(content), "## Stability")
	for _, required := range []string{
		"Stable core package list: `VERSIONING.md` is the source of truth",
		"`scripts/apicheck.sh` must cover the same package list",
	} {
		if !strings.Contains(section, required) {
			t.Fatalf("README stability section missing %q", required)
		}
	}
	if strings.Contains(section, "- Stable (core):") {
		t.Fatal("README stability section reintroduced an independent stable core package summary")
	}
}

var tokenPattern = regexp.MustCompile(`github\.com/aatuh/[A-Za-z0-9./_-]+`)
var packageLiteralPattern = regexp.MustCompile("`(" + regexp.QuoteMeta(rootModulePath) + `/v2[^` + "`" + `]*)` + "`")
var bashPackageLiteralPattern = regexp.MustCompile(`"` + regexp.QuoteMeta(rootModulePath) + `/v2[^"]*"`)

func publicMarkdownFiles(t *testing.T, repoRoot string) []string {
	t.Helper()

	files := []string{
		filepath.Join(repoRoot, "README.md"),
		filepath.Join(repoRoot, "VERSIONING.md"),
	}

	docPaths, err := filepath.Glob(filepath.Join(repoRoot, "docs", "*.md"))
	if err != nil {
		t.Fatalf("glob docs markdown: %v", err)
	}
	files = append(files, docPaths...)
	return files
}

func forbiddenModuleToken(token string) bool {
	if strings.HasPrefix(token, rootModulePath+"-contrib") {
		return true
	}
	if token == rootModulePath {
		return true
	}
	if token == rootModulePath+"/v2" {
		return false
	}
	if strings.HasPrefix(token, rootModulePath+"/v2/") {
		return false
	}
	if token == contribModulePath {
		return false
	}
	if strings.HasPrefix(token, contribModulePath+"/") {
		return false
	}
	return strings.HasPrefix(token, rootModulePath+"/")
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join(".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func extractSectionCodeBlock(t *testing.T, doc, heading string) string {
	t.Helper()

	sectionStart := strings.Index(doc, heading)
	if sectionStart == -1 {
		t.Fatalf("heading %q not found", heading)
	}
	section := doc[sectionStart:]

	start := strings.Index(section, "```go\n")
	if start == -1 {
		t.Fatalf("go code block not found under %q", heading)
	}
	section = section[start+len("```go\n"):]

	end := strings.Index(section, "\n```")
	if end == -1 {
		t.Fatalf("unterminated go code block under %q", heading)
	}
	return section[:end] + "\n"
}

func writeFile(t *testing.T, root *os.Root, name, content string) {
	t.Helper()
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer func() {
		_ = f.Close()
	}()
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func runGoCmd(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd.CombinedOutput()
}

func stablePackagesFromVersioning(t *testing.T, path string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read VERSIONING.md: %v", err)
	}
	section := markdownSection(t, string(content), "## Stable API surface (core module)")
	matches := packageLiteralPattern.FindAllStringSubmatch(section, -1)
	var packages []string
	for _, match := range matches {
		packages = append(packages, match[1])
	}
	if len(packages) == 0 {
		t.Fatal("VERSIONING.md stable API surface package list is empty")
	}
	return uniqueSorted(packages)
}

func stablePackagesFromAPICheck(t *testing.T, path string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read scripts/apicheck.sh: %v", err)
	}
	matches := bashPackageLiteralPattern.FindAllString(string(content), -1)
	var packages []string
	for _, match := range matches {
		packages = append(packages, strings.Trim(match, `"`))
	}
	if len(packages) == 0 {
		t.Fatal("scripts/apicheck.sh package list is empty")
	}
	return uniqueSorted(packages)
}

func markdownSection(t *testing.T, doc, heading string) string {
	t.Helper()

	start := strings.Index(doc, heading)
	if start == -1 {
		t.Fatalf("heading %q not found", heading)
	}
	section := doc[start+len(heading):]
	end := strings.Index(section, "\n## ")
	if end == -1 {
		return section
	}
	return section[:end]
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func assertStringSlicesEqual(t *testing.T, name string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d\ngot:  %v\nwant: %v", name, len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s differs at %d\ngot:  %v\nwant: %v", name, i, got, want)
		}
	}
}

func compatibilitySensitivePortsSymbols(t *testing.T, repoRoot string) []string {
	t.Helper()

	var symbols []string
	for _, name := range exportedTopLevelNames(t, filepath.Join(repoRoot, "ports", "billing.go")) {
		symbols = append(symbols, "ports."+name)
	}
	symbols = append(symbols, databaseStatsSymbols(t, filepath.Join(repoRoot, "ports", "database.go"))...)
	sort.Strings(symbols)
	return symbols
}

func exportedTopLevelNames(t *testing.T, path string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			switch spec := spec.(type) {
			case *ast.TypeSpec:
				if spec.Name.IsExported() {
					names = append(names, spec.Name.Name)
				}
			case *ast.ValueSpec:
				for _, name := range spec.Names {
					if name.IsExported() {
						names = append(names, name.Name)
					}
				}
			}
		}
	}
	return names
}

func databaseStatsSymbols(t *testing.T, path string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var symbols []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typeSpec.Name.Name != "DatabasePool" && typeSpec.Name.Name != "DatabaseStats" {
				continue
			}
			iface, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			if typeSpec.Name.Name == "DatabaseStats" {
				symbols = append(symbols, "ports.DatabaseStats")
			}
			for _, method := range iface.Methods.List {
				for _, name := range method.Names {
					if typeSpec.Name.Name == "DatabasePool" && name.Name != "Stat" {
						continue
					}
					symbols = append(symbols, "ports."+typeSpec.Name.Name+"."+name.Name)
				}
			}
		}
	}
	return symbols
}
