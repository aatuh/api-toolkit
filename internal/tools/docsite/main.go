package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const (
	rootModulePath    = "github.com/aatuh/api-toolkit/v4"
	contribModulePath = "github.com/aatuh/api-toolkit/contrib/v4"
	githubBlobBase    = "https://github.com/aatuh/api-toolkit/blob/master"
)

type config struct {
	Root  string
	Out   string
	Check bool
}

type siteData struct {
	GeneratedBy   string
	Packages      []packageRow
	Documents     []documentRow
	Examples      []exampleRow
	StableCount   int
	CompatCount   int
	ContribCount  int
	SearchEntries int
}

type packageRow struct {
	ImportPath string `json:"import_path"`
	APIStatus  string `json:"api_status"`
	TestStatus string `json:"test_status"`
	Notes      string `json:"notes"`
	PkgGoDev   string `json:"pkg_go_dev"`
	ExampleURL string `json:"example_url,omitempty"`
}

type documentRow struct {
	Title    string
	Category string
	Path     string
	URL      string
	Summary  string
	Text     string
}

type exampleRow struct {
	Title      string `json:"title"`
	ImportPath string `json:"import_path"`
	Path       string `json:"path"`
	URL        string `json:"url"`
}

type searchEntry struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	URL      string `json:"url"`
	Text     string `json:"text"`
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.Root, "root", ".", "repository root")
	flag.StringVar(&cfg.Out, "out", filepath.Join("docs", "site"), "generated docs site output directory")
	flag.BoolVar(&cfg.Check, "check", false, "verify generated site is current")
	flag.Parse()

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "docsite:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	root := filepath.Clean(cfg.Root)
	out := cfg.Out
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	files, err := renderSite(root)
	if err != nil {
		return err
	}
	if cfg.Check {
		return checkFiles(out, files)
	}
	return writeFiles(out, files)
}

func renderSite(root string) (map[string][]byte, error) {
	packages, err := loadPackageRows(root)
	if err != nil {
		return nil, err
	}
	documents, err := loadDocumentRows(root)
	if err != nil {
		return nil, err
	}
	examples := exampleRowsFromPackages(packages)
	entries := searchEntries(documents, packages, examples)
	searchJSON, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal search index: %w", err)
	}
	searchJSON = append(searchJSON, '\n')

	data := siteData{
		GeneratedBy:   "internal/tools/docsite",
		Packages:      packages,
		Documents:     documents,
		Examples:      examples,
		StableCount:   countStatus(packages, "stable"),
		CompatCount:   countStatus(packages, "compatibility-only"),
		ContribCount:  countContrib(packages),
		SearchEntries: len(entries),
	}
	var index bytes.Buffer
	if err := indexTemplate.Execute(&index, data); err != nil {
		return nil, fmt.Errorf("render index: %w", err)
	}
	return map[string][]byte{
		"index.html":        index.Bytes(),
		"site.css":          []byte(siteCSS),
		"site.js":           []byte(siteJS),
		"search-index.json": searchJSON,
	}, nil
}

func loadPackageRows(root string) ([]packageRow, error) {
	path := filepath.Join(root, "docs", "package-classification.tsv")
	// #nosec G304 -- path is fixed beneath the selected local repository root.
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read package classification: %w", err)
	}
	var rows []packageRow
	for lineNo, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 4 {
			return nil, fmt.Errorf("%s:%d expected at least 4 tab-separated columns", filepath.ToSlash(path), lineNo+1)
		}
		importPath := strings.TrimSpace(cols[0])
		row := packageRow{
			ImportPath: importPath,
			APIStatus:  strings.TrimSpace(cols[1]),
			TestStatus: strings.TrimSpace(cols[2]),
			Notes:      strings.TrimSpace(cols[3]),
			PkgGoDev:   "https://pkg.go.dev/" + importPath,
		}
		if example := firstExamplePath(root, importPath); example != "" {
			row.ExampleURL = sourceURL(example)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].APIStatus == rows[j].APIStatus {
			return rows[i].ImportPath < rows[j].ImportPath
		}
		return statusRank(rows[i].APIStatus) < statusRank(rows[j].APIStatus)
	})
	return rows, nil
}

func firstExamplePath(root, importPath string) string {
	rel, ok := importRel(importPath)
	if !ok {
		return ""
	}
	dir := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var candidates []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") || !strings.Contains(name, "example") {
			continue
		}
		candidates = append(candidates, filepath.ToSlash(filepath.Join(rel, name)))
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func importRel(importPath string) (string, bool) {
	switch {
	case importPath == rootModulePath:
		return ".", true
	case strings.HasPrefix(importPath, rootModulePath+"/"):
		return strings.TrimPrefix(importPath, rootModulePath+"/"), true
	case importPath == contribModulePath:
		return "contrib", true
	case strings.HasPrefix(importPath, contribModulePath+"/"):
		return filepath.ToSlash(filepath.Join("contrib", strings.TrimPrefix(importPath, contribModulePath+"/"))), true
	default:
		return "", false
	}
}

