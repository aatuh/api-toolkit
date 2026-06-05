package docscheck

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var (
	rootModulePath    = strings.Join([]string{"github.com", "aatuh", "api-toolkit"}, "/")
	contribModulePath = rootModulePath + "/contrib/v3"
)

func skipV2CompatibilitySurfaceChecksOnV3(t *testing.T) {
	t.Helper()
	mod, err := os.ReadFile(filepath.Join(mustRepoRoot(t), "go.mod"))
	if err == nil && strings.Contains(string(mod), "module "+rootModulePath+"/v3") {
		t.Skip("v3 branch has removed the v2 compatibility-only surfaces")
	}
}

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

func TestGettingStartedGuideUsesGeneratedServiceScaffold(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	docPath := filepath.Join(repoRoot, "docs", "getting-started.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}

	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "my-api")
	doc := string(content)
	for _, required := range []string{
		"go run github.com/aatuh/api-toolkit/contrib/v3/cmd/api-toolkit@latest new service",
		"--module example.com/my-api",
		"--profile saas-api",
		"make finalize",
		"make openapi-check",
		"make contracts-lint",
		"make contracts-diff",
		"GET /readyz",
		"GET /docs/openapi.json",
		"POST /widgets",
		"Idempotency-Key",
	} {
		if !strings.Contains(doc, required) {
			t.Fatalf("getting-started guide missing scaffold instruction %q", required)
		}
	}

	cmd := exec.CommandContext(context.Background(), "go", "run", "./cmd/api-toolkit",
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	)
	cmd.Dir = filepath.Join(repoRoot, "contrib")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate getting-started service:\n%s\nerror: %v", out, err)
	}

	out, err := runGoCmd(serviceDir, "mod", "tidy")
	if err != nil {
		t.Fatalf("getting-started guide dependencies do not resolve:\n%s\nerror: %v", out, err)
	}

	out, err = runGoCmd(serviceDir, "test", "./...")
	if err != nil {
		t.Fatalf("getting-started guide does not build:\n%s\nerror: %v", out, err)
	}

	for _, target := range []string{"openapi-check", "contracts-lint", "contracts-diff"} {
		cmd := exec.CommandContext(context.Background(), "make", target)
		cmd.Dir = serviceDir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("getting-started guide %s failed:\n%s\nerror: %v", target, out, err)
		}
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
			case ".git", ".ci-result", "audit", "contrib", "examples":
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

func TestRootTestsUseContribModulePath(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	fset := token.NewFileSet()
	forbidden := rootModulePath + "/v3/contrib"

	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".ci-result", ".audits", "audit", "contrib":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
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
			if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
				rel, relErr := filepath.Rel(repoRoot, path)
				if relErr != nil {
					rel = path
				}
				t.Fatalf("%s imports contrib through root module path %q; use %q", rel, importPath, contribModulePath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan root test imports: %v", err)
	}
}

func TestRootModuleDependencyBoundaryExcludesContribAdapters(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	goMod := readText(t, filepath.Join(repoRoot, "go.mod"))
	boundaryDocs := readText(t, filepath.Join(repoRoot, "docs", "dependency-boundary.md"))

	for _, forbidden := range []string{
		contribModulePath,
		"github.com/alicebob/miniredis/v2",
		"github.com/redis/go-redis/v9",
	} {
		if strings.Contains(goMod, forbidden) {
			t.Fatalf("root go.mod must not require adapter/test dependency %q", forbidden)
		}
		if !strings.Contains(boundaryDocs, "`"+forbidden+"`") {
			t.Fatalf("docs/dependency-boundary.md missing dependency-boundary rationale for %q", forbidden)
		}
	}
	for _, required := range []string{
		"`middleware/idempotency` root tests use the package-local in-memory test store",
		"`contrib/adapters/idempotencytest` owns reusable adapter release-contract coverage",
	} {
		if !strings.Contains(boundaryDocs, required) {
			t.Fatalf("docs/dependency-boundary.md missing root/contrib test boundary text %q", required)
		}
	}
}

func TestStableCoreDependencyBoundariesExcludeContribProvidersAndGeneratedApps(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	rootV3Module := rootModulePath + "/v3"
	forbiddenPrefixes := []string{
		contribModulePath,
		rootModulePath + "/contrib",
		rootV3Module + "/contrib",
		rootV3Module + "/examples",
		"github.com/alicebob/miniredis",
		"github.com/aws/aws-sdk-go",
		"github.com/aws/aws-sdk-go-v2",
		"github.com/cedar-policy/",
		"github.com/clerkinc/",
		"github.com/go-chi/chi",
		"github.com/golang-migrate/migrate",
		"github.com/google/uuid",
		"github.com/jackc/pgx",
		"github.com/minio/minio-go",
		"github.com/oklog/ulid",
		"github.com/open-policy-agent/opa",
		"github.com/redis/",
		"github.com/resend/resend-go",
		"github.com/rs/cors",
		"github.com/stripe/stripe-go",
		"go.opentelemetry.io/otel",
		"go.uber.org/zap",
	}
	forbiddenExact := map[string]bool{
		"github.com/aatuh/api-toolkit/contrib": true,
		"github.com/redis/go-redis/v9":         true,
	}

	var violations []string
	for _, cls := range classes {
		if !inModule(cls.ImportPath, rootV3Module) {
			continue
		}
		if cls.APIStatus != "stable" && cls.APIStatus != "compatibility-only" {
			continue
		}
		dir := classifiedPackageDir(repoRoot, cls.ImportPath)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read package dir %s: %v", slashRel(repoRoot, dir), err)
		}
		fset := token.NewFileSet()
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse imports from %s: %v", slashRel(repoRoot, path), err)
			}
			for _, imp := range file.Imports {
				importPath, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("parse import path in %s: %v", slashRel(repoRoot, path), err)
				}
				if forbiddenExact[importPath] {
					violations = append(violations, sourceViolation(repoRoot, path, fset, imp.Pos(), "stable core imports forbidden dependency "+importPath))
					continue
				}
				for _, prefix := range forbiddenPrefixes {
					if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
						violations = append(violations, sourceViolation(repoRoot, path, fset, imp.Pos(), "stable core imports forbidden dependency "+importPath))
						break
					}
				}
			}
		}
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("stable core dependency boundary violations:\n%s", strings.Join(violations, "\n"))
	}
}

func TestArchitectureAndDependencyBoundaryDocsAreExecutable(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	architecture := readText(t, filepath.Join(repoRoot, "docs", "architecture.md"))
	boundaryDocs := readText(t, filepath.Join(repoRoot, "docs", "dependency-boundary.md"))
	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))
	ci := readText(t, filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
	script := readText(t, filepath.Join(repoRoot, "scripts", "dependency_boundary_check.sh"))

	for _, required := range []string{
		"## Boundary Map",
		"Stable core packages",
		"Contrib adapters and integrations",
		"CLI and generators",
		"Generated application code",
		"Examples and reference apps",
		"## Forbidden Dependency Directions",
		"must not import contrib, scaffold-only packages, provider SDKs, database drivers, router adapters, generated application code",
		"make dependency-boundary-check",
	} {
		if !strings.Contains(architecture, required) {
			t.Fatalf("docs/architecture.md missing architecture boundary text %q", required)
		}
	}
	for _, required := range []string{
		"GOTOOLCHAIN=local make dependency-boundary-check",
		"`scripts/dependency_boundary_check.sh`",
		"`TestStableCoreDependencyBoundariesExcludeContribProvidersAndGeneratedApps`",
		"stable or compatibility-only root package imports contrib",
		"provider SDKs",
		"generated app code",
		".github/workflows/ci.yml",
	} {
		if !strings.Contains(boundaryDocs, required) {
			t.Fatalf("docs/dependency-boundary.md missing executable boundary text %q", required)
		}
	}
	if !strings.Contains(makefile, "dependency-boundary-check:") {
		t.Fatal("Makefile missing dependency-boundary-check target")
	}
	if !strings.Contains(ci, "make dependency-boundary-check") {
		t.Fatal(".github/workflows/ci.yml missing dependency-boundary-check step")
	}
	if !strings.Contains(script, "TestStableCoreDependencyBoundariesExcludeContribProvidersAndGeneratedApps") {
		t.Fatal("scripts/dependency_boundary_check.sh must run the stable core dependency boundary test")
	}
}

func TestHealthDocsShowSafeDetailedHealthMounting(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	section := markdownSection(t, readme, "## Health endpoint contract")

	for _, required := range []string{
		"Mount detailed health, pprof, and metrics behind admin/internal access control",
		"operator-only dependency detail",
		"RegisterPublicRoutesTo",
		"RegisterAdminDetailedHealthRoute",
		"pprof.RegisterAdminRoutes",
		"MountSystemEndpointsToWithAdmin",
		"metrics",
		"requireAdmin",
	} {
		if !strings.Contains(section, required) {
			t.Fatalf("README health contract missing safe detailed-health guidance %q", required)
		}
	}
}

func TestPublicDocsDoNotTeachPolicyFreeAdminMounts(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	for _, mdPath := range append([]string{filepath.Join(repoRoot, "README.md")}, markdownFilesUnder(t, filepath.Join(repoRoot, "docs"))...) {
		for _, block := range markdownCodeBlocks(readText(t, mdPath)) {
			if strings.Contains(block, "pprof.RegisterRoutes(") {
				rel, _ := filepath.Rel(repoRoot, mdPath)
				t.Fatalf("%s code block teaches policy-free pprof mounting", rel)
			}
			if strings.Contains(block, "EnableDetailed: true") &&
				(strings.Contains(block, ".RegisterRoutes(") || strings.Contains(block, ".RegisterRoutesTo(") || strings.Contains(block, ".RegisterCustomRoutes")) &&
				!strings.Contains(block, "RegisterAdminDetailedHealthRoute") {
				rel, _ := filepath.Rel(repoRoot, mdPath)
				t.Fatalf("%s code block teaches policy-free detailed-health mounting", rel)
			}
			if strings.Contains(block, "Metrics:") &&
				strings.Contains(block, "MountSystemEndpoints") &&
				!strings.Contains(block, "MountSystemEndpointsToWithAdmin") {
				rel, _ := filepath.Rel(repoRoot, mdPath)
				t.Fatalf("%s code block teaches policy-free metrics mounting", rel)
			}
			if strings.Contains(block, `"/metrics"`) &&
				strings.Contains(block, "metricsHandler") &&
				!strings.Contains(block, "requireAdmin") {
				rel, _ := filepath.Rel(repoRoot, mdPath)
				t.Fatalf("%s code block teaches policy-free metrics handler mounting", rel)
			}
		}
	}
}

func TestCompatibilitySensitivePortsManifestIsCurrent(t *testing.T) {
	skipV2CompatibilitySurfaceChecksOnV3(t)
	repoRoot := mustRepoRoot(t)
	manifestPath := filepath.Join(repoRoot, "docs", "ports-surface.md")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read ports surface docs: %v", err)
	}
	manifestText := string(manifest)
	if !strings.Contains(manifestText, "## V3 cleanup checklist") {
		t.Fatal("docs/ports-surface.md missing V3 cleanup checklist")
	}
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
	manifestPackages := stableRootPackagesFromClassification(t, repoRoot)

	assertStringSlicesEqual(t, "VERSIONING.md stable API surface vs scripts/apicheck.sh", versioningPackages, apiCheckPackages)
	assertStringSlicesEqual(t, "VERSIONING.md stable API surface vs docs/package-classification.tsv", versioningPackages, manifestPackages)
}

func TestStableAPIPackagesHaveCompileCheckedExamples(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	packages := stablePackagesFromAPICheck(t, filepath.Join(repoRoot, "scripts", "apicheck.sh"))
	examplePattern := regexp.MustCompile(`(?m)^func Example[A-Za-z0-9_]*\(`)

	var missing []string
	for _, importPath := range packages {
		rel := strings.TrimPrefix(importPath, rootModulePath+"/v3")
		rel = strings.TrimPrefix(rel, "/")
		dir := filepath.Join(repoRoot, rel)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read package dir %s: %v", slashRel(repoRoot, dir), err)
		}
		hasExample := false
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
				continue
			}
			content := readText(t, filepath.Join(dir, name))
			if examplePattern.MatchString(content) {
				hasExample = true
				break
			}
		}
		if !hasExample {
			missing = append(missing, importPath)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("stable API packages missing compile-checked examples: %v", missing)
	}
}

func TestAPIReferenceTrustProofAndMaturityBadgesMatchClassification(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	index := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	classification := readText(t, filepath.Join(repoRoot, "docs", "package-classification.md"))
	apiReference := readText(t, filepath.Join(repoRoot, "docs", "api-reference.md"))

	for _, required := range []string{
		"## Trust Proof",
		"`make coverage-check`",
		"`make test-race`",
		"`make vuln`",
		"`make docs-check`",
		"`make v3-readiness-check`",
		"`make release-api-check`",
		"`make fuzz`",
		"`docs/package-classification.tsv`",
		"SBOM assets",
		"signatures",
		"provenance/attestation policy",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("README.md missing trust proof text %q", required)
		}
	}
	for _, required := range []string{
		"docs/api-reference.md",
		"[stable]",
		"[compatibility-only]",
		"[supported-adapter]",
		"[experimental]",
		"[generated]",
		"[tooling]",
	} {
		if !strings.Contains(readme+"\n"+index, required) {
			t.Fatalf("README.md or docs/README.md missing API reference or maturity badge text %q", required)
		}
	}

	seenStatuses := map[string]bool{}
	for _, cls := range classes {
		seenStatuses[cls.APIStatus] = true
	}
	for status := range seenStatuses {
		if !strings.Contains(classification, "["+status+"]") {
			t.Fatalf("docs/package-classification.md missing maturity badge for api_status=%s", status)
		}
		if !strings.Contains(classification, "`"+status+"`") {
			t.Fatalf("docs/package-classification.md missing status definition for api_status=%s", status)
		}
	}
	for _, required := range []string{
		"## Maturity Tier Badges",
		"package-specific badges must match the TSV",
		"`docs/api-reference.md` renders the `[stable]` and `[compatibility-only]`",
		"The TSV remains the",
	} {
		if !strings.Contains(classification, required) {
			t.Fatalf("docs/package-classification.md missing maturity-tier guidance %q", required)
		}
	}

	for _, importPath := range stableRootPackagesFromClassification(t, repoRoot) {
		cls, ok := classes[importPath]
		if !ok {
			t.Fatalf("stable package %s missing from classification map", importPath)
		}
		if !strings.Contains(apiReference, "https://pkg.go.dev/"+importPath) {
			t.Fatalf("docs/api-reference.md missing pkg.go.dev link for %s", importPath)
		}
		if !strings.Contains(apiReference, "["+cls.APIStatus+"]") {
			t.Fatalf("docs/api-reference.md missing maturity badge [%s] for %s", cls.APIStatus, importPath)
		}
		dir := classifiedPackageDir(repoRoot, importPath)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read package dir %s: %v", slashRel(repoRoot, dir), err)
		}
		var examplePath string
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, "_test.go") || !strings.Contains(name, "example") {
				continue
			}
			examplePath = filepath.Join(dir, name)
			break
		}
		if examplePath == "" {
			t.Fatalf("%s has no example test file for API reference", importPath)
		}
		expectedLink := "../" + slashRel(repoRoot, examplePath)
		if !strings.Contains(apiReference, "]("+expectedLink+")") {
			t.Fatalf("docs/api-reference.md missing tested example link %s for %s", expectedLink, importPath)
		}
	}
}

func TestPublicPackageClassificationManifestCoversRootPackages(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	packages := listedGoPackages(t, repoRoot)

	assertClassifiedPackages(t, "root", packages, classes, rootModulePath+"/v3", map[string]bool{
		"stable":             true,
		"compatibility-only": true,
		"experimental":       true,
		"test-only":          true,
		"example-only":       true,
		"excluded":           true,
	})

	for _, required := range []struct {
		importPath string
		apiStatus  string
		testStatus string
	}{
		{rootModulePath + "/v3/docscheck", "excluded", "direct-tests"},
		{rootModulePath + "/v3/testutil/authtest", "test-only", "test-support"},
	} {
		cls, ok := classes[required.importPath]
		if !ok {
			t.Fatalf("docs/package-classification.tsv missing %s", required.importPath)
		}
		if cls.APIStatus != required.apiStatus || cls.TestStatus != required.testStatus {
			t.Fatalf("%s classification = %s/%s, want %s/%s", required.importPath, cls.APIStatus, cls.TestStatus, required.apiStatus, required.testStatus)
		}
	}
}

func TestContribPackageClassificationAndCompatibilityPolicy(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	packages := listedGoPackages(t, filepath.Join(repoRoot, "contrib"))

	assertClassifiedPackages(t, "contrib", packages, classes, contribModulePath, map[string]bool{
		"supported-adapter": true,
		"experimental":      true,
		"wrapper-only":      true,
		"test-only":         true,
		"example-only":      true,
		"generated":         true,
		"tooling":           true,
		"excluded":          true,
	})

	var stable []string
	for _, cls := range classes {
		if inModule(cls.ImportPath, contribModulePath) && (cls.APIStatus == "stable" || cls.APIStatus == "compatibility-only") {
			stable = append(stable, cls.ImportPath)
		}
	}
	sort.Strings(stable)
	if len(stable) != 0 {
		t.Fatalf("contrib packages are classified as stable without a release-contrib-api-check gate: %v", stable)
	}

	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))
	for _, required := range []string{
		"The contrib module is outside the stable API compatibility promise",
		"`make release-api-check` covers only the core module",
		"`docs/package-classification.tsv`",
		"supported-adapter",
		"release-contrib-api-check",
		"supported-adapter incompatible drift fails this gate",
		"`make contrib-release-notes-check` is a lightweight",
	} {
		if !strings.Contains(versioning, required) {
			t.Fatalf("VERSIONING.md missing contrib compatibility policy text %q", required)
		}
	}
}

func TestPackageOwnersManifestMatchesClassification(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	ownerRows := loadTSVRecords(t, filepath.Join(repoRoot, "docs", "package-owners.tsv"))
	owners := recordsByField(t, ownerRows, "import_path")
	if len(owners) != len(classes) {
		t.Fatalf("docs/package-owners.tsv has %d rows, want %d package classification rows", len(owners), len(classes))
	}

	var missing []string
	for importPath, cls := range classes {
		row, ok := owners[importPath]
		if !ok {
			missing = append(missing, importPath+" missing owner row")
			continue
		}
		for _, field := range []string{"maintainer_owner", "stability_tier", "test_owner", "release_blocker_status"} {
			if strings.TrimSpace(row[field]) == "" {
				missing = append(missing, importPath+" has empty "+field)
			}
		}
		if got := row["stability_tier"]; got != cls.APIStatus {
			missing = append(missing, importPath+" stability_tier = "+got+", want "+cls.APIStatus)
		}
		if got, want := row["release_blocker_status"], releaseBlockerStatusForAPIStatus(cls.APIStatus); got != want {
			missing = append(missing, importPath+" release_blocker_status = "+got+", want "+want)
		}
	}
	for importPath := range owners {
		if _, ok := classes[importPath]; !ok {
			missing = append(missing, "stale owner row "+importPath)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("docs/package-owners.tsv does not match package classification:\n%s", strings.Join(missing, "\n"))
	}

	for _, source := range []struct {
		name string
		text string
	}{
		{"README.md", readText(t, filepath.Join(repoRoot, "README.md"))},
		{"docs/README.md", readText(t, filepath.Join(repoRoot, "docs", "README.md"))},
		{"docs/release-manifests.md", readText(t, filepath.Join(repoRoot, "docs", "release-manifests.md"))},
	} {
		if !strings.Contains(source.text, "docs/package-owners.tsv") {
			t.Fatalf("%s missing docs/package-owners.tsv", source.name)
		}
	}
}

func TestListedGoPackagesParserIgnoresGoDownloadChatter(t *testing.T) {
	packages := parseListedGoPackagesOutput(t, []byte(strings.Join([]string{
		"go: downloading golang.org/x/crypto v0.50.0",
		"go: downloading github.com/redis/go-redis/v9 v9.19.0",
		"github.com/aatuh/api-toolkit/contrib/v3/adapters/httpclient\t3\t1",
		"github.com/aatuh/api-toolkit/contrib/v3/config\t2\t0",
	}, "\n")))

	want := []listedPackage{
		{ImportPath: "github.com/aatuh/api-toolkit/contrib/v3/adapters/httpclient", DirectTestFiles: 4},
		{ImportPath: "github.com/aatuh/api-toolkit/contrib/v3/config", DirectTestFiles: 2},
	}
	if !reflect.DeepEqual(packages, want) {
		t.Fatalf("packages = %+v, want %+v", packages, want)
	}
}

func TestContribReviewScriptsAreDocumented(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	runbook := readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))
	notes := readText(t, filepath.Join(repoRoot, "docs", "release-notes.md"))
	manifest := readText(t, filepath.Join(repoRoot, "docs", "contrib-api-drift-packages.txt"))

	for _, required := range []string{
		"contrib-api-drift-report",
		"contrib-release-notes-check",
		"contrib-review-contract",
	} {
		if !strings.Contains(makefile, required) {
			t.Fatalf("Makefile missing contrib review target %q", required)
		}
	}
	for _, required := range []string{
		"contrib-api-drift-report",
		"contrib-release-notes-check",
		"docs/contrib-api-drift-packages.txt",
		"docs/contrib-api-drift-dispositions.tsv",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("README missing contrib review command %q", required)
		}
		if !strings.Contains(runbook, required) {
			t.Fatalf("release runbook missing contrib review command %q", required)
		}
		if !strings.Contains(notes, required) {
			t.Fatalf("release notes checklist missing contrib review command %q", required)
		}
	}
	for _, required := range []string{
		"contrib-api-drift-report.log",
		"incompatible report-only contrib drift",
		"does not make contrib stable",
	} {
		if !strings.Contains(runbook, required) && !strings.Contains(notes, required) {
			t.Fatalf("contrib review docs missing %q", required)
		}
	}
	if !strings.Contains(manifest, contribModulePath+"/middleware/auth/devheaders") {
		t.Fatal("contrib drift manifest must include the current devheaders drift package")
	}
}

func TestContribAPIDriftPackageManifest(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	manifestPackages := contribDriftManifestPackages(t, repoRoot)
	script := readText(t, filepath.Join(repoRoot, "scripts", "contrib_api_drift_report.sh"))

	if len(manifestPackages) == 0 {
		t.Fatal("docs/contrib-api-drift-packages.txt has no packages")
	}
	if !strings.Contains(script, "docs/contrib-api-drift-packages.txt") {
		t.Fatal("contrib drift script must read docs/contrib-api-drift-packages.txt")
	}
	for _, pkg := range manifestPackages {
		if !inModule(pkg, contribModulePath) {
			t.Fatalf("drift manifest package %s is outside contrib module", pkg)
		}
		cls, ok := classes[pkg]
		if !ok {
			t.Fatalf("drift manifest package %s is missing from docs/package-classification.tsv", pkg)
		}
		if cls.APIStatus != "supported-adapter" && cls.APIStatus != "experimental" && cls.APIStatus != "wrapper-only" {
			t.Fatalf("drift manifest package %s has api_status=%s, want supported-adapter, experimental, or wrapper-only", pkg, cls.APIStatus)
		}
	}
	for _, required := range []string{
		contribModulePath + "/adapters/pgxpool",
		contribModulePath + "/adapters/idempotencyredis",
		contribModulePath + "/adapters/stripe",
		contribModulePath + "/bootstrap",
		contribModulePath + "/integrations/auth/devheaders",
		contribModulePath + "/middleware/cors",
		contribModulePath + "/middleware/metrics",
		contribModulePath + "/middleware/openapi",
		contribModulePath + "/middleware/oteltrace",
		contribModulePath + "/middleware/requestlog",
		contribModulePath + "/middleware/auth/devheaders",
		contribModulePath + "/telemetry",
	} {
		if !containsString(manifestPackages, required) {
			t.Fatalf("drift manifest missing high-use package %s", required)
		}
	}
}

func TestToolchainPolicyMatchesModulesAndWorkflows(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	rootGo := moduleGoDirective(t, filepath.Join(repoRoot, "go.mod"))
	contribGo := moduleGoDirective(t, filepath.Join(repoRoot, "contrib", "go.mod"))
	if rootGo != "1.25.0" || contribGo != rootGo {
		t.Fatalf("module go directives drifted: root=%s contrib=%s; supported policy is Go 1.25.x", rootGo, contribGo)
	}

	for _, path := range []string{
		filepath.Join(repoRoot, ".github", "workflows", "ci.yml"),
		filepath.Join(repoRoot, ".github", "workflows", "codeql.yml"),
		filepath.Join(repoRoot, ".github", "workflows", "release.yml"),
	} {
		content := readText(t, path)
		for _, version := range workflowGoVersions(content) {
			if version != "1.25.x" {
				t.Fatalf("%s provisions Go %s, want 1.25.x to match module policy", slashRel(repoRoot, path), version)
			}
		}
	}

	for _, source := range []struct {
		name string
		text string
	}{
		{"README.md", readText(t, filepath.Join(repoRoot, "README.md"))},
		{"docs/release-runbook.md", readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))},
	} {
		for _, required := range []string{"Go 1.25.x", "`GOTOOLCHAIN=local`", "root and contrib"} {
			if !strings.Contains(source.text, required) {
				t.Fatalf("%s missing toolchain policy text %q", source.name, required)
			}
		}
	}
}

func TestContribAPIDriftDispositionManifest(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	records := loadTSVRecords(t, filepath.Join(repoRoot, "docs", "contrib-api-drift-dispositions.tsv"))
	byPackage := recordsByField(t, records, "package")
	releaseNotes := readText(t, filepath.Join(repoRoot, "docs", "release-notes.md"))
	releaseReview := readText(t, filepath.Join(repoRoot, "docs", "release-review.md"))
	runbook := readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))
	summaryScript := readText(t, filepath.Join(repoRoot, "scripts", "release_check_summary.sh"))
	notesScript := readText(t, filepath.Join(repoRoot, "scripts", "contrib_release_notes_check.sh"))

	if len(byPackage) == 0 {
		t.Fatal("docs/contrib-api-drift-dispositions.tsv has no disposition rows")
	}
	hasIncompatible := false
	for pkg, record := range byPackage {
		if !inModule(pkg, contribModulePath) {
			t.Fatalf("contrib drift disposition package %s is outside contrib module", pkg)
		}
		if record["status"] != "compatible" && record["status"] != "incompatible" {
			t.Fatalf("%s status = %q, want compatible or incompatible", pkg, record["status"])
		}
		if record["status"] == "incompatible" {
			hasIncompatible = true
			if record["release_note_acknowledgement"] == "not_required" {
				t.Fatalf("%s incompatible disposition must require package-tied release notes", pkg)
			}
		}
		for _, field := range []string{"reason", "release_note_acknowledgement", "reviewed_on", "expires_on", "owner"} {
			if strings.TrimSpace(record[field]) == "" {
				t.Fatalf("%s disposition missing %s", pkg, field)
			}
		}
		requireISODate(t, "reviewed_on for "+pkg, record["reviewed_on"])
		requireISODate(t, "expires_on for "+pkg, record["expires_on"])
	}
	if hasIncompatible && !strings.Contains(strings.ToLower(releaseNotes), "incompatible") {
		t.Fatal("release notes must contain package-tied incompatible contrib drift acknowledgement when incompatible drift is present")
	}
	for _, source := range []struct {
		name string
		text string
	}{
		{"docs/release-review.md", releaseReview},
		{"docs/release-runbook.md", runbook},
		{"scripts/release_check_summary.sh", summaryScript},
	} {
		if !strings.Contains(source.text, "docs/contrib-api-drift-dispositions.tsv") {
			t.Fatalf("%s missing contrib drift disposition manifest reference", source.name)
		}
	}
	if !strings.Contains(notesScript, "requires release notes tied to package") {
		t.Fatal("contrib release notes script must require package-tied incompatible drift acknowledgement")
	}
	for _, required := range []string{
		"contrib_drift_packages_from_log",
		"missing_disposition_count",
		"expired_disposition_count",
		"failed_disposition_review",
	} {
		if !strings.Contains(summaryScript, required) {
			t.Fatalf("release summary script missing dynamic contrib disposition enforcement text %q", required)
		}
	}
	for _, required := range []string{
		"docs/contrib-api-drift-dispositions.tsv",
		"non-expired disposition coverage",
	} {
		if !strings.Contains(notesScript, required) {
			t.Fatalf("contrib release notes script missing disposition enforcement text %q", required)
		}
	}
}

