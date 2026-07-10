package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	historyHeader = "release_tag\trelease_commit\tmodule\tpackage\tapi_status\ttest_status\tobserved_percent"
	summaryHeader = "module\tpackage\tapi_status\ttest_status\tfloor_env\tfloor_percent\tobserved_percent\tbranch_notes"
)

var (
	releasePattern = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)(-rc\.([0-9]+))?$`)
	commitPattern  = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
)

type config struct {
	History      string
	Out          string
	Summary      string
	Record       string
	Commit       string
	RootLog      string
	RootFunc     string
	RootTotal    string
	ContribLog   string
	ContribFunc  string
	ContribTotal string
	Check        bool
}

type summaryRow struct {
	Module   string
	Package  string
	API      string
	Test     string
	Observed string
}

type historyRow struct {
	Release string
	Commit  string
	Summary summaryRow
}

type releaseVersion struct {
	Major int
	Minor int
	Patch int
	RC    int
	IsRC  bool
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "coveragetrend:", err)
		os.Exit(2)
	}
}

func run(args []string, stdout io.Writer) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	rows, err := readHistory(cfg.History)
	if err != nil {
		return err
	}
	if cfg.Record != "" {
		if hasRelease(rows, cfg.Record) {
			return fmt.Errorf("coverage history already contains release %s", cfg.Record)
		}
		var snapshot []summaryRow
		if cfg.RootLog != "" {
			snapshot, err = readLegacySnapshot(cfg)
		} else {
			snapshot, err = readSummary(cfg.Summary)
		}
		if err != nil {
			return err
		}
		for _, row := range snapshot {
			rows = append(rows, historyRow{Release: cfg.Record, Commit: cfg.Commit, Summary: row})
		}
	}
	if len(rows) == 0 {
		return errors.New("coverage history has no release snapshots")
	}
	sortHistory(rows)
	if err := validateHistory(rows); err != nil {
		return err
	}
	history := encodeHistory(rows)
	document, err := renderDocument(rows)
	if err != nil {
		return err
	}
	if cfg.Check {
		if err := checkFile(cfg.History, history); err != nil {
			return fmt.Errorf("coverage history is not canonical: %w", err)
		}
		if err := checkFile(cfg.Out, document); err != nil {
			return fmt.Errorf("coverage trend document is stale: %w", err)
		}
		fmt.Fprintf(stdout, "coverage trend check: releases=%d packages=%d\n", len(releases(rows)), len(packageKeys(rows)))
		return nil
	}
	if cfg.Record != "" {
		if err := writeFile(cfg.History, history); err != nil {
			return err
		}
	}
	if err := writeFile(cfg.Out, document); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "coverage trend: releases=%d packages=%d history=%s out=%s\n", len(releases(rows)), len(packageKeys(rows)), cfg.History, cfg.Out)
	return nil
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("coveragetrend", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cfg := config{
		History: "docs/coverage-trend.tsv",
		Out:     "docs/coverage-trend.md",
		Summary: ".ci-result/coverage/package-summary.tsv",
	}
	fs.StringVar(&cfg.History, "history", cfg.History, "coverage history TSV")
	fs.StringVar(&cfg.Out, "out", cfg.Out, "rendered Markdown output")
	fs.StringVar(&cfg.Summary, "summary", cfg.Summary, "current package-summary.tsv")
	fs.StringVar(&cfg.Record, "record", "", "release tag to record from the current coverage summary")
	fs.StringVar(&cfg.Commit, "commit", "", "release commit for -record")
	fs.StringVar(&cfg.RootLog, "root-log", "", "legacy root go test coverage log")
	fs.StringVar(&cfg.RootFunc, "root-func", "", "legacy root go tool cover output")
	fs.StringVar(&cfg.RootTotal, "root-total", "", "legacy root aggregate coverage when no function summary is available")
	fs.StringVar(&cfg.ContribLog, "contrib-log", "", "legacy contrib go test coverage log")
	fs.StringVar(&cfg.ContribFunc, "contrib-func", "", "legacy contrib go tool cover output")
	fs.StringVar(&cfg.ContribTotal, "contrib-total", "", "legacy contrib aggregate coverage when no function summary is available")
	fs.BoolVar(&cfg.Check, "check", false, "verify canonical history and rendered output")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(cfg.History) == "" || strings.TrimSpace(cfg.Out) == "" {
		return config{}, errors.New("history and out paths are required")
	}
	if cfg.Check && cfg.Record != "" {
		return config{}, errors.New("-check cannot be combined with -record")
	}
	legacySet := strings.TrimSpace(cfg.RootLog) != "" || strings.TrimSpace(cfg.ContribLog) != ""
	legacyMetadataSet := strings.TrimSpace(cfg.RootFunc) != "" || strings.TrimSpace(cfg.ContribFunc) != "" || strings.TrimSpace(cfg.RootTotal) != "" || strings.TrimSpace(cfg.ContribTotal) != ""
	if cfg.Record == "" && (legacySet || legacyMetadataSet) {
		return config{}, errors.New("legacy coverage inputs require -record")
	}
	if cfg.Record != "" {
		if _, err := parseRelease(cfg.Record); err != nil {
			return config{}, err
		}
		if !commitPattern.MatchString(cfg.Commit) {
			return config{}, fmt.Errorf("commit must be a lowercase hexadecimal Git commit: %q", cfg.Commit)
		}
		if legacySet {
			if strings.TrimSpace(cfg.RootLog) == "" || strings.TrimSpace(cfg.ContribLog) == "" {
				return config{}, errors.New("legacy recording requires root and contrib logs together")
			}
			if (strings.TrimSpace(cfg.RootFunc) == "") == (strings.TrimSpace(cfg.RootTotal) == "") {
				return config{}, errors.New("legacy recording requires exactly one root aggregate source")
			}
			if (strings.TrimSpace(cfg.ContribFunc) == "") == (strings.TrimSpace(cfg.ContribTotal) == "") {
				return config{}, errors.New("legacy recording requires exactly one contrib aggregate source")
			}
		} else if legacyMetadataSet {
			return config{}, errors.New("legacy aggregate sources require root and contrib logs")
		}
		if !legacySet && strings.TrimSpace(cfg.Summary) == "" {
			return config{}, errors.New("summary path is required when recording current coverage")
		}
	}
	return cfg, nil
}

func readSummary(path string) ([]summaryRow, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read coverage summary %s: %w", path, err)
	}
	lines := splitLines(content)
	if len(lines) == 0 || lines[0] != summaryHeader {
		return nil, fmt.Errorf("coverage summary %s has an unexpected header", path)
	}
	rows := make([]summaryRow, 0, len(lines)-1)
	seen := map[string]bool{}
	for index, line := range lines[1:] {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 8 {
			return nil, fmt.Errorf("coverage summary %s line %d has %d columns, want 8", path, index+2, len(fields))
		}
		row := summaryRow{
			Module:   fields[0],
			Package:  fields[1],
			API:      fields[2],
			Test:     fields[3],
			Observed: fields[6],
		}
		if err := validateSummaryRow(row); err != nil {
			return nil, fmt.Errorf("coverage summary %s line %d: %w", path, index+2, err)
		}
		key := rowKey(row)
		if seen[key] {
			return nil, fmt.Errorf("coverage summary %s line %d duplicates %s", path, index+2, row.Package)
		}
		seen[key] = true
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("coverage summary %s has no data rows", path)
	}
	return rows, nil
}

func readLegacySnapshot(cfg config) ([]summaryRow, error) {
	scope, err := readSummary(cfg.Summary)
	if err != nil {
		return nil, fmt.Errorf("read legacy scope: %w", err)
	}
	root, err := readLegacyLog(cfg.RootLog)
	if err != nil {
		return nil, err
	}
	contrib, err := readLegacyLog(cfg.ContribLog)
	if err != nil {
		return nil, err
	}
	rootTotal, err := readAggregate(cfg.RootFunc, cfg.RootTotal)
	if err != nil {
		return nil, err
	}
	contribTotal, err := readAggregate(cfg.ContribFunc, cfg.ContribTotal)
	if err != nil {
		return nil, err
	}
	for index := range scope {
		row := &scope[index]
		if row.Package == "(aggregate)" {
			if row.Module == "root" {
				row.Observed = rootTotal
			} else {
				row.Observed = contribTotal
			}
			continue
		}
		observed := "not-reported"
		switch row.Module {
		case "root":
			if value, ok := root[row.Package]; ok {
				observed = value
			}
		case "contrib":
			if value, ok := contrib[row.Package]; ok {
				observed = value
			}
		}
		row.Observed = observed
	}
	return scope, nil
}

func readLegacyLog(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read legacy coverage log %s: %w", path, err)
	}
	values := map[string]string{}
	for _, line := range splitLines(content) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "?" && strings.Contains(line, "[no test files]") {
			values[fields[1]] = "no-test-files"
			continue
		}
		if fields[0] != "ok" {
			continue
		}
		if observed, ok := coverageFromLogLine(line); ok {
			values[fields[1]] = observed
		}
	}
	return values, nil
}

func coverageFromLogLine(line string) (string, bool) {
	marker := "coverage:"
	index := strings.Index(line, marker)
	if index < 0 {
		return "", false
	}
	value := strings.TrimSpace(line[index+len(marker):])
	if strings.HasPrefix(value, "[no statements]") {
		return "no-statements", true
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return "", false
	}
	observed := strings.TrimSuffix(fields[0], "%")
	if err := validateObserved(observed); err != nil {
		return "", false
	}
	return observed, true
}

func readCoverageTotal(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read coverage function summary %s: %w", path, err)
	}
	for _, line := range splitLines(content) {
		if !strings.HasPrefix(line, "total:") {
			continue
		}
		fields := strings.Fields(line)
		for index := len(fields) - 1; index >= 0; index-- {
			if !strings.HasSuffix(fields[index], "%") {
				continue
			}
			observed := strings.TrimSuffix(fields[index], "%")
			if err := validateObserved(observed); err != nil {
				return "", fmt.Errorf("coverage function summary %s: %w", path, err)
			}
			return observed, nil
		}
	}
	return "", fmt.Errorf("coverage function summary %s has no total", path)
}

func readAggregate(funcPath, explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		if err := validateObserved(explicit); err != nil {
			return "", err
		}
		if _, err := strconv.ParseFloat(explicit, 64); err != nil {
			return "", fmt.Errorf("aggregate coverage must be numeric: %q", explicit)
		}
		return explicit, nil
	}
	return readCoverageTotal(funcPath)
}

func readHistory(path string) ([]historyRow, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read coverage history %s: %w", path, err)
	}
	lines := splitLines(content)
	if len(lines) == 0 || lines[0] != historyHeader {
		return nil, fmt.Errorf("coverage history %s has an unexpected header", path)
	}
	rows := make([]historyRow, 0, len(lines)-1)
	for index, line := range lines[1:] {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			return nil, fmt.Errorf("coverage history %s line %d has %d columns, want 7", path, index+2, len(fields))
		}
		row := historyRow{
			Release: fields[0],
			Commit:  fields[1],
			Summary: summaryRow{
				Module:   fields[2],
				Package:  fields[3],
				API:      fields[4],
				Test:     fields[5],
				Observed: fields[6],
			},
		}
		if _, err := parseRelease(row.Release); err != nil {
			return nil, fmt.Errorf("coverage history %s line %d: %w", path, index+2, err)
		}
		if !commitPattern.MatchString(row.Commit) {
			return nil, fmt.Errorf("coverage history %s line %d has invalid commit %q", path, index+2, row.Commit)
		}
		if err := validateSummaryRow(row.Summary); err != nil {
			return nil, fmt.Errorf("coverage history %s line %d: %w", path, index+2, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func validateHistory(rows []historyRow) error {
	seen := map[string]bool{}
	commits := map[string]string{}
	aggregates := map[string]map[string]bool{}
	for _, row := range rows {
		key := row.Release + "\x00" + rowKey(row.Summary)
		if seen[key] {
			return fmt.Errorf("coverage history has duplicate row for %s %s", row.Release, row.Summary.Package)
		}
		seen[key] = true
		if commit, ok := commits[row.Release]; ok && commit != row.Commit {
			return fmt.Errorf("coverage history release %s has conflicting commits", row.Release)
		}
		commits[row.Release] = row.Commit
		if row.Summary.Package == "(aggregate)" {
			if aggregates[row.Release] == nil {
				aggregates[row.Release] = map[string]bool{}
			}
			aggregates[row.Release][row.Summary.Module] = true
		}
	}
	for release := range commits {
		modules := aggregates[release]
		if !modules["root"] || !modules["contrib"] {
			return fmt.Errorf("coverage history release %s must include root and contrib aggregate rows", release)
		}
	}
	return nil
}

func validateSummaryRow(row summaryRow) error {
	if row.Module != "root" && row.Module != "contrib" {
		return fmt.Errorf("unknown module %q", row.Module)
	}
	if strings.TrimSpace(row.Package) == "" || strings.ContainsAny(row.Package, "\t\n\r") {
		return fmt.Errorf("invalid package %q", row.Package)
	}
	if strings.TrimSpace(row.API) == "" || strings.TrimSpace(row.Test) == "" {
		return errors.New("api_status and test_status are required")
	}
	return validateObserved(row.Observed)
}

func validateObserved(value string) error {
	switch value {
	case "no-statements", "no-test-files", "not-reported":
		return nil
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 || number > 100 {
		return fmt.Errorf("invalid observed coverage %q", value)
	}
	return nil
}

func encodeHistory(rows []historyRow) []byte {
	sortHistory(rows)
	var out strings.Builder
	out.WriteString(historyHeader)
	out.WriteByte('\n')
	for _, row := range rows {
		fmt.Fprintf(&out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.Release, row.Commit, row.Summary.Module, row.Summary.Package, row.Summary.API, row.Summary.Test, row.Summary.Observed)
	}
	return []byte(out.String())
}

func renderDocument(rows []historyRow) ([]byte, error) {
	orderedReleases := releases(rows)
	byRelease := map[string]map[string]historyRow{}
	commits := map[string]string{}
	for _, row := range rows {
		if byRelease[row.Release] == nil {
			byRelease[row.Release] = map[string]historyRow{}
		}
		byRelease[row.Release][rowKey(row.Summary)] = row
		commits[row.Release] = row.Commit
	}
	var out strings.Builder
	out.WriteString("# Package Coverage Trend\n\n")
	out.WriteString("Audience: maintainers and release reviewers comparing package-level test coverage across published releases.\n\n")
	out.WriteString("The machine-readable source is `docs/coverage-trend.tsv`. Coverage is a review signal, not a substitute for behavior, contract, race, fuzz, or security tests.\n\n")
	out.WriteString("## Method\n\n")
	out.WriteString("Each snapshot is measured at the tagged release commit with `GOWORK=off`, `GOTOOLCHAIN=local`, and the repository coverage command. Historical v3 snapshots were backfilled from the tagged source because early releases only retained aggregate coverage logs. `no statements`, `no test files`, and `not reported` are not numeric values and do not contribute to a percentage delta.\n\n")
	out.WriteString("Record the next release snapshot before tagging after `make coverage-check` succeeds:\n\n")
	out.WriteString("```sh\n")
	out.WriteString("COVERAGE_TREND_RELEASE=vX.Y.Z \\\n")
	out.WriteString("COVERAGE_TREND_COMMIT=\"$(git rev-parse --verify HEAD)\" \\\n")
	out.WriteString("GOTOOLCHAIN=local make coverage-trend-record\n")
	out.WriteString("GOTOOLCHAIN=local make coverage-trend-check\n")
	out.WriteString("```\n\n")
	out.WriteString("## Aggregate Trend\n\n")
	out.WriteString("| Release | Commit | Root | Contrib |\n| --- | --- | ---: | ---: |\n")
	for _, release := range orderedReleases {
		root, ok := byRelease[release][rowKey(summaryRow{Module: "root", Package: "(aggregate)"})]
		if !ok {
			return nil, fmt.Errorf("release %s is missing root aggregate coverage", release)
		}
		contrib, ok := byRelease[release][rowKey(summaryRow{Module: "contrib", Package: "(aggregate)"})]
		if !ok {
			return nil, fmt.Errorf("release %s is missing contrib aggregate coverage", release)
		}
		fmt.Fprintf(&out, "| %s | `%s` | %s | %s |\n", release, commits[release], markdownCoverage(root.Summary.Observed), markdownCoverage(contrib.Summary.Observed))
	}
	out.WriteString("\n## Package Trend\n\n")
	out.WriteString("The scope follows the package dashboard: stable and compatibility-only root packages plus the selected supported contrib adapters that have explicit floors. Values are ordered by release, and `Delta` compares the earliest and latest numeric values.\n\n")
	out.WriteString("| Module | Package | Status |")
	for _, release := range orderedReleases {
		fmt.Fprintf(&out, " %s |", release)
	}
	out.WriteString(" Delta |\n| --- | --- | --- |")
	for range orderedReleases {
		out.WriteString(" ---: |")
	}
	out.WriteString(" ---: |\n")
	for _, key := range packageKeys(rows) {
		latest, ok := byRelease[orderedReleases[len(orderedReleases)-1]][key]
		if !ok {
			latest = latestHistoryRow(rows, key)
		}
		fmt.Fprintf(&out, "| %s | `%s` | `%s` |", latest.Summary.Module, latest.Summary.Package, latest.Summary.API)
		values := make([]string, 0, len(orderedReleases))
		for _, release := range orderedReleases {
			row, ok := byRelease[release][key]
			if !ok {
				values = append(values, "not-reported")
				out.WriteString(" n/a |")
				continue
			}
			values = append(values, row.Summary.Observed)
			fmt.Fprintf(&out, " %s |", markdownCoverage(row.Summary.Observed))
		}
		fmt.Fprintf(&out, " %s |\n", coverageDelta(values[0], values[len(values)-1]))
	}
	return []byte(out.String()), nil
}