func loadDocumentRows(root string) ([]documentRow, error) {
	sources := []struct {
		Path     string
		Category string
	}{
		{"README.md", "overview"},
		{"docs/README.md", "navigation"},
		{"docs/api-reference.md", "api"},
		{"docs/core-package-guide.md", "api"},
		{"docs/security-review.md", "security"},
		{"docs/coverage-trend.md", "test-evidence"},
		{"docs/package-classification.md", "package-status"},
		{"docs/core-readiness.md", "package-status"},
		{"VERSIONING.md", "compatibility"},
		{"docs/deprecations.md", "compatibility"},
		{"docs/v3-compatibility-roadmap.md", "compatibility"},
		{"docs/downstream-compatibility.md", "compatibility"},
		{"docs/provenance.md", "release-evidence"},
		{"docs/reproducible-builds.md", "release-evidence"},
		{"docs/migration/v3.md", "migration"},
		{"docs/release-notes.md", "migration"},
	}
	rows := make([]documentRow, 0, len(sources))
	for _, source := range sources {
		path := filepath.Join(root, filepath.FromSlash(source.Path))
		// #nosec G304 -- source.Path comes from the fixed documentation-source manifest.
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", source.Path, err)
		}
		text := string(content)
		title := markdownTitle(text)
		if title == "" {
			title = source.Path
		}
		rows = append(rows, documentRow{
			Title:    title,
			Category: source.Category,
			Path:     source.Path,
			URL:      sourceURL(source.Path),
			Summary:  summarizeMarkdown(text),
			Text:     normalizeMarkdown(text),
		})
	}
	return rows, nil
}

