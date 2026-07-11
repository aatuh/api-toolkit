// Command portsledger reports current non-test consumers of exported root ports.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const portsImportPath = "github.com/aatuh/api-toolkit/v4/ports"

type exportedSymbol struct {
	name       string
	kind       string
	deprecated bool
}

func main() {
	verifyPath := flag.String("verify", "", "verify a ports migration ledger")
	flag.Parse()

	repoRoot, err := os.Getwd()
	if err != nil {
		fail("resolve working directory: %v", err)
	}

	symbols, err := exportedPorts(filepath.Join(repoRoot, "ports"))
	if err != nil {
		fail("read exported ports: %v", err)
	}
	consumers, err := findConsumers(repoRoot)
	if err != nil {
		fail("find ports consumers: %v", err)
	}
	if *verifyPath != "" {
		if err := verifyLedger(repoRoot, *verifyPath, symbols, consumers); err != nil {
			fail("verify ledger: %v", err)
		}
		return
	}

	fmt.Println("symbol\tkind\tconsumer_count\tconsumer_packages")
	for _, symbol := range symbols {
		packages := sortedKeys(consumers[symbol.name])
		if len(packages) == 0 {
			fmt.Printf("%s\t%s\t0\t-\n", symbol.name, symbol.kind)
			continue
		}
		fmt.Printf("%s\t%s\t%d\t%s\n", symbol.name, symbol.kind, len(packages), strings.Join(packages, ";"))
	}
}

func exportedPorts(dir string) ([]exportedSymbol, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var symbols []exportedSymbol
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil && decl.Name.IsExported() {
					symbols = append(symbols, exportedSymbol{
						name:       decl.Name.Name,
						kind:       "func",
						deprecated: hasDeprecatedComment(decl.Doc),
					})
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							symbols = append(symbols, exportedSymbol{
								name:       spec.Name.Name,
								kind:       "type",
								deprecated: hasDeprecatedComment(spec.Doc) || hasDeprecatedComment(decl.Doc),
							})
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if name.IsExported() {
								kind := "var"
								if decl.Tok == token.CONST {
									kind = "const"
								}
								symbols = append(symbols, exportedSymbol{
									name:       name.Name,
									kind:       kind,
									deprecated: hasDeprecatedComment(spec.Doc) || hasDeprecatedComment(decl.Doc),
								})
							}
						}
					}
				}
			}
		}
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].name < symbols[j].name })
	return symbols, nil
}

func hasDeprecatedComment(group *ast.CommentGroup) bool {
	return group != nil && strings.Contains(group.Text(), "Deprecated:")
}

func findConsumers(repoRoot string) (map[string]map[string]bool, error) {
	consumers := make(map[string]map[string]bool)
	err := filepath.WalkDir(repoRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".audits", ".ci-result", ".trash", "ports", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		aliases := portsAliases(file)
		if len(aliases) == 0 {
			return nil
		}
		file, err = parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		relDir, err := filepath.Rel(repoRoot, filepath.Dir(path))
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || !selector.Sel.IsExported() {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || !aliases[ident.Name] {
				return true
			}
			if consumers[selector.Sel.Name] == nil {
				consumers[selector.Sel.Name] = make(map[string]bool)
			}
			consumers[selector.Sel.Name][filepath.ToSlash(relDir)] = true
			return true
		})
		return nil
	})
	return consumers, err
}

func portsAliases(file *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, "\"") != portsImportPath {
			continue
		}
		if imp.Name == nil {
			aliases["ports"] = true
			continue
		}
		if imp.Name.Name != "_" && imp.Name.Name != "." {
			aliases[imp.Name.Name] = true
		}
	}
	return aliases
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func verifyLedger(repoRoot, path string, symbols []exportedSymbol, consumers map[string]map[string]bool) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("ledger path must be relative to the repository root")
	}
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return err
	}
	defer root.Close()

	file, err := root.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	expected := make(map[string]exportedSymbol, len(symbols))
	for _, symbol := range symbols {
		expected[symbol.name] = symbol
	}

	const header = "symbol\tkind\tconsumers\timplementations\treplacement_path\tv3_deprecation_status\tv4_disposition\tmigration_evidence"
	seenHeader := false
	seen := make(map[string]bool, len(symbols))
	for lineNumber, line := range strings.Split(string(content), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !seenHeader {
			if line != header {
				return fmt.Errorf("line %d header = %q, want %q", lineNumber+1, line, header)
			}
			seenHeader = true
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 8 {
			return fmt.Errorf("line %d has %d fields, want 8", lineNumber+1, len(fields))
		}
		symbol, ok := expected[fields[0]]
		if !ok {
			return fmt.Errorf("line %d lists unknown symbol %q", lineNumber+1, fields[0])
		}
		if seen[symbol.name] {
			return fmt.Errorf("line %d duplicates %q", lineNumber+1, symbol.name)
		}
		seen[symbol.name] = true
		if fields[1] != symbol.kind {
			return fmt.Errorf("line %d kind for %s = %q, want %q", lineNumber+1, symbol.name, fields[1], symbol.kind)
		}
		if fields[2] != consumerPackages(consumers[symbol.name]) {
			return fmt.Errorf("line %d consumers for %s = %q, want %q", lineNumber+1, symbol.name, fields[2], consumerPackages(consumers[symbol.name]))
		}
		status := "active"
		if symbol.deprecated {
			status = "deprecated"
		}
		if fields[5] != status {
			return fmt.Errorf("line %d v3 deprecation status for %s = %q, want %q", lineNumber+1, symbol.name, fields[5], status)
		}
		if !map[string]bool{"keep": true, "move": true, "narrow": true, "remove": true}[fields[6]] {
			return fmt.Errorf("line %d v4 disposition for %s = %q, want keep, move, narrow, or remove", lineNumber+1, symbol.name, fields[6])
		}
		for _, index := range []int{3, 4, 6, 7} {
			if fields[index] == "" {
				return fmt.Errorf("line %d %s is empty", lineNumber+1, ledgerColumn(index))
			}
		}
	}
	if !seenHeader {
		return fmt.Errorf("missing header")
	}
	for _, symbol := range symbols {
		if !seen[symbol.name] {
			return fmt.Errorf("missing exported symbol %q", symbol.name)
		}
	}
	return nil
}

func consumerPackages(consumers map[string]bool) string {
	packages := sortedKeys(consumers)
	if len(packages) == 0 {
		return "-"
	}
	return strings.Join(packages, ";")
}

func ledgerColumn(index int) string {
	return []string{"symbol", "kind", "consumers", "implementations", "replacement_path", "v3_deprecation_status", "v4_disposition", "migration_evidence"}[index]
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