func TestNoTestPackagesAreExplicitlyClassified(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	packages := append(listedGoPackages(t, repoRoot), listedGoPackages(t, filepath.Join(repoRoot, "contrib"))...)

	acceptedNoTestStatus := map[string]bool{
		"example-only": true,
		"test-support": true,
		"generated":    true,
		"excluded":     true,
		"tooling":      true,
	}
	var invalid []string
	var needsTests []string
	for _, pkg := range packages {
		if pkg.DirectTestFiles > 0 {
			continue
		}
		cls, ok := classes[pkg.ImportPath]
		if !ok {
			invalid = append(invalid, pkg.ImportPath+" is missing from docs/package-classification.tsv")
			continue
		}
		if cls.TestStatus == "needs-tests" {
			needsTests = append(needsTests, pkg.ImportPath)
			continue
		}
		if !acceptedNoTestStatus[cls.TestStatus] {
			invalid = append(invalid, pkg.ImportPath+" has no direct tests and test_status="+cls.TestStatus)
		}
	}
	sort.Strings(invalid)
	sort.Strings(needsTests)
	if len(needsTests) > 0 {
		t.Fatalf("packages classified as needs-tests must receive tests before release: %v", needsTests)
	}
	if len(invalid) > 0 {
		t.Fatalf("packages with no direct tests must be classified as example-only, test-support, generated, tooling, or excluded: %v", invalid)
	}
}

func TestPackageClassificationTestStatusEvidence(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	packages := append(listedGoPackages(t, repoRoot), listedGoPackages(t, filepath.Join(repoRoot, "contrib"))...)
	packageByImport := make(map[string]listedPackage, len(packages))
	for _, pkg := range packages {
		packageByImport[pkg.ImportPath] = pkg
	}

	for importPath, cls := range classes {
		pkg, ok := packageByImport[importPath]
		if !ok {
			continue
		}
		switch cls.TestStatus {
		case "direct-tests":
			if pkg.DirectTestFiles == 0 {
				t.Fatalf("%s is classified direct-tests but has no direct test files", importPath)
			}
		case "wrapper-smoke-tested":
			if cls.APIStatus != "wrapper-only" {
				t.Fatalf("%s uses wrapper-smoke-tested with api_status=%s, want wrapper-only", importPath, cls.APIStatus)
			}
			if pkg.DirectTestFiles == 0 {
				t.Fatalf("%s is classified wrapper-smoke-tested but has no direct smoke tests", importPath)
			}
		case "example-only":
			if cls.APIStatus != "example-only" || !strings.Contains(importPath, "/examples/") {
				t.Fatalf("%s uses example-only test status outside an example-only package", importPath)
			}
		case "test-support":
			if cls.APIStatus != "test-only" || !strings.Contains(importPath, "test") {
				t.Fatalf("%s uses test-support outside a test-only support package", importPath)
			}
		case "generated":
			if cls.APIStatus != "generated" || !strings.Contains(importPath, "/gen") {
				t.Fatalf("%s uses generated test status outside generated code", importPath)
			}
		case "tooling":
			if cls.APIStatus != "tooling" || !strings.Contains(importPath, "/cmd/") {
				t.Fatalf("%s uses tooling test status outside command tooling", importPath)
			}
		case "excluded":
			if cls.APIStatus != "excluded" {
				t.Fatalf("%s uses excluded test status with api_status=%s", importPath, cls.APIStatus)
			}
		case "needs-tests":
			t.Fatalf("%s is still classified needs-tests", importPath)
		}
	}
}

func TestWrapperSmokeClassificationsRemainThinAndExplained(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)

	for importPath, cls := range classes {
		if cls.TestStatus != "wrapper-smoke-tested" {
			continue
		}
		if cls.APIStatus != "wrapper-only" {
			t.Fatalf("%s uses wrapper-smoke-tested with api_status=%s", importPath, cls.APIStatus)
		}
		if !strings.Contains(strings.ToLower(cls.Notes), "smoke coverage is sufficient because") {
			t.Fatalf("%s wrapper-smoke-tested note must explain why smoke coverage is sufficient: %q", importPath, cls.Notes)
		}
		if strings.Contains(importPath, "/integrations/") {
			t.Fatalf("%s integration wrappers must stay direct-tests unless explicitly re-reviewed", importPath)
		}
	}
}

func TestExampleOnlyPackagesBuildSmoke(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	var examplePackages []string
	for _, cls := range classes {
		if cls.TestStatus == "example-only" && inModule(cls.ImportPath, contribModulePath) {
			examplePackages = append(examplePackages, cls.ImportPath)
		}
	}
	sort.Strings(examplePackages)
	if len(examplePackages) == 0 {
		t.Fatal("no contrib example packages are classified for build smoke")
	}
	args := append([]string{"test"}, examplePackages...)
	out, err := runGoCmd(filepath.Join(repoRoot, "contrib"), args...)
	if err != nil {
		t.Fatalf("example-only package build smoke failed:\n%s\nerror: %v", out, err)
	}
}

func TestMakefileGateIntent(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))

	finalize := makeTargetRecipe(t, makefile, "finalize")
	if strings.Contains(finalize, "$(MAKE) api-check") || strings.Contains(finalize, "$(MAKE) release-api-check") {
		t.Fatal("finalize must not directly call api-check or release-api-check")
	}
	releaseCheck := makeTargetRecipe(t, makefile, "release-check")
	if !strings.Contains(releaseCheck, "$(MAKE) release-api-check") {
		t.Fatal("release-check must call release-api-check")
	}
	if !strings.Contains(releaseCheck, "$(MAKE) contrib-api-drift-report") {
		t.Fatal("release-check must call contrib-api-drift-report")
	}
	if !strings.Contains(releaseCheck, "$(MAKE) contrib-release-notes-check") {
		t.Fatal("release-check must call contrib-release-notes-check")
	}
	if !strings.Contains(releaseCheck, "$(MAKE) v3-readiness-check") {
		t.Fatal("release-check must call v3-readiness-check")
	}
	v3Readiness := makeTargetRecipe(t, makefile, "v3-readiness-check")
	for _, required := range []string{
		"TestV3DebtChecklistRowsStayExecutable",
		"TestCompatibilityRoadmapCoversDocumentedSensitiveSurfaces",
		"TestCompatibilitySensitivePortsGovernanceDocs",
		"TestPublicExamplesDoNotTeachLegacyCompatibilitySurfaces",
		"TestReleaseNotesIncludeStableSurfaceChecklist",
	} {
		if !strings.Contains(v3Readiness, required) {
			t.Fatalf("v3-readiness-check must run %s", required)
		}
	}
	contribReport := makeTargetRecipe(t, makefile, "contrib-api-drift-report")
	if !strings.Contains(contribReport, "scripts/contrib_api_drift_report.sh") {
		t.Fatal("contrib-api-drift-report must call the contrib API drift script")
	}
	contribNotes := makeTargetRecipe(t, makefile, "contrib-release-notes-check")
	if !strings.Contains(contribNotes, "scripts/contrib_release_notes_check.sh") {
		t.Fatal("contrib-release-notes-check must call the contrib release notes review script")
	}
}

func TestCIGovernanceWorkflowRunsDocsAndContribReleaseNoteGates(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	workflow := readText(t, filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))

	for _, required := range []string{
		"make docs-check",
		"make v3-readiness-check",
		"make contrib-release-notes-check",
		"Contrib API drift gate (pull request base)",
		"make contrib-api-drift-report",
		"CONTRIB_RELEASE_BASE_REF: origin/${{ github.base_ref }}",
		"API_BASE_REF: origin/${{ github.base_ref }}",
		"git fetch origin \"${{ github.base_ref }}:refs/remotes/origin/${{ github.base_ref }}\"",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf(".github/workflows/ci.yml missing governance requirement %q", required)
		}
	}
}

func TestReleaseEvidenceTarget(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))
	target := makeTargetRecipe(t, makefile, "release-evidence")
	script := readText(t, filepath.Join(repoRoot, "scripts", "release_check_summary.sh"))

	for _, required := range []string{
		"API_BASE_REF is required for release-evidence",
		"mktemp",
		"scripts/release_check_summary.sh --run",
		"release-check-summary.json",
	} {
		if !strings.Contains(target, required) {
			t.Fatalf("release-evidence target missing %q", required)
		}
	}
	for _, required := range []string{
		"ALLOW_DIRTY_RELEASE_EVIDENCE",
		"skipped_by_provenance_policy",
		"publication_eligible",
		"publication_artifact_expectations",
		"release-evidence-logs.tgz",
		"provenance_policy",
		"vulnerability_evidence",
		"missing_disposition_count",
		"expired_disposition_count",
		"contrib_drift_packages_from_log",
		"full_profile_scaffold_evidence",
		"openapi_31_full_scaffold",
		"typed_client_generation",
		"resource_generator_check",
		"provider_flag_generation",
		"worker_check",
		"integration_workflow",
		"asset_validation",
		"make full-profile-scaffold-check",
		"FULL_PROFILE_INTEGRATION_CHECK_STATUS",
		"publication_artifact_checksums",
		"release-asset-manifest.tsv",
		"docs/vulnerability-dispositions.tsv",
		"docs/contrib-api-drift-dispositions.tsv",
		"make contrib-release-notes-check",
		"make v3-readiness-check",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("release evidence script missing %q", required)
		}
	}
}

func TestReleaseReviewChecklistDiscoverability(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	checklist := readText(t, filepath.Join(repoRoot, "docs", "release-review.md"))

	for _, required := range []string{
		"docs/release-review.md",
		"Release review checklist",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("README missing release review checklist link/text %q", required)
		}
	}
	for _, required := range []string{
		"docs/release-runbook.md",
		"docs/release-notes.md",
		"VERSIONING.md",
		"docs/package-classification.tsv",
		"docs/dependency-risk.md",
		"docs/vulnerability-dispositions.tsv",
		"docs/contrib-api-drift-dispositions.tsv",
		"docs/ports-surface.md",
		"docs/v3-compatibility-roadmap.md",
		"release-check-summary.json",
		"publication_eligible",
		"release-evidence-logs.tgz",
		"release-asset-manifest.tsv",
		"make release-artifact-verify",
		".ci-result/release-evidence/logs/contrib-api-drift-report.log",
		"ALLOW_DIRTY_RELEASE_EVIDENCE=1",
		"dirty local evidence is rejected before publishing",
		"GitHub release workflow evidence is the publication-grade source",
	} {
		if !strings.Contains(checklist, required) {
			t.Fatalf("docs/release-review.md missing %q", required)
		}
	}
}

func TestReleaseEvidenceModePolicyDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))
	runbook := readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))
	review := readText(t, filepath.Join(repoRoot, "docs", "release-review.md"))
	workflow := readText(t, filepath.Join(repoRoot, ".github", "workflows", "release.yml"))

	for _, source := range []struct {
		name string
		text string
	}{
		{"README.md", readme},
		{"VERSIONING.md", versioning},
		{"docs/release-review.md", review},
	} {
		for _, required := range []string{
			"API_BASE_REF=v2.1.0",
			"ALLOW_DIRTY_RELEASE_EVIDENCE=1",
			"local dirty-tree audit",
			"not acceptable before publishing",
			"docs/release-runbook.md",
		} {
			if !strings.Contains(source.text, required) {
				t.Fatalf("%s missing release evidence mode policy text %q", source.name, required)
			}
		}
	}
	for _, required := range []string{
		"API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make release-evidence",
		"API_BASE_REF=v2.1.0",
		"ALLOW_DIRTY_RELEASE_EVIDENCE=1",
		"local dirty-tree audit",
		"not acceptable before publishing",
	} {
		if !strings.Contains(runbook, required) {
			t.Fatalf("docs/release-runbook.md missing release evidence mode policy text %q", required)
		}
	}
	for _, required := range []string{
		"tags:",
		"draft: true",
		"make release-evidence",
		"release-evidence-logs.tgz",
		"Verify SBOM signatures",
		"release-asset-manifest.tsv",
		"make release-artifact-verify",
		"Upload evidence and assets to draft release",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf(".github/workflows/release.yml missing pre-publication release workflow text %q", required)
		}
	}
}

