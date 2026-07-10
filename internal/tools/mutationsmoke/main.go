package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPackages = "./binding,./queryparams,./negotiation,./webhooks"
	defaultLimit    = 12
	defaultTimeout  = 30 * time.Second
)

type config struct {
	Root     string
	Go       string
	Packages []string
	Limit    int
	Timeout  time.Duration
	Out      string
	Keep     bool
}

type packageTarget struct {
	Pattern string
	Import  string
	Dir     string
}

type mutation struct {
	Package     string
	File        string
	Start       int
	End         int
	Original    string
	Replacement string
	Rule        string
}

type mutationResult struct {
	Mutation   mutation
	Status     string
	Duration   time.Duration
	Err        string
	Workdir    string
	CommandOut string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	targets, err := packageTargets(cfg)
	if err != nil {
		return err
	}
	mutations, err := discoverMutations(cfg.Root, targets)
	if err != nil {
		return err
	}
	if cfg.Limit > 0 && len(mutations) > cfg.Limit {
		mutations = mutations[:cfg.Limit]
	}
	results := make([]mutationResult, 0, len(mutations))
	for _, candidate := range mutations {
		result := runMutation(cfg, candidate)
		results = append(results, result)
		if result.Status == "error" {
			fmt.Fprintf(stderr, "mutation setup error for %s %s: %s\n", candidate.File, candidate.Rule, result.Err)
		}
	}
	if err := writeReport(cfg.Out, results); err != nil {
		return err
	}
	printSummary(stdout, cfg.Out, results)
	return nil
}

func parseConfig(args []string) (config, error) {
	root, err := os.Getwd()
	if err != nil {
		return config{}, err
	}
	fs := flag.NewFlagSet("mutationsmoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cfg := config{
		Root:    root,
		Go:      envDefault("GO", "go"),
		Limit:   envIntDefault("MUTATION_LIMIT", defaultLimit),
		Timeout: envDurationDefault("MUTATION_TIMEOUT", defaultTimeout),
		Out:     filepath.Join(envDefault("OUTPUT_DIR", ".ci-result"), "mutation", "mutation-smoke.tsv"),
	}
	packages := envDefault("MUTATION_PACKAGES", defaultPackages)
	fs.StringVar(&cfg.Root, "workdir", cfg.Root, "repository root")
	fs.StringVar(&cfg.Go, "go", cfg.Go, "go command")
	fs.StringVar(&packages, "packages", packages, "comma or space separated package patterns")
	fs.IntVar(&cfg.Limit, "limit", cfg.Limit, "maximum mutants to run")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-mutant go test timeout")
	fs.StringVar(&cfg.Out, "out", cfg.Out, "TSV report path")
	fs.BoolVar(&cfg.Keep, "keep-workdir", false, "keep temporary worktrees for debugging")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.Root, err = filepath.Abs(cfg.Root)
	if err != nil {
		return config{}, err
	}
	cfg.Out, err = resolveOutputPath(cfg.Root, cfg.Out)
	if err != nil {
		return config{}, err
	}
	cfg.Packages, err = parsePackageList(packages)
	if err != nil {
		return config{}, err
	}
	if cfg.Limit < 0 {
		return config{}, errors.New("limit must be non-negative")
	}
	if cfg.Timeout <= 0 {
		return config{}, errors.New("timeout must be greater than zero")
	}
	if strings.TrimSpace(cfg.Go) == "" || strings.HasPrefix(strings.TrimSpace(cfg.Go), "-") {
		return config{}, errors.New("go command is required")
	}
	return cfg, nil
}

