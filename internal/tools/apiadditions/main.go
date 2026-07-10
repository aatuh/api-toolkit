package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type config struct {
	Root             string
	BaseInventory    string
	CurrentInventory string
	ReleaseNotes     string
	Exceptions       string
}

type inventoryEntry struct {
	ImportPath string
	Symbol     string
	Kind       string
}

type symbolEvidence struct {
	HasDoc     bool
	HasExample bool
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.Root, "root", ".", "repository root")
	flag.StringVar(&cfg.BaseInventory, "base-inventory", "", "base docs/api-inventory.md path")
	flag.StringVar(&cfg.CurrentInventory, "current-inventory", "docs/api-inventory.md", "current docs/api-inventory.md path")
	flag.StringVar(&cfg.ReleaseNotes, "release-notes", "docs/release-notes.md", "release notes or changelog path")
	flag.StringVar(&cfg.Exceptions, "exceptions", "docs/api-addition-exceptions.tsv", "exact example-exception manifest path")
	flag.Parse()

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "apiadditions:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	root := filepath.Clean(cfg.Root)
	if cfg.BaseInventory == "" {
		return fmt.Errorf("-base-inventory is required")
	}
	currentInventory := resolvePath(root, cfg.CurrentInventory)
	releaseNotesPath := resolvePath(root, cfg.ReleaseNotes)
	exceptionsPath := resolvePath(root, cfg.Exceptions)

	baseEntries, err := readInventory(cfg.BaseInventory)
	if err != nil {
		return fmt.Errorf("read base inventory: %w", err)
	}
	currentEntries, err := readInventory(currentInventory)
	if err != nil {
		return fmt.Errorf("read current inventory: %w", err)
	}

	added := addedEntries(baseEntries, currentEntries)
	if len(added) == 0 {
		fmt.Println("No new stable exported identifiers detected.")
		return nil
	}

	modulePath, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	sourceEvidence, err := sourceEvidenceForEntries(root, modulePath, currentEntries)
	if err != nil {
		return err
	}
	exceptions, err := readExceptions(exceptionsPath)
	if err != nil {
		return err
	}
	// #nosec G304 -- releaseNotesPath is an explicit local operator path resolved from the repository root.
	releaseNotesBytes, err := os.ReadFile(releaseNotesPath)
	if err != nil {
		return fmt.Errorf("read release notes: %w", err)
	}
	releaseNotes := string(releaseNotesBytes)

	var problems []string
	for _, entry := range added {
		key := entryKey(entry)
		evidence, ok := sourceEvidence[key]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s.%s is missing from current source evidence", entry.ImportPath, entry.Symbol))
			continue
		}
		if !evidence.HasDoc {
			problems = append(problems, fmt.Sprintf("%s.%s missing source doc comment", entry.ImportPath, entry.Symbol))
		}
		if !evidence.HasExample && !exceptions[key] {
			problems = append(problems, fmt.Sprintf("%s.%s missing compile-checked example or exact docs/api-addition-exceptions.tsv row", entry.ImportPath, entry.Symbol))
		}
		if !releaseNotesMentionSymbol(releaseNotes, entry) {
			problems = append(problems, fmt.Sprintf("%s.%s missing package-tied release note or changelog entry", entry.ImportPath, entry.Symbol))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("new stable exported identifiers require doc comments, examples or exceptions, API inventory entries, and release notes:\n - %s", strings.Join(problems, "\n - "))
	}

	fmt.Printf("API additions gate passed for %d new stable exported identifier(s).\n", len(added))
	return nil
}

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(root, value)
}

func readInventory(path string) (map[string]inventoryEntry, error) {
	// #nosec G304 -- path is an explicit local inventory path selected by the operator.
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	entries := map[string]inventoryEntry{}
	currentPackage := ""
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## `") {
			rest := strings.TrimPrefix(line, "## `")
			if end := strings.Index(rest, "`"); end >= 0 {
				currentPackage = rest[:end]
			}
			continue
		}
		if currentPackage == "" || !strings.HasPrefix(line, "| `") {
			continue
		}
		cols := markdownRowColumns(line)
		if len(cols) < 4 {
			continue
		}
		symbol := strings.Trim(cols[0], "` ")
		if symbol == "" || symbol == "Symbol" {
			continue
		}
		entry := inventoryEntry{
			ImportPath: currentPackage,
			Symbol:     symbol,
			Kind:       strings.TrimSpace(cols[1]),
		}
		entries[entryKey(entry)] = entry
	}
	return entries, nil
}