func TestReleaseEvidenceSummarySchema(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	cmd := exec.CommandContext(context.Background(), "bash", "scripts/release_check_summary.sh")
	cmd.Dir = repoRoot
	cmd.Env = releaseEvidenceEnv("ALLOW_DIRTY_RELEASE_EVIDENCE=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("release evidence schema command failed: %v", err)
	}

	var summary struct {
		Schema   string `json:"schema"`
		Commit   string `json:"commit"`
		GitState struct {
			Commit         string  `json:"commit"`
			Branch         *string `json:"branch"`
			Detached       bool    `json:"detached"`
			Dirty          bool    `json:"dirty"`
			StagedCount    int     `json:"staged_count"`
			UnstagedCount  int     `json:"unstaged_count"`
			UntrackedCount int     `json:"untracked_count"`
			DeletedCount   int     `json:"deleted_count"`
		} `json:"git_state"`
		ProvenancePolicy struct {
			Mode                      string `json:"mode"`
			AllowDirtyReleaseEvidence bool   `json:"allow_dirty_release_evidence"`
			Status                    string `json:"status"`
			Message                   string `json:"message"`
		} `json:"provenance_policy"`
		APIBaseRef       string `json:"api_base_ref"`
		APICompatibility struct {
			PreviousTag             string   `json:"previous_tag"`
			PreviousRef             string   `json:"previous_ref"`
			CheckedPackageCount     int      `json:"checked_package_count"`
			CheckedPackages         []string `json:"checked_packages"`
			IncompatibleChangeCount *int     `json:"incompatible_change_count"`
			IgnoredExceptionCount   int      `json:"ignored_exception_count"`
			GeneratedReportPath     string   `json:"generated_report_path"`
			LogAvailable            bool     `json:"log_available"`
		} `json:"api_compatibility"`
		QualityCommand      string `json:"quality_command"`
		EvidenceCommand     string `json:"evidence_command"`
		Status              string `json:"status"`
		PublicationEligible bool   `json:"publication_eligible"`
		Checks              []struct {
			Name         string `json:"name"`
			CommandLine  string `json:"command_line"`
			Status       string `json:"status"`
			ExitCode     *int   `json:"exit_code"`
			DurationMS   *int64 `json:"duration_ms"`
			LogAvailable bool   `json:"log_available"`
			LogPath      string `json:"log_path"`
			Artifacts    []any  `json:"artifacts"`
		} `json:"checks"`
		ToolVersions []struct {
			Name        string `json:"name"`
			CommandLine string `json:"command_line"`
			Status      string `json:"status"`
			ExitCode    int    `json:"exit_code"`
			Version     string `json:"version"`
		} `json:"tool_versions"`
		VulnerabilityEvidence struct {
			SourceLogPath                             string   `json:"source_log_path"`
			Status                                    string   `json:"status"`
			ReviewDate                                string   `json:"review_date"`
			CalledVulnerabilityCount                  *int     `json:"called_vulnerability_count"`
			ImportedNotCalledVulnerabilityCount       *int     `json:"imported_not_called_vulnerability_count"`
			RequiredNotCalledModuleVulnerabilityCount *int     `json:"required_not_called_module_vulnerability_count"`
			ImportedNotCalledIDs                      []string `json:"imported_not_called_ids"`
			DispositionManifestPath                   string   `json:"disposition_manifest_path"`
			MissingDispositionCount                   int      `json:"missing_disposition_count"`
			ExpiredDispositionCount                   int      `json:"expired_disposition_count"`
			DispositionIssues                         []struct {
				ID        string `json:"id"`
				Status    string `json:"status"`
				ExpiresOn string `json:"expires_on"`
				Owner     string `json:"owner"`
				Message   string `json:"message"`
			} `json:"disposition_issues"`
			ReviewDisposition string `json:"review_disposition"`
		} `json:"vulnerability_evidence"`
		ContribDrift struct {
			CommandLine             string `json:"command_line"`
			Status                  string `json:"status"`
			ExitCode                *int   `json:"exit_code"`
			DurationMS              *int64 `json:"duration_ms"`
			LogAvailable            bool   `json:"log_available"`
			ArtifactPath            string `json:"artifact_path"`
			DispositionManifestPath string `json:"disposition_manifest_path"`
			ReviewDate              string `json:"review_date"`
			DriftPackageCount       *int   `json:"drift_package_count"`
			SkippedPackageCount     *int   `json:"skipped_package_count"`
			CompatibleDriftCount    *int   `json:"compatible_drift_count"`
			IncompatibleDriftCount  *int   `json:"incompatible_drift_count"`
			Packages                []struct {
				Package string `json:"package"`
				Status  string `json:"status"`
			} `json:"packages"`
			MissingDispositionCount int `json:"missing_disposition_count"`
			ExpiredDispositionCount int `json:"expired_disposition_count"`
			DispositionIssues       []struct {
				ID        string `json:"id"`
				Status    string `json:"status"`
				ExpiresOn string `json:"expires_on"`
				Owner     string `json:"owner"`
				Message   string `json:"message"`
			} `json:"disposition_issues"`
		} `json:"contrib_drift"`
		FullProfileScaffoldEvidence struct {
			Profile            string `json:"profile"`
			ContractTest       string `json:"contract_test"`
			ScaffoldValidation struct {
				CheckName       string `json:"check_name"`
				CommandLine     string `json:"command_line"`
				Status          string `json:"status"`
				LogPath         string `json:"log_path"`
				ReleaseBlocking bool   `json:"release_blocking"`
			} `json:"scaffold_validation"`
			OpenAPI31FullScaffold struct {
				Version         string `json:"version"`
				Source          string `json:"source"`
				GoldenPath      string `json:"golden_path"`
				Status          string `json:"status"`
				ReleaseBlocking bool   `json:"release_blocking"`
			} `json:"openapi_31_full_scaffold"`
			ClientGeneration struct {
				MakeTarget      string `json:"make_target"`
				CommandLine     string `json:"command_line"`
				Status          string `json:"status"`
				LogPath         string `json:"log_path"`
				CheckedInOutput bool   `json:"checked_in_output"`
				Style           string `json:"style"`
			} `json:"client_generation"`
			TypedClientGeneration struct {
				MakeTarget            string `json:"make_target"`
				CommandLine           string `json:"command_line"`
				Status                string `json:"status"`
				LogPath               string `json:"log_path"`
				CheckedInOutput       bool   `json:"checked_in_output"`
				RawStyleCompatibility string `json:"raw_style_compatibility"`
			} `json:"typed_client_generation"`
			TypeScriptClientGeneration struct {
				MakeTarget      string `json:"make_target"`
				CommandLine     string `json:"command_line"`
				ContractTest    string `json:"contract_test"`
				Status          string `json:"status"`
				LogPath         string `json:"log_path"`
				CheckedInOutput string `json:"checked_in_output"`
			} `json:"typescript_client_generation"`
			ResourceGeneratorCheck struct {
				MakeTarget      string `json:"make_target"`
				CommandLine     string `json:"command_line"`
				ContractTest    string `json:"contract_test"`
				Status          string `json:"status"`
				LogPath         string `json:"log_path"`
				ReleaseBlocking bool   `json:"release_blocking"`
			} `json:"resource_generator_check"`
			ProviderFlagGeneration struct {
				Flags           []string `json:"flags"`
				ContractTest    string   `json:"contract_test"`
				Status          string   `json:"status"`
				LogPath         string   `json:"log_path"`
				ReleaseBlocking bool     `json:"release_blocking"`
			} `json:"provider_flag_generation"`
			ObservabilityBundle struct {
				CommandLine     string `json:"command_line"`
				ContractTest    string `json:"contract_test"`
				Status          string `json:"status"`
				ReleaseBlocking bool   `json:"release_blocking"`
			} `json:"observability_bundle"`
			AssetValidation struct {
				MakeTargets     []string `json:"make_targets"`
				CommandLine     string   `json:"command_line"`
				ContractTest    string   `json:"contract_test"`
				Status          string   `json:"status"`
				LogPath         string   `json:"log_path"`
				ReleaseBlocking bool     `json:"release_blocking"`
			} `json:"asset_validation"`
			DeploymentPackaging struct {
				Helm struct {
					CommandLine     string `json:"command_line"`
					Status          string `json:"status"`
					ReleaseBlocking bool   `json:"release_blocking"`
				} `json:"helm"`
				TerraformAWS struct {
					CommandLine     string `json:"command_line"`
					Status          string `json:"status"`
					ReleaseBlocking bool   `json:"release_blocking"`
				} `json:"terraform_aws"`
			} `json:"deployment_packaging"`
			MigrationLifecycle struct {
				Commands        []string `json:"commands"`
				Guard           string   `json:"guard"`
				Status          string   `json:"status"`
				ReleaseBlocking bool     `json:"release_blocking"`
			} `json:"migration_lifecycle"`
			SessionProfile struct {
				Profile         string   `json:"profile"`
				AuthModes       []string `json:"auth_modes"`
				ContractTest    string   `json:"contract_test"`
				Status          string   `json:"status"`
				ReleaseBlocking bool     `json:"release_blocking"`
			} `json:"session_profile"`
			WorkerCheck struct {
				Binary          string `json:"binary"`
				CommandLine     string `json:"command_line"`
				Status          string `json:"status"`
				LogPath         string `json:"log_path"`
				ReleaseBlocking bool   `json:"release_blocking"`
			} `json:"worker_check"`
			IntegrationWorkflow struct {
				Path            string `json:"path"`
				TriggerPolicy   string `json:"trigger_policy"`
				Status          string `json:"status"`
				LogPath         string `json:"log_path"`
				ReleaseBlocking bool   `json:"release_blocking"`
			} `json:"integration_workflow"`
			IntegrationCheck struct {
				CommandLine     string  `json:"command_line"`
				Status          string  `json:"status"`
				LogPath         *string `json:"log_path"`
				ReleaseBlocking bool    `json:"release_blocking"`
			} `json:"integration_check"`
		} `json:"full_profile_scaffold_evidence"`
		ArtifactTiers                   map[string]any `json:"artifact_tiers"`
		PublicationArtifactExpectations struct {
			LocalEvidenceAssets       []string `json:"local_evidence_assets"`
			GitHubDraftReleaseAssets  []string `json:"github_draft_release_assets"`
			GitHubAttestationSubjects []string `json:"github_attestation_subjects"`
			LocalGeneratesSignedSBOMs bool     `json:"local_generates_signed_sboms"`
		} `json:"publication_artifact_expectations"`
		SBOMStatus string   `json:"sbom_status"`
		SBOMAssets []string `json:"sbom_assets"`
	}
	if err := json.Unmarshal(out, &summary); err != nil {
		t.Fatalf("release evidence summary is not valid JSON:\n%s\nerror: %v", out, err)
	}
	if summary.Schema != "github.com/aatuh/api-toolkit/release-check-summary/v2" {
		t.Fatalf("schema = %q, want v2", summary.Schema)
	}
	if summary.APIBaseRef != "v2.1.0" {
		t.Fatalf("api_base_ref = %q, want v2.1.0", summary.APIBaseRef)
	}
	if summary.APICompatibility.PreviousTag != summary.APIBaseRef || summary.APICompatibility.PreviousRef != summary.APIBaseRef {
		t.Fatalf("api_compatibility previous ref/tag = %q/%q, want api_base_ref %q", summary.APICompatibility.PreviousRef, summary.APICompatibility.PreviousTag, summary.APIBaseRef)
	}
	wantAPIPackages := stablePackagesFromAPICheck(t, filepath.Join(repoRoot, "scripts", "apicheck.sh"))
	assertStringSlicesEqual(t, "api_compatibility checked packages", uniqueSorted(summary.APICompatibility.CheckedPackages), wantAPIPackages)
	if summary.APICompatibility.CheckedPackageCount != len(wantAPIPackages) {
		t.Fatalf("api_compatibility checked_package_count = %d, want %d", summary.APICompatibility.CheckedPackageCount, len(wantAPIPackages))
	}
	if summary.APICompatibility.IgnoredExceptionCount != 0 {
		t.Fatalf("api_compatibility ignored_exception_count = %d, want 0", summary.APICompatibility.IgnoredExceptionCount)
	}
	if summary.APICompatibility.GeneratedReportPath != ".ci-result/release-evidence/logs/release-api-check.log" {
		t.Fatalf("api_compatibility report metadata invalid: %+v", summary.APICompatibility)
	}
	if summary.APICompatibility.LogAvailable {
		if summary.APICompatibility.IncompatibleChangeCount == nil || *summary.APICompatibility.IncompatibleChangeCount < 0 {
			t.Fatalf("api_compatibility incompatible_change_count = %v, want non-negative when log is available", summary.APICompatibility.IncompatibleChangeCount)
		}
	} else if summary.APICompatibility.IncompatibleChangeCount != nil {
		t.Fatalf("api_compatibility incompatible_change_count = %v, want null when log is unavailable", summary.APICompatibility.IncompatibleChangeCount)
	}
	if summary.Commit == "" || summary.GitState.Commit != summary.Commit {
		t.Fatalf("git_state commit = %q, want top-level commit %q", summary.GitState.Commit, summary.Commit)
	}
	if summary.GitState.StagedCount < 0 || summary.GitState.UnstagedCount < 0 ||
		summary.GitState.UntrackedCount < 0 || summary.GitState.DeletedCount < 0 {
		t.Fatalf("git_state counts must be non-negative: %+v", summary.GitState)
	}
	if summary.ProvenancePolicy.Mode == "" || summary.ProvenancePolicy.Status == "" || summary.ProvenancePolicy.Message == "" {
		t.Fatalf("provenance_policy missing required fields: %+v", summary.ProvenancePolicy)
	}
	if !summary.ProvenancePolicy.AllowDirtyReleaseEvidence {
		t.Fatalf("schema-only test runs in local audit mode and should record dirty override: %+v", summary.ProvenancePolicy)
	}
	if summary.PublicationEligible {
		t.Fatal("schema-only dirty local audit summary must not be publication eligible")
	}
	for _, required := range []string{
		"API_BASE_REF=v2.1.0",
		"GOTOOLCHAIN=local",
		"make release-check",
	} {
		if !strings.Contains(summary.QualityCommand, required) {
			t.Fatalf("quality_command missing %q: %s", required, summary.QualityCommand)
		}
	}
	if !strings.Contains(summary.EvidenceCommand, "make release-evidence") {
		t.Fatalf("evidence_command = %q, want make release-evidence", summary.EvidenceCommand)
	}

	wantChecks := makeSubtargets(t, readText(t, filepath.Join(repoRoot, "Makefile")), "release-check")
	var gotChecks []string
	for _, check := range summary.Checks {
		gotChecks = append(gotChecks, check.Name)
		if check.CommandLine == "" {
			t.Fatalf("check %s missing command_line", check.Name)
		}
		if check.Status == "" {
			t.Fatalf("check %s missing status", check.Name)
		}
		if check.Artifacts == nil {
			t.Fatalf("check %s missing artifacts array", check.Name)
		}
	}
	assertStringSlicesEqual(t, "release evidence checks vs make release-check", gotChecks, wantChecks)

	toolNames := make(map[string]bool)
	for _, tool := range summary.ToolVersions {
		toolNames[tool.Name] = true
		if tool.CommandLine == "" || tool.Status == "" {
			t.Fatalf("tool version entry for %s missing command or status", tool.Name)
		}
		if tool.Name == "govulncheck" && tool.Status == "error" {
			t.Fatalf("govulncheck tool version must not be recorded as usage-error evidence: %+v", tool)
		}
	}
	for _, required := range []string{"go", "golangci-lint", "govulncheck", "gosec", "apidiff", "syft", "cosign"} {
		if !toolNames[required] {
			t.Fatalf("tool_versions missing %s", required)
		}
	}
	if !strings.Contains(summary.ContribDrift.CommandLine, "make contrib-api-drift-report") {
		t.Fatalf("contrib_drift command_line = %q, want contrib-api-drift-report", summary.ContribDrift.CommandLine)
	}
	if summary.VulnerabilityEvidence.SourceLogPath != ".ci-result/release-evidence/logs/vuln.log" {
		t.Fatalf("vulnerability evidence log path = %q", summary.VulnerabilityEvidence.SourceLogPath)
	}
	if !strings.Contains(summary.VulnerabilityEvidence.ReviewDisposition, "docs/dependency-risk.md") {
		t.Fatalf("vulnerability review disposition missing dependency-risk pointer: %q", summary.VulnerabilityEvidence.ReviewDisposition)
	}
	if summary.VulnerabilityEvidence.DispositionManifestPath != "docs/vulnerability-dispositions.tsv" {
		t.Fatalf("vulnerability disposition manifest = %q", summary.VulnerabilityEvidence.DispositionManifestPath)
	}
	if summary.VulnerabilityEvidence.ReviewDate == "" {
		t.Fatal("vulnerability evidence missing review_date")
	}
	if summary.VulnerabilityEvidence.MissingDispositionCount < 0 || summary.VulnerabilityEvidence.ExpiredDispositionCount < 0 {
		t.Fatalf("vulnerability disposition counts must be non-negative: %+v", summary.VulnerabilityEvidence)
	}
	if summary.VulnerabilityEvidence.DispositionIssues == nil {
		t.Fatal("vulnerability evidence missing disposition_issues array")
	}
	if summary.ContribDrift.Status != "not_run" {
		t.Fatalf("contrib_drift status = %q, want not_run for schema-only summary", summary.ContribDrift.Status)
	}
	if summary.ContribDrift.ArtifactPath != ".ci-result/release-evidence/logs/contrib-api-drift-report.log" {
		t.Fatalf("contrib_drift artifact_path = %q", summary.ContribDrift.ArtifactPath)
	}
	if summary.ContribDrift.DispositionManifestPath != "docs/contrib-api-drift-dispositions.tsv" {
		t.Fatalf("contrib drift disposition manifest = %q", summary.ContribDrift.DispositionManifestPath)
	}
	if summary.ContribDrift.ReviewDate == "" {
		t.Fatal("contrib drift summary missing review_date")
	}
	if summary.ContribDrift.Packages == nil {
		t.Fatal("contrib drift summary missing packages array")
	}
	if summary.ContribDrift.MissingDispositionCount < 0 || summary.ContribDrift.ExpiredDispositionCount < 0 {
		t.Fatalf("contrib disposition counts must be non-negative: %+v", summary.ContribDrift)
	}
	if summary.ContribDrift.DispositionIssues == nil {
		t.Fatal("contrib drift summary missing disposition_issues array")
	}
	if summary.FullProfileScaffoldEvidence.Profile != "saas-api-full" {
		t.Fatalf("full profile evidence profile = %q, want saas-api-full", summary.FullProfileScaffoldEvidence.Profile)
	}
	if !strings.Contains(summary.FullProfileScaffoldEvidence.ContractTest, "TestNewServiceGeneratesBuildableSaaSAPIFull") {
		t.Fatalf("full profile evidence contract_test = %q", summary.FullProfileScaffoldEvidence.ContractTest)
	}
	if summary.FullProfileScaffoldEvidence.ScaffoldValidation.CheckName != "full-profile-scaffold-check" {
		t.Fatalf("full profile scaffold check_name = %q", summary.FullProfileScaffoldEvidence.ScaffoldValidation.CheckName)
	}
	if !strings.Contains(summary.FullProfileScaffoldEvidence.ScaffoldValidation.CommandLine, "make full-profile-scaffold-check") {
		t.Fatalf("full profile scaffold command_line = %q", summary.FullProfileScaffoldEvidence.ScaffoldValidation.CommandLine)
	}
	if summary.FullProfileScaffoldEvidence.ScaffoldValidation.LogPath != ".ci-result/release-evidence/logs/full-profile-scaffold-check.log" {
		t.Fatalf("full profile scaffold log path = %q", summary.FullProfileScaffoldEvidence.ScaffoldValidation.LogPath)
	}
	if !summary.FullProfileScaffoldEvidence.ScaffoldValidation.ReleaseBlocking {
		t.Fatal("full profile scaffold validation must be release blocking")
	}
	if summary.FullProfileScaffoldEvidence.OpenAPI31FullScaffold.Version != "3.1.0" ||
		summary.FullProfileScaffoldEvidence.OpenAPI31FullScaffold.GoldenPath != "testdata/openapi.golden.json" ||
		!summary.FullProfileScaffoldEvidence.OpenAPI31FullScaffold.ReleaseBlocking {
		t.Fatalf("full profile OpenAPI 3.1 evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.OpenAPI31FullScaffold)
	}
	if summary.FullProfileScaffoldEvidence.ClientGeneration.MakeTarget != "client-check" ||
		!summary.FullProfileScaffoldEvidence.ClientGeneration.CheckedInOutput ||
		summary.FullProfileScaffoldEvidence.ClientGeneration.Style != "typed" ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.ClientGeneration.CommandLine, "make client-check") {
		t.Fatalf("full profile client generation evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.ClientGeneration)
	}
	if summary.FullProfileScaffoldEvidence.TypedClientGeneration.MakeTarget != "client-check" ||
		!summary.FullProfileScaffoldEvidence.TypedClientGeneration.CheckedInOutput ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.TypedClientGeneration.CommandLine, "--style typed") ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.TypedClientGeneration.RawStyleCompatibility, "--style raw") {
		t.Fatalf("full profile typed client evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.TypedClientGeneration)
	}
	if summary.FullProfileScaffoldEvidence.TypeScriptClientGeneration.MakeTarget != "client-ts-check" ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.TypeScriptClientGeneration.CommandLine, "clients typescript") ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.TypeScriptClientGeneration.ContractTest, "TestClientsTypeScriptGeneratesFetchPackage") {
		t.Fatalf("full profile TypeScript client evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.TypeScriptClientGeneration)
	}
	if summary.FullProfileScaffoldEvidence.ResourceGeneratorCheck.MakeTarget != "resource-check" ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.ResourceGeneratorCheck.ContractTest, "TestGenerateResourceAddsTenantScopedCRUDToFullProfile") ||
		!summary.FullProfileScaffoldEvidence.ResourceGeneratorCheck.ReleaseBlocking {
		t.Fatalf("full profile resource generator evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.ResourceGeneratorCheck)
	}
	for _, want := range []string{"--with stripe-billing", "--with resend-email", "--with clerk-webhooks", "--with entitlements"} {
		if !containsString(summary.FullProfileScaffoldEvidence.ProviderFlagGeneration.Flags, want) {
			t.Fatalf("full profile provider flag evidence missing %q: %+v", want, summary.FullProfileScaffoldEvidence.ProviderFlagGeneration)
		}
	}
	if !strings.Contains(summary.FullProfileScaffoldEvidence.ProviderFlagGeneration.ContractTest, "TestNewServiceGeneratesFullProfileProviderWorkflows") ||
		!summary.FullProfileScaffoldEvidence.ProviderFlagGeneration.ReleaseBlocking {
		t.Fatalf("full profile provider flag evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.ProviderFlagGeneration)
	}
	if !strings.Contains(summary.FullProfileScaffoldEvidence.ObservabilityBundle.CommandLine, "ops observability") ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.ObservabilityBundle.ContractTest, "TestOpsObservabilityGeneratesBundle") ||
		!summary.FullProfileScaffoldEvidence.ObservabilityBundle.ReleaseBlocking {
		t.Fatalf("full profile observability evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.ObservabilityBundle)
	}
	for _, want := range []string{"asset-check", "observability-check", "deploy-check"} {
		if !containsString(summary.FullProfileScaffoldEvidence.AssetValidation.MakeTargets, want) {
			t.Fatalf("full profile asset validation evidence missing %q: %+v", want, summary.FullProfileScaffoldEvidence.AssetValidation)
		}
	}
	if !strings.Contains(summary.FullProfileScaffoldEvidence.AssetValidation.CommandLine, "make asset-check observability-check deploy-check") ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.AssetValidation.ContractTest, "TestNewServiceGeneratesFullProfileProviderWorkflows") ||
		summary.FullProfileScaffoldEvidence.AssetValidation.LogPath != ".ci-result/release-evidence/logs/full-profile-scaffold-check.log" ||
		!summary.FullProfileScaffoldEvidence.AssetValidation.ReleaseBlocking {
		t.Fatalf("full profile asset validation evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.AssetValidation)
	}
	if !strings.Contains(summary.FullProfileScaffoldEvidence.DeploymentPackaging.Helm.CommandLine, "deploy helm") ||
		!summary.FullProfileScaffoldEvidence.DeploymentPackaging.Helm.ReleaseBlocking ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.DeploymentPackaging.TerraformAWS.CommandLine, "deploy terraform --cloud aws") ||
		!summary.FullProfileScaffoldEvidence.DeploymentPackaging.TerraformAWS.ReleaseBlocking {
		t.Fatalf("full profile deployment evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.DeploymentPackaging)
	}
	for _, want := range []string{"plan", "up", "status", "check", "verify", "down_guarded"} {
		if !containsString(summary.FullProfileScaffoldEvidence.MigrationLifecycle.Commands, want) {
			t.Fatalf("full profile migration lifecycle evidence missing %q: %+v", want, summary.FullProfileScaffoldEvidence.MigrationLifecycle)
		}
	}
	if !strings.Contains(summary.FullProfileScaffoldEvidence.MigrationLifecycle.Guard, "ALLOW_DANGEROUS_MIGRATION_DOWN=true") ||
		!summary.FullProfileScaffoldEvidence.MigrationLifecycle.ReleaseBlocking {
		t.Fatalf("full profile migration lifecycle evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.MigrationLifecycle)
	}
	if summary.FullProfileScaffoldEvidence.SessionProfile.Profile != "saas-web" ||
		!containsString(summary.FullProfileScaffoldEvidence.SessionProfile.AuthModes, "session") ||
		!containsString(summary.FullProfileScaffoldEvidence.SessionProfile.AuthModes, "oidc-session") ||
		!strings.Contains(summary.FullProfileScaffoldEvidence.SessionProfile.ContractTest, "TestNewServiceGeneratesSaaSWebSessionProfile") ||
		!summary.FullProfileScaffoldEvidence.SessionProfile.ReleaseBlocking {
		t.Fatalf("full profile session evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.SessionProfile)
	}
	if summary.FullProfileScaffoldEvidence.WorkerCheck.Binary != "cmd/worker" ||
		!summary.FullProfileScaffoldEvidence.WorkerCheck.ReleaseBlocking {
		t.Fatalf("full profile worker evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.WorkerCheck)
	}
	if summary.FullProfileScaffoldEvidence.IntegrationWorkflow.Path != ".github/workflows/integration.yml" ||
		summary.FullProfileScaffoldEvidence.IntegrationWorkflow.TriggerPolicy != "workflow_dispatch_and_schedule_only" ||
		!summary.FullProfileScaffoldEvidence.IntegrationWorkflow.ReleaseBlocking {
		t.Fatalf("full profile integration workflow evidence incomplete: %+v", summary.FullProfileScaffoldEvidence.IntegrationWorkflow)
	}
	if summary.FullProfileScaffoldEvidence.IntegrationCheck.Status != "not_run_opt_in" ||
		summary.FullProfileScaffoldEvidence.IntegrationCheck.ReleaseBlocking {
		t.Fatalf("full profile integration evidence should be opt-in and non-blocking by default: %+v", summary.FullProfileScaffoldEvidence.IntegrationCheck)
	}
	if !strings.Contains(summary.FullProfileScaffoldEvidence.IntegrationCheck.CommandLine, "make generated-integration-check") {
		t.Fatalf("full profile integration command = %q", summary.FullProfileScaffoldEvidence.IntegrationCheck.CommandLine)
	}
	for _, tier := range []string{"local_release_evidence", "github_release_workflow"} {
		if _, ok := summary.ArtifactTiers[tier]; !ok {
			t.Fatalf("artifact_tiers missing %s", tier)
		}
	}
	for _, required := range []string{"release-check-summary.json", "release-evidence-logs.tgz", "sbom-root.spdx.json", "sbom-contrib.spdx.json"} {
		if !containsString(summary.PublicationArtifactExpectations.GitHubDraftReleaseAssets, required) {
			t.Fatalf("publication artifact expectations missing draft release asset %s", required)
		}
	}
	if summary.PublicationArtifactExpectations.LocalGeneratesSignedSBOMs {
		t.Fatal("local release evidence must not claim it generates signed SBOMs")
	}
	if len(summary.SBOMAssets) == 0 || summary.SBOMStatus == "" {
		t.Fatal("release evidence summary missing SBOM status/assets")
	}
}

func TestReleaseEvidenceGitStateContract(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts", "release_check_summary.sh")

	cleanRepo := newTempGitRepo(t)
	cleanSummary := releaseEvidenceSummaryForRepo(t, cleanRepo, scriptPath)
	if cleanSummary.GitState.Dirty {
		t.Fatalf("clean repo marked dirty: %+v", cleanSummary.GitState)
	}
	if cleanSummary.GitState.StagedCount != 0 || cleanSummary.GitState.UnstagedCount != 0 ||
		cleanSummary.GitState.UntrackedCount != 0 || cleanSummary.GitState.DeletedCount != 0 {
		t.Fatalf("clean repo git_state counts = %+v, want all zero", cleanSummary.GitState)
	}
	if cleanSummary.GitState.Branch == nil || *cleanSummary.GitState.Branch == "" {
		t.Fatalf("clean repo git_state branch missing: %+v", cleanSummary.GitState)
	}
	if cleanSummary.PublicationEligible {
		t.Fatal("schema-only clean summary without --run must not be publication eligible")
	}

	dirtyRepo := newTempGitRepo(t)
	writeTempFile(t, dirtyRepo, "tracked.txt", "changed\n")
	writeTempFile(t, dirtyRepo, "staged.txt", "staged\n")
	runGit(t, dirtyRepo, "add", "staged.txt")
	if err := os.Remove(filepath.Join(dirtyRepo, "deleted.txt")); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}
	writeTempFile(t, dirtyRepo, "untracked.txt", "untracked\n")

	dirtyFailure := releaseEvidenceFailureForRepo(t, dirtyRepo, scriptPath)
	if dirtyFailure.Status != "failed" || dirtyFailure.ProvenancePolicy.Status != "failed" {
		t.Fatalf("dirty repo without override should fail publication evidence: %+v", dirtyFailure.ProvenancePolicy)
	}
	if dirtyFailure.PublicationEligible {
		t.Fatal("dirty failure summary must not be publication eligible")
	}
	if !strings.Contains(dirtyFailure.ProvenancePolicy.Message, "ALLOW_DIRTY_RELEASE_EVIDENCE=1") {
		t.Fatalf("dirty failure message missing local-audit override guidance: %q", dirtyFailure.ProvenancePolicy.Message)
	}

	dirtySummary := releaseEvidenceSummaryForRepoWithEnv(t, dirtyRepo, scriptPath, "ALLOW_DIRTY_RELEASE_EVIDENCE=1")
	if !dirtySummary.GitState.Dirty {
		t.Fatalf("dirty repo marked clean: %+v", dirtySummary.GitState)
	}
	if dirtySummary.ProvenancePolicy.Status != "allowed_dirty_local_audit" {
		t.Fatalf("dirty override provenance status = %q", dirtySummary.ProvenancePolicy.Status)
	}
	if dirtySummary.PublicationEligible {
		t.Fatal("dirty local-audit summary must not be publication eligible")
	}
	if dirtySummary.GitState.StagedCount != 1 {
		t.Fatalf("dirty staged_count = %d, want 1", dirtySummary.GitState.StagedCount)
	}
	if dirtySummary.GitState.UnstagedCount != 2 {
		t.Fatalf("dirty unstaged_count = %d, want 2", dirtySummary.GitState.UnstagedCount)
	}
	if dirtySummary.GitState.UntrackedCount != 1 {
		t.Fatalf("dirty untracked_count = %d, want 1", dirtySummary.GitState.UntrackedCount)
	}
	if dirtySummary.GitState.DeletedCount != 1 {
		t.Fatalf("dirty deleted_count = %d, want 1", dirtySummary.GitState.DeletedCount)
	}
}

func TestDependencyRiskDispositionDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	review := readText(t, filepath.Join(repoRoot, "docs", "release-review.md"))
	risk := readText(t, filepath.Join(repoRoot, "docs", "dependency-risk.md"))
	manifest := strings.TrimSpace(readText(t, filepath.Join(repoRoot, "docs", "vulnerability-dispositions.tsv")))

	if !strings.Contains(readme, "docs/dependency-risk.md") {
		t.Fatal("README missing dependency risk documentation link")
	}
	if !strings.Contains(readme, "docs/vulnerability-dispositions.tsv") {
		t.Fatal("README missing vulnerability disposition manifest link")
	}
	if !strings.Contains(review, "dependency-risk.md") {
		t.Fatal("release review checklist missing dependency risk documentation link")
	}
	if !strings.Contains(review, "docs/vulnerability-dispositions.tsv") {
		t.Fatal("release review checklist missing vulnerability disposition manifest link")
	}
	for _, required := range []string{
		"Current imported-but-not-called count: `0`",
		"Owner decision",
		"Upgrade plan",
		"V39 advisory ownership map",
		"GO-2026-4762",
		"GO-2026-4771",
		"GO-2026-4772",
		"docs/vulnerability-dispositions.tsv",
		"release-check-summary.json",
		".ci-result/release-evidence/logs/vuln.log",
		"govulncheck",
		"does not fail solely because findings are imported but not called",
	} {
		if !strings.Contains(risk, required) {
			t.Fatalf("docs/dependency-risk.md missing %q", required)
		}
	}
	if strings.Contains(manifest, "\n") {
		t.Fatal("docs/vulnerability-dispositions.tsv should be header-only while current imported-only count is zero")
	}
}

func TestVulnerabilityDispositionManifestSupportsDynamicImportedIDs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	manifestPath := filepath.Join(repoRoot, "docs", "vulnerability-dispositions.tsv")
	manifest := strings.TrimSpace(readText(t, manifestPath))
	var records []map[string]string
	if strings.Contains(manifest, "\n") {
		records = loadTSVRecords(t, manifestPath)
	}
	summaryScript := readText(t, filepath.Join(repoRoot, "scripts", "release_check_summary.sh"))

	for _, record := range records {
		id := record["vulnerability_id"]
		if !strings.HasPrefix(id, "GO-") {
			t.Fatalf("vulnerability_id = %q, want GO-* advisory ID", id)
		}
		if record["called_status"] != "imported_not_called" {
			t.Fatalf("%s called_status = %q, want imported_not_called", id, record["called_status"])
		}
		for _, field := range []string{"owning_dependency", "affected_module", "affected_package", "reviewed_on", "expires_on", "owner", "upgrade_trigger"} {
			if strings.TrimSpace(record[field]) == "" {
				t.Fatalf("%s disposition missing %s", id, field)
			}
		}
		requireISODate(t, "reviewed_on for "+id, record["reviewed_on"])
		requireISODate(t, "expires_on for "+id, record["expires_on"])
	}
	if !strings.Contains(summaryScript, "docs/vulnerability-dispositions.tsv") {
		t.Fatal("release summary script must surface vulnerability disposition manifest path")
	}
	for _, required := range []string{
		"vulnerability_ids_from_log",
		"vulnerability_disposition_issues",
		"expires_on <= review_date",
		"missing_disposition_count",
		"expired_disposition_count",
	} {
		if !strings.Contains(summaryScript, required) {
			t.Fatalf("release summary script missing dynamic vulnerability disposition enforcement text %q", required)
		}
	}
	if regexp.MustCompile(`GO-[0-9]{4}-[0-9]+`).MatchString(summaryScript) {
		t.Fatal("release summary script must compare current vulnerability evidence dynamically, not hard-code GO advisory IDs")
	}
}

func TestAdapterCoveragePolicyReferencesPackageClassification(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	classification := readText(t, filepath.Join(repoRoot, "docs", "package-classification.tsv"))
	section := markdownSection(t, readme, "### Adapter coverage policy")

	for _, required := range []string{
		"`docs/package-classification.tsv`",
		"`wrapper-only`",
		"`wrapper-smoke-tested`",
		"`example-only`",
		"`needs-tests`",
		"direct tests unless explicitly classified",
		"interface satisfaction",
		"constructor/defaults",
		"disabled or nil behavior",
		"option propagation",
		"not behavior-complete",
	} {
		if !strings.Contains(section, required) {
			t.Fatalf("README adapter coverage policy missing %q", required)
		}
	}
	for _, required := range []string{
		"interface satisfaction",
		"constructor/defaults",
		"disabled or nil behavior",
		"option propagation",
		"not behavior-complete coverage",
	} {
		if !strings.Contains(classification, required) {
			t.Fatalf("docs/package-classification.tsv missing wrapper/example coverage policy %q", required)
		}
	}
}

func TestCompatibilityShimLifecycleRoadmap(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	roadmap := readText(t, filepath.Join(repoRoot, "docs", "v3-compatibility-roadmap.md"))

	for _, required := range []string{
		"## V3 Removal Matrix",
		"## V3 Owner Checklist",
		"`github.com/aatuh/api-toolkit/v3/compat/billing`",
		"`DatabasePoolSnapshotProvider`",
		"`ports.SnapshotDatabasePoolStats`",
		"`response_writer`",
		"`github.com/aatuh/api-toolkit/v3/httpx`",
		"`ports.IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)`",
		"`middleware/auth/authz.NewRequireRoleMiddlewareChecked`",
		"`ValidateRequireRoleMiddlewareRoutes`",
		"`ParseListQueryChecked`",
		"`DefaultFilterParserChecked`",
		"`DefaultSortParserChecked`",
		"Required evidence",
		"## Remaining Guardrails",
		"bounded",
		"streaming, SSE, websocket, and large-download",
	} {
		if !strings.Contains(roadmap, required) {
			t.Fatalf("docs/v3-compatibility-roadmap.md missing %q", required)
		}
	}
}

func TestIdempotencyCompatibilityMetricDocsStayBounded(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	docs := readText(t, filepath.Join(repoRoot, "docs", "metrics.md"))
	section := markdownSection(t, docs, "## Idempotency compatibility telemetry")
	code := readText(t, filepath.Join(repoRoot, "middleware", "idempotency", "legacy_compatibility.go"))

	for _, required := range []string{
		"`method`",
		"`store_class`",
		"`outcome`",
		"Do not add raw paths",
		"key hashes",
		"structured logs or traces",
		"bounded queue",
	} {
		if !strings.Contains(section, required) {
			t.Fatalf("docs/metrics.md missing idempotency metric guidance %q", required)
		}
	}
	if !strings.Contains(code, "legacyInFlightCompatibilityEventStoreClassLabel") {
		t.Fatal("idempotency compatibility metrics must expose store_class, not raw store types")
	}
	for _, forbidden := range []string{
		"legacyInFlightCompatibilityEventPathLabel:      event.Path",
		"legacyInFlightCompatibilityEventKeyLabel:       event.Key",
		"legacyInFlightCompatibilityEventErrorLabel:     event.Error",
	} {
		if strings.Contains(code, forbidden) {
			t.Fatalf("MetricLabels still exposes unbounded compatibility label expression %q", forbidden)
		}
	}
}

func TestHardTimeoutDocsCoverCaptureAndStreamingLimits(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	code := readText(t, filepath.Join(repoRoot, "middleware", "timeout", "timeout.go"))
	doc := readText(t, filepath.Join(repoRoot, "middleware", "timeout", "doc.go"))
	security := readText(t, filepath.Join(repoRoot, "docs", "security.md"))
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	readiness := readText(t, filepath.Join(repoRoot, "docs", "production-readiness.md"))

	for _, required := range []string{
		"MaxCaptureBytes",
		"defaultHardTimeoutMaxCaptureBytes",
		"ErrHardTimeoutCaptureLimitExceeded",
		"defaultHardTimeoutCaptureOverflowProblem",
	} {
		if !strings.Contains(code, required) {
			t.Fatalf("middleware/timeout missing capture limit implementation %q", required)
		}
	}
	for _, source := range []struct {
		name string
		text string
	}{
		{"middleware/timeout/doc.go", doc},
		{"docs/security.md", security},
		{"README.md", readme},
		{"docs/production-readiness.md", readiness},
	} {
		for _, required := range []string{"streaming", "server-sent events", "websocket", "http.ResponseWriter"} {
			if !strings.Contains(source.text, required) {
				t.Fatalf("%s missing hard-timeout streaming/interface limitation %q", source.name, required)
			}
		}
	}
}

func TestMaturityGovernanceDocsAreReleaseVisible(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	readiness := readText(t, filepath.Join(repoRoot, "docs", "production-readiness.md"))
	referenceService := readText(t, filepath.Join(repoRoot, "docs", "reference-service.md"))
	governance := readText(t, filepath.Join(repoRoot, "docs", "governance.md"))
	codeowners := readText(t, filepath.Join(repoRoot, ".github", "CODEOWNERS"))
	integration := readText(t, filepath.Join(repoRoot, ".github", "workflows", "integration.yml"))

	for _, required := range []string{
		"Production readiness",
		"docs/production-readiness.md",
		"not a universal backend platform",
		"Generated code is app-owned",
		"Stable core packages",
		"Supported contrib adapters",
		"Experimental contrib packages",
		"Streaming, SSE, WebSockets, and large downloads",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("README.md missing production readiness text %q", required)
		}
	}
	for _, required := range []string{
		"production-credible for conventional Go JSON/HTTP APIs",
		"not a universal backend platform",
		"Generated code is app-owned",
		"Supported-adapter incompatible drift is gate-enforced",
		"`saas-api-full` scaffold",
		"`x-api-toolkit-streaming`",
		"`securityprofile.StreamingRouteOverride`",
		"## Adapter Maturity Review",
		"evidence-complete supported adapter set",
		"intentionally not promoted",
		"docs/supported-adapter-contracts.tsv",
		"docs/contrib-api-drift-packages.txt",
		"Load, soak, and rollback evidence",
		"reference-service.md#adoption-evidence-template",
	} {
		if !strings.Contains(readiness, required) {
			t.Fatalf("docs/production-readiness.md missing %q", required)
		}
	}
	if !strings.Contains(readme, "adapter maturity review") {
		t.Fatal("docs/README.md missing adapter maturity review link text")
	}
	template := markdownSection(t, referenceService, "## Adoption Evidence Template")
	for _, required := range []string{
		"Setup time",
		"Upgrade result",
		"OpenAPI/client result",
		"Tenant isolation notes",
		"Idempotency notes",
		"Backup/restore notes",
		"Load-smoke notes",
		"Known pain points",
		"does not replace deployment-owned evidence",
	} {
		if !strings.Contains(template, required) {
			t.Fatalf("docs/reference-service.md adoption evidence template missing %q", required)
		}
	}
	for _, required := range []string{
		"Require CODEOWNERS review",
		"Protect `master`",
		"Protect `v*` release tags",
		"latest published v3 tag",
		"make github-governance-check",
		"Live provider checks are opt-in only",
	} {
		if !strings.Contains(governance, required) {
			t.Fatalf("docs/governance.md missing %q", required)
		}
	}
	for _, required := range []string{
		"* @aatuh",
		"/contrib/cmd/api-toolkit/",
		"/.github/",
		"/scripts/",
	} {
		if !strings.Contains(codeowners, required) {
			t.Fatalf(".github/CODEOWNERS missing %q", required)
		}
	}
	for _, required := range []string{
		"workflow_dispatch",
		"schedule:",
		"--profile saas-api-full",
		"make contracts-lint",
		"make contracts-diff",
		"make openapi-check",
		"make client-check",
		"make integration-check",
		"include-minio",
	} {
		if !strings.Contains(integration, required) {
			t.Fatalf(".github/workflows/integration.yml missing %q", required)
		}
	}
}