func packageTargets(cfg config) ([]packageTarget, error) {
	var targets []packageTarget
	for _, pattern := range cfg.Packages {
		output, err := runCommand(context.Background(), cfg.Root, cfg.Go, []string{"list", "-f", "{{.ImportPath}}\t{{.Dir}}", pattern}, cfg.Timeout, nil)
		if err != nil {
			return nil, fmt.Errorf("go list %s: %w", pattern, err)
		}
		scanner := bufio.NewScanner(strings.NewReader(output))
		for scanner.Scan() {
			fields := strings.Split(scanner.Text(), "\t")
			if len(fields) != 2 {
				continue
			}
			dir, err := filepath.Abs(fields[1])
			if err != nil {
				return nil, err
			}
			if !isPathInside(cfg.Root, dir) {
				return nil, fmt.Errorf("package %s resolved outside repository: %s", pattern, dir)
			}
			targets = append(targets, packageTarget{Pattern: pattern, Import: fields[0], Dir: dir})
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	if len(targets) == 0 {
		return nil, errors.New("no packages resolved for mutation smoke")
	}
	return targets, nil
}

func discoverMutations(root string, targets []packageTarget) ([]mutation, error) {
	var out []mutation
	for _, target := range targets {
		entries, err := os.ReadDir(target.Dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(target.Dir, name)
			mutations, err := fileMutations(root, target, path)
			if err != nil {
				return nil, err
			}
			out = append(out, mutations...)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].Rule < out[j].Rule
	})
	return out, nil
}

func fileMutations(root string, target packageTarget, path string) ([]mutation, error) {
	// #nosec G304 -- path is a non-test Go file enumerated from the selected local package directory.
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, err
	}
	file := fset.File(parsed.Pos())
	if file == nil {
		return nil, fmt.Errorf("missing token file for %s", path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	var out []mutation
	add := func(pos token.Pos, original, replacement, rule string) {
		offset := file.Offset(pos)
		if offset < 0 || offset+len(original) > len(source) {
			return
		}
		if string(source[offset:offset+len(original)]) != original {
			return
		}
		out = append(out, mutation{
			Package:     target.Import,
			File:        filepath.ToSlash(rel),
			Start:       offset,
			End:         offset + len(original),
			Original:    original,
			Replacement: replacement,
			Rule:        rule,
		})
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.Ident:
			switch n.Name {
			case "true":
				add(n.Pos(), "true", "false", "bool-literal")
			case "false":
				add(n.Pos(), "false", "true", "bool-literal")
			}
		case *ast.BinaryExpr:
			if replacement, ok := operatorMutation(n.Op); ok {
				add(n.OpPos, n.Op.String(), replacement, "operator")
			}
		}
		return true
	})
	return out, nil
}

func operatorMutation(op token.Token) (string, bool) {
	switch op.String() {
	case "==":
		return "!=", true
	case "!=":
		return "==", true
	case "<":
		return "<=", true
	case "<=":
		return "<", true
	case ">":
		return ">=", true
	case ">=":
		return ">", true
	case "&&":
		return "||", true
	case "||":
		return "&&", true
	default:
		return "", false
	}
}

func runMutation(cfg config, candidate mutation) mutationResult {
	start := time.Now()
	workdir, err := copyRepository(cfg.Root)
	result := mutationResult{Mutation: candidate, Workdir: workdir}
	if err != nil {
		result.Status = "error"
		result.Err = err.Error()
		return result
	}
	if !cfg.Keep {
		defer os.RemoveAll(workdir)
	}
	if err := applyMutation(workdir, candidate); err != nil {
		result.Status = "error"
		result.Err = err.Error()
		return result
	}
	output, err := runCommand(context.Background(), workdir, cfg.Go, []string{"test", candidate.Package, "-count=1", "-timeout=" + cfg.Timeout.String()}, cfg.Timeout+5*time.Second, []string{"GOWORK=off", "GOTOOLCHAIN=local"})
	result.Duration = time.Since(start)
	result.CommandOut = firstLine(output)
	if err == nil {
		result.Status = "survived"
		return result
	}
	if errors.Is(err, context.DeadlineExceeded) {
		result.Status = "timeout"
		result.Err = "go test timed out"
		return result
	}
	result.Status = "killed"
	result.Err = firstLine(err.Error())
	return result
}

func applyMutation(root string, candidate mutation) error {
	path := filepath.Join(root, filepath.FromSlash(candidate.File))
	if !isPathInside(root, path) {
		return fmt.Errorf("mutation path escapes root: %s", candidate.File)
	}
	// #nosec G304 -- isPathInside verifies that the candidate path is contained by the local repository root.
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if candidate.Start < 0 || candidate.End > len(source) || candidate.Start >= candidate.End {
		return fmt.Errorf("invalid mutation range %d:%d", candidate.Start, candidate.End)
	}
	if string(source[candidate.Start:candidate.End]) != candidate.Original {
		return fmt.Errorf("mutation original mismatch in %s", candidate.File)
	}
	var mutated bytes.Buffer
	mutated.Write(source[:candidate.Start])
	mutated.WriteString(candidate.Replacement)
	mutated.Write(source[candidate.End:])
	// #nosec G306 -- The existing source fixture was read above; overwriting it preserves its mode.
	return os.WriteFile(path, mutated.Bytes(), 0o644)
}