func markdownRowColumns(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	rawCols := strings.Split(line, "|")
	cols := make([]string, 0, len(rawCols))
	for _, col := range rawCols {
		cols = append(cols, strings.TrimSpace(col))
	}
	return cols
}

func addedEntries(base, current map[string]inventoryEntry) []inventoryEntry {
	var added []inventoryEntry
	for key, entry := range current {
		if _, ok := base[key]; !ok {
			added = append(added, entry)
		}
	}
	sort.Slice(added, func(i, j int) bool {
		if added[i].ImportPath == added[j].ImportPath {
			return added[i].Symbol < added[j].Symbol
		}
		return added[i].ImportPath < added[j].ImportPath
	})
	return added
}

func sourceEvidenceForEntries(root, modulePath string, entries map[string]inventoryEntry) (map[string]symbolEvidence, error) {
	packages := map[string]struct{}{}
	for _, entry := range entries {
		packages[entry.ImportPath] = struct{}{}
	}

	out := map[string]symbolEvidence{}
	for importPath := range packages {
		dir, ok := packageDir(root, modulePath, importPath)
		if !ok {
			continue
		}
		packageEvidence, err := packageSourceEvidence(dir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", importPath, err)
		}
		for symbol, evidence := range packageEvidence {
			out[importPath+"\t"+symbol] = evidence
		}
	}
	return out, nil
}

func packageDir(root, modulePath, importPath string) (string, bool) {
	if importPath != modulePath && !strings.HasPrefix(importPath, modulePath+"/") {
		return "", false
	}
	rel := strings.TrimPrefix(importPath, modulePath)
	rel = strings.TrimPrefix(rel, "/")
	return filepath.Join(root, filepath.FromSlash(rel)), true
}

func packageSourceEvidence(dir string) (map[string]symbolEvidence, error) {
	fset := token.NewFileSet()
	//nolint:staticcheck // The additions gate intentionally includes all non-test source variants, not only active build tags.
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no Go package found")
	}

	evidence := map[string]symbolEvidence{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.GenDecl:
					recordGenDeclEvidence(evidence, decl)
				case *ast.FuncDecl:
					if decl.Name == nil || !decl.Name.IsExported() {
						continue
					}
					name := decl.Name.Name
					if decl.Recv != nil && len(decl.Recv.List) > 0 {
						recv := receiverName(decl.Recv.List[0].Type)
						if recv == "" || !ast.IsExported(recv) {
							continue
						}
						name = recv + "." + decl.Name.Name
					}
					evidence[name] = symbolEvidence{HasDoc: hasDoc(decl.Doc)}
				}
			}
		}
		break
	}

	examples, err := exampleSymbols(dir)
	if err != nil {
		return nil, err
	}
	for symbol, item := range evidence {
		item.HasExample = exampleMentionsSymbol(examples, symbol)
		evidence[symbol] = item
	}
	return evidence, nil
}

func recordGenDeclEvidence(evidence map[string]symbolEvidence, decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			if spec.Name == nil || !spec.Name.IsExported() {
				continue
			}
			typeDoc := hasDoc(spec.Doc) || hasDoc(decl.Doc)
			typeName := spec.Name.Name
			evidence[typeName] = symbolEvidence{HasDoc: typeDoc}
			for symbol, documented := range memberDocs(typeName, spec.Type) {
				evidence[symbol] = symbolEvidence{HasDoc: documented}
			}
		case *ast.ValueSpec:
			valueDoc := hasDoc(spec.Doc) || hasDoc(decl.Doc)
			for _, name := range spec.Names {
				if name != nil && name.IsExported() {
					evidence[name.Name] = symbolEvidence{HasDoc: valueDoc}
				}
			}
		}
	}
}

func memberDocs(typeName string, expr ast.Expr) map[string]bool {
	out := map[string]bool{}
	switch typ := expr.(type) {
	case *ast.StructType:
		if typ.Fields == nil {
			return out
		}
		for _, field := range typ.Fields.List {
			documented := hasDoc(field.Doc) || hasDoc(field.Comment)
			for _, name := range exportedFieldNames(field) {
				out[typeName+"."+name] = documented
			}
		}
	case *ast.InterfaceType:
		if typ.Methods == nil {
			return out
		}
		for _, method := range typ.Methods.List {
			documented := hasDoc(method.Doc) || hasDoc(method.Comment)
			for _, name := range exportedFieldNames(method) {
				out[typeName+"."+name] = documented
			}
		}
	}
	return out
}