func TestReferenceServiceAppLocalDocsAreDiscoverable(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	referenceService := readText(t, filepath.Join(repoRoot, "docs", "reference-service.md"))
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	combined := referenceService + "\n" + docsIndex

	requiredTargets := map[string][]string{
		"examples/reference-saas-api/README.md": {
			"This service is app-owned generated code",
			"make openapi-check",
			"make contracts-lint",
		},
		"examples/reference-saas-api/deploy/helm/README.md": {
			"Required Values",
			"Required Secrets",
			"admin Service",
		},
		"examples/reference-saas-api/deploy/kubernetes/README.md": {
			"Required Configuration",
			"internal admin Service",
			"`/livez`",
			"`/readyz`",
		},
		"examples/reference-saas-api/deploy/terraform/aws/README.md": {
			"Outputs To Wire Into The Service",
			"RDS",
			"ElastiCache Redis",
		},
		"examples/reference-saas-api/observability/runbooks/observability.md": {
			"bounded labels",
			"SLO Defaults",
			"admin listener",
		},
		"examples/reference-saas-api/docs/providers/provider-runbook.md": {
			"app-owned starter code",
			"Live checks are operator-initiated only",
			"provider-live-check",
		},
	}

	for rel, required := range requiredTargets {
		if !strings.Contains(combined, rel) {
			t.Fatalf("reference-service docs must link or name app-local document %s", rel)
		}
		content := readText(t, filepath.Join(repoRoot, filepath.FromSlash(rel)))
		for _, text := range required {
			if !strings.Contains(content, text) {
				t.Fatalf("%s missing audience-purpose guidance %q", rel, text)
			}
		}
	}
}

func TestReferenceServicePackageTestInventoryIsExplicit(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	packages := listedGoPackages(t, filepath.Join(repoRoot, "examples", "reference-saas-api"))
	allowedNoDirectTests := map[string]string{
		"example.com/reference-saas-api/cmd/api":                   "entrypoint exercised by reference-service-check and Docker integration evidence",
		"example.com/reference-saas-api/cmd/migrate":               "entrypoint exercised by migration lifecycle and Docker integration evidence",
		"example.com/reference-saas-api/cmd/worker":                "entrypoint exercised by worker wiring and Docker integration evidence",
		"example.com/reference-saas-api/internal/client/apiclient": "generated typed client checked by client-check",
		"example.com/reference-saas-api/internal/domain":           "domain value types exercised through app, adapter, and HTTP tests",
	}

	var missing []string
	for _, pkg := range packages {
		if pkg.DirectTestFiles > 0 {
			continue
		}
		rationale, ok := allowedNoDirectTests[pkg.ImportPath]
		if !ok || strings.TrimSpace(rationale) == "" {
			missing = append(missing, pkg.ImportPath)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("reference service packages without direct tests need explicit inventory rationale:\n%s", strings.Join(missing, "\n"))
	}

	for importPath := range allowedNoDirectTests {
		found := false
		for _, pkg := range packages {
			if pkg.ImportPath == importPath {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("reference service test inventory has stale package %s", importPath)
		}
	}
}

func TestTrashArchiveStaysOutOfActiveDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	archiveReadme := readText(t, filepath.Join(repoRoot, ".trash", "README.md"))
	for _, required := range []string{
		"not active product documentation",
		"not active product",
		"should not link into `.trash/`",
		"explicit maintainer decision",
	} {
		if !strings.Contains(archiveReadme, required) {
			t.Fatalf(".trash/README.md missing archive policy %q", required)
		}
	}

	var violations []string
	for _, path := range docsQualityMarkdownFiles(t, repoRoot) {
		content := readText(t, path)
		if strings.Contains(content, ".trash/") || strings.Contains(content, "../.trash") {
			violations = append(violations, slashRel(repoRoot, path))
		}
	}
	if len(violations) > 0 {
		t.Fatalf("active docs must not link to .trash archive files:\n%s", strings.Join(violations, "\n"))
	}
}

func TestCoverageCheckIncludesHighRiskPackageFloors(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	script := readText(t, filepath.Join(repoRoot, "scripts", "coverage_check.sh"))

	for _, required := range []string{
		"(cd \"$dir\" && \"$go_cmd\" tool cover",
		"AUTH_APIKEY_COVERAGE_MIN",
		"AUTH_AUTHZ_COVERAGE_MIN",
		"AUTH_JWT_COVERAGE_MIN",
		"AUTH_TENANT_COVERAGE_MIN",
		"ENDPOINTS_DOCS_COVERAGE_MIN",
		"HTTPX_IDENTITY_COVERAGE_MIN",
		"HTTPX_RECOVER_COVERAGE_MIN",
		"IDEMPOTENCY_COVERAGE_MIN",
		"JSON_MIDDLEWARE_COVERAGE_MIN",
		"MAXBODY_COVERAGE_MIN",
		"QUERYLIMITS_COVERAGE_MIN",
		"RATELIMIT_COVERAGE_MIN",
		"SECURITYPROFILE_COVERAGE_MIN",
		"WEBHOOKS_COVERAGE_MIN",
		"HEALTH_COVERAGE_MIN",
		"ROUTECONTRACTS_COVERAGE_MIN",
		"SPECS_COVERAGE_MIN",
		"CONTRACTTEST_COVERAGE_MIN",
		"CONTRIB_BOOTSTRAP_COVERAGE_MIN",
		"CONTRIB_IDEMPOTENCYREDIS_COVERAGE_MIN",
		"CONTRIB_RATELIMITREDIS_COVERAGE_MIN",
		"CONTRIB_AUTH_OIDC_COVERAGE_MIN",
		"CONTRIB_METRICS_COVERAGE_MIN",
		"CONTRIB_OPENAPI_COVERAGE_MIN",
		"CONTRIB_OTELTRACE_COVERAGE_MIN",
		"CONTRIB_REQUESTLOG_COVERAGE_MIN",
		"CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("scripts/coverage_check.sh missing high-risk coverage gate %q", required)
		}
	}
}

func TestOptionalGovernanceAndGeneratedIntegrationChecksStayDocumented(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))
	governanceScript := readText(t, filepath.Join(repoRoot, "scripts", "github_governance_check.sh"))
	actionsAuditScript := readText(t, filepath.Join(repoRoot, "scripts", "actions_audit.sh"))
	integrationScript := readText(t, filepath.Join(repoRoot, "scripts", "generated_integration_check.sh"))
	upgradeCompatScript := readText(t, filepath.Join(repoRoot, "scripts", "generated_upgrade_compat_check.sh"))
	upgradeSmokeScript := readText(t, filepath.Join(repoRoot, "scripts", "upgrade_smoke_check.sh"))
	referenceCoverageScript := readText(t, filepath.Join(repoRoot, "scripts", "reference_service_coverage.sh"))
	referenceEvidenceScript := readText(t, filepath.Join(repoRoot, "scripts", "reference_service_evidence.sh"))
	ci := readText(t, filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
	runbook := readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))
	referenceService := readText(t, filepath.Join(repoRoot, "docs", "reference-service.md"))
	governance := readText(t, filepath.Join(repoRoot, "docs", "governance.md"))
	summaryScript := readText(t, filepath.Join(repoRoot, "scripts", "release_check_summary.sh"))
	coverageBacklog := readText(t, filepath.Join(repoRoot, "docs", "coverage-hardening-backlog.md"))

	for _, required := range []string{
		"generated-integration-check:",
		"generated-integration-check-minio:",
		"generated-upgrade-compat-check:",
		"upgrade-smoke-check:",
		"upgrade-smoke-contract:",
		"reference-service-evidence:",
		"reference-service-coverage:",
		"reference-service-evidence-contract:",
		"github-governance-check:",
		"actions-audit:",
		"actions-audit-contract:",
		"timeout-determinism-check:",
	} {
		if !strings.Contains(makefile, required) {
			t.Fatalf("Makefile missing optional target %q", required)
		}
	}
	if !strings.Contains(makeTargetRecipe(t, makefile, "audit-check"), "$(MAKE) actions-audit") {
		t.Fatal("audit-check must include actions-audit")
	}
	if !strings.Contains(makeTargetRecipe(t, makefile, "audit-check"), "$(MAKE) timeout-determinism-check") {
		t.Fatal("audit-check must include timeout-determinism-check")
	}
	if !strings.Contains(makeTargetRecipe(t, makefile, "docs-check"), "$(MAKE) upgrade-smoke-contract") {
		t.Fatal("docs-check must include upgrade-smoke-contract")
	}
	for _, required := range []string{
		"actions/attest-build-provenance v1",
		"unpinned action ref",
		"stale generated checkout action",
		"stale generated setup-go action",
	} {
		if !strings.Contains(actionsAuditScript, required) {
			t.Fatalf("actions audit script missing %q", required)
		}
	}
	for _, required := range []string{
		"gh is not installed; skipping optional governance verification",
		"require_code_owner_reviews",
		"allow_force_pushes.enabled == false",
		"rulesets?includes_parents=true",
		"refs/tags/contrib/v*",
	} {
		if !strings.Contains(governanceScript, required) {
			t.Fatalf("github governance script missing %q", required)
		}
	}
	for _, required := range []string{
		"--profile saas-api-full",
		"make integration-check",
		"INTEGRATION_OBJECT_STORE",
		".ci-result/generated-integration",
	} {
		if !strings.Contains(integrationScript, required) {
			t.Fatalf("generated integration script missing %q", required)
		}
	}
	for _, required := range []string{
		"GENERATED_UPGRADE_COMPAT_REFS",
		"GENERATOR_REF",
		"v3.0.0",
		"v3.1.2",
		"status.tsv",
		"go mod edit -replace=github.com/aatuh/api-toolkit/v3=",
		"go mod edit -replace=github.com/aatuh/api-toolkit/contrib/v3=",
		"make contracts-diff",
		".ci-result/generated-upgrade-compat",
	} {
		if !strings.Contains(upgradeCompatScript, required) {
			t.Fatalf("generated upgrade compatibility script missing %q", required)
		}
	}
	for _, required := range []string{
		"UPGRADE_SMOKE_BASE_REF",
		"v3.1.2",
		"go get \"github.com/aatuh/api-toolkit/v3@$base_ref\"",
		"go mod edit -replace=github.com/aatuh/api-toolkit/v3=",
		"go test ./...",
		".ci-result/upgrade-smoke",
		"status.tsv",
	} {
		if !strings.Contains(upgradeSmokeScript, required) {
			t.Fatalf("upgrade smoke script missing %q", required)
		}
	}
	for _, required := range []string{
		"Downstream upgrade smoke",
		"UPGRADE_SMOKE_BASE_REF: v3.1.2",
		"make upgrade-smoke-check",
	} {
		if !strings.Contains(ci, required) {
			t.Fatalf(".github/workflows/ci.yml missing upgrade smoke CI text %q", required)
		}
	}
	for _, required := range []string{
		"REFERENCE_SERVICE_DOCKER",
		"REFERENCE_SERVICE_MINIO",
		".ci-result/reference-service",
		"summary.json",
		"reference-service-check",
		"make -C \"$repo_root/examples/reference-saas-api\" integration-check",
	} {
		if !strings.Contains(referenceEvidenceScript, required) {
			t.Fatalf("reference service evidence script missing %q", required)
		}
	}
	for _, required := range []string{
		"${OUTPUT_DIR:-.ci-result}/coverage",
		"reference-service.coverprofile",
		"reference-service.func",
		"reference-service-summary.md",
		"app-owned evidence",
	} {
		if !strings.Contains(referenceCoverageScript, required) {
			t.Fatalf("reference service coverage script missing %q", required)
		}
	}
	for _, required := range []string{
		"make generated-integration-check",
		"make generated-integration-check-minio",
		"make generated-upgrade-compat-check",
		"make upgrade-smoke-check",
		"make reference-service-evidence",
		"make reference-service-coverage",
		"make github-governance-check",
		"make actions-audit",
		"make timeout-determinism-check",
		"make coverage-check",
		"docs/coverage-hardening-backlog.md",
		"not part of `finalize`",
	} {
		if !strings.Contains(runbook, required) && !strings.Contains(referenceService, required) && !strings.Contains(governance, required) {
			t.Fatalf("governance docs missing optional check guidance %q", required)
		}
	}
	for _, required := range []string{
		".ci-result/generated-integration/status",
		"make generated-integration-check",
		"not_run_opt_in",
	} {
		if !strings.Contains(summaryScript, required) {
			t.Fatalf("release summary script missing generated integration evidence %q", required)
		}
	}
	for _, required := range []string{
		"AUTH_JWT_COVERAGE_MIN",
		"HEALTH_COVERAGE_MIN",
		"CONTRIB_PGXPOOL_COVERAGE_MIN",
		"CONTRIB_OPENAPI_COVERAGE_MIN",
		"CONTRIB_WEBHOOKDELIVERY_COVERAGE_MIN",
		"raise only after behavior tests are merged",
		"reference-service-coverage",
	} {
		if !strings.Contains(coverageBacklog, required) {
			t.Fatalf("coverage hardening backlog missing %q", required)
		}
	}
}

func TestGeneratedUpgradeDefaultRefsStayDocumented(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	script := readText(t, filepath.Join(repoRoot, "scripts", "generated_upgrade_compat_check.sh"))
	defaultRefs := generatedUpgradeDefaultRefs(t, script)

	for _, source := range []struct {
		name string
		text string
		re   *regexp.Regexp
	}{
		{
			name: "docs/release-runbook.md",
			text: readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md")),
			re:   regexp.MustCompile("`GENERATED_UPGRADE_COMPAT_REFS` defaulting to `([^`]+)`"),
		},
		{
			name: "docs/reference-service.md",
			text: readText(t, filepath.Join(repoRoot, "docs", "reference-service.md")),
			re:   regexp.MustCompile("By default this checks `([^`]+)` and `([^`]+)`"),
		},
		{
			name: "docs/release-notes.md",
			text: readText(t, filepath.Join(repoRoot, "docs", "release-notes.md")),
			re:   regexp.MustCompile("defaults to checking both `([^`]+)` and\\s+`([^`]+)`"),
		},
	} {
		got := documentedGeneratedUpgradeRefs(t, source.name, source.text, source.re)
		if !reflect.DeepEqual(got, defaultRefs) {
			t.Fatalf("%s generated-upgrade default refs = %v, want %v from scripts/generated_upgrade_compat_check.sh", source.name, got, defaultRefs)
		}
	}
}

func TestContribModuleRemainsInstallableByVersion(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	goMod := readText(t, filepath.Join(repoRoot, "contrib", "go.mod"))
	if strings.Contains(goMod, "\nreplace ") {
		t.Fatalf("contrib/go.mod must not contain replace directives; published contrib/v3 CLI installs reject versioned modules with replace directives")
	}
	for _, required := range []string{
		"github.com/aatuh/api-toolkit/v3 v3.1.2",
		"module github.com/aatuh/api-toolkit/contrib/v3",
	} {
		if !strings.Contains(goMod, required) {
			t.Fatalf("contrib/go.mod missing %q", required)
		}
	}
}

func TestContribGovernanceDocsDescribeSupportedAdapterDriftConsistently(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	required := []string{
		"supported-adapter incompatible drift is gate-enforced",
		"does not make contrib stable",
	}
	for _, rel := range []string{
		"README.md",
		filepath.Join("docs", "release-runbook.md"),
		filepath.Join("docs", "release-review.md"),
		filepath.Join("docs", "release-notes.md"),
		filepath.Join("docs", "release-manifests.md"),
	} {
		text := readText(t, filepath.Join(repoRoot, rel))
		normalized := strings.ToLower(normalizeWhitespace(text))
		for _, phrase := range required {
			if !strings.Contains(normalized, strings.ToLower(phrase)) {
				t.Fatalf("%s missing contrib governance phrase %q", rel, phrase)
			}
		}
	}
}

func TestSupportedAdapterPackagesAreInContribDriftManifest(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classification := readText(t, filepath.Join(repoRoot, "docs", "package-classification.tsv"))
	manifest := readText(t, filepath.Join(repoRoot, "docs", "contrib-api-drift-packages.txt"))
	manifestSet := map[string]bool{}
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		manifestSet[line] = true
	}

	var missing []string
	for _, line := range strings.Split(classification, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[1] != "supported-adapter" {
			continue
		}
		if !manifestSet[fields[0]] {
			missing = append(missing, fields[0])
		}
	}
	if len(missing) > 0 {
		t.Fatalf("supported-adapter packages missing from docs/contrib-api-drift-packages.txt:\n%s", strings.Join(missing, "\n"))
	}
}

func TestSupportedAdapterContractsManifestCoversSupportedAdapters(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	contracts := loadSupportedAdapterContracts(t, repoRoot)

	var missing []string
	for _, cls := range classes {
		if cls.APIStatus != "supported-adapter" {
			continue
		}
		if _, ok := contracts[cls.ImportPath]; !ok {
			missing = append(missing, cls.ImportPath)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("supported-adapter packages missing from docs/supported-adapter-contracts.tsv:\n%s", strings.Join(missing, "\n"))
	}

	var stale []string
	for importPath := range contracts {
		if classes[importPath].APIStatus != "supported-adapter" {
			stale = append(stale, importPath)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Fatalf("docs/supported-adapter-contracts.tsv lists non-supported adapters:\n%s", strings.Join(stale, "\n"))
	}

	for _, importPath := range []string{
		"github.com/aatuh/api-toolkit/contrib/v3/adapters/chi",
		"github.com/aatuh/api-toolkit/contrib/v3/adapters/idempotencyredis",
		"github.com/aatuh/api-toolkit/contrib/v3/adapters/logzap",
		"github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool",
		"github.com/aatuh/api-toolkit/contrib/v3/adapters/ratelimitredis",
		"github.com/aatuh/api-toolkit/contrib/v3/adapters/resend",
		"github.com/aatuh/api-toolkit/contrib/v3/adapters/stripe",
		"github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/clerk",
		"github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi",
		"github.com/aatuh/api-toolkit/contrib/v3/middleware/oteltrace",
		"github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog",
		"github.com/aatuh/api-toolkit/contrib/v3/telemetry",
	} {
		contract, ok := contracts[importPath]
		if !ok {
			t.Fatalf("required supported adapter contract missing for %s", importPath)
		}
		for _, required := range []string{"direct tests", "release drift"} {
			if !strings.Contains(strings.ToLower(contract.Evidence), required) {
				t.Fatalf("supported adapter contract for %s evidence missing %q: %q", importPath, required, contract.Evidence)
			}
		}
	}
}

func TestSupportedAdapterRealismManifestCoversSupportedAdapters(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	realism := loadSupportedAdapterRealism(t, repoRoot)

	var missing []string
	for _, cls := range classes {
		if cls.APIStatus != "supported-adapter" {
			continue
		}
		if _, ok := realism[cls.ImportPath]; !ok {
			missing = append(missing, cls.ImportPath)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("supported-adapter packages missing from docs/supported-adapter-test-realism.tsv:\n%s", strings.Join(missing, "\n"))
	}

	var stale []string
	for importPath, row := range realism {
		if classes[importPath].APIStatus != "supported-adapter" {
			stale = append(stale, importPath)
		}
		if !strings.Contains(strings.ToLower(row.DefaultPREvidence), "tests") {
			stale = append(stale, importPath+" default PR evidence must mention tests")
		}
		if !containsAny(row.ScheduledManualEvidence, []string{
			"not_applicable",
			"generated-integration-check",
			"reference-service-evidence",
			"provider-live-check",
			"manual",
		}) {
			stale = append(stale, importPath+" scheduled/manual evidence must identify the external-evidence path")
		}
		for _, token := range strings.Split(row.RealismStatus, "+") {
			if !map[string]bool{
				"direct-unit":               true,
				"fake-db":                   true,
				"hermetic-fixture":          true,
				"hermetic-provider-fixture": true,
				"manual-real-service":       true,
				"miniredis":                 true,
				"scheduled-real-service":    true,
			}[token] {
				stale = append(stale, importPath+" has unknown realism status token "+token)
			}
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Fatalf("docs/supported-adapter-test-realism.tsv has invalid rows:\n%s", strings.Join(stale, "\n"))
	}

	for _, source := range []struct {
		name string
		text string
	}{
		{"docs/README.md", readText(t, filepath.Join(repoRoot, "docs", "README.md"))},
		{"docs/production-readiness.md", readText(t, filepath.Join(repoRoot, "docs", "production-readiness.md"))},
		{"docs/release-manifests.md", readText(t, filepath.Join(repoRoot, "docs", "release-manifests.md"))},
		{"docs/release-runbook.md", readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))},
	} {
		if !strings.Contains(source.text, "docs/supported-adapter-test-realism.tsv") {
			t.Fatalf("%s missing docs/supported-adapter-test-realism.tsv", source.name)
		}
	}
}

func TestSupportedAdaptersHaveCompleteEvidence(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	contracts := loadSupportedAdapterContracts(t, repoRoot)
	driftPackages := make(map[string]bool)
	for _, importPath := range contribDriftManifestPackages(t, repoRoot) {
		driftPackages[importPath] = true
	}
	listedPackages := make(map[string]listedPackage)
	for _, pkg := range listedGoPackages(t, filepath.Join(repoRoot, "contrib")) {
		listedPackages[pkg.ImportPath] = pkg
	}

	var missing []string
	for _, cls := range classes {
		if cls.APIStatus != "supported-adapter" {
			continue
		}
		if !inModule(cls.ImportPath, contribModulePath) {
			missing = append(missing, cls.ImportPath+" is not in the contrib module")
			continue
		}
		if cls.TestStatus != "direct-tests" {
			missing = append(missing, cls.ImportPath+" has test_status "+cls.TestStatus+", want direct-tests")
		}
		contract, ok := contracts[cls.ImportPath]
		if !ok {
			missing = append(missing, cls.ImportPath+" is missing from docs/supported-adapter-contracts.tsv")
		}
		if !driftPackages[cls.ImportPath] {
			missing = append(missing, cls.ImportPath+" is missing from docs/contrib-api-drift-packages.txt")
		}
		pkg, ok := listedPackages[cls.ImportPath]
		if !ok {
			missing = append(missing, cls.ImportPath+" is missing from go list ./...")
			continue
		}
		if pkg.DirectTestFiles == 0 {
			missing = append(missing, cls.ImportPath+" has no direct test files")
		}
		docPath := filepath.Join(contribPackageDir(repoRoot, cls.ImportPath), "doc.go")
		if _, err := os.Stat(docPath); err != nil {
			missing = append(missing, cls.ImportPath+" is missing package docs at "+docPath)
		} else if ok {
			docEvidence := "package docs in " + slashRel(repoRoot, docPath)
			if !strings.Contains(contract.Evidence, docEvidence) {
				missing = append(missing, cls.ImportPath+" contract evidence must cite "+docEvidence)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("supported-adapter evidence is incomplete:\n%s", strings.Join(missing, "\n"))
	}
}

func TestStableAndSupportedAdapterPackageDocsMeetMinimumDepth(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	classes := loadPackageClassifications(t, repoRoot)
	var weak []string

	for _, cls := range classes {
		needsDepth := cls.APIStatus == "stable" || cls.APIStatus == "compatibility-only" || cls.APIStatus == "supported-adapter"
		needsCoreFields := cls.APIStatus == "stable" || cls.APIStatus == "compatibility-only"
		if !needsDepth && !needsCoreFields {
			continue
		}
		dir := classifiedPackageDir(repoRoot, cls.ImportPath)
		if dir == "" {
			weak = append(weak, cls.ImportPath+" has unsupported import path for package-doc check")
			continue
		}
		docPath := filepath.Join(dir, "doc.go")
		comment := packageDocCommentText(t, docPath)
		if needsDepth && len(strings.Fields(comment)) < 25 {
			weak = append(weak, slashRel(repoRoot, docPath)+" has package docs below the minimum depth for "+cls.APIStatus)
		}
		if !needsCoreFields {
			continue
		}
		normalized := strings.ToLower(normalizeWhitespace(comment))
		for field, accepted := range map[string][]string{
			"purpose":      {"purpose:"},
			"install":      {"import:", "install:"},
			"example":      {"example:"},
			"errors":       {"errors:"},
			"concurrency":  {"concurrency:"},
			"stability":    {"stability:"},
			"when-not-use": {"when not to use:"},
		} {
			if !containsAny(normalized, accepted) {
				weak = append(weak, slashRel(repoRoot, docPath)+" missing "+field+" package-doc field for "+cls.APIStatus)
			}
		}
	}
	sort.Strings(weak)
	if len(weak) > 0 {
		t.Fatalf("package docs do not meet the package-doc standard:\n%s", strings.Join(weak, "\n"))
	}
}

func TestStableOptionsStructAuditCoversExportedOptions(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))
	audit := readText(t, filepath.Join(repoRoot, "docs", "options-structs.md"))

	for name, text := range map[string]string{
		"README.md/docs/README.md": readme + "\n" + docsIndex,
		"VERSIONING.md":            versioning,
	} {
		if !strings.Contains(text, "docs/options-structs.md") && !strings.Contains(text, "options-structs.md") {
			t.Fatalf("%s missing docs/options-structs.md", name)
		}
	}
	for _, required := range []string{
		"Defaults",
		"Validation Behavior",
		"Zero-Value Behavior",
		"Example Evidence",
		"docs/package-classification.tsv",
	} {
		if !strings.Contains(audit, required) {
			t.Fatalf("docs/options-structs.md missing %q", required)
		}
	}

	rows := optionsAuditRows(t, audit)
	want := stableOptionStructs(t, repoRoot)
	wantSet := map[string]bool{}
	var missing []string
	for _, ref := range want {
		key := optionStructKey(ref.ImportPath, ref.TypeName)
		wantSet[key] = true
		row, ok := rows[key]
		if !ok {
			missing = append(missing, ref.ImportPath+" "+ref.TypeName)
			continue
		}
		for _, field := range []string{"defaults", "validation", "zero", "example"} {
			value := row[field]
			if strings.TrimSpace(value) == "" {
				missing = append(missing, ref.ImportPath+" "+ref.TypeName+" has empty "+field+" audit cell")
			}
			if strings.Contains(strings.ToLower(value), "tbd") || strings.Contains(strings.ToLower(value), "todo") {
				missing = append(missing, ref.ImportPath+" "+ref.TypeName+" has placeholder "+field+" audit cell")
			}
		}
		if example := row["example"]; !strings.Contains(example, ".go") && !strings.Contains(example, ".md") {
			missing = append(missing, ref.ImportPath+" "+ref.TypeName+" example evidence must cite a Go file or markdown guide")
		}
	}
	for key := range rows {
		if !wantSet[key] {
			missing = append(missing, "docs/options-structs.md has stale or non-stable row "+key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("stable options-struct audit is incomplete:\n%s", strings.Join(missing, "\n"))
	}
}

func TestStableGlobalStateAuditCoversPackageVars(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	audit := readText(t, filepath.Join(repoRoot, "docs", "global-state-audit.md"))

	if !strings.Contains(docsIndex, "global-state-audit.md") {
		t.Fatal("docs/README.md missing global-state-audit.md")
	}
	for _, required := range []string{
		"package-level `var` declarations",
		"stable` or `compatibility-only`",
		"Exported preset vars are retained",
		"Mutation Policy",
		"Concurrency Evidence",
	} {
		if !strings.Contains(audit, required) {
			t.Fatalf("docs/global-state-audit.md missing %q", required)
		}
	}

	rows := globalStateAuditRows(t, audit)
	want := stablePackageVarRefs(t, repoRoot)
	wantSet := map[string]bool{}
	var missing []string
	for _, ref := range want {
		key := packageVarKey(ref.ImportPath, ref.Name)
		wantSet[key] = true
		row, ok := rows[key]
		if !ok {
			missing = append(missing, ref.ImportPath+" "+ref.Name+" missing global-state audit row")
			continue
		}
		for _, field := range []string{"classification", "mutation", "concurrency"} {
			value := row[field]
			if strings.TrimSpace(value) == "" {
				missing = append(missing, ref.ImportPath+" "+ref.Name+" has empty "+field+" audit cell")
			}
			lower := strings.ToLower(value)
			if strings.Contains(lower, "tbd") || strings.Contains(lower, "todo") {
				missing = append(missing, ref.ImportPath+" "+ref.Name+" has placeholder "+field+" audit cell")
			}
		}
		if !validGlobalStateClassification(row["classification"]) {
			missing = append(missing, ref.ImportPath+" "+ref.Name+" has invalid global-state classification "+row["classification"])
		}
	}
	for key := range rows {
		if !wantSet[key] {
			missing = append(missing, "docs/global-state-audit.md has stale or non-stable row "+key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("stable global-state audit is incomplete:\n%s", strings.Join(missing, "\n"))
	}
}

func TestReleaseReviewerSummaryAndArtifactVerifierContracts(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))
	runbook := readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))
	review := readText(t, filepath.Join(repoRoot, "docs", "release-review.md"))
	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))
	workflow := readText(t, filepath.Join(repoRoot, ".github", "workflows", "release.yml"))
	verifier := readText(t, filepath.Join(repoRoot, "scripts", "release_artifact_verify.sh"))
	reviewerSummary := readText(t, filepath.Join(repoRoot, "scripts", "release_review_summary.sh"))
	artifactContract := readText(t, filepath.Join(repoRoot, "scripts", "release_artifact_verify_contract_test.sh"))
	parserContract := readText(t, filepath.Join(repoRoot, "scripts", "release_evidence_parser_contract_test.sh"))

	for _, required := range []string{
		"release-review-summary",
		"scripts/release_review_summary.sh",
		"release-artifact-verify-fixture",
		"scripts/release_artifact_verify_fixture.sh",
		"release-artifact-verify-contract",
		"release-evidence-parser-contract",
	} {
		if !strings.Contains(makefile, required) {
			t.Fatalf("Makefile missing release reviewer/verifier contract target %q", required)
		}
	}
	for _, source := range []struct {
		name string
		text string
	}{
		{"docs/release-runbook.md", runbook},
		{"docs/release-review.md", review},
		{"VERSIONING.md", versioning},
	} {
		for _, required := range []string{
			"make release-review-summary",
			"make release-artifact-verify-fixture",
			"RELEASE_ARTIFACT_VERIFY_MODE=publication",
			"RELEASE_TAG",
			"GITHUB_REPOSITORY",
		} {
			if !strings.Contains(source.text, required) {
				t.Fatalf("%s missing consolidated reviewer path text %q", source.name, required)
			}
		}
	}
	for _, required := range []string{
		"Download and verify uploaded draft release assets",
		"gh release download",
		"RELEASE_ARTIFACT_VERIFY_MODE=publication",
		"RELEASE_TAG=\"$GITHUB_REF_NAME\"",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf(".github/workflows/release.yml missing post-upload verification text %q", required)
		}
	}
	for _, required := range []string{
		"RELEASE_TAG is required when RELEASE_ARTIFACT_VERIFY_MODE=publication",
		"json.load",
		"publication_eligible",
		"provenance_policy",
		"called_vulnerability_count",
		"missing_disposition_count",
		"expired_disposition_count",
		"checks[].log_path",
		"contrib_drift.artifact_path",
		"gh attestation verify",
	} {
		if !strings.Contains(verifier, required) {
			t.Fatalf("release artifact verifier missing invariant %q", required)
		}
	}
	for _, required := range []string{
		"publication_eligible",
		"vulnerability_dispositions",
		"contrib_drift",
		"artifact_expectations",
		"review_decision",
	} {
		if !strings.Contains(reviewerSummary, required) {
			t.Fatalf("release reviewer summary script missing field %q", required)
		}
	}
	for _, required := range []string{
		"failed-summary",
		"missing-summary-log",
		"publication-missing-tag",
		"publication verifier should run three gh attestation checks",
	} {
		if !strings.Contains(artifactContract, required) {
			t.Fatalf("release artifact verifier contract missing fixture %q", required)
		}
	}
	for _, required := range []string{
		"vuln-called.log",
		"vuln-imported.log",
		"vuln-none.log",
		"vuln-unexpected.log",
		"contrib-compatible.log",
		"contrib-incompatible.log",
		"contrib-mixed.log",
		"contrib-none.log",
		"contrib-skipped.log",
		"contrib-malformed.log",
		"govulncheck-imported-id-parser",
		"status\": \"unknown\"",
	} {
		if !strings.Contains(parserContract, required) {
			t.Fatalf("release evidence parser contract missing fixture %q", required)
		}
	}
}