func markdownCoverage(value string) string {
	switch value {
	case "no-statements":
		return "no statements"
	case "no-test-files":
		return "no test files"
	case "not-reported":
		return "n/a"
	default:
		return value + "%"
	}
}

func coverageDelta(first, last string) string {
	start, startErr := strconv.ParseFloat(first, 64)
	end, endErr := strconv.ParseFloat(last, 64)
	if startErr != nil || endErr != nil {
		return "n/a"
	}
	delta := end - start
	if delta > 0 {
		return fmt.Sprintf("+%.1f pp", delta)
	}
	return fmt.Sprintf("%.1f pp", delta)
}

func releases(rows []historyRow) []string {
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.Release] = true
	}
	values := make([]string, 0, len(seen))
	for release := range seen {
		values = append(values, release)
	}
	sort.Slice(values, func(i, j int) bool { return compareRelease(values[i], values[j]) < 0 })
	return values
}

func packageKeys(rows []historyRow) []string {
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Summary.Package != "(aggregate)" {
			seen[rowKey(row.Summary)] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func latestHistoryRow(rows []historyRow, key string) historyRow {
	var latest historyRow
	for _, row := range rows {
		if rowKey(row.Summary) != key {
			continue
		}
		if latest.Release == "" || compareRelease(latest.Release, row.Release) < 0 {
			latest = row
		}
	}
	return latest
}

func hasRelease(rows []historyRow, release string) bool {
	for _, row := range rows {
		if row.Release == release {
			return true
		}
	}
	return false
}

func rowKey(row summaryRow) string {
	return row.Module + "\x00" + row.Package
}

func sortHistory(rows []historyRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Release != rows[j].Release {
			return compareRelease(rows[i].Release, rows[j].Release) < 0
		}
		if rows[i].Summary.Module != rows[j].Summary.Module {
			return rows[i].Summary.Module < rows[j].Summary.Module
		}
		return rows[i].Summary.Package < rows[j].Summary.Package
	})
}

func parseRelease(value string) (releaseVersion, error) {
	match := releasePattern.FindStringSubmatch(value)
	if len(match) == 0 {
		return releaseVersion{}, fmt.Errorf("release must be vX.Y.Z or vX.Y.Z-rc.N: %q", value)
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	version := releaseVersion{Major: major, Minor: minor, Patch: patch}
	if match[4] != "" {
		rc, _ := strconv.Atoi(match[5])
		version.IsRC = true
		version.RC = rc
	}
	return version, nil
}

func compareRelease(left, right string) int {
	a, _ := parseRelease(left)
	b, _ := parseRelease(right)
	for _, values := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if values[0] < values[1] {
			return -1
		}
		if values[0] > values[1] {
			return 1
		}
	}
	if a.IsRC != b.IsRC {
		if a.IsRC {
			return -1
		}
		return 1
	}
	if a.RC < b.RC {
		return -1
	}
	if a.RC > b.RC {
		return 1
	}
	return 0
}

func checkFile(path string, want []byte) error {
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s differs; run go run ./internal/tools/coveragetrend", path)
	}
	return nil
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func splitLines(content []byte) []string {
	text := strings.TrimSuffix(string(content), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
