package docscheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
	writeFile(t, filepath.Join(tmpDir, "main.go"), mainSrc)
	writeFile(t, filepath.Join(tmpDir, "main_test.go"), testSrc)

	goMod := strings.Join([]string{
		"module example.com/my-api",
		"",
		"go 1.24.0",
		"",
		"require (",
		"\tgithub.com/aatuh/api-toolkit/v2 v2.0.0",
		"\tgithub.com/aatuh/api-toolkit/contrib/v2 v2.0.0",
		")",
		"",
		"replace github.com/aatuh/api-toolkit/v2 => " + repoRoot,
		"replace github.com/aatuh/api-toolkit/contrib/v2 => " + filepath.Join(repoRoot, "contrib"),
		"",
	}, "\n")
	writeFile(t, filepath.Join(tmpDir, "go.mod"), goMod)

	out, err := runGoCmd(tmpDir, "go", "mod", "tidy")
	if err != nil {
		t.Fatalf("getting-started guide dependencies do not resolve:\n%s\nerror: %v", out, err)
	}

	out, err = runGoCmd(tmpDir, "go", "test", "./...")
	if err != nil {
		t.Fatalf("getting-started guide does not build:\n%s\nerror: %v", out, err)
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
	if strings.HasPrefix(token, "github.com/aatuh/api-toolkit-contrib") {
		return true
	}
	if token == "github.com/aatuh/api-toolkit" {
		return true
	}
	if token == "github.com/aatuh/api-toolkit/v2" {
		return false
	}
	if strings.HasPrefix(token, "github.com/aatuh/api-toolkit/v2/") {
		return false
	}
	if token == "github.com/aatuh/api-toolkit/contrib/v2" {
		return false
	}
	if strings.HasPrefix(token, "github.com/aatuh/api-toolkit/contrib/v2/") {
		return false
	}
	return strings.HasPrefix(token, "github.com/aatuh/api-toolkit/")
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGoCmd(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd.CombinedOutput()
}