func TestResponseWriterInventoryMatchesCurrentImports(t *testing.T) {
	skipV2CompatibilitySurfaceChecksOnV3(t)
	repoRoot := mustRepoRoot(t)
	inventory := readText(t, filepath.Join(repoRoot, "docs", "response-writer-inventory.md"))
	fset := token.NewFileSet()
	var importers []string

	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".ci-result", ".audits", "audit":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
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
			if importPath == rootModulePath+"/v3/response_writer" {
				rel, relErr := filepath.Rel(repoRoot, path)
				if relErr != nil {
					rel = path
				}
				importers = append(importers, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan response_writer imports: %v", err)
	}
	sort.Strings(importers)
	if len(importers) == 0 {
		if !strings.Contains(inventory, "No current root or contrib runtime package imports") {
			t.Fatal("docs/response-writer-inventory.md must record that current runtime imports are cleared")
		}
	} else {
		for _, importer := range importers {
			if !strings.Contains(inventory, "`"+importer+"`") {
				t.Fatalf("docs/response-writer-inventory.md missing current importer %s", importer)
			}
		}
	}
	for _, required := range []string{
		"compatibility-only",
		"httpx",
		"package-local",
		"v3",
	} {
		if !strings.Contains(inventory, required) {
			t.Fatalf("docs/response-writer-inventory.md missing replacement guidance %q", required)
		}
	}
}

func TestPublicExamplesDoNotTeachLegacyCompatibilitySurfaces(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	forbidden := []string{
		`"github.com/aatuh/api-toolkit/v3/response_writer"`,
		"ports.CheckoutSessionRequest",
		"ports.PaymentProvider",
		"ports.BillingProvider",
		"ports.DatabaseStats",
		"DatabasePool.Stat()",
		"pprof.RegisterRoutes(",
	}

	for _, mdPath := range append([]string{filepath.Join(repoRoot, "README.md")}, markdownFilesUnder(t, filepath.Join(repoRoot, "docs"))...) {
		for _, block := range markdownCodeBlocks(readText(t, mdPath)) {
			for _, token := range forbidden {
				if strings.Contains(block, token) {
					rel, err := filepath.Rel(repoRoot, mdPath)
					if err != nil {
						rel = mdPath
					}
					t.Fatalf("%s code block teaches legacy compatibility surface %q", rel, token)
				}
			}
		}
	}

	for _, dir := range []string{
		filepath.Join(repoRoot, "examples"),
		filepath.Join(repoRoot, "contrib", "examples"),
	} {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			text := readText(t, path)
			for _, token := range forbidden {
				if strings.Contains(text, token) {
					rel, relErr := filepath.Rel(repoRoot, path)
					if relErr != nil {
						rel = path
					}
					t.Fatalf("%s teaches legacy compatibility surface %q", rel, token)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan examples in %s: %v", dir, err)
		}
	}
}

func TestV3RemovalMatrixHasExecutableEvidence(t *testing.T) {
	skipV2CompatibilitySurfaceChecksOnV3(t)
	repoRoot := mustRepoRoot(t)
	roadmap := readText(t, filepath.Join(repoRoot, "docs", "v3-compatibility-roadmap.md"))
	matrix := markdownSection(t, roadmap, "## V3 removal matrix")
	rows := markdownTableRows(matrix)
	if len(rows) < 6 {
		t.Fatalf("v3 removal matrix has %d rows, want at least 6", len(rows))
	}
	for _, row := range rows {
		if len(row) != 6 {
			t.Fatalf("v3 removal matrix row has %d columns, want 6: %v", len(row), row)
		}
		for i, field := range row {
			if strings.TrimSpace(field) == "" || strings.Contains(strings.ToLower(field), "tbd") {
				t.Fatalf("v3 removal matrix row %q has incomplete field %d", row[0], i)
			}
		}
	}
	evidence := markdownSection(t, roadmap, "## Executable v3 evidence requirements")
	for _, required := range []string{
		"adapter_contract_status=passed",
		"legacy_in_flight_fallback_entered",
		"legacy_in_flight_fallback_recovered",
		"legacy_in_flight_fallback_rejected",
		"legacy_in_flight_fallback_unknown",
		"support-window signal",
		"docs/response-writer-inventory.md",
		"docscheck legacy-code-snippet guardrails",
	} {
		if !strings.Contains(evidence, required) {
			t.Fatalf("executable v3 evidence requirements missing %q", required)
		}
	}
}

func markdownFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk markdown files under %s: %v", root, err)
	}
	sort.Strings(paths)
	return paths
}

func firstLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}
	return strings.Join(lines[:maxLines], "\n")
}

func markdownCodeBlocks(markdown string) []string {
	var blocks []string
	var current []string
	inBlock := false
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "```") {
			if inBlock {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
				inBlock = false
			} else {
				inBlock = true
			}
			continue
		}
		if inBlock {
			current = append(current, line)
		}
	}
	return blocks
}

func markdownTableRows(section string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		if strings.Contains(line, "---") || strings.Contains(line, "Surface |") {
			continue
		}
		parts := strings.Split(strings.Trim(line, "|"), "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		rows = append(rows, parts)
	}
	return rows
}

func normalizeWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func TestV3DebtChecklistRowsStayExecutable(t *testing.T) {
	skipV2CompatibilitySurfaceChecksOnV3(t)
	repoRoot := mustRepoRoot(t)
	roadmap := readText(t, filepath.Join(repoRoot, "docs", "v3-compatibility-roadmap.md"))
	matrix := markdownSection(t, roadmap, "## V3 removal matrix")
	checklist := markdownSection(t, roadmap, "## V3 owner checklist")

	for _, surface := range []string{
		"Provider-shaped billing ports",
		"Driver-shaped database stats",
		"Legacy response helpers",
		"Tokenless idempotency release",
		"Unchecked authz constructor",
		"Checked list parser shims",
	} {
		matrixRow := markdownTableRowContaining(t, matrix, surface)
		matrixColumns := markdownTableColumns(matrixRow)
		if len(matrixColumns) < 6 {
			t.Fatalf("v3 removal matrix row for %s has too few columns: %q", surface, matrixRow)
		}
		if strings.TrimSpace(matrixColumns[4]) == "" || strings.TrimSpace(matrixColumns[5]) == "" {
			t.Fatalf("v3 removal matrix row for %s must include required tests and removal condition: %q", surface, matrixRow)
		}

		checklistRow := markdownTableRowContaining(t, checklist, surface)
		checklistColumns := markdownTableColumns(checklistRow)
		if len(checklistColumns) < 4 {
			t.Fatalf("v3 owner checklist row for %s has too few columns: %q", surface, checklistRow)
		}
		if strings.TrimSpace(checklistColumns[2]) == "" || strings.TrimSpace(checklistColumns[3]) == "" {
			t.Fatalf("v3 owner checklist row for %s must include required tests and release-note requirements: %q", surface, checklistRow)
		}
		releaseNoteColumn := strings.ToLower(checklistColumns[3])
		if !strings.Contains(releaseNoteColumn, "release note") &&
			!strings.Contains(releaseNoteColumn, "release-note") &&
			!strings.Contains(releaseNoteColumn, "release-notes") {
			t.Fatalf("v3 owner checklist row for %s must require release notes: %q", surface, checklistRow)
		}
	}
}

func TestCompatibilityRoadmapCoversDocumentedSensitiveSurfaces(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	roadmap := readText(t, filepath.Join(repoRoot, "docs", "v3-compatibility-roadmap.md"))
	portsSurface := readText(t, filepath.Join(repoRoot, "docs", "ports-surface.md"))
	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))

	for _, surface := range []string{
		"ports/billing.go",
		"DatabasePool.Stat",
		"DatabaseStats",
		"response_writer",
		"IdempotencyReleaser.Release(ctx, key)",
		"IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)",
		"NewRequireRoleMiddleware",
		"NewRequireRoleMiddlewareChecked",
		"ParseListQuery",
		"ParseListQueryChecked",
	} {
		if !strings.Contains(roadmap, surface) {
			t.Fatalf("v3 roadmap missing compatibility-sensitive surface %q", surface)
		}
	}
	for _, source := range []struct {
		name string
		text string
	}{
		{"docs/ports-surface.md", portsSurface},
		{"VERSIONING.md", versioning},
	} {
		for _, surface := range []string{"ports/billing.go", "DatabasePool.Stat", "DatabaseStats", "response_writer"} {
			if strings.Contains(source.text, surface) && !strings.Contains(roadmap, surface) {
				t.Fatalf("v3 roadmap missing %s surface %q", source.name, surface)
			}
		}
	}
}

func TestCompatibilitySensitivePortsGovernanceDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))
	portsSurface := readText(t, filepath.Join(repoRoot, "docs", "ports-surface.md"))
	releaseNotes := readText(t, filepath.Join(repoRoot, "docs", "release-notes.md"))

	for _, required := range []string{
		"`docs/ports-surface.md`",
		"`docs/v3-compatibility-roadmap.md`",
		"release notes",
		"upgrade notes",
	} {
		if !strings.Contains(releaseNotes, required) {
			t.Fatalf("release notes checklist missing compatibility governance text %q", required)
		}
	}
	for _, required := range []string{
		"compatibility-sensitive",
		"docs/v3-compatibility-roadmap.md",
		"docs/release-notes.md",
	} {
		if !strings.Contains(portsSurface, required) {
			t.Fatalf("ports surface docs missing governance text %q", required)
		}
	}
	if !strings.Contains(versioning, "docs/package-classification.tsv") || !strings.Contains(versioning, "Compatibility-sensitive") {
		t.Fatal("VERSIONING.md must keep package classification and compatibility-sensitive policy together")
	}
}

func TestCompatibilitySensitivePackageDocsPointToReplacements(t *testing.T) {
	skipV2CompatibilitySurfaceChecksOnV3(t)
	repoRoot := mustRepoRoot(t)
	requirements := map[string][]string{
		filepath.Join(repoRoot, "ports", "doc.go"): {
			"Compatibility-sensitive exceptions",
			"github.com/aatuh/api-toolkit/v3/compat/billing",
			"DatabasePoolSnapshotProvider",
			"SnapshotDatabasePoolStats",
			"httpx",
			"docs/v3-compatibility-roadmap.md",
		},
		filepath.Join(repoRoot, "compat", "billing", "doc.go"): {
			"compatibility-sensitive",
			"provider-neutral",
			"app-owned port",
		},
		filepath.Join(repoRoot, "response_writer", "doc.go"): {
			"source compatibility",
			"github.com/aatuh/api-toolkit/v3/httpx",
			"compatibility-sensitive",
			"not a template",
		},
	}

	for path, required := range requirements {
		content := readText(t, path)
		for _, text := range required {
			if !strings.Contains(content, text) {
				rel, _ := filepath.Rel(repoRoot, path)
				t.Fatalf("%s missing package-doc compatibility guidance %q", rel, text)
			}
		}
	}
}

func TestCurrentV3PackageDocsAvoidStaleV2CompatibilityClaims(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	forbidden := []string{
		"v2 source compatibility",
		"v2 source-compatible",
		"v2-compatible",
		"v2 convenience",
		"explicit v2 compat",
		"for the rest of v2",
		"provider-shaped billing contracts in billing.go are deprecated",
		"aliases to the existing ports exports",
		"legacy response writer package is similarly retained",
	}

	var violations []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".ci-result", ".audits", ".trash", "audit", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) != "doc.go" {
			return nil
		}
		content := readText(t, path)
		for _, text := range forbidden {
			if strings.Contains(content, text) {
				violations = append(violations, slashRel(repoRoot, path)+" contains "+strconv.Quote(text))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan package docs: %v", err)
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("current v3 package docs contain stale v2-only claims:\n%s", strings.Join(violations, "\n"))
	}

	portsDoc := readText(t, filepath.Join(repoRoot, "ports", "doc.go"))
	for _, required := range []string{
		"stable core boundary contracts",
		"github.com/aatuh/api-toolkit/v3/compat/billing",
		"Response",
		"httpx",
		"DatabasePoolSnapshotProvider",
		"SnapshotDatabasePoolStats",
		"docs/ports-surface.md",
		"docs/v3-compatibility-roadmap.md",
	} {
		if !strings.Contains(portsDoc, required) {
			t.Fatalf("ports/doc.go missing current v3 guidance %q", required)
		}
	}

	billingDoc := readText(t, filepath.Join(repoRoot, "compat", "billing", "doc.go"))
	for _, required := range []string{
		"v3 compatibility model",
		"hosted checkout",
		"billing portal",
		"provider-shaped billing",
		"generic ports package",
		"app-owned port",
	} {
		if !strings.Contains(billingDoc, required) {
			t.Fatalf("compat/billing/doc.go missing current v3 guidance %q", required)
		}
	}
}

func TestStablePortsAvoidNewProviderSpecificNaming(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	blocked := regexp.MustCompile(`(?i)(stripe|pgx|postgres|redis|dynamo|s3|aws|gcp|azure|checkout|invoice|paymentmethod|priceid|customerid)`)
	allowedFiles := map[string]bool{
		filepath.Join(repoRoot, "ports", "billing.go"):  true,
		filepath.Join(repoRoot, "ports", "database.go"): true,
	}

	paths, err := filepath.Glob(filepath.Join(repoRoot, "ports", "*.go"))
	if err != nil {
		t.Fatalf("glob ports files: %v", err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") || allowedFiles[path] {
			continue
		}
		for _, ident := range exportedIdentifiersInFile(t, path) {
			if blocked.MatchString(ident) {
				rel, _ := filepath.Rel(repoRoot, path)
				t.Fatalf("%s exports provider- or driver-shaped name %q outside documented compatibility files", rel, ident)
			}
		}
	}
}

func TestExamplesAndGuidesPreferCompatibilityReplacements(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	forbidden := []string{
		"ports.CheckoutSessionRequest",
		"ports.PaymentProvider",
		"ports.BillingProvider",
		"ports.DatabasePool.Stat",
		"ports.DatabaseStats",
		"DatabasePool.Stat",
		"DatabaseStats",
		"NewRequireRoleMiddleware(",
		"DefaultFilterParser(",
		"DefaultSortParser(",
		"response_writer",
		"ParseListQuery(",
		"pprof.RegisterRoutes(",
	}
	allowedMarkdown := map[string]bool{
		filepath.Join(repoRoot, "VERSIONING.md"):                        true,
		filepath.Join(repoRoot, "docs", "architecture.md"):              true,
		filepath.Join(repoRoot, "docs", "ports-surface.md"):             true,
		filepath.Join(repoRoot, "docs", "response-writer-inventory.md"): true,
		filepath.Join(repoRoot, "docs", "security.md"):                  true,
		filepath.Join(repoRoot, "docs", "release-review.md"):            true,
		filepath.Join(repoRoot, "docs", "release-notes.md"):             true,
		filepath.Join(repoRoot, "docs", "v3-compatibility-roadmap.md"):  true,
	}

	var files []string
	docPaths, err := filepath.Glob(filepath.Join(repoRoot, "docs", "*.md"))
	if err != nil {
		t.Fatalf("glob docs markdown: %v", err)
	}
	for _, path := range append([]string{filepath.Join(repoRoot, "README.md"), filepath.Join(repoRoot, "VERSIONING.md")}, docPaths...) {
		if !allowedMarkdown[path] {
			files = append(files, path)
		}
	}
	err = filepath.WalkDir(filepath.Join(repoRoot, "contrib", "examples"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan contrib examples: %v", err)
	}

	for _, path := range files {
		content := readText(t, path)
		for _, pattern := range forbidden {
			if strings.Contains(content, pattern) {
				rel, _ := filepath.Rel(repoRoot, path)
				t.Fatalf("%s teaches deprecated compatibility API %q outside allowed compatibility docs", rel, pattern)
			}
		}
	}
}

func TestReleaseDocsDocumentExplicitAPICheckBaseRef(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))
	runbook := readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))
	apiCheck := readText(t, filepath.Join(repoRoot, "scripts", "apicheck.sh"))

	if !strings.Contains(apiCheck, "API_BASE_REF") {
		t.Fatal("scripts/apicheck.sh no longer documents or honors API_BASE_REF")
	}
	for _, required := range []string{
		"Current supported v3 API baseline: see `docs/release-runbook.md`.",
		"Release readiness and publication evidence require an explicit `API_BASE_REF`",
		"API_BASE_REF=v2.1.0",
		"`make finalize` is not release evidence",
		"`make release-api-check`",
		"docs/release-runbook.md",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("README missing release command intent text %q", required)
		}
	}
	for _, required := range []string{
		"`make api-check` is a local compatibility helper",
		"`make release-api-check` fails closed unless `API_BASE_REF` names an available supported baseline",
		"`make release-check` is the release-readiness gate",
		"`make release-evidence` runs the release-readiness subchecks through the evidence",
		"`make contrib-api-drift-report` enforces supported-adapter incompatible drift",
		"`make contrib-release-notes-check` is a lightweight",
	} {
		if !strings.Contains(versioning, required) {
			t.Fatalf("VERSIONING.md missing release command intent text %q", required)
		}
	}
	for _, required := range []string{
		"Supported v3 release baseline: `v3.1.2`",
		"API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make release-check",
		"API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make release-evidence",
		"API_BASE_REF=v2.1.0",
		"schema v2",
		"local release evidence",
		"GitHub release workflow evidence",
		"Do not publish the release",
	} {
		if !strings.Contains(runbook, required) {
			t.Fatalf("docs/release-runbook.md missing release runbook text %q", required)
		}
	}
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

func TestQualityAuditP0AdoptionAndProcessDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	firstSixtyLines := firstLines(readme, 60)
	for _, required := range []string{
		"small, composable Go HTTP API building blocks",
		"Target user:",
		"Non-goals:",
		"go get github.com/aatuh/api-toolkit/v3",
		"func main()",
		"Stable core package list: `VERSIONING.md` is the source of truth",
	} {
		if !strings.Contains(firstSixtyLines, required) {
			t.Fatalf("README first 60 lines missing adoption-first text %q", required)
		}
	}
	for _, required := range []string{
		"docs/alternatives.md",
		"docs/stable-core.md",
		"CONTRIBUTING.md",
		".github/pull_request_template.md",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("README missing adoption/process link %q", required)
		}
	}

	alternatives := readText(t, filepath.Join(repoRoot, "docs", "alternatives.md"))
	for _, required := range []string{
		"Use `net/http` or chi",
		"Use oapi-codegen",
		"Use Goa",
		"Use Connect",
	} {
		if !strings.Contains(alternatives, required) {
			t.Fatalf("docs/alternatives.md missing %q", required)
		}
	}

	stableCore := readText(t, filepath.Join(repoRoot, "docs", "stable-core.md"))
	for _, required := range []string{
		"small Go HTTP API building blocks",
		"Stable Core Packages",
		"Compatibility-Only Packages",
		"docs",
		"direct tests",
		"examples",
		"benchmark decision",
		"compatibility notes",
	} {
		if !strings.Contains(stableCore, required) {
			t.Fatalf("docs/stable-core.md missing %q", required)
		}
	}

	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))
	portsSurface := readText(t, filepath.Join(repoRoot, "docs", "ports-surface.md"))
	for _, source := range []struct {
		name string
		text string
	}{
		{"VERSIONING.md", versioning},
		{"docs/ports-surface.md", portsSurface},
	} {
		normalized := normalizeWhitespace(source.text)
		for _, required := range []string{
			"No new `ports` export",
			"at least two real implementations",
			"the application should not own the interface",
		} {
			if !strings.Contains(normalized, required) {
				t.Fatalf("%s missing ports freeze requirement %q", source.name, required)
			}
		}
	}

	contributing := readText(t, filepath.Join(repoRoot, "CONTRIBUTING.md"))
	for _, required := range []string{
		"Local setup",
		"make docs-check",
		"make fast-check",
		"make finalize",
		"API compatibility",
		"Documentation policy",
		"Security reports",
		"Pull request review",
	} {
		if !strings.Contains(contributing, required) {
			t.Fatalf("CONTRIBUTING.md missing %q", required)
		}
	}

	for _, rel := range []string{
		".github/ISSUE_TEMPLATE/bug_report.md",
		".github/ISSUE_TEMPLATE/feature_request.md",
		".github/ISSUE_TEMPLATE/docs.md",
		".github/ISSUE_TEMPLATE/api_change.md",
		".github/ISSUE_TEMPLATE/security.md",
		".github/pull_request_template.md",
	} {
		if _, err := os.Stat(filepath.Join(repoRoot, rel)); err != nil {
			t.Fatalf("missing GitHub template %s: %v", rel, err)
		}
	}

	prTemplate := readText(t, filepath.Join(repoRoot, ".github", "pull_request_template.md"))
	for _, required := range []string{
		"Tests",
		"Documentation",
		"Compatibility impact",
		"Security impact",
		"Benchmark impact",
	} {
		if !strings.Contains(prTemplate, required) {
			t.Fatalf("pull request template missing %q", required)
		}
	}
}

func TestAlternativesComparisonExamplesAndReleaseNoteCategories(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	alternatives := readText(t, filepath.Join(repoRoot, "docs", "alternatives.md"))
	comparison := markdownSection(t, alternatives, "## Comparison Examples")
	comparisonText := normalizeWhitespace(comparison)

	for _, required := range []string{
		"### Plain chi version",
		"### api-toolkit library version",
		"### Generated scaffold version",
		"http.MaxBytesReader",
		"binding.DecodeJSON",
		"binding.WriteValidationProblem",
		"httpx.WriteJSON",
		"github.com/aatuh/api-toolkit/contrib/v3/cmd/api-toolkit@latest new service",
		"Existing services should start with the library version",
	} {
		if !strings.Contains(comparisonText, required) {
			t.Fatalf("docs/alternatives.md comparison examples missing %q", required)
		}
	}

	releaseNotes := readText(t, filepath.Join(repoRoot, "docs", "release-notes.md"))
	categorySection := markdownSection(t, releaseNotes, "## Release Note Categories")
	for _, required := range []string{
		"Breaking",
		"Behavior",
		"Security",
		"Docs",
		"Dependencies",
		"Generated scaffold",
		"Migration",
	} {
		if !strings.Contains(categorySection, required) {
			t.Fatalf("docs/release-notes.md release category taxonomy missing %q", required)
		}
	}

	checklist := markdownSection(t, releaseNotes, "## Release checklist")
	for _, required := range []string{
		"Choose one or more release note categories",
		"breaking, behavior, security",
		"dependency, generated scaffold, or migration impact",
	} {
		if !strings.Contains(checklist, required) {
			t.Fatalf("docs/release-notes.md release checklist missing category discipline %q", required)
		}
	}
}

func TestReadmeMinimalExampleMatchesTestedSnippet(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	section := markdownSection(t, readme, "Minimal existing-service example")
	blocks := markdownCodeBlocks(section)
	if len(blocks) == 0 {
		t.Fatal("README.md minimal existing-service example missing Go code block")
	}

	snippetPath := filepath.Join(repoRoot, "examples", "snippets", "minimal-existing-service", "main.go")
	snippet := readText(t, snippetPath)
	if strings.TrimSpace(blocks[0]) != strings.TrimSpace(snippet) {
		t.Fatal("README.md minimal existing-service example drifted from examples/snippets/minimal-existing-service/main.go")
	}
	if !strings.Contains(section, "examples/snippets/minimal-existing-service/main.go") {
		t.Fatal("README.md minimal existing-service example must link to its tested source file")
	}

	out, err := runGoCmd(filepath.Dir(snippetPath), "test", ".")
	if err != nil {
		t.Fatalf("minimal existing-service snippet does not compile:\n%s\nerror: %v", out, err)
	}
}