func exampleRowsFromPackages(packages []packageRow) []exampleRow {
	var rows []exampleRow
	for _, pkg := range packages {
		if pkg.ExampleURL == "" {
			continue
		}
		rel := strings.TrimPrefix(pkg.ExampleURL, githubBlobBase+"/")
		rows = append(rows, exampleRow{
			Title:      "Example: " + pkg.ImportPath,
			ImportPath: pkg.ImportPath,
			Path:       rel,
			URL:        pkg.ExampleURL,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ImportPath < rows[j].ImportPath })
	return rows
}

func searchEntries(documents []documentRow, packages []packageRow, examples []exampleRow) []searchEntry {
	var entries []searchEntry
	for _, doc := range documents {
		entries = append(entries, searchEntry{
			Title:    doc.Title,
			Category: doc.Category,
			URL:      doc.URL,
			Text:     doc.Path + " " + doc.Summary + " " + doc.Text,
		})
	}
	for _, pkg := range packages {
		entries = append(entries, searchEntry{
			Title:    pkg.ImportPath,
			Category: "package-status",
			URL:      pkg.PkgGoDev,
			Text: strings.Join([]string{
				pkg.ImportPath,
				pkg.APIStatus,
				pkg.TestStatus,
				pkg.Notes,
				pkg.ExampleURL,
			}, " "),
		})
	}
	for _, example := range examples {
		entries = append(entries, searchEntry{
			Title:    example.Title,
			Category: "example",
			URL:      example.URL,
			Text:     example.ImportPath + " " + example.Path,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Category == entries[j].Category {
			return entries[i].Title < entries[j].Title
		}
		return entries[i].Category < entries[j].Category
	})
	return entries
}

func writeFiles(out string, files map[string][]byte) error {
	// #nosec G301 -- The generated static site is intentionally public.
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	for name, content := range files {
		path := filepath.Join(out, name)
		// #nosec G306 -- Generated static-site assets are intentionally public.
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", filepath.ToSlash(path), err)
		}
	}
	return nil
}

func checkFiles(out string, files map[string][]byte) error {
	var drift []string
	for name, want := range files {
		path := filepath.Join(out, name)
		// #nosec G304 -- name is a fixed key from the generated static-site asset map.
		got, err := os.ReadFile(path)
		if err != nil {
			drift = append(drift, fmt.Sprintf("%s: %v", filepath.ToSlash(path), err))
			continue
		}
		if !bytes.Equal(got, want) {
			drift = append(drift, filepath.ToSlash(path))
		}
	}
	if len(drift) > 0 {
		sort.Strings(drift)
		return fmt.Errorf("generated docs site is out of date; run `make docs-site`:\n - %s", strings.Join(drift, "\n - "))
	}
	return nil
}

func markdownTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func summarizeMarkdown(text string) string {
	normalized := normalizeMarkdown(text)
	words := strings.Fields(normalized)
	if len(words) > 28 {
		words = words[:28]
	}
	return strings.Join(words, " ")
}

func normalizeMarkdown(text string) string {
	var b strings.Builder
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" || strings.HasPrefix(trimmed, "| ---") {
			continue
		}
		for _, r := range trimmed {
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				b.WriteRune(r)
			case strings.ContainsRune("/._:-@[]`", r):
				b.WriteRune(r)
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteByte(' ')
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func countStatus(packages []packageRow, status string) int {
	count := 0
	for _, pkg := range packages {
		if pkg.APIStatus == status {
			count++
		}
	}
	return count
}

func countContrib(packages []packageRow) int {
	count := 0
	for _, pkg := range packages {
		if strings.HasPrefix(pkg.ImportPath, contribModulePath) {
			count++
		}
	}
	return count
}

func statusRank(status string) int {
	switch status {
	case "stable":
		return 0
	case "compatibility-only":
		return 1
	case "supported-adapter":
		return 2
	case "experimental":
		return 3
	case "wrapper-only":
		return 4
	case "test-only":
		return 5
	case "example-only":
		return 6
	case "generated":
		return 7
	case "tooling":
		return 8
	case "excluded":
		return 9
	default:
		return 10
	}
}

func sourceURL(rel string) string {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	return githubBlobBase + "/" + rel
}

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="generator" content="{{ .GeneratedBy }}">
  <title>api-toolkit API docs</title>
  <link rel="stylesheet" href="site.css">
</head>
<body>
  <header class="site-header">
    <div>
      <p class="eyebrow">Generated API docs site</p>
      <h1>api-toolkit API docs</h1>
      <p class="lede">Search packages, examples, stability status, compatibility notes, and migration guides from one static artifact.</p>
    </div>
    <dl class="stats" aria-label="Package status summary">
      <div><dt>Stable</dt><dd>{{ .StableCount }}</dd></div>
      <div><dt>Compatibility-only</dt><dd>{{ .CompatCount }}</dd></div>
      <div><dt>Contrib rows</dt><dd>{{ .ContribCount }}</dd></div>
      <div><dt>Search entries</dt><dd>{{ .SearchEntries }}</dd></div>
    </dl>
  </header>

  <main>
    <section class="search-panel" aria-labelledby="search-heading">
      <h2 id="search-heading">Search</h2>
      <label class="search-label" for="site-search">Find packages, examples, compatibility, or migration docs</label>
      <input id="site-search" type="search" autocomplete="off" placeholder="Try: ratelimit migration, compatibility-only, Problem Details">
      <p id="search-status" class="muted" role="status">Loading search index...</p>
      <ol id="search-results" class="results"></ol>
    </section>

    <section aria-labelledby="quick-links-heading">
      <h2 id="quick-links-heading">Key docs</h2>
      <div class="link-grid">
{{ range .Documents }}
        <a class="doc-link" href="{{ .URL }}">
          <span>{{ .Category }}</span>
          <strong>{{ .Title }}</strong>
          <small>{{ .Path }}</small>
        </a>
{{ end }}
      </div>
    </section>

    <section aria-labelledby="package-heading">
      <h2 id="package-heading">Package status</h2>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Package</th>
              <th>API status</th>
              <th>Test status</th>
              <th>Notes</th>
              <th>Example</th>
            </tr>
          </thead>
          <tbody>
{{ range .Packages }}
            <tr>
              <td><a href="{{ .PkgGoDev }}">{{ .ImportPath }}</a></td>
              <td><span class="badge">{{ .APIStatus }}</span></td>
              <td>{{ .TestStatus }}</td>
              <td>{{ .Notes }}</td>
              <td>{{ if .ExampleURL }}<a href="{{ .ExampleURL }}">example</a>{{ else }}<span class="muted">n/a</span>{{ end }}</td>
            </tr>
{{ end }}
          </tbody>
        </table>
      </div>
    </section>

    <section aria-labelledby="examples-heading">
      <h2 id="examples-heading">Examples</h2>
      <ul class="example-list">
{{ range .Examples }}
        <li><a href="{{ .URL }}">{{ .ImportPath }}</a><span>{{ .Path }}</span></li>
{{ end }}
      </ul>
    </section>
  </main>

  <footer>
    Generated by <code>{{ .GeneratedBy }}</code>. Run <code>make docs-site</code> after package, compatibility, migration, or example documentation changes.
  </footer>
  <script src="site.js" defer></script>
</body>
</html>
`))

const siteCSS = `:root {
  color-scheme: light;
  --bg: #f7f8fa;
  --panel: #ffffff;
  --text: #17202a;
  --muted: #5d6b7a;
  --line: #d9dee6;
  --accent: #126b62;
  --accent-2: #8a5a00;
  --code: #eef3f2;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 15px/1.5 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

a { color: var(--accent); }

.site-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 32px;
  padding: 40px max(24px, calc((100vw - 1160px) / 2));
  background: #ffffff;
  border-bottom: 1px solid var(--line);
}

.eyebrow {
  margin: 0 0 8px;
  color: var(--accent-2);
  font-weight: 700;
  text-transform: uppercase;
  font-size: 12px;
}

h1, h2 { line-height: 1.15; margin: 0; }
h1 { font-size: 40px; }
h2 { font-size: 22px; margin-bottom: 16px; }

.lede { max-width: 720px; color: var(--muted); font-size: 17px; }

.stats {
  display: grid;
  grid-template-columns: repeat(2, 150px);
  gap: 12px;
  margin: 0;
}

.stats div {
  padding: 14px;
  background: var(--code);
  border: 1px solid var(--line);
  border-radius: 6px;
}

.stats dt { color: var(--muted); font-size: 12px; }
.stats dd { margin: 4px 0 0; font-size: 28px; font-weight: 700; }

main {
  width: min(1160px, calc(100vw - 32px));
  margin: 28px auto 48px;
}

section {
  margin: 0 0 28px;
  padding: 22px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 6px;
}

.search-panel input {
  width: 100%;
  min-height: 44px;
  margin-top: 8px;
  padding: 10px 12px;
  border: 1px solid #aeb8c5;
  border-radius: 6px;
  font: inherit;
}

.search-label, .muted { color: var(--muted); }

.results {
  display: grid;
  gap: 10px;
  padding-left: 0;
  list-style: none;
}

.results li {
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 6px;
}

.results span, .doc-link span, .example-list span {
  display: block;
  color: var(--muted);
  font-size: 12px;
}

.link-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.doc-link {
  display: grid;
  gap: 4px;
  min-height: 108px;
  padding: 14px;
  border: 1px solid var(--line);
  border-radius: 6px;
  text-decoration: none;
}

.doc-link strong { color: var(--text); }

.table-wrap { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; }
th, td { padding: 9px 10px; border-bottom: 1px solid var(--line); text-align: left; vertical-align: top; }
th { font-size: 12px; color: var(--muted); text-transform: uppercase; }

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--code);
  color: #24433f;
  font-size: 12px;
  white-space: nowrap;
}