func exportedFieldNames(field *ast.Field) []string {
	if len(field.Names) == 0 {
		name := exprName(field.Type)
		if name != "" && ast.IsExported(name) {
			return []string{name}
		}
		return nil
	}
	var names []string
	for _, name := range field.Names {
		if name != nil && name.IsExported() {
			names = append(names, name.Name)
		}
	}
	return names
}

func exampleSymbols(dir string) ([]exampleEvidence, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var examples []exampleEvidence
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		// #nosec G304 -- name is a non-directory entry returned by os.ReadDir for the package directory.
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Example") {
				continue
			}
			start := fset.Position(fn.Pos()).Offset
			end := fset.Position(fn.End()).Offset
			body := ""
			if start >= 0 && end >= start && end <= len(src) {
				body = string(src[start:end])
			}
			examples = append(examples, exampleEvidence{Name: fn.Name.Name, Body: body})
		}
	}
	return examples, nil
}

type exampleEvidence struct {
	Name string
	Body string
}

func exampleMentionsSymbol(examples []exampleEvidence, symbol string) bool {
	for _, example := range examples {
		if exampleNameMatchesSymbol(example.Name, symbol) || symbolTextMentions(example.Body, symbol) {
			return true
		}
	}
	return false
}

func exampleNameMatchesSymbol(exampleName, symbol string) bool {
	suffix := strings.TrimPrefix(exampleName, "Example")
	if suffix == "" {
		return false
	}
	symbol = strings.ReplaceAll(symbol, ".", "_")
	return suffix == symbol || strings.HasPrefix(suffix, symbol+"_")
}

func symbolTextMentions(text, symbol string) bool {
	candidates := []string{symbol}
	if strings.Contains(symbol, ".") {
		parts := strings.Split(symbol, ".")
		last := parts[len(parts)-1]
		candidates = append(candidates, last, "."+last, last+":")
	}
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(text, candidate) {
			return true
		}
	}
	return false
}

func readExceptions(path string) (map[string]bool, error) {
	// #nosec G304 -- path is an explicit local exceptions manifest path selected by the operator.
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read API addition exceptions: %w", err)
	}
	exceptions := map[string]bool{}
	for lineNo, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 5 {
			return nil, fmt.Errorf("%s:%d expected 5 tab-separated fields", path, lineNo+1)
		}
		for i, col := range cols {
			cols[i] = strings.TrimSpace(col)
			if cols[i] == "" {
				return nil, fmt.Errorf("%s:%d contains an empty field", path, lineNo+1)
			}
		}
		if cols[0] == "import_path" && cols[1] == "symbol" {
			continue
		}
		exceptions[cols[0]+"\t"+cols[1]] = true
	}
	return exceptions, nil
}

func releaseNotesMentionSymbol(notes string, entry inventoryEntry) bool {
	qualified := entry.ImportPath + "." + entry.Symbol
	shortQualified := path.Base(entry.ImportPath) + "." + entry.Symbol
	if strings.Contains(notes, qualified) || strings.Contains(notes, shortQualified) {
		return true
	}
	packageLeaf := path.Base(entry.ImportPath)
	for _, line := range strings.Split(notes, "\n") {
		if strings.Contains(line, entry.Symbol) &&
			(strings.Contains(line, entry.ImportPath) || strings.Contains(line, packageLeaf)) {
			return true
		}
	}
	return false
}

func modulePath(goMod string) (string, error) {
	// #nosec G304 -- goMod is derived from the selected local repository root.
	content, err := os.ReadFile(goMod)
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("module directive not found in %s", goMod)
}

func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		return exprName(star.X)
	}
	return exprName(expr)
}

func exprName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		return expr.Sel.Name
	case *ast.IndexExpr:
		return exprName(expr.X)
	case *ast.IndexListExpr:
		return exprName(expr.X)
	}
	return ""
}

func hasDoc(group *ast.CommentGroup) bool {
	return group != nil && strings.TrimSpace(group.Text()) != ""
}

func entryKey(entry inventoryEntry) string {
	return entry.ImportPath + "\t" + entry.Symbol
}