func TestQualityAuditP0EvidenceAndProcessDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)

	governance := readText(t, filepath.Join(repoRoot, "docs", "governance.md"))
	for _, required := range []string{
		"ci / test",
		"make coverage-check",
		"make test-race",
		"make vuln",
		"ci / lint",
		"ci / governance",
		"make docs-check",
		"make v3-readiness-check",
		"ci / api-check",
		"make release-api-check",
		"ci / fuzz",
		"codeql",
		"scorecard",
		"make github-governance-check",
	} {
		if !strings.Contains(governance, required) {
			t.Fatalf("docs/governance.md missing required branch protection evidence %q", required)
		}
	}

	dependencyRisk := readText(t, filepath.Join(repoRoot, "docs", "dependency-risk.md"))
	for _, required := range []string{
		"Dependency PR SLA",
		"Dependabot is configured weekly",
		"Security update, critical or high",
		"Routine update open 14 days",
		"Routine update open 30 days",
		"owner and next review date",
	} {
		if !strings.Contains(dependencyRisk, required) {
			t.Fatalf("docs/dependency-risk.md missing dependency SLA text %q", required)
		}
	}

	coverageDoc := readText(t, filepath.Join(repoRoot, "docs", "test-coverage.md"))
	for _, required := range []string{
		"GOTOOLCHAIN=local make coverage-check",
		".ci-result/coverage/",
		"package-summary.tsv",
		"ROOT_COVERAGE_MIN",
		"CONTRIB_COVERAGE_MIN",
		"API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make release-evidence",
	} {
		if !strings.Contains(coverageDoc, required) {
			t.Fatalf("docs/test-coverage.md missing coverage evidence text %q", required)
		}
	}

	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	for _, source := range []struct {
		name string
		text string
	}{
		{"README.md", readme},
		{"docs/README.md", docsIndex},
	} {
		if !strings.Contains(source.text, "docs/test-coverage.md") && !strings.Contains(source.text, "test-coverage.md") {
			t.Fatalf("%s missing docs/test-coverage.md link", source.name)
		}
	}

	coverageScript := readText(t, filepath.Join(repoRoot, "scripts", "coverage_check.sh"))
	for _, required := range []string{
		"package-summary.tsv",
		"coverage_row root",
		"coverage_row contrib",
		"floor_env",
		"observed_percent",
	} {
		if !strings.Contains(coverageScript, required) {
			t.Fatalf("scripts/coverage_check.sh missing package coverage summary text %q", required)
		}
	}

	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))
	releaseCheckTargets := makeSubtargets(t, makefile, "release-check")
	if !containsString(releaseCheckTargets, "coverage-check") {
		t.Fatalf("release-check target must include coverage-check: %v", releaseCheckTargets)
	}
	summaryScript := readText(t, filepath.Join(repoRoot, "scripts", "release_check_summary.sh"))
	if !strings.Contains(summaryScript, "\"coverage-check\"") || !strings.Contains(summaryScript, "\"make coverage-check\"") {
		t.Fatal("release evidence summary script must record coverage-check")
	}
}

func TestQualityAuditP0BenchmarkBaselineDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	performance := readText(t, filepath.Join(repoRoot, "docs", "performance.md"))
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))

	for _, required := range []string{
		"GOWORK=off GOTOOLCHAIN=local go test",
		"-bench 'Benchmark'",
		"-benchmem",
		"BenchmarkBindingDecodeJSON",
		"BenchmarkBindingDecodeQuery",
		"BenchmarkQueryParamsParseRequestShape",
		"BenchmarkRegistryOpenAPI100Operations",
		"BenchmarkRouteContractsRegisterAndValidate",
		"BenchmarkMaxBodyWithinLimit",
		"BenchmarkPropagatorSuccess",
		"BenchmarkHardTimeoutSuccess",
		"BenchmarkIdempotencyNew",
		"BenchmarkIdempotencyReplay",
		"BenchmarkRateLimit",
		"BenchmarkOpenAPIRequestValidation",
		"BenchmarkOpenAPIResponseValidation",
		"BenchmarkRequestLog",
		"BenchmarkRequestLogWithHeaders",
		"BenchmarkNewServiceSaaSAPIGeneration",
		"Generated service scaffold",
		"benchmark temp directories only",
	} {
		if !strings.Contains(performance, required) {
			t.Fatalf("docs/performance.md missing benchmark baseline text %q", required)
		}
	}
	for _, source := range []struct {
		name string
		text string
	}{
		{"README.md", readme},
		{"docs/README.md", docsIndex},
	} {
		if !strings.Contains(source.text, "docs/performance.md") && !strings.Contains(source.text, "performance.md") {
			t.Fatalf("%s missing docs/performance.md link", source.name)
		}
	}
}

func TestQualityAuditP1AdoptionPathDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))

	for _, path := range []string{
		"docs/library-first.md",
		"docs/minimal-core.md",
		"docs/core-package-guide.md",
		"docs/scaffold-first.md",
		"docs/contrib-adapters.md",
	} {
		if !strings.Contains(readme, path) {
			t.Fatalf("README.md missing adoption-path link %s", path)
		}
		short := strings.TrimPrefix(path, "docs/")
		if !strings.Contains(docsIndex, short) {
			t.Fatalf("docs/README.md missing adoption-path link %s", short)
		}
	}
	if !strings.Contains(readme, "Differentiator: production guardrails for conventional Go JSON APIs") {
		t.Fatal("README.md missing one-sentence differentiator")
	}

	libraryFirst := readText(t, filepath.Join(repoRoot, "docs", "library-first.md"))
	for _, required := range []string{
		"`net/http`, chi",
		"app-owned router service",
		"Do not start with the scaffold for this path",
		"Five-minute adoption",
		"minimal-core.md",
		"contrib-adapters.md",
	} {
		if !strings.Contains(libraryFirst, required) {
			t.Fatalf("docs/library-first.md missing %q", required)
		}
	}

	minimalCore := readText(t, filepath.Join(repoRoot, "docs", "minimal-core.md"))
	for _, required := range []string{
		"`httpx`",
		"`binding`",
		"`middleware/maxbody`",
		"`middleware/timeout`",
		"It does not use contrib",
		"generated scaffolds",
		"provider adapters",
		"business ports.",
	} {
		if !strings.Contains(minimalCore, required) {
			t.Fatalf("docs/minimal-core.md missing %q", required)
		}
	}

	coreGuide := readText(t, filepath.Join(repoRoot, "docs", "core-package-guide.md"))
	for _, required := range []string{
		"| Package | Use case | Use when | Do not use when | Stability | Dependency note | Example |",
		"`middleware/auth/jwt`",
		"auth/JWK",
		"`compat/billing`",
		"compatibility-only",
		"[example](../binding/example_test.go)",
	} {
		if !strings.Contains(coreGuide, required) {
			t.Fatalf("docs/core-package-guide.md missing %q", required)
		}
	}

	scaffoldFirst := readText(t, filepath.Join(repoRoot, "docs", "scaffold-first.md"))
	gettingStarted := readText(t, filepath.Join(repoRoot, "docs", "getting-started.md"))
	for _, source := range []struct {
		name string
		text string
	}{
		{"docs/scaffold-first.md", scaffoldFirst},
		{"docs/getting-started.md", gettingStarted},
	} {
		for _, required := range []string{
			"app-owned generated code",
			"library-first.md",
		} {
			if !strings.Contains(source.text, required) {
				t.Fatalf("%s missing %q", source.name, required)
			}
		}
	}

	contribAdapters := readText(t, filepath.Join(repoRoot, "docs", "contrib-adapters.md"))
	for _, required := range []string{
		"Contrib is outside the stable core API promise",
		"`supported-adapter`",
		"`docs/package-classification.tsv`",
		"Generated scaffold code is app-owned",
	} {
		if !strings.Contains(contribAdapters, required) {
			t.Fatalf("docs/contrib-adapters.md missing %q", required)
		}
	}
}

func TestQualityAuditP1DependencyWorthinessDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))
	ci := readText(t, filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
	releaseSummary := readText(t, filepath.Join(repoRoot, "scripts", "release_check_summary.sh"))
	dependencyReport := readText(t, filepath.Join(repoRoot, "scripts", "dependency_report.sh"))

	for _, path := range []string{
		"docs/dependency-policy.md",
		"docs/dependency-footprint.md",
		"docs/support-policy.md",
		"docs/adr/0001-module-boundaries.md",
	} {
		if !strings.Contains(readme, path) && !strings.Contains(docsIndex, strings.TrimPrefix(path, "docs/")) {
			t.Fatalf("README.md or docs/README.md missing dependency-worthiness link %s", path)
		}
	}

	policy := readText(t, filepath.Join(repoRoot, "docs", "dependency-policy.md"))
	for _, required := range []string{
		"Allowed Dependency Classes",
		"Banned Patterns",
		"Auth, crypto, JWT, and JWK dependencies",
		"Provider SDKs",
		"Root stable packages importing contrib packages",
		"GOTOOLCHAIN=local make dependency-report",
		"GOTOOLCHAIN=local make vuln",
		"critical or high security updates are reviewed immediately",
	} {
		if !strings.Contains(policy, required) {
			t.Fatalf("docs/dependency-policy.md missing %q", required)
		}
	}

	footprint := readText(t, filepath.Join(repoRoot, "docs", "dependency-footprint.md"))
	for _, required := range []string{
		"GOTOOLCHAIN=local make dependency-report",
		"API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make dependency-report",
		".ci-result/dependencies/",
		"minimal-core-summary.tsv",
		"vulnerability_evidence",
	} {
		if !strings.Contains(footprint, required) {
			t.Fatalf("docs/dependency-footprint.md missing %q", required)
		}
	}

	support := readText(t, filepath.Join(repoRoot, "docs", "support-policy.md"))
	for _, required := range []string{
		"Root and contrib target Go `1.25.x`",
		"`go 1.25.0`",
		"`GOTOOLCHAIN=local`",
		"| Linux | amd64 | Supported |",
		"macOS",
		"Windows",
		"Do not claim broad OS/architecture support",
	} {
		if !strings.Contains(support, required) {
			t.Fatalf("docs/support-policy.md missing %q", required)
		}
	}

	adr := readText(t, filepath.Join(repoRoot, "docs", "adr", "0001-module-boundaries.md"))
	for _, required := range []string{
		"Status: Accepted",
		"Keep the current two-module layout for v3",
		"Root remains the stable API module",
		"Contrib remains the adapter, integration, example, and tooling module",
		"v4 plan may split auth-heavy packages",
		"root JWT/JWK dependency inheritance is accepted for v3",
	} {
		if !strings.Contains(adr, required) {
			t.Fatalf("docs/adr/0001-module-boundaries.md missing %q", required)
		}
	}

	for _, required := range []string{
		"dependency-report:",
		"scripts/dependency_report.sh",
	} {
		if !strings.Contains(makefile, required) {
			t.Fatalf("Makefile missing dependency-report target text %q", required)
		}
	}
	if !containsString(makeSubtargets(t, makefile, "release-check"), "dependency-report") {
		t.Fatal("release-check target must include dependency-report")
	}
	if !strings.Contains(ci, "Dependency footprint report") || !strings.Contains(ci, "make dependency-report") {
		t.Fatal(".github/workflows/ci.yml missing dependency footprint report step")
	}
	if !strings.Contains(releaseSummary, "\"dependency-report\"") || !strings.Contains(releaseSummary, "\"make dependency-report\"") {
		t.Fatal("release evidence summary script must record dependency-report")
	}
	for _, required := range []string{
		"minimal-core-summary.tsv",
		"root.added.modules",
		"contrib.removed.modules",
		"API_BASE_REF",
		"vulnerability_evidence",
	} {
		if !strings.Contains(dependencyReport, required) {
			t.Fatalf("scripts/dependency_report.sh missing %q", required)
		}
	}
}

func TestQualityAuditP1APIGovernanceDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	versioning := readText(t, filepath.Join(repoRoot, "VERSIONING.md"))
	makefile := readText(t, filepath.Join(repoRoot, "Makefile"))

	for _, path := range []string{
		"docs/api-inventory.md",
		"docs/api-review-checklist.md",
		"docs/api-addition-exceptions.tsv",
		"docs/options-structs.md",
		"docs/deprecations.md",
	} {
		if !strings.Contains(readme, path) && !strings.Contains(docsIndex, strings.TrimPrefix(path, "docs/")) {
			t.Fatalf("README.md or docs/README.md missing API governance link %s", path)
		}
		if !strings.Contains(versioning, path) {
			t.Fatalf("VERSIONING.md missing API governance link %s", path)
		}
	}

	inventory := readText(t, filepath.Join(repoRoot, "docs", "api-inventory.md"))
	for _, required := range []string{
		"Code generated by internal/tools/apiinventory",
		"| Symbol | Kind | Added version | Deprecation status |",
		"`github.com/aatuh/api-toolkit/v3/binding`",
		"`DecodeJSON`",
		"`github.com/aatuh/api-toolkit/v3/compat/billing`",
		"`compatibility-only`",
		"v3 compatibility surface",
	} {
		if !strings.Contains(inventory, required) {
			t.Fatalf("docs/api-inventory.md missing %q", required)
		}
	}

	checklist := readText(t, filepath.Join(repoRoot, "docs", "api-review-checklist.md"))
	for _, required := range []string{
		"Zero value",
		"Options validation",
		"Context and cancellation",
		"Error behavior",
		"Concurrency",
		"Return types",
		"Exported interface necessity",
		"GOTOOLCHAIN=local make api-inventory-check",
		"GOTOOLCHAIN=local make api-additions-check",
		"API Additions Are Forever",
		"docs/api-addition-exceptions.tsv",
		"compile-checked Go example",
	} {
		if !strings.Contains(checklist, required) {
			t.Fatalf("docs/api-review-checklist.md missing %q", required)
		}
	}

	exceptions := readText(t, filepath.Join(repoRoot, "docs", "api-addition-exceptions.tsv"))
	for _, required := range []string{
		"import_path\tsymbol\trationale\towner\treviewed_on",
		"docs/api-inventory.md entry",
		"package-tied release note",
	} {
		if !strings.Contains(exceptions, required) {
			t.Fatalf("docs/api-addition-exceptions.tsv missing %q", required)
		}
	}

	deprecations := readText(t, filepath.Join(repoRoot, "docs", "deprecations.md"))
	for _, required := range []string{
		"Fully qualified package symbol",
		"Replacement",
		"Removal earliest major",
		"Migration snippet",
		"Deprecated:",
		"Active Deprecation Register",
	} {
		if !strings.Contains(deprecations, required) {
			t.Fatalf("docs/deprecations.md missing %q", required)
		}
	}

	for _, required := range []string{
		"api-inventory:",
		"api-inventory-check:",
		"api-additions-check:",
		"internal/tools/apiinventory",
		"scripts/api_inventory_check.sh",
		"scripts/api_additions_check.sh",
	} {
		if !strings.Contains(makefile, required) {
			t.Fatalf("Makefile missing API inventory target text %q", required)
		}
	}
	if !containsString(makeSubtargets(t, makefile, "docs-check"), "api-inventory-check") {
		t.Fatal("docs-check target must include api-inventory-check")
	}
	if !containsString(makeSubtargets(t, makefile, "docs-check"), "api-additions-check") {
		t.Fatal("docs-check target must include api-additions-check")
	}
}

func TestAPIAdditionsForeverGateIsWired(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	script := readText(t, filepath.Join(repoRoot, "scripts", "api_additions_check.sh"))
	tool := readText(t, filepath.Join(repoRoot, "internal", "tools", "apiadditions", "main.go"))
	workflow := readText(t, filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
	prTemplate := readText(t, filepath.Join(repoRoot, ".github", "pull_request_template.md"))
	issueTemplate := readText(t, filepath.Join(repoRoot, ".github", "ISSUE_TEMPLATE", "api_change.md"))
	runbook := readText(t, filepath.Join(repoRoot, "docs", "release-runbook.md"))

	for _, required := range []string{
		"API_ADDITIONS_BASE_REF",
		"API_BASE_REF",
		"GITHUB_BASE_REF",
		"git worktree add",
		"go run ./internal/tools/apiadditions",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("scripts/api_additions_check.sh missing %q", required)
		}
	}
	for _, required := range []string{
		"missing source doc comment",
		"missing compile-checked example",
		"missing package-tied release note",
		"docs/api-addition-exceptions.tsv",
		"releaseNotesMentionSymbol",
	} {
		if !strings.Contains(tool, required) {
			t.Fatalf("internal/tools/apiadditions/main.go missing %q", required)
		}
	}
	for _, required := range []string{
		"API additions are forever gate (pull request base)",
		"API_ADDITIONS_BASE_REF: origin/${{ github.base_ref }}",
		"API additions are forever gate (push previous commit)",
		"API_ADDITIONS_BASE_REF: ${{ github.event.before }}",
		"make api-additions-check",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf(".github/workflows/ci.yml missing %q", required)
		}
	}
	for _, required := range []string{
		"New stable exported identifiers have doc comments",
		"examples or exact exception rows",
		"release notes",
	} {
		if !strings.Contains(prTemplate, required) {
			t.Fatalf(".github/pull_request_template.md missing %q", required)
		}
	}
	for _, required := range []string{
		"Example or exact exception",
		"Release note or changelog entry",
	} {
		if !strings.Contains(issueTemplate, required) {
			t.Fatalf(".github/ISSUE_TEMPLATE/api_change.md missing %q", required)
		}
	}
	for _, required := range []string{
		"API_ADDITIONS_BASE_REF=v3.1.2 GOTOOLCHAIN=local make api-additions-check",
		"stable identifiers need doc comments",
		"compile-checked examples or exact exceptions",
	} {
		if !strings.Contains(runbook, required) {
			t.Fatalf("docs/release-runbook.md missing %q", required)
		}
	}
}

func TestQualityAuditP1APIDesignOperationalDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))

	for _, path := range []string{
		"docs/interface-ownership.md",
		"docs/context-cancellation.md",
		"docs/errors.md",
		"docs/concurrency.md",
		"docs/resource-lifecycle.md",
	} {
		if !strings.Contains(readme, path) && !strings.Contains(docsIndex, strings.TrimPrefix(path, "docs/")) {
			t.Fatalf("README.md or docs/README.md missing API design operations link %s", path)
		}
	}

	interfaces := readText(t, filepath.Join(repoRoot, "docs", "interface-ownership.md"))
	for _, required := range []string{
		"`authorization.Owner`",
		"`compat/billing.BillingProvider`",
		"`middleware/auth/apikey.Verifier`",
		"`oauth2.Validator`",
		"`ports.DatabasePool`",
		"`ports.IdempotencyStore`",
		"`ports.PolicyEngine`",
		"`routecontracts.Router`",
		"`scheduler.Recorder`",
		"`webhooks.Verifier`",
		"Challenge any new exported interface with fewer than two real implementations",
	} {
		if !strings.Contains(interfaces, required) {
			t.Fatalf("docs/interface-ownership.md missing %q", required)
		}
	}

	contextDocs := readText(t, filepath.Join(repoRoot, "docs", "context-cancellation.md"))
	for _, required := range []string{
		"incoming request work uses `r.Context()`",
		"provider, store, validator, scheduler, and client boundaries accept",
		"`middleware/timeout.NewPropagator`",
		"`middleware/timeout.NewHard`",
		"bounded cleanup context",
		"httptest.NewRequestWithContext",
	} {
		if !strings.Contains(contextDocs, required) {
			t.Fatalf("docs/context-cancellation.md missing %q", required)
		}
	}

	errorDocs := readText(t, filepath.Join(repoRoot, "docs", "errors.md"))
	for _, required := range []string{
		"Sentinel errors",
		"`httpx.ErrUnauthorized`",
		"`httpx.HTTPError`",
		"`fielderrors.FieldErrors`",
		"Use `errors.Is`",
		"Use `errors.As`",
		"Problem Details",
		"Do not expose secrets",
	} {
		if !strings.Contains(errorDocs, required) {
			t.Fatalf("docs/errors.md missing %q", required)
		}
	}

	concurrency := readText(t, filepath.Join(repoRoot, "docs", "concurrency.md"))
	for _, required := range []string{
		"construct middleware, registries, codecs, and adapters",
		"during startup",
		"`middleware/ratelimit` in-process state",
		"`middleware/idempotency`",
		"`routecontracts.Registry` and `specs.Registry`",
		"Run `make test-race`",
	} {
		if !strings.Contains(concurrency, required) {
			t.Fatalf("docs/concurrency.md missing %q", required)
		}
	}

	lifecycle := readText(t, filepath.Join(repoRoot, "docs", "resource-lifecycle.md"))
	for _, required := range []string{
		"`ports.DatabasePool`, `ports.DatabaseConnection`, `ports.DatabaseRows`, `ports.Migrator`",
		"`middleware/auth/jwt.Middleware`",
		"`middleware/ratelimit` in-process limiter",
		"`apiclient` transports",
		"Generated services",
		"Every new package that opens network connections, files, timers, goroutines",
	} {
		if !strings.Contains(lifecycle, required) {
			t.Fatalf("docs/resource-lifecycle.md missing %q", required)
		}
	}
}

func TestSecurityDocsMentionDangerousBypassConfiguration(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	docs := readText(t, filepath.Join(repoRoot, "docs", "security.md"))

	requirements := map[string][]string{
		filepath.Join(repoRoot, "contrib", "bootstrap", "http.go"): {
			"RATE_LIMIT_SKIP_ENABLED",
			"RATE_LIMIT_SKIP_HEADER",
			"RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES",
			"TRUSTED_PROXIES",
		},
		filepath.Join(repoRoot, "contrib", "integrations", "auth", "jwt", "jwt.go"): {
			"JWT_SKIP_HEADER_ENABLED",
			"JWT_SKIP_HEADER_NAME",
			"JWT_SKIP_TRUSTED_PROXIES",
			"JWT_ALLOW_DANGEROUS_DEV_BYPASSES",
		},
		filepath.Join(repoRoot, "contrib", "middleware", "auth", "clerk", "config.go"): {
			"CLERK_SKIP_HEADER_ENABLED",
			"CLERK_SKIP_HEADER_NAME",
			"CLERK_SKIP_TRUSTED_PROXIES",
			"CLERK_ALLOW_DANGEROUS_DEV_BYPASSES",
		},
		filepath.Join(repoRoot, "contrib", "middleware", "auth", "devheaders", "config.go"): {
			"DEV_AUTH_FALLBACK_ENABLED",
			"DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES",
			"DEV_AUTH_TRUSTED_PROXIES",
		},
		filepath.Join(repoRoot, "contrib", "adapters", "stripe", "config.go"): {
			"STRIPE_WEBHOOK_SKIP_VERIFY",
			"STRIPE_WEBHOOK_DEV_MODE",
		},
	}

	for path, names := range requirements {
		code := readText(t, path)
		for _, name := range names {
			if !strings.Contains(code, `"`+name+`"`) {
				rel, _ := filepath.Rel(repoRoot, path)
				t.Fatalf("%s no longer contains bypass env var %s", rel, name)
			}
			if !strings.Contains(docs, "`"+name+"`") {
				t.Fatalf("docs/security.md missing bypass env var %s", name)
			}
		}
	}

	if !strings.Contains(strings.ToLower(docs), "trusted prox") {
		t.Fatal("docs/security.md must mention the trusted-proxy restriction for dev bypasses")
	}
}

func TestReleaseNotesIncludeStableSurfaceChecklist(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	notes := readText(t, filepath.Join(repoRoot, "docs", "release-notes.md"))

	for _, required := range []string{
		"For stable surface changes, deprecations, or compatibility-sensitive updates",
		"`VERSIONING.md`",
		"`docs/ports-surface.md`",
		"`docs/v3-compatibility-roadmap.md`",
		"`scripts/apicheck.sh`",
		"docscheck coverage",
		"release notes and upgrade notes",
		"contrib-api-drift-report",
		"contrib-release-notes-check",
	} {
		if !strings.Contains(notes, required) {
			t.Fatalf("docs/release-notes.md missing release checklist text %q", required)
		}
	}
}

func TestDeprecatedBillingPortsPointToCompatPackage(t *testing.T) {
	skipV2CompatibilitySurfaceChecksOnV3(t)
	repoRoot := mustRepoRoot(t)
	path := filepath.Join(repoRoot, "ports", "billing.go")
	code := readText(t, path)

	for _, name := range exportedTopLevelNames(t, path) {
		replacement := rootModulePath + "/v3/compat/billing." + name
		deprecation := "Deprecated: use " + replacement + "."
		if !strings.Contains(code, deprecation) {
			t.Fatalf("ports/billing.go missing deprecation replacement for %s: %s", name, replacement)
		}
	}
}

func TestDeprecatedBillingPortsStayInCompatibilitySource(t *testing.T) {
	skipV2CompatibilitySurfaceChecksOnV3(t)
	repoRoot := mustRepoRoot(t)
	deprecatedNames := stringSet(exportedTopLevelNames(t, filepath.Join(repoRoot, "ports", "billing.go")))
	violations := scanGoSourceViolations(t, repoRoot, func(fset *token.FileSet, path string, file *ast.File) []string {
		if allowedDeprecatedBillingSource(repoRoot, path) {
			return nil
		}
		aliases, dotImport := importAliases(file, rootModulePath+"/v3/ports")
		if len(aliases) == 0 && !dotImport {
			return nil
		}
		var out []string
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SelectorExpr:
				ident, ok := node.X.(*ast.Ident)
				if ok && aliases[ident.Name] && deprecatedNames[node.Sel.Name] {
					out = append(out, sourceViolation(repoRoot, path, fset, node.Pos(), "uses deprecated ports."+node.Sel.Name+"; use compat/billing or an app-owned port"))
				}
			case *ast.Ident:
				if dotImport && deprecatedNames[node.Name] {
					out = append(out, sourceViolation(repoRoot, path, fset, node.Pos(), "uses deprecated dot-imported billing symbol "+node.Name))
				}
			}
			return true
		})
		return out
	})
	if len(violations) > 0 {
		t.Fatalf("deprecated billing ports used outside compatibility source:\n%s", strings.Join(violations, "\n"))
	}
}

func TestDatabaseStatsStayInCompatibilityOrAdapterSource(t *testing.T) {
	skipV2CompatibilitySurfaceChecksOnV3(t)
	repoRoot := mustRepoRoot(t)
	violations := scanGoSourceViolations(t, repoRoot, func(fset *token.FileSet, path string, file *ast.File) []string {
		if allowedDatabaseStatsSource(repoRoot, path) {
			return nil
		}
		aliases, dotImport := importAliases(file, rootModulePath+"/v3/ports")
		if len(aliases) == 0 && !dotImport {
			return nil
		}
		var out []string
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SelectorExpr:
				ident, ok := node.X.(*ast.Ident)
				if ok && aliases[ident.Name] && node.Sel.Name == "DatabaseStats" {
					out = append(out, sourceViolation(repoRoot, path, fset, node.Pos(), "uses direct ports.DatabaseStats; prefer DatabasePoolSnapshotProvider or SnapshotDatabasePoolStats"))
				}
			case *ast.CallExpr:
				selector, ok := node.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, identOK := selector.X.(*ast.Ident)
				if identOK && selector.Sel.Name == "Stat" && strings.Contains(strings.ToLower(ident.Name), "pool") {
					out = append(out, sourceViolation(repoRoot, path, fset, selector.Sel.Pos(), "calls Stat directly in code that imports ports; prefer snapshot APIs"))
				}
			case *ast.Ident:
				if dotImport && node.Name == "DatabaseStats" {
					out = append(out, sourceViolation(repoRoot, path, fset, node.Pos(), "uses dot-imported DatabaseStats; prefer snapshot APIs"))
				}
			}
			return true
		})
		return out
	})
	if len(violations) > 0 {
		t.Fatalf("direct database stats usage found outside compatibility or adapter source:\n%s", strings.Join(violations, "\n"))
	}
}

func TestSecurityDocsCoverHardTimeoutCaptureAndPanicProfileOptions(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	security := readText(t, filepath.Join(repoRoot, "docs", "security.md"))
	profile := readText(t, filepath.Join(repoRoot, "securityprofile", "profile.go"))
	timeout := readText(t, filepath.Join(repoRoot, "middleware", "timeout", "timeout.go"))

	for _, required := range []string{
		"WithHardTimeoutMaxCaptureBytes",
		"RouteOverride.HardTimeoutMaxCaptureBytes",
		"large non-streaming responses",
		"do not make streaming routes safe",
		"Handler panics inside hard",
		"timeout are contained",
	} {
		if !strings.Contains(security, required) {
			t.Fatalf("docs/security.md missing hard-timeout profile guidance %q", required)
		}
	}
	for _, required := range []string{
		"WithHardTimeoutMaxCaptureBytes",
		"HardTimeoutMaxCaptureBytes",
		"MaxCaptureBytes: cfg.hardTimeoutMaxCaptureBytes",
	} {
		if !strings.Contains(profile, required) {
			t.Fatalf("securityprofile/profile.go missing capture option propagation %q", required)
		}
	}
	if !strings.Contains(timeout, "defaultHardTimeoutPanicProblem") || !strings.Contains(timeout, "recover()") {
		t.Fatal("middleware/timeout must recover child goroutine panics with deterministic Problem Details")
	}
}