func runCommand(ctx context.Context, dir string, name string, args []string, timeout time.Duration, extraEnv []string) (string, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	// #nosec G204 -- The local operator controls the configured Go executable; arguments are passed without a shell.
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), ctx.Err()
	}
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, firstLine(string(output)))
	}
	return string(output), nil
}

func copyRepository(root string) (string, error) {
	tmp, err := os.MkdirTemp("", "api-toolkit-mutation-*")
	if err != nil {
		return "", err
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() && shouldSkipCopyDir(rel) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			// #nosec G301 -- This is an isolated temporary copy of public source files.
			return os.MkdirAll(filepath.Join(tmp, rel), 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || shouldSkipCopyFile(rel) {
			return nil
		}
		return copyFile(path, filepath.Join(tmp, rel), info.Mode().Perm())
	})
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	return tmp, nil
}

func shouldSkipCopyDir(rel string) bool {
	switch filepath.ToSlash(rel) {
	case ".git", ".ci-result", ".trash", ".audits":
		return true
	default:
		return false
	}
}

func shouldSkipCopyFile(rel string) bool {
	switch filepath.ToSlash(rel) {
	case ".workspace.code-workspace", "coverage.out", "coverage-db.out":
		return true
	default:
		return false
	}
}

func copyFile(src string, dst string, perm fs.FileMode) error {
	// #nosec G301 -- dst is beneath the isolated temporary copy directory.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// #nosec G304 -- src is a regular non-symlink file returned by filepath.Walk on the local repository root.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// #nosec G304 -- dst is derived from the relative path of a walked local source file inside a fresh temporary directory.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func writeReport(path string, results []mutationResult) error {
	// #nosec G301 -- Mutation reports are public local review artifacts.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("package\tfile\trule\toriginal\treplacement\tstatus\tduration_ms\n")
	for _, result := range results {
		fmt.Fprintf(
			&b,
			"%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
			result.Mutation.Package,
			result.Mutation.File,
			result.Mutation.Rule,
			result.Mutation.Original,
			result.Mutation.Replacement,
			result.Status,
			result.Duration.Milliseconds(),
		)
	}
	// #nosec G306 -- The mutation report is public review output.
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func printSummary(w io.Writer, path string, results []mutationResult) {
	counts := map[string]int{}
	for _, result := range results {
		counts[result.Status]++
	}
	fmt.Fprintf(w, "mutation smoke: mutants=%d killed=%d survived=%d timeout=%d error=%d report=%s\n",
		len(results),
		counts["killed"],
		counts["survived"],
		counts["timeout"],
		counts["error"],
		path,
	)
	if counts["survived"] > 0 {
		fmt.Fprintln(w, "mutation smoke is non-blocking; review survived mutants as weak-assertion signals")
	}
}

func parsePackageList(raw string) ([]string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if err := validatePackagePattern(field); err != nil {
			return nil, err
		}
		out = append(out, field)
	}
	if len(out) == 0 {
		return nil, errors.New("at least one package is required")
	}
	return out, nil
}

func validatePackagePattern(pattern string) error {
	if strings.HasPrefix(pattern, "-") || filepath.IsAbs(pattern) || !strings.HasPrefix(pattern, "./") {
		return fmt.Errorf("package pattern must be a relative ./ path: %q", pattern)
	}
	if strings.ContainsAny(pattern, ";&|`$<>\\") {
		return fmt.Errorf("package pattern contains unsupported shell metacharacter: %q", pattern)
	}
	scope := strings.TrimPrefix(strings.TrimSuffix(pattern, "/..."), "./")
	if scope == "" || scope == "." {
		return nil
	}
	for _, part := range strings.Split(scope, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("package pattern must not contain traversal: %q", pattern)
		}
	}
	return nil
}

func resolveOutputPath(root string, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("out path is required")
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path = filepath.Clean(path)
	if !isPathInside(root, path) {
		return "", fmt.Errorf("out path escapes repository root: %s", raw)
	}
	return path, nil
}

func isPathInside(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func envDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envIntDefault(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envDurationDefault(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		return value[:idx]
	}
	return value
}
