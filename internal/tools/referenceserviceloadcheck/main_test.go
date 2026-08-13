package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfigRejectsEscapingPaths(t *testing.T) {
	_, err := parseConfig([]string{"-root", t.TempDir(), "-out", "../comparison.json"})
	if err == nil || !strings.Contains(err.Error(), "repository root") {
		t.Fatalf("parseConfig() error = %v, want repository-root rejection", err)
	}
}

func TestReadBaselineAcceptsFailureScenarioText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.tsv")
	row := strings.Join([]string{
		"reference_service_router", "local_in_process", "240", "8", "100", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"missing_api_key_get_widgets", "401", "0", "0", "make reference-service-load",
	}, "\t")
	if err := os.WriteFile(path, []byte(row+"\n"), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	b, err := readBaseline(path)
	if err != nil {
		t.Fatalf("readBaseline: %v", err)
	}
	if b.FailureScenario != "missing_api_key_get_widgets" || b.ExpectedFailureStatus != 401 {
		t.Fatalf("baseline = %#v", b)
	}
}

func TestCompareRejectsSecretLeakAndUnexpectedStatus(t *testing.T) {
	b := baseline{Throughput: 100, P95: 2, TotalAlloc: 1000, AllocsPerRequest: 10, BytesPerRequest: 100, ExpectedFailureStatus: 401}
	var s loadSummary
	s.Schema, s.Status, s.ThroughputRPS = "reference-service-load-smoke.v1", "passed", 100
	s.LatencyMS.P95 = 1
	s.Memory.TotalAllocDeltaBytes = 100
	s.Allocations.AllocsPerRequest, s.Allocations.BytesPerRequest = 1, 10
	s.FailureBehavior.ExpectedStatus = 401
	setRequiredRuntimeEvidence(&s)
	s.SecretLeakCount = 1
	c := compare(b, s)
	if c.Status != "failed" {
		t.Fatalf("status = %q, want failed", c.Status)
	}
}

func TestCompareAcceptsWithinReviewedBudgets(t *testing.T) {
	b := baseline{Throughput: 100, P95: 2, TotalAlloc: 1000, AllocsPerRequest: 10, BytesPerRequest: 100, ExpectedFailureStatus: 401}
	var s loadSummary
	s.Schema, s.Status, s.ThroughputRPS = "reference-service-load-smoke.v1", "passed", 20
	s.LatencyMS.P95 = 5
	s.Memory.TotalAllocDeltaBytes = 2000
	s.Allocations.AllocsPerRequest, s.Allocations.BytesPerRequest = 20, 200
	s.FailureBehavior.ExpectedStatus = 401
	setRequiredRuntimeEvidence(&s)
	c := compare(b, s)
	if c.Status != "passed" {
		t.Fatalf("status = %q, want passed", c.Status)
	}
}

func TestCompareRejectsAllocationRegression(t *testing.T) {
	b := baseline{Throughput: 100, P95: 2, TotalAlloc: 1000, AllocsPerRequest: 10, BytesPerRequest: 100, ExpectedFailureStatus: 401}
	var s loadSummary
	s.Schema, s.Status, s.ThroughputRPS = "reference-service-load-smoke.v1", "passed", 100
	s.LatencyMS.P95 = 1
	s.Memory.TotalAllocDeltaBytes = 100
	s.Allocations.AllocsPerRequest, s.Allocations.BytesPerRequest = 31, 100
	s.FailureBehavior.ExpectedStatus = 401
	setRequiredRuntimeEvidence(&s)
	c := compare(b, s)
	if c.Status != "failed" {
		t.Fatalf("status = %q, want allocation regression to fail", c.Status)
	}
}

func setRequiredRuntimeEvidence(s *loadSummary) {
	s.GoroutinePeak = 4
	s.GracefulShutdownMS = 1
	s.Environment.Profile = "test-profile"
	s.Environment.Commit = "unknown"
	s.Environment.GoVersion = "go-test"
	s.Environment.GOOS = "linux"
	s.Environment.GOARCH = "amd64"
	s.Environment.GOMAXPROCS = 1
}