func TestAdapterLegacyRecoveryTelemetryRedactsKeysByDefault(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	metrics := readText(t, filepath.Join(repoRoot, "docs", "metrics.md"))
	security := readText(t, filepath.Join(repoRoot, "docs", "security.md"))
	for _, path := range []string{
		filepath.Join(repoRoot, "contrib", "adapters", "idempotency", "memory.go"),
		filepath.Join(repoRoot, "contrib", "adapters", "idempotencyredis", "redis.go"),
	} {
		code := readText(t, path)
		for _, required := range []string{
			"LegacyInFlightRecoveryRawKey",
			"KeyHash",
			"RawKey",
			"legacyInFlightRecoveryEventKey(key, false)",
		} {
			if !strings.Contains(code, required) {
				rel, _ := filepath.Rel(repoRoot, path)
				t.Fatalf("%s missing adapter recovery privacy contract %q", rel, required)
			}
		}
	}
	for _, required := range []string{
		"Adapter-level legacy idempotency recovery events",
		"hash the `Key` field by default",
		"`RawKey` empty unless",
	} {
		if !strings.Contains(metrics, required) {
			t.Fatalf("docs/metrics.md missing adapter telemetry privacy guidance %q", required)
		}
	}
	if !strings.Contains(security, "Adapter legacy idempotency recovery events hash keys by default") {
		t.Fatal("docs/security.md missing adapter telemetry privacy guidance")
	}
}

func TestGeneratedScaffoldDocumentsFailClosedIdempotentWrites(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	scaffold := readText(t, filepath.Join(repoRoot, "contrib", "cmd", "api-toolkit", "main.go"))
	security := readText(t, filepath.Join(repoRoot, "docs", "security.md"))

	for _, required := range []string{
		"RequireKey:     true",
		"TenantScopedStorageKeyFunc()",
		"Unsafe writes without",
		"Idempotency-Key",
		"Problem Details 400",
		"Idempotency storage keys are tenant and actor scoped",
	} {
		if !strings.Contains(scaffold, required) {
			t.Fatalf("api-toolkit service scaffold missing idempotency contract %q", required)
		}
	}
	for _, required := range []string{
		"Options.RequireKey",
		"Problem Details 400",
		"TenantScopedStorageKeyFunc()",
	} {
		if !strings.Contains(security, required) {
			t.Fatalf("docs/security.md missing generated idempotency guidance %q", required)
		}
	}
}

func TestGeneratedScaffoldDocumentsProductionRateLimitStore(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	scaffold := readText(t, filepath.Join(repoRoot, "contrib", "cmd", "api-toolkit", "main.go"))
	security := readText(t, filepath.Join(repoRoot, "docs", "security.md"))

	for _, required := range []string{
		"RATE_LIMIT_STORE=memory",
		"RATE_LIMIT_STORE=redis",
		"RATE_LIMIT_REDIS_ADDR",
		"rate-limit-redis",
	} {
		if !strings.Contains(scaffold, required) {
			t.Fatalf("api-toolkit service scaffold missing rate-limit production contract %q", required)
		}
	}
	for _, required := range []string{
		"RATE_LIMIT_STORE=memory",
		"RATE_LIMIT_STORE=redis",
		"RATE_LIMIT_REDIS_ADDR",
	} {
		if !strings.Contains(security, required) {
			t.Fatalf("docs/security.md missing generated rate-limit guidance %q", required)
		}
	}
}

func TestGeneratedScaffoldDocumentsTelemetryDefaults(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	scaffold := readText(t, filepath.Join(repoRoot, "contrib", "cmd", "api-toolkit", "main.go"))
	security := readText(t, filepath.Join(repoRoot, "docs", "security.md"))

	for _, required := range []string{
		"OTEL_TRACING_ENABLED=false",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"telemetry.InitTracing",
		"otel-tracing",
	} {
		if !strings.Contains(scaffold, required) {
			t.Fatalf("api-toolkit service scaffold missing telemetry contract %q", required)
		}
	}
	for _, required := range []string{
		"OTEL_TRACING_ENABLED=false",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"tracer provider",
	} {
		if !strings.Contains(security, required) {
			t.Fatalf("docs/security.md missing generated telemetry guidance %q", required)
		}
	}
}

func TestFullScaffoldPlatformContractIsDocumented(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	docsIndex := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	gettingStarted := readText(t, filepath.Join(repoRoot, "docs", "getting-started.md"))
	fullScaffold := readText(t, filepath.Join(repoRoot, "docs", "full-service-scaffold.md"))
	releaseManifests := readText(t, filepath.Join(repoRoot, "docs", "release-manifests.md"))

	for _, required := range []string{
		"saas-api-full",
		"full-service-scaffold.md",
	} {
		if !strings.Contains(docsIndex, required) {
			t.Fatalf("docs/README.md missing full scaffold contract reference %q", required)
		}
		if !strings.Contains(gettingStarted, required) {
			t.Fatalf("docs/getting-started.md missing full scaffold guidance %q", required)
		}
	}
	for _, required := range []string{
		"Postgres",
		"Redis",
		"organizations",
		"memberships",
		"invitations",
		"scoped API keys",
		"durable async",
		"transactional outbox",
		"audit events",
		"webhook delivery",
		"OpenAPI 3.1",
		"typed Go client",
		"opt-in Docker integration tests",
		"Kubernetes manifests",
		"experimental",
	} {
		if !strings.Contains(fullScaffold, required) {
			t.Fatalf("docs/full-service-scaffold.md missing full scaffold contract %q", required)
		}
	}
	for _, required := range []string{
		"saas-api-full",
		"full-profile runtime assets",
		"opt-in integration checks",
	} {
		if !strings.Contains(releaseManifests, required) {
			t.Fatalf("docs/release-manifests.md missing full scaffold governance text %q", required)
		}
	}
}

func TestFullProfileRuntimeAssetsRequireReleaseNotes(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	notesScript := readText(t, filepath.Join(repoRoot, "scripts", "contrib_release_notes_check.sh"))
	contractScript := readText(t, filepath.Join(repoRoot, "scripts", "contrib_review_contract_test.sh"))
	releaseManifests := readText(t, filepath.Join(repoRoot, "docs", "release-manifests.md"))

	for _, required := range []string{
		"full-profile runtime assets",
		"contrib/cmd/api-toolkit",
		"*.tmpl",
		"*.yaml",
	} {
		if !strings.Contains(notesScript, required) {
			t.Fatalf("contrib release notes script missing full-profile runtime asset governance text %q", required)
		}
	}
	for _, required := range []string{
		"full-profile-runtime-asset-release-notes",
		"contrib/cmd/api-toolkit/templates/saas-api-full/k8s/deployment.yaml",
	} {
		if !strings.Contains(contractScript, required) {
			t.Fatalf("contrib review contract test missing full-profile runtime asset case %q", required)
		}
	}
	for _, required := range []string{
		"full-profile runtime assets",
		"release-note reviewed",
	} {
		if !strings.Contains(releaseManifests, required) {
			t.Fatalf("release manifest docs missing full-profile runtime asset rule %q", required)
		}
	}
}

func TestIdempotencyCaptureDoesNotUseLegacyResponseWriter(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	capture := readText(t, filepath.Join(repoRoot, "middleware", "idempotency", "capture.go"))
	middleware := readText(t, filepath.Join(repoRoot, "middleware", "idempotency", "idempotency.go"))

	if !strings.Contains(capture, "type responseCapture struct") || !strings.Contains(middleware, "newLimitedResponseCapture") {
		t.Fatal("middleware/idempotency must keep response capture package-local")
	}
	violations := scanGoSourceViolations(t, filepath.Join(repoRoot, "middleware", "idempotency"), func(fset *token.FileSet, path string, file *ast.File) []string {
		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return []string{err.Error()}
			}
			if importPath == rootModulePath+"/v3/response_writer" {
				return []string{sourceViolation(repoRoot, path, fset, imp.Pos(), "imports legacy response_writer from idempotency")}
			}
		}
		return nil
	})
	if len(violations) > 0 {
		t.Fatalf("idempotency response capture must not depend on response_writer:\n%s", strings.Join(violations, "\n"))
	}
}

func TestPublicMarkdownMakeTargetsExist(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	targets := makefileTargetSet(readText(t, filepath.Join(repoRoot, "Makefile")))
	commandPattern := regexp.MustCompile(`\bmake\s+([A-Za-z0-9_.-]+)`)
	codeSpanPattern := regexp.MustCompile("`(?:[A-Za-z0-9_./=-]+\\s+)*make\\s+([A-Za-z0-9_.-]+)[^`]*`")

	for _, path := range docsQualityMarkdownFiles(t, repoRoot) {
		rel := slashRel(repoRoot, path)
		content := readText(t, path)
		for _, block := range markdownCodeBlocks(content) {
			for _, match := range commandPattern.FindAllStringSubmatch(block, -1) {
				target := strings.TrimRight(match[1], ".,;:)")
				if target == "" {
					continue
				}
				if likelyProseAfterMake(target) {
					continue
				}
				if generatedScaffoldMakeTarget(rel, target) {
					continue
				}
				if !targets[target] {
					t.Fatalf("%s references absent Makefile target %q", rel, target)
				}
			}
		}
		for _, match := range codeSpanPattern.FindAllStringSubmatch(content, -1) {
			target := strings.TrimRight(match[1], ".,;:)")
			if target == "" {
				continue
			}
			if likelyProseAfterMake(target) {
				continue
			}
			if generatedScaffoldMakeTarget(rel, target) {
				continue
			}
			if !targets[target] {
				t.Fatalf("%s references absent Makefile target %q", rel, target)
			}
		}
	}
}

func generatedScaffoldMakeTarget(rel, target string) bool {
	switch rel {
	case "docs/getting-started.md":
		switch target {
		case "openapi-check", "contracts-lint", "contracts-diff", "client-check", "client-ts-check", "observability-check", "deploy-check", "resource-check", "migrate-plan", "migrate-check":
			return true
		}
	case "docs/full-service-scaffold.md":
		switch target {
		case "migrate-plan", "migrate-up", "migrate-status", "migrate-check", "migrate-verify", "migrate-down", "client-check", "client-ts-check", "asset-check", "observability-check", "deploy-check", "resource-check", "integration-check":
			return true
		}
	case "docs/release-notes.md":
		switch target {
		case "asset-check", "observability-check", "deploy-check":
			return true
		}
	case "docs/release-runbook.md":
		switch target {
		case "integration-check":
			return true
		}
	case "docs/reference-service.md":
		switch target {
		case "integration-check":
			return true
		}
	}
	return false
}

func TestPublicMarkdownLocalLinksResolve(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	linkPattern := regexp.MustCompile(`!?\[[^\]\n]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

	for _, path := range docsQualityMarkdownFiles(t, repoRoot) {
		rel := slashRel(repoRoot, path)
		content := readText(t, path)
		for _, match := range linkPattern.FindAllStringSubmatch(content, -1) {
			rawTarget := strings.Trim(match[1], "<>")
			if rawTarget == "" || strings.HasPrefix(rawTarget, "#") || externalMarkdownTarget(rawTarget) {
				continue
			}
			targetPath, anchor, _ := strings.Cut(rawTarget, "#")
			targetFile := targetPath
			if !filepath.IsAbs(targetFile) {
				targetFile = filepath.Join(filepath.Dir(path), targetPath)
			}
			targetFile = filepath.Clean(targetFile)
			info, err := os.Stat(targetFile)
			if err != nil {
				t.Fatalf("%s has broken local link %q: %v", rel, rawTarget, err)
			}
			if info.IsDir() {
				t.Fatalf("%s links to directory %q; link to a concrete document", rel, rawTarget)
			}
			if anchor != "" && strings.HasSuffix(targetFile, ".md") {
				anchors := markdownAnchors(readText(t, targetFile))
				if !anchors[anchor] {
					t.Fatalf("%s links to missing anchor %q in %s", rel, anchor, slashRel(repoRoot, targetFile))
				}
			}
		}
	}
}

func TestDocsIndexCoversHighCentralityDocs(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	index := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	combined := readme + "\n" + index

	for _, required := range []string{
		"README.md",
		"docs/getting-started.md",
		"docs/cookbook.md",
		"docs/architecture.md",
		"docs/migration/v3.md",
		"docs/troubleshooting.md",
		"docs/security.md",
		"docs/auth.md",
		"docs/idempotency.md",
		"docs/operations.md",
		"docs/openapi-workflow.md",
		"docs/configuration.md",
		"docs/observability.md",
		"docs/scaffold-support.md",
		"docs/adapter-maturity.md",
		"docs/safe-defaults.md",
		"docs/middleware-safety.md",
		"SECURITY.md",
		"docs/metrics.md",
		"VERSIONING.md",
		"docs/release-runbook.md",
		"docs/release-review.md",
		"docs/release-notes.md",
		"docs/release-manifests.md",
		"docs/api-reference.md",
		"docs/core-readiness.md",
		"docs/ports-surface.md",
		"docs/v3-compatibility-roadmap.md",
		"docs/performance.md",
		"docs/response-writer-inventory.md",
		"docs/dependency-boundary.md",
		"docs/dependency-risk.md",
		"docs/package-doc-standard.md",
		"docs/package-classification.tsv",
		"docs/contrib-api-drift-packages.txt",
		"docs/contrib-api-drift-dispositions.tsv",
		"docs/vulnerability-dispositions.tsv",
		"contrib/examples/README.md",
		"examples/reference-saas-api/README.md",
		"examples/reference-saas-api/deploy/helm/README.md",
		"examples/reference-saas-api/deploy/kubernetes/README.md",
		"examples/reference-saas-api/deploy/terraform/aws/README.md",
		"examples/reference-saas-api/observability/runbooks/observability.md",
		"examples/reference-saas-api/docs/providers/provider-runbook.md",
		"PANIC_POLICY.md",
		"release-check-summary.json",
	} {
		if !strings.Contains(combined, required) {
			t.Fatalf("README.md or docs/README.md must link or name high-centrality document %s", required)
		}
	}
}

func TestSafetyAndCoreReadinessDocsAreComplete(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	index := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	safeDefaults := readText(t, filepath.Join(repoRoot, "docs", "safe-defaults.md"))
	middlewareSafety := readText(t, filepath.Join(repoRoot, "docs", "middleware-safety.md"))
	coreReadiness := readText(t, filepath.Join(repoRoot, "docs", "core-readiness.md"))
	productionReadiness := readText(t, filepath.Join(repoRoot, "docs", "production-readiness.md"))

	for _, required := range []string{
		"docs/safe-defaults.md",
		"docs/middleware-safety.md",
		"docs/core-readiness.md",
	} {
		if !strings.Contains(readme+"\n"+index, required) {
			t.Fatalf("README.md or docs/README.md missing safety/readiness document %s", required)
		}
	}

	for _, required := range []string{
		"fail-open",
		"fail-closed",
		"Evidence",
		"tested",
		"middleware/timeout",
		"middleware/idempotency",
		"contrib/v3/middleware/openapi",
	} {
		if !strings.Contains(safeDefaults, required) {
			t.Fatalf("docs/safe-defaults.md missing %q", required)
		}
	}

	classes := loadPackageClassifications(t, repoRoot)
	for _, cls := range classes {
		if !strings.Contains(cls.ImportPath, "/middleware/") {
			continue
		}
		if cls.APIStatus != "stable" && cls.APIStatus != "supported-adapter" {
			continue
		}
		if !strings.Contains(safeDefaults, "`"+cls.ImportPath+"`") {
			t.Fatalf("docs/safe-defaults.md missing middleware default posture row for %s", cls.ImportPath)
		}
	}

	for _, required := range []string{
		"safe global middleware",
		"route-specific middleware",
		"forbidden for streaming",
		"required opt-outs",
		"`middleware/timeout`",
		"`middleware/idempotency`",
		"`openapi.ResponseValidationOptions.ShouldValidate`",
		"`securityprofile.StreamingRouteOverride`",
		"streaming, SSE, websocket, and large-download",
	} {
		if !strings.Contains(middlewareSafety, required) {
			t.Fatalf("docs/middleware-safety.md missing %q", required)
		}
	}

	for _, required := range []string{
		"Docs",
		"Examples",
		"Tests",
		"Fuzz",
		"Benchmark",
		"Compatibility",
		"Security review",
		"Production caveat",
	} {
		if !strings.Contains(coreReadiness, required) {
			t.Fatalf("docs/core-readiness.md missing readiness column %q", required)
		}
	}
	for _, pkg := range stableRootPackagesFromClassification(t, repoRoot) {
		if !strings.Contains(coreReadiness, "`"+pkg+"`") {
			t.Fatalf("docs/core-readiness.md missing stable package row for %s", pkg)
		}
	}
	for _, required := range []string{
		"`docs/core-readiness.md`",
		"package-specific production checklist",
		"fuzz decision",
		"benchmark decision",
		"security review notes",
	} {
		if !strings.Contains(productionReadiness, required) {
			t.Fatalf("docs/production-readiness.md missing package checklist text %q", required)
		}
	}
}

func TestRuntimeCompatibilityGoldenCoversStableHTTPContracts(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	testSource := readText(t, filepath.Join(repoRoot, "contracttest", "runtime_compat_test.go"))
	golden := readText(t, filepath.Join(repoRoot, "contracttest", "testdata", "golden", "runtime_compatibility.json"))

	var decoded map[string]any
	if err := json.Unmarshal([]byte(golden), &decoded); err != nil {
		t.Fatalf("runtime compatibility golden is not valid JSON: %v", err)
	}
	for _, required := range []string{
		"UPDATE_RUNTIME_COMPAT_GOLDEN=1",
		"timeout.NewHard",
		"specs.RegisterProblemCatalog",
		"runtimeCompatJSONMiddlewareRejection",
		"runtimeCompatDetailedHealthDisabled",
	} {
		if !strings.Contains(testSource, required) {
			t.Fatalf("contracttest/runtime_compat_test.go missing runtime compatibility coverage anchor %q", required)
		}
	}
	for _, required := range []string{
		`"httpx_write_json"`,
		`"httpx_write_problem"`,
		`"json_middleware_rejection"`,
		`"version_endpoint"`,
		`"health_detailed_disabled"`,
		`"hard_timeout_problem_response"`,
		`"openapi_metadata"`,
		`"application/problem+json"`,
		`"operationId": "createWidget"`,
	} {
		if !strings.Contains(golden, required) {
			t.Fatalf("runtime compatibility golden missing %q", required)
		}
	}
}

func TestMigrationAndTroubleshootingGuidesCoverCommonAdoptionFailures(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	index := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	migration := readText(t, filepath.Join(repoRoot, "docs", "migration", "v3.md"))
	troubleshooting := readText(t, filepath.Join(repoRoot, "docs", "troubleshooting.md"))

	for _, required := range []string{
		"docs/migration/v3.md",
		"docs/troubleshooting.md",
	} {
		if !strings.Contains(readme+"\n"+index, required) {
			t.Fatalf("README.md or docs/README.md missing adoption guide %s", required)
		}
	}
	for _, required := range []string{
		"`v3.1.2`",
		"`github.com/aatuh/api-toolkit/v3`",
		"`github.com/aatuh/api-toolkit/contrib/v3`",
		"go get github.com/aatuh/api-toolkit/v3@v3.1.2",
		"go get github.com/aatuh/api-toolkit/contrib/v3@v3.1.2",
		"go mod tidy",
		"GOTOOLCHAIN=local make docs-check",
		"`VERSIONING.md`",
		"`docs/release-notes.md`",
		"`docs/api-reference.md`",
		"`docs/deprecations.md`",
		"Generated services are app-owned generated code",
		"streaming, SSE, websocket, and large-download",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("docs/migration/v3.md missing migration guidance %q", required)
		}
	}
	for _, required := range []string{
		"Go version mismatch",
		"Contrib instability surprise",
		"Timeout buffering breaks streaming",
		"Missing health checks",
		"Idempotency storage confusion",
		"Auth config fails closed",
		"`GOTOOLCHAIN=local`",
		"`securityprofile.StreamingRouteOverride`",
		"`openapi.ResponseValidationOptions.ShouldValidate`",
		"`GOTOOLCHAIN=local make dependency-boundary-check`",
		"Generated services are app-owned code",
	} {
		if !strings.Contains(troubleshooting, required) {
			t.Fatalf("docs/troubleshooting.md missing troubleshooting guidance %q", required)
		}
	}
}

func TestFeatureProductionGuidesCoverCriticalContracts(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	index := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	security := readText(t, filepath.Join(repoRoot, "docs", "security.md"))
	idempotency := readText(t, filepath.Join(repoRoot, "docs", "idempotency.md"))
	auth := readText(t, filepath.Join(repoRoot, "docs", "auth.md"))
	operations := readText(t, filepath.Join(repoRoot, "docs", "operations.md"))
	openapiWorkflow := readText(t, filepath.Join(repoRoot, "docs", "openapi-workflow.md"))

	for _, required := range []string{
		"docs/auth.md",
		"docs/idempotency.md",
		"docs/operations.md",
		"docs/openapi-workflow.md",
	} {
		if !strings.Contains(readme+"\n"+index, required) {
			t.Fatalf("README.md or docs/README.md missing feature production guide %s", required)
		}
	}
	for _, required := range []string{
		"## Security-Sensitive Production Settings",
		"Auth",
		"Tenant",
		"Admin endpoints",
		"Pprof",
		"Metrics",
		"Idempotency",
		"Webhooks",
		"OpenAPI validation",
	} {
		if !strings.Contains(security, required) {
			t.Fatalf("docs/security.md missing security-sensitive production setting %q", required)
		}
	}
	for _, required := range []string{
		"## Storage Contract",
		"## TTL and Locking",
		"## Redis Example",
		"## Postgres Example",
		"`middleware/idempotency.Options.RequireKey`",
		"`idempotency.TenantScopedStorageKeyFunc()`",
		"request hash",
		"conflict",
		"replay",
		"tenant A cannot replay tenant B's key",
	} {
		if !strings.Contains(idempotency, required) {
			t.Fatalf("docs/idempotency.md missing production guidance %q", required)
		}
	}
	for _, required := range []string{
		"## API Keys",
		"## JWT, OIDC, and Clerk",
		"## Tenant and Role Authorization",
		"JWK rotation",
		"Clock skew",
		"wrong audience",
		"wrong tenant",
		"dev bypass disabled in production",
	} {
		if !strings.Contains(auth, required) {
			t.Fatalf("docs/auth.md missing production guidance %q", required)
		}
	}
	for _, required := range []string{
		"## Endpoint Split",
		"## Safe Mounting",
		"## Network Policy",
		"## Fail-Closed Checks",
		"`pprof.RegisterAdminRoutes`",
		"`bootstrap.MountSystemEndpointsToWithAdmin`",
		"public `/livez`",
		"public `/readyz`",
	} {
		if !strings.Contains(operations, required) {
			t.Fatalf("docs/operations.md missing operations guidance %q", required)
		}
	}
	for _, required := range []string{
		"## Route Metadata",
		"## Local Workflow",
		"## Runtime Validation",
		"## Drift Handling",
		"`routecontracts`",
		"`specs`",
		"`x-api-toolkit-streaming`",
		"`openapi.ResponseValidationOptions.ShouldValidate`",
		"api-toolkit contracts lint",
		"api-toolkit contracts diff",
	} {
		if !strings.Contains(openapiWorkflow, required) {
			t.Fatalf("docs/openapi-workflow.md missing workflow guidance %q", required)
		}
	}
}

func TestSupportConfigurationAdapterAndObservabilityGuidesCoverRuntimeCompleteness(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	readme := readText(t, filepath.Join(repoRoot, "README.md"))
	index := readText(t, filepath.Join(repoRoot, "docs", "README.md"))
	scaffold := readText(t, filepath.Join(repoRoot, "docs", "scaffold-support.md"))
	adapter := readText(t, filepath.Join(repoRoot, "docs", "adapter-maturity.md"))
	config := readText(t, filepath.Join(repoRoot, "docs", "configuration.md"))
	observability := readText(t, filepath.Join(repoRoot, "docs", "observability.md"))

	for _, required := range []string{
		"docs/scaffold-support.md",
		"docs/adapter-maturity.md",
		"docs/configuration.md",
		"docs/observability.md",
	} {
		if !strings.Contains(readme+"\n"+index, required) {
			t.Fatalf("README.md or docs/README.md missing runtime completeness guide %s", required)
		}
	}
	for _, required := range []string{
		"Generated services are app-owned generated code",
		"## Support Matrix",
		"## What Breaks On Regeneration",
		"## Migration Expectations",
		"temporary generation directory",
		"Generated CI and Makefile",
	} {
		if !strings.Contains(scaffold, required) {
			t.Fatalf("docs/scaffold-support.md missing scaffold support guidance %q", required)
		}
	}
	for _, required := range []string{
		"## Maturity Matrix",
		"Postgres",
		"Redis",
		"Stripe",
		"Resend",
		"Clerk and OIDC",
		"OpenTelemetry",
		"CORS",
		"Validation",
		"supported-adapter",
		"experimental",
	} {
		if !strings.Contains(adapter, required) {
			t.Fatalf("docs/adapter-maturity.md missing adapter maturity guidance %q", required)
		}
	}
	for _, required := range []string{
		"## Production Variables",
		"## Unsafe Development Defaults",
		"## Startup Validation",
		"`DATABASE_URL`",
		"`REDIS_ADDR`",
		"`API_KEY_PEPPER`",
		"`WEBHOOK_SECRET_KEY`",
		"`OTEL_EXPORTER_OTLP_ENDPOINT`",
		"Production startup should fail closed",
	} {
		if !strings.Contains(config, required) {
			t.Fatalf("docs/configuration.md missing runtime configuration guidance %q", required)
		}
	}
	for _, required := range []string{
		"## Metrics",
		"## Logs",
		"## Traces",
		"## Correlation IDs",
		"## Dashboards and Alerts",
		"route patterns, not raw paths",
		"tenant IDs, user IDs, API keys",
		"`OTEL_TRACING_ENABLED=true`",
		"Keep span attributes low-cardinality",
	} {
		if !strings.Contains(observability, required) {
			t.Fatalf("docs/observability.md missing observability guidance %q", required)
		}
	}
}

func TestExampleCatalogCoversExampleDirectories(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	examplesRoot := filepath.Join(repoRoot, "contrib", "examples")
	catalog := readText(t, filepath.Join(examplesRoot, "README.md"))
	for _, required := range []string{
		"| Example | Task | Command | Required environment | Endpoint | Expected result | Safety caveat |",
		"Task",
		"Command",
		"Expected result",
	} {
		if !strings.Contains(catalog, required) {
			t.Fatalf("contrib/examples/README.md missing catalog metadata %q", required)
		}
	}

	entries, err := os.ReadDir(examplesRoot)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := entry.Name()
		for _, required := range []string{
			"`" + name + "`",
			"go run ./examples/" + name,
		} {
			if !strings.Contains(catalog, required) {
				t.Fatalf("contrib/examples/README.md missing %s metadata %q", name, required)
			}
		}
	}
}

func TestPublicPackageDocsAvoidPlaceholderUtilities(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	placeholder := regexp.MustCompile(`(?m)^// Package ([A-Za-z0-9_]+) provides ([A-Za-z0-9_]+) utilities\.$`)
	var violations []string

	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".ci-result", ".audits", "audit", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) != "doc.go" {
			return nil
		}
		for _, match := range placeholder.FindAllStringSubmatch(readText(t, path), -1) {
			if match[1] == match[2] {
				violations = append(violations, slashRel(repoRoot, path))
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan package docs: %v", err)
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("placeholder package docs remain:\n%s", strings.Join(violations, "\n"))
	}
}

var tokenPattern = regexp.MustCompile(`github\.com/aatuh/[A-Za-z0-9./_-]+`)
var packageLiteralPattern = regexp.MustCompile("`(" + regexp.QuoteMeta(rootModulePath) + `/v3[^` + "`" + `]*)` + "`")
var bashPackageLiteralPattern = regexp.MustCompile(`"` + regexp.QuoteMeta(rootModulePath) + `/v3[^"]*"`)

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

func docsQualityMarkdownFiles(t *testing.T, repoRoot string) []string {
	t.Helper()

	files := []string{
		filepath.Join(repoRoot, "README.md"),
		filepath.Join(repoRoot, "VERSIONING.md"),
		filepath.Join(repoRoot, "SECURITY.md"),
		filepath.Join(repoRoot, "PANIC_POLICY.md"),
	}
	files = append(files, markdownFilesUnder(t, filepath.Join(repoRoot, "docs"))...)
	files = append(files, markdownFilesUnder(t, filepath.Join(repoRoot, "contrib", "examples"))...)
	return uniqueSorted(files)
}

func externalMarkdownTarget(target string) bool {
	lower := strings.ToLower(target)
	return strings.Contains(lower, "://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:")
}

func makefileTargetSet(makefile string) map[string]bool {
	targets := map[string]bool{}
	pattern := regexp.MustCompile(`^([A-Za-z0-9_.-]+):(?:\s|$)`)
	for _, line := range strings.Split(makefile, "\n") {
		match := pattern.FindStringSubmatch(line)
		if len(match) == 2 {
			targets[match[1]] = true
		}
	}
	return targets
}

func likelyProseAfterMake(target string) bool {
	switch target {
	case "a", "an", "contrib", "it", "safe", "sure", "that", "the", "this":
		return true
	default:
		return false
	}
}

func markdownAnchors(markdown string) map[string]bool {
	anchors := map[string]bool{}
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if heading == "" {
			continue
		}
		anchors[markdownAnchorSlug(heading)] = true
	}
	return anchors
}

