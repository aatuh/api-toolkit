package docscheck

import (
	"context"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

var tokenPattern = regexp.MustCompile(`github\.com/aatuh/[A-Za-z0-9./_-]+`)

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
