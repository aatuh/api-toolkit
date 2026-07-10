package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSummaryRejectsMalformedRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.tsv")
	writeTestFile(t, path, summaryHeader+"\nroot\t(aggregate)\taggregate\taggregate\tfloor\t70\t101\tnote\n")
	if _, err := readSummary(path); err == nil {
		t.Fatal("readSummary accepted coverage above 100%")
	}
}

func TestReadLegacySnapshotUsesLogsAndFunctionTotals(t *testing.T) {
	dir := t.TempDir()
	summary := filepath.Join(dir, "summary.tsv")
	rootLog := filepath.Join(dir, "root.log")
	contribLog := filepath.Join(dir, "contrib.log")
	rootFunc := filepath.Join(dir, "root.func")
	contribFunc := filepath.Join(dir, "contrib.func")
	writeTestFile(t, summary, strings.Join([]string{
		summaryHeader,
		"root\t(aggregate)\taggregate\taggregate\tfloor\t70\t0\tnote",
		"contrib\t(aggregate)\taggregate\taggregate\tfloor\t52\t0\tnote",
		"root\texample.com/root/a\tstable\tdirect-tests\tfloor\t70\t0\tnote",
		"root\texample.com/root/b\tstable\tdirect-tests\tfloor\t70\t0\tnote",
		"contrib\texample.com/contrib/c\tsupported-adapter\tdirect-tests\tfloor\t52\t0\tnote",
	}, "\n")+"\n")
	writeTestFile(t, rootLog, "ok  \texample.com/root/a\t0.01s\tcoverage: 75.5% of statements\n?   \texample.com/root/b\t[no test files]\n")
	writeTestFile(t, contribLog, "ok  \texample.com/contrib/c\t0.01s\tcoverage: [no statements]\n")
	writeTestFile(t, rootFunc, "total:\t(statements)\t72.1%\n")
	writeTestFile(t, contribFunc, "total:\t(statements)\t61.2%\n")

	rows, err := readLegacySnapshot(config{Summary: summary, RootLog: rootLog, RootFunc: rootFunc, ContribLog: contribLog, ContribFunc: contribFunc})
	if err != nil {
		t.Fatalf("readLegacySnapshot: %v", err)
	}
	got := map[string]string{}
	for _, row := range rows {
		got[rowKey(row)] = row.Observed
	}
	for key, want := range map[string]string{
		"root\x00(aggregate)":              "72.1",
		"contrib\x00(aggregate)":           "61.2",
		"root\x00example.com/root/a":       "75.5",
		"root\x00example.com/root/b":       "no-test-files",
		"contrib\x00example.com/contrib/c": "no-statements",
	} {
		if got[key] != want {
			t.Fatalf("coverage %s = %q, want %q", key, got[key], want)
		}
	}
}

func TestRenderDocumentOrdersReleasesAndCalculatesDelta(t *testing.T) {
	rows := []historyRow{
		{Release: "v3.1.0", Commit: "1234567", Summary: summaryRow{Module: "root", Package: "(aggregate)", API: "aggregate", Test: "aggregate", Observed: "75.0"}},
		{Release: "v3.1.0", Commit: "1234567", Summary: summaryRow{Module: "contrib", Package: "(aggregate)", API: "aggregate", Test: "aggregate", Observed: "66.0"}},
		{Release: "v3.0.0", Commit: "abcdef0", Summary: summaryRow{Module: "root", Package: "(aggregate)", API: "aggregate", Test: "aggregate", Observed: "70.0"}},
		{Release: "v3.0.0", Commit: "abcdef0", Summary: summaryRow{Module: "contrib", Package: "(aggregate)", API: "aggregate", Test: "aggregate", Observed: "60.0"}},
		{Release: "v3.1.0", Commit: "1234567", Summary: summaryRow{Module: "root", Package: "example.com/root/a", API: "stable", Test: "direct-tests", Observed: "80.0"}},
		{Release: "v3.0.0", Commit: "abcdef0", Summary: summaryRow{Module: "root", Package: "example.com/root/a", API: "stable", Test: "direct-tests", Observed: "70.0"}},
	}
	document, err := renderDocument(rows)
	if err != nil {
		t.Fatalf("renderDocument: %v", err)
	}
	text := string(document)
	if strings.Index(text, "| v3.0.0") > strings.Index(text, "| v3.1.0") {
		t.Fatal("aggregate releases are not ordered")
	}
	if !strings.Contains(text, "| root | `example.com/root/a` | `stable` | 70.0% | 80.0% | +10.0 pp |") {
		t.Fatalf("document missing package delta:\n%s", text)
	}
}

func TestValidateHistoryRequiresBothAggregateModules(t *testing.T) {
	err := validateHistory([]historyRow{{
		Release: "v3.0.0",
		Commit:  "abcdef0",
		Summary: summaryRow{Module: "root", Package: "example.com/root/a", API: "stable", Test: "direct-tests", Observed: "70.0"},
	}})
	if err == nil || !strings.Contains(err.Error(), "aggregate") {
		t.Fatalf("validateHistory error = %v, want aggregate requirement", err)
	}
}

func TestReadAggregateAcceptsExplicitNumericCoverage(t *testing.T) {
	got, err := readAggregate("", "66.1")
	if err != nil {
		t.Fatalf("readAggregate: %v", err)
	}
	if got != "66.1" {
		t.Fatalf("aggregate = %q, want 66.1", got)
	}
	if _, err := readAggregate("", "no-statements"); err == nil {
		t.Fatal("readAggregate accepted non-numeric aggregate coverage")
	}
}

func TestRunRecordsAndChecksCanonicalHistory(t *testing.T) {
	dir := t.TempDir()
	history := filepath.Join(dir, "coverage-trend.tsv")
	out := filepath.Join(dir, "coverage-trend.md")
	summary := filepath.Join(dir, "summary.tsv")
	writeTestFile(t, summary, strings.Join([]string{
		summaryHeader,
		"root\t(aggregate)\taggregate\taggregate\tfloor\t70\t70.0\tnote",
		"contrib\t(aggregate)\taggregate\taggregate\tfloor\t52\t60.0\tnote",
		"root\texample.com/root/a\tstable\tdirect-tests\tfloor\t70\t75.0\tnote",
	}, "\n")+"\n")
	var stdout bytes.Buffer
	if err := run([]string{"-history", history, "-out", out, "-summary", summary, "-record", "v3.0.0", "-commit", "abcdef0"}, &stdout); err != nil {
		t.Fatalf("record coverage trend: %v", err)
	}
	if err := run([]string{"-history", history, "-out", out, "-check"}, &stdout); err != nil {
		t.Fatalf("check coverage trend: %v", err)
	}
	if err := run([]string{"-history", history, "-out", out, "-summary", summary, "-record", "v3.0.0", "-commit", "abcdef0"}, &stdout); err == nil {
		t.Fatal("record accepted duplicate release")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	//nolint:gosec // Test fixture reports must be readable by the test process.
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