func markdownAnchorSlug(heading string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func forbiddenModuleToken(token string) bool {
	if strings.HasPrefix(token, rootModulePath+"-contrib") {
		return true
	}
	if token == rootModulePath {
		return true
	}
	if token == rootModulePath+"/" || strings.HasPrefix(token, rootModulePath+"/.") {
		return false
	}
	if token == rootModulePath+"/v3" {
		return false
	}
	if strings.HasPrefix(token, rootModulePath+"/v3/") {
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

func runGoCmd(dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd.CombinedOutput()
}

type releaseEvidenceGitStateSummary struct {
	Commit              string `json:"commit"`
	Status              string `json:"status"`
	PublicationEligible bool   `json:"publication_eligible"`
	ProvenancePolicy    struct {
		Mode                      string `json:"mode"`
		AllowDirtyReleaseEvidence bool   `json:"allow_dirty_release_evidence"`
		Status                    string `json:"status"`
		Message                   string `json:"message"`
	} `json:"provenance_policy"`
	GitState struct {
		Commit         string  `json:"commit"`
		Branch         *string `json:"branch"`
		Detached       bool    `json:"detached"`
		Dirty          bool    `json:"dirty"`
		StagedCount    int     `json:"staged_count"`
		UnstagedCount  int     `json:"unstaged_count"`
		UntrackedCount int     `json:"untracked_count"`
		DeletedCount   int     `json:"deleted_count"`
	} `json:"git_state"`
}

func releaseEvidenceSummaryForRepo(t *testing.T, dir, scriptPath string) releaseEvidenceGitStateSummary {
	t.Helper()

	return releaseEvidenceSummaryForRepoWithEnv(t, dir, scriptPath)
}

func releaseEvidenceSummaryForRepoWithEnv(t *testing.T, dir, scriptPath string, extraEnv ...string) releaseEvidenceGitStateSummary {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), "bash", scriptPath)
	cmd.Dir = dir
	cmd.Env = releaseEvidenceEnv(extraEnv...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("release evidence summary in %s failed: %v", dir, err)
	}

	var summary releaseEvidenceGitStateSummary
	if err := json.Unmarshal(out, &summary); err != nil {
		t.Fatalf("release evidence summary in %s is not valid JSON:\n%s\nerror: %v", dir, out, err)
	}
	if summary.Commit == "" || summary.GitState.Commit != summary.Commit {
		t.Fatalf("release evidence git_state commit = %q, want %q", summary.GitState.Commit, summary.Commit)
	}
	return summary
}

func releaseEvidenceFailureForRepo(t *testing.T, dir, scriptPath string) releaseEvidenceGitStateSummary {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), "bash", scriptPath)
	cmd.Dir = dir
	cmd.Env = releaseEvidenceEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("release evidence summary in %s passed, want dirty-tree failure:\n%s", dir, out)
	}

	var summary releaseEvidenceGitStateSummary
	if unmarshalErr := json.Unmarshal(out, &summary); unmarshalErr != nil {
		t.Fatalf("dirty release evidence failure in %s did not emit JSON:\n%s\nerror: %v", dir, out, unmarshalErr)
	}
	return summary
}

func releaseEvidenceEnv(extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+2+len(extra))
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "ALLOW_DIRTY_RELEASE_EVIDENCE=") {
			continue
		}
		if strings.HasPrefix(value, "FULL_PROFILE_INTEGRATION_CHECK_STATUS=") ||
			strings.HasPrefix(value, "FULL_PROFILE_INTEGRATION_CHECK_LOG_PATH=") {
			continue
		}
		env = append(env, value)
	}
	env = append(env, "API_BASE_REF=v2.1.0", "GOTOOLCHAIN=local", "FULL_PROFILE_INTEGRATION_CHECK_STATUS=not_run_opt_in")
	env = append(env, extra...)
	return env
}

func newTempGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	writeTempFile(t, dir, "tracked.txt", "initial\n")
	writeTempFile(t, dir, "deleted.txt", "delete me\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	return dir
}

func writeTempFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=api-toolkit-tests",
		"GIT_AUTHOR_EMAIL=api-toolkit-tests@example.com",
		"GIT_COMMITTER_NAME=api-toolkit-tests",
		"GIT_COMMITTER_EMAIL=api-toolkit-tests@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s failed:\n%s\nerror: %v", args, dir, out, err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func generatedUpgradeDefaultRefs(t *testing.T, script string) []string {
	t.Helper()

	re := regexp.MustCompile(`(?m)^else\s*\n\s*generator_refs="([^"]+)"\s*\nfi$`)
	matches := re.FindAllStringSubmatch(script, -1)
	if len(matches) != 1 {
		t.Fatalf("scripts/generated_upgrade_compat_check.sh must have one default generator_refs assignment, found %d", len(matches))
	}
	refs := strings.Fields(matches[0][1])
	if len(refs) == 0 {
		t.Fatal("scripts/generated_upgrade_compat_check.sh default generator_refs assignment is empty")
	}
	return refs
}

func documentedGeneratedUpgradeRefs(t *testing.T, name, text string, re *regexp.Regexp) []string {
	t.Helper()

	match := re.FindStringSubmatch(text)
	if match == nil {
		t.Fatalf("%s missing generated-upgrade default refs in expected wording", name)
	}
	var refs []string
	for _, value := range match[1:] {
		refs = append(refs, strings.Fields(value)...)
	}
	return refs
}

func loadTSVRecords(t *testing.T, path string) []map[string]string {
	t.Helper()

	content := strings.TrimSpace(readText(t, path))
	if content == "" {
		t.Fatalf("%s is empty", path)
	}
	lines := strings.Split(content, "\n")
	headers := strings.Split(lines[0], "\t")
	var records []map[string]string
	for lineNumber, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != len(headers) {
			t.Fatalf("%s line %d has %d fields, want %d: %q", path, lineNumber+2, len(fields), len(headers), line)
		}
		record := make(map[string]string, len(headers))
		for i, header := range headers {
			record[header] = fields[i]
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		t.Fatalf("%s has no records", path)
	}
	return records
}

func recordsByField(t *testing.T, records []map[string]string, field string) map[string]map[string]string {
	t.Helper()

	out := make(map[string]map[string]string, len(records))
	for _, record := range records {
		value := record[field]
		if value == "" {
			t.Fatalf("TSV record missing field %s: %+v", field, record)
		}
		if _, exists := out[value]; exists {
			t.Fatalf("duplicate TSV %s value %s", field, value)
		}
		out[value] = record
	}
	return out
}

func releaseBlockerStatusForAPIStatus(apiStatus string) string {
	switch apiStatus {
	case "stable", "compatibility-only":
		return "release-blocking-stable"
	case "supported-adapter":
		return "release-blocking-supported-adapter"
	case "wrapper-only":
		return "touch-scoped-wrapper"
	case "experimental":
		return "non-blocking-experimental"
	case "tooling":
		return "touch-scoped-tooling"
	case "generated":
		return "generated-evidence"
	case "example-only":
		return "example-smoke"
	case "test-only":
		return "test-support"
	case "excluded":
		return "repo-governance"
	default:
		return "release-blocker-review-required"
	}
}

func requireISODate(t *testing.T, field, value string) {
	t.Helper()

	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(value) {
		t.Fatalf("%s = %q, want YYYY-MM-DD", field, value)
	}
}

func markdownTableRowContaining(t *testing.T, section, needle string) string {
	t.Helper()

	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "|") && strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("markdown table missing row containing %q", needle)
	return ""
}

func markdownTableColumns(row string) []string {
	trimmed := strings.Trim(row, "|")
	parts := strings.Split(trimmed, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
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

func stableRootPackagesFromClassification(t *testing.T, repoRoot string) []string {
	t.Helper()

	classes := loadPackageClassifications(t, repoRoot)
	var packages []string
	for _, cls := range classes {
		if !inModule(cls.ImportPath, rootModulePath+"/v3") {
			continue
		}
		if cls.APIStatus == "stable" || cls.APIStatus == "compatibility-only" {
			packages = append(packages, cls.ImportPath)
		}
	}
	if len(packages) == 0 {
		t.Fatal("docs/package-classification.tsv has no stable root packages")
	}
	return uniqueSorted(packages)
}

type packageClassification struct {
	ImportPath string
	APIStatus  string
	TestStatus string
	Notes      string
}

type supportedAdapterContract struct {
	ImportPath string
	Contract   string
	Evidence   string
}

type supportedAdapterRealism struct {
	ImportPath              string
	DefaultPREvidence       string
	ScheduledManualEvidence string
	RealismStatus           string
}

func loadPackageClassifications(t *testing.T, repoRoot string) map[string]packageClassification {
	t.Helper()

	content := readText(t, filepath.Join(repoRoot, "docs", "package-classification.tsv"))
	classes := make(map[string]packageClassification)
	allowedTestStatuses := map[string]bool{
		"direct-tests":         true,
		"wrapper-smoke-tested": true,
		"test-support":         true,
		"example-only":         true,
		"generated":            true,
		"tooling":              true,
		"excluded":             true,
		"needs-tests":          true,
	}
	allowedAPIStatuses := map[string]bool{
		"stable":             true,
		"compatibility-only": true,
		"supported-adapter":  true,
		"experimental":       true,
		"wrapper-only":       true,
		"test-only":          true,
		"example-only":       true,
		"generated":          true,
		"tooling":            true,
		"excluded":           true,
	}

	for lineNo, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(raw, "\t")
		if len(cols) != 4 {
			t.Fatalf("docs/package-classification.tsv:%d: expected 4 tab-separated columns, got %d", lineNo+1, len(cols))
		}
		cls := packageClassification{
			ImportPath: strings.TrimSpace(cols[0]),
			APIStatus:  strings.TrimSpace(cols[1]),
			TestStatus: strings.TrimSpace(cols[2]),
			Notes:      strings.TrimSpace(cols[3]),
		}
		if cls.ImportPath == "" || cls.APIStatus == "" || cls.TestStatus == "" || cls.Notes == "" {
			t.Fatalf("docs/package-classification.tsv:%d: empty classification field", lineNo+1)
		}
		if !allowedTestStatuses[cls.TestStatus] {
			t.Fatalf("docs/package-classification.tsv:%d: unknown test_status %q", lineNo+1, cls.TestStatus)
		}
		if !allowedAPIStatuses[cls.APIStatus] {
			t.Fatalf("docs/package-classification.tsv:%d: unknown api_status %q", lineNo+1, cls.APIStatus)
		}
		if _, exists := classes[cls.ImportPath]; exists {
			t.Fatalf("docs/package-classification.tsv:%d: duplicate import path %s", lineNo+1, cls.ImportPath)
		}
		classes[cls.ImportPath] = cls
	}
	if len(classes) == 0 {
		t.Fatal("docs/package-classification.tsv has no classifications")
	}
	return classes
}

func loadSupportedAdapterContracts(t *testing.T, repoRoot string) map[string]supportedAdapterContract {
	t.Helper()

	content := readText(t, filepath.Join(repoRoot, "docs", "supported-adapter-contracts.tsv"))
	contracts := make(map[string]supportedAdapterContract)
	for lineNo, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(raw, "\t")
		if len(cols) != 3 {
			t.Fatalf("docs/supported-adapter-contracts.tsv:%d: expected 3 tab-separated columns, got %d", lineNo+1, len(cols))
		}
		contract := supportedAdapterContract{
			ImportPath: strings.TrimSpace(cols[0]),
			Contract:   strings.TrimSpace(cols[1]),
			Evidence:   strings.TrimSpace(cols[2]),
		}
		if contract.ImportPath == "" || contract.Contract == "" || contract.Evidence == "" {
			t.Fatalf("docs/supported-adapter-contracts.tsv:%d: empty contract field", lineNo+1)
		}
		if _, exists := contracts[contract.ImportPath]; exists {
			t.Fatalf("docs/supported-adapter-contracts.tsv:%d: duplicate import path %s", lineNo+1, contract.ImportPath)
		}
		contracts[contract.ImportPath] = contract
	}
	if len(contracts) == 0 {
		t.Fatal("docs/supported-adapter-contracts.tsv has no contracts")
	}
	return contracts
}

func loadSupportedAdapterRealism(t *testing.T, repoRoot string) map[string]supportedAdapterRealism {
	t.Helper()

	content := readText(t, filepath.Join(repoRoot, "docs", "supported-adapter-test-realism.tsv"))
	rows := make(map[string]supportedAdapterRealism)
	for lineNo, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(raw, "\t")
		if len(cols) != 4 {
			t.Fatalf("docs/supported-adapter-test-realism.tsv:%d: expected 4 tab-separated columns, got %d", lineNo+1, len(cols))
		}
		row := supportedAdapterRealism{
			ImportPath:              strings.TrimSpace(cols[0]),
			DefaultPREvidence:       strings.TrimSpace(cols[1]),
			ScheduledManualEvidence: strings.TrimSpace(cols[2]),
			RealismStatus:           strings.TrimSpace(cols[3]),
		}
		if row.ImportPath == "" || row.DefaultPREvidence == "" || row.ScheduledManualEvidence == "" || row.RealismStatus == "" {
			t.Fatalf("docs/supported-adapter-test-realism.tsv:%d: empty realism field", lineNo+1)
		}
		if _, exists := rows[row.ImportPath]; exists {
			t.Fatalf("docs/supported-adapter-test-realism.tsv:%d: duplicate import path %s", lineNo+1, row.ImportPath)
		}
		rows[row.ImportPath] = row
	}
	if len(rows) == 0 {
		t.Fatal("docs/supported-adapter-test-realism.tsv has no rows")
	}
	return rows
}

func moduleGoDirective(t *testing.T, path string) string {
	t.Helper()

	for _, line := range strings.Split(readText(t, path), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	t.Fatalf("%s missing go directive", path)
	return ""
}

func workflowGoVersions(content string) []string {
	pattern := regexp.MustCompile(`go-version:\s*(?:\$\{\{\s*matrix\.go-version\s*\}\}|['"]?([0-9]+\.[0-9]+\.x)['"]?)`)
	matrixPattern := regexp.MustCompile(`go-version:\s*\[([^\]]+)\]`)
	matrixVersion := ""
	if match := matrixPattern.FindStringSubmatch(content); len(match) == 2 {
		for _, value := range strings.Split(match[1], ",") {
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if value != "" {
				matrixVersion = value
				break
			}
		}
	}
	var versions []string
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		version := match[1]
		if version == "" {
			version = matrixVersion
		}
		if version != "" {
			versions = append(versions, version)
		}
	}
	return versions
}

func contribDriftManifestPackages(t *testing.T, repoRoot string) []string {
	t.Helper()

	content := readText(t, filepath.Join(repoRoot, "docs", "contrib-api-drift-packages.txt"))
	var packages []string
	seen := map[string]bool{}
	for lineNo, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "#") {
			line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		}
		if line == "" {
			continue
		}
		if seen[line] {
			t.Fatalf("docs/contrib-api-drift-packages.txt:%d duplicate package %s", lineNo+1, line)
		}
		seen[line] = true
		packages = append(packages, line)
	}
	sort.Strings(packages)
	return packages
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsAny(value string, wants []string) bool {
	for _, want := range wants {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

type listedPackage struct {
	ImportPath      string
	DirectTestFiles int
}

func listedGoPackages(t *testing.T, dir string) []listedPackage {
	t.Helper()

	out, err := runGoCmd(dir, "list", "-f", "{{.ImportPath}}\t{{len .TestGoFiles}}\t{{len .XTestGoFiles}}", "./...")
	if err != nil {
		t.Fatalf("go list packages in %s:\n%s\nerror: %v", dir, out, err)
	}
	return parseListedGoPackagesOutput(t, out)
}

func parseListedGoPackagesOutput(t *testing.T, out []byte) []listedPackage {
	t.Helper()

	var packages []listedPackage
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "go: downloading ") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 3 {
			t.Fatalf("unexpected go list output line %q", line)
		}
		testFiles, err := strconv.Atoi(cols[1])
		if err != nil {
			t.Fatalf("parse TestGoFiles count from %q: %v", line, err)
		}
		externalTestFiles, err := strconv.Atoi(cols[2])
		if err != nil {
			t.Fatalf("parse XTestGoFiles count from %q: %v", line, err)
		}
		packages = append(packages, listedPackage{
			ImportPath:      cols[0],
			DirectTestFiles: testFiles + externalTestFiles,
		})
	}
	return packages
}

func assertClassifiedPackages(t *testing.T, name string, packages []listedPackage, classes map[string]packageClassification, modulePath string, allowedAPIStatuses map[string]bool) {
	t.Helper()

	seen := make(map[string]struct{}, len(packages))
	for _, pkg := range packages {
		seen[pkg.ImportPath] = struct{}{}
		cls, ok := classes[pkg.ImportPath]
		if !ok {
			t.Fatalf("%s package %s is missing from docs/package-classification.tsv", name, pkg.ImportPath)
		}
		if !allowedAPIStatuses[cls.APIStatus] {
			t.Fatalf("%s package %s has invalid api_status %q", name, pkg.ImportPath, cls.APIStatus)
		}
	}

	for importPath := range classes {
		if !inModule(importPath, modulePath) {
			continue
		}
		if _, ok := seen[importPath]; !ok {
			t.Fatalf("docs/package-classification.tsv contains stale %s package %s", name, importPath)
		}
	}
}

func inModule(importPath, modulePath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

func contribPackageDir(repoRoot, importPath string) string {
	rel := strings.TrimPrefix(importPath, contribModulePath)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return filepath.Join(repoRoot, "contrib")
	}
	return filepath.Join(repoRoot, "contrib", filepath.FromSlash(rel))
}

func classifiedPackageDir(repoRoot, importPath string) string {
	rootV3Module := rootModulePath + "/v3"
	switch {
	case importPath == rootV3Module:
		return repoRoot
	case strings.HasPrefix(importPath, rootV3Module+"/"):
		rel := strings.TrimPrefix(importPath, rootV3Module+"/")
		return filepath.Join(repoRoot, filepath.FromSlash(rel))
	case importPath == contribModulePath || strings.HasPrefix(importPath, contribModulePath+"/"):
		return contribPackageDir(repoRoot, importPath)
	default:
		return ""
	}
}

func packageDocCommentText(t *testing.T, path string) string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if file.Doc == nil || strings.TrimSpace(file.Doc.Text()) == "" {
		t.Fatalf("%s missing package doc comment", path)
	}
	return file.Doc.Text()
}

type optionStructRef struct {
	ImportPath string
	TypeName   string
}

func stableOptionStructs(t *testing.T, repoRoot string) []optionStructRef {
	t.Helper()

	classes := loadPackageClassifications(t, repoRoot)
	var refs []optionStructRef
	for _, cls := range classes {
		if cls.APIStatus != "stable" && cls.APIStatus != "compatibility-only" {
			continue
		}
		dir := classifiedPackageDir(repoRoot, cls.ImportPath)
		if dir == "" {
			t.Fatalf("%s has unsupported import path for options audit", cls.ImportPath)
		}
		for _, typeName := range exportedOptionsStructsInDir(t, dir) {
			refs = append(refs, optionStructRef{ImportPath: cls.ImportPath, TypeName: typeName})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ImportPath == refs[j].ImportPath {
			return refs[i].TypeName < refs[j].TypeName
		}
		return refs[i].ImportPath < refs[j].ImportPath
	})
	return refs
}

func exportedOptionsStructsInDir(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir %s: %v", dir, err)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil || !typeSpec.Name.IsExported() {
					continue
				}
				typeName := typeSpec.Name.Name
				if typeName != "Options" && !strings.HasSuffix(typeName, "Options") {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); !ok {
					continue
				}
				seen[typeName] = true
			}
		}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func optionsAuditRows(t *testing.T, content string) map[string]map[string]string {
	t.Helper()

	rows := map[string]map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `"+rootModulePath+"/v3") {
			continue
		}
		cols := markdownTableColumns(line)
		if len(cols) != 6 {
			t.Fatalf("docs/options-structs.md row has %d columns, want 6: %s", len(cols), line)
		}
		importPath := trimMarkdownCode(cols[0])
		typeName := trimMarkdownCode(cols[1])
		key := optionStructKey(importPath, typeName)
		if rows[key] != nil {
			t.Fatalf("docs/options-structs.md has duplicate row for %s", key)
		}
		rows[key] = map[string]string{
			"defaults":   cols[2],
			"validation": cols[3],
			"zero":       cols[4],
			"example":    cols[5],
		}
	}
	if len(rows) == 0 {
		t.Fatal("docs/options-structs.md has no options rows")
	}
	return rows
}

func trimMarkdownCode(value string) string {
	return strings.Trim(strings.TrimSpace(value), "`")
}

func optionStructKey(importPath, typeName string) string {
	return importPath + "\t" + typeName
}

type packageVarRef struct {
	ImportPath string
	Name       string
}

func stablePackageVarRefs(t *testing.T, repoRoot string) []packageVarRef {
	t.Helper()

	classes := loadPackageClassifications(t, repoRoot)
	var refs []packageVarRef
	for _, cls := range classes {
		if cls.APIStatus != "stable" && cls.APIStatus != "compatibility-only" {
			continue
		}
		dir := classifiedPackageDir(repoRoot, cls.ImportPath)
		if dir == "" {
			t.Fatalf("%s has unsupported import path for global-state audit", cls.ImportPath)
		}
		for _, name := range packageVarNamesInDir(t, dir) {
			refs = append(refs, packageVarRef{ImportPath: cls.ImportPath, Name: name})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ImportPath == refs[j].ImportPath {
			return refs[i].Name < refs[j].Name
		}
		return refs[i].ImportPath < refs[j].ImportPath
	})
	return refs
}

func packageVarNamesInDir(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir %s: %v", dir, err)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					if name != nil {
						seen[name.Name] = true
					}
				}
			}
		}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func globalStateAuditRows(t *testing.T, content string) map[string]map[string]string {
	t.Helper()

	rows := map[string]map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `"+rootModulePath+"/v3") {
			continue
		}
		cols := markdownTableColumns(line)
		if len(cols) != 5 {
			t.Fatalf("docs/global-state-audit.md row has %d columns, want 5: %s", len(cols), line)
		}
		importPath := trimMarkdownCode(cols[0])
		name := trimMarkdownCode(cols[1])
		key := packageVarKey(importPath, name)
		if rows[key] != nil {
			t.Fatalf("docs/global-state-audit.md has duplicate row for %s", key)
		}
		rows[key] = map[string]string{
			"classification": strings.TrimSpace(cols[2]),
			"mutation":       cols[3],
			"concurrency":    cols[4],
		}
	}
	if len(rows) == 0 {
		t.Fatal("docs/global-state-audit.md has no audited global-state rows")
	}
	return rows
}

func validGlobalStateClassification(value string) bool {
	switch value {
	case "sentinel-error",
		"immutable-default",
		"private-default-registry",
		"precompiled-template",
		"precompiled-regexp",
		"embedded-fs",
		"synchronized-registry",
		"test-hook",
		"atomic-counter":
		return true
	default:
		return false
	}
}

func packageVarKey(importPath, name string) string {
	return importPath + "\t" + name
}

func makeTargetRecipe(t *testing.T, makefile, target string) string {
	t.Helper()

	lines := strings.Split(makefile, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, target+":") {
			continue
		}
		var recipe []string
		for _, bodyLine := range lines[i+1:] {
			if strings.HasPrefix(bodyLine, "\t") {
				recipe = append(recipe, bodyLine)
				continue
			}
			if strings.TrimSpace(bodyLine) == "" {
				if len(recipe) > 0 {
					break
				}
				continue
			}
			break
		}
		return strings.Join(recipe, "\n")
	}
	t.Fatalf("Makefile target %s not found", target)
	return ""
}

func makeSubtargets(t *testing.T, makefile, target string) []string {
	t.Helper()

	recipe := makeTargetRecipe(t, makefile, target)
	pattern := regexp.MustCompile(`\$\(MAKE\)\s+([A-Za-z0-9_.-]+)`)
	matches := pattern.FindAllStringSubmatch(recipe, -1)
	var targets []string
	for _, match := range matches {
		targets = append(targets, match[1])
	}
	if len(targets) == 0 {
		t.Fatalf("Makefile target %s has no $(MAKE) subtargets", target)
	}
	return targets
}

func exportedIdentifiersInFile(t *testing.T, path string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Name.IsExported() {
				names = append(names, decl.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.IsExported() {
						names = append(names, spec.Name.Name)
					}
					names = append(names, exportedMembers(spec.Type)...)
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.IsExported() {
							names = append(names, name.Name)
						}
					}
				}
			}
		}
	}
	sort.Strings(names)
	return names
}

func exportedMembers(expr ast.Expr) []string {
	var names []string
	switch typ := expr.(type) {
	case *ast.StructType:
		for _, field := range typ.Fields.List {
			for _, name := range field.Names {
				if name.IsExported() {
					names = append(names, name.Name)
				}
			}
		}
	case *ast.InterfaceType:
		for _, method := range typ.Methods.List {
			for _, name := range method.Names {
				if name.IsExported() {
					names = append(names, name.Name)
				}
			}
		}
	}
	return names
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

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func scanGoSourceViolations(t *testing.T, root string, check func(fset *token.FileSet, path string, file *ast.File) []string) []string {
	t.Helper()

	fset := token.NewFileSet()
	var violations []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".ci-result", ".audits", "audit", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		violations = append(violations, check(fset, path, file)...)
		return nil
	})
	if err != nil {
		t.Fatalf("scan Go source under %s: %v", root, err)
	}
	sort.Strings(violations)
	return violations
}

func importAliases(file *ast.File, importPath string) (map[string]bool, bool) {
	aliases := map[string]bool{}
	dotImport := false
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		if imp.Name == nil {
			parts := strings.Split(path, "/")
			aliases[parts[len(parts)-1]] = true
			continue
		}
		switch imp.Name.Name {
		case ".":
			dotImport = true
		case "_":
		default:
			aliases[imp.Name.Name] = true
		}
	}
	return aliases, dotImport
}

func allowedDeprecatedBillingSource(repoRoot, path string) bool {
	rel := slashRel(repoRoot, path)
	return rel == "ports/billing.go" || strings.HasPrefix(rel, "compat/billing/")
}

func allowedDatabaseStatsSource(repoRoot, path string) bool {
	rel := slashRel(repoRoot, path)
	return rel == "ports/database.go" ||
		strings.HasPrefix(rel, "compat/") ||
		strings.HasPrefix(rel, "contrib/adapters/") ||
		strings.HasPrefix(rel, "contrib/integrations/")
}

func sourceViolation(repoRoot, path string, fset *token.FileSet, pos token.Pos, message string) string {
	position := fset.Position(pos)
	rel := slashRel(repoRoot, path)
	return rel + ":" + strconv.Itoa(position.Line) + ": " + message
}

func slashRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