.example-list {
  columns: 2 320px;
  padding-left: 18px;
}

.example-list li {
  break-inside: avoid;
  margin-bottom: 10px;
}

footer {
  width: min(1160px, calc(100vw - 32px));
  margin: 0 auto 40px;
  color: var(--muted);
}

code {
  padding: 1px 5px;
  background: var(--code);
  border-radius: 4px;
}

@media (max-width: 780px) {
  .site-header { grid-template-columns: 1fr; padding: 28px 16px; }
  .stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  h1 { font-size: 32px; }
}
`

const siteJS = `"use strict";

const input = document.getElementById("site-search");
const results = document.getElementById("search-results");
const statusLine = document.getElementById("search-status");
let entries = [];

function normalize(value) {
  return String(value || "").toLowerCase();
}

function tokens(value) {
  return normalize(value).split(/\s+/).filter(Boolean);
}

function clearResults() {
  while (results.firstChild) {
    results.removeChild(results.firstChild);
  }
}

function renderResult(entry) {
  const item = document.createElement("li");
  const category = document.createElement("span");
  const link = document.createElement("a");
  const summary = document.createElement("p");

  category.textContent = entry.category;
  link.href = entry.url;
  link.textContent = entry.title;
  summary.textContent = String(entry.text || "").slice(0, 220);

  item.appendChild(category);
  item.appendChild(link);
  item.appendChild(summary);
  results.appendChild(item);
}

function render(query) {
  clearResults();
  const queryTokens = tokens(query);
  const matches = entries.filter((entry) => {
    if (queryTokens.length === 0) {
      return entry.category === "api" || entry.category === "package-status" || entry.category === "migration";
    }
    const haystack = normalize([entry.title, entry.category, entry.text].join(" "));
    return queryTokens.every((token) => haystack.includes(token));
  }).slice(0, 30);

  statusLine.textContent = matches.length + " result" + (matches.length === 1 ? "" : "s");
  matches.forEach(renderResult);
}

fetch("search-index.json", { credentials: "same-origin" })
  .then((response) => {
    if (!response.ok) {
      throw new Error("search index request failed");
    }
    return response.json();
  })
  .then((data) => {
    entries = Array.isArray(data) ? data : [];
    render("");
  })
  .catch(() => {
    statusLine.textContent = "Search index unavailable";
  });

input.addEventListener("input", () => render(input.value));
`
