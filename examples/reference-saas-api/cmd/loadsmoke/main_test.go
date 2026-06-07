package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateOptionsRejectsUnboundedLoad(t *testing.T) {
	tests := []struct {
		name string
		opts loadOptions
	}{
		{name: "zero requests", opts: loadOptions{Requests: 0, Concurrency: 1, OutDir: "out"}},
		{name: "excessive requests", opts: loadOptions{Requests: maxRequests + 1, Concurrency: 1, OutDir: "out"}},
		{name: "zero concurrency", opts: loadOptions{Requests: 1, Concurrency: 0, OutDir: "out"}},
		{name: "excessive concurrency", opts: loadOptions{Requests: maxConcurrency + 1, Concurrency: maxConcurrency + 1, OutDir: "out"}},
		{name: "concurrency exceeds requests", opts: loadOptions{Requests: 1, Concurrency: 2, OutDir: "out"}},
		{name: "empty output", opts: loadOptions{Requests: 1, Concurrency: 1, OutDir: "   "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateOptions(tt.opts); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSummarizeLatencyUsesNearestRankPercentiles(t *testing.T) {
	got := summarizeLatency([]time.Duration{
		time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
	})
	if got.Min != 1 || got.Mean != 2.5 || got.P50 != 2 || got.P95 != 4 || got.P99 != 4 || got.Max != 4 {
		t.Fatalf("latency summary = %#v", got)
	}
}

func TestBuildSummaryRecordsFailureBehaviorAndSecretLeaks(t *testing.T) {
	results := []requestResult{
		{Operation: "list_widgets", ExpectedStatus: 200, Status: 200, Duration: time.Millisecond},
		{Operation: "auth_failure", ExpectedStatus: 401, ExpectedFailure: true, Status: 401, ProblemDetails: true, Duration: time.Millisecond},
		{Operation: "create_widget", ExpectedStatus: 201, Status: 429, Duration: time.Millisecond},
		{Operation: "auth_failure", ExpectedStatus: 401, ExpectedFailure: true, Status: 401, ProblemDetails: true, SecretLeak: true, Duration: time.Millisecond},
	}
	summary := buildSummary(
		loadOptions{Requests: len(results), Concurrency: 2},
		time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC),
		10*time.Millisecond,
		runtime.MemStats{HeapAlloc: 100, TotalAlloc: 1000, Mallocs: 10},
		runtime.MemStats{HeapAlloc: 400, TotalAlloc: 1600, Mallocs: 22},
		results,
	)
	if summary.Status != "failed" {
		t.Fatalf("status = %q, want failed", summary.Status)
	}
	if summary.UnexpectedStatusCount != 1 || summary.SecretLeakCount != 1 || summary.Limits.RateLimitResponses != 1 {
		t.Fatalf("summary counts = unexpected %d secret %d rate-limit %d", summary.UnexpectedStatusCount, summary.SecretLeakCount, summary.Limits.RateLimitResponses)
	}
	if summary.FailureBehavior.ExpectedStatus != 401 || summary.FailureBehavior.ExpectedFailureCount != 2 || summary.FailureBehavior.ProblemDetailsCount != 2 || summary.FailureBehavior.SecretLeakCount != 1 {
		t.Fatalf("failure behavior = %#v", summary.FailureBehavior)
	}
	if summary.Allocations.MallocsDelta != 12 || summary.Memory.TotalAllocDeltaBytes != 600 {
		t.Fatalf("allocation summary = %#v memory = %#v", summary.Allocations, summary.Memory)
	}
}

func TestWriteEvidenceCreatesStatusJSONAndMarkdown(t *testing.T) {
	outDir := t.TempDir()
	summary := loadSummary{
		Schema:        "reference-service-load-smoke.v1",
		Status:        "passed",
		Requests:      2,
		Concurrency:   1,
		ThroughputRPS: 100,
		LatencyMS:     latencySummary{P95: 1.25},
		FailureBehavior: failureBehaviorSummary{
			Scenario:             "missing API key on GET /widgets",
			ExpectedStatus:       401,
			ExpectedFailureCount: 1,
			ProblemDetailsCount:  1,
		},
		Operations: map[string]operationSummary{
			"auth_failure": {Count: 1, ExpectedStatus: 401, ExpectedFailure: true, StatusCounts: map[string]int{"401": 1}, ProblemDetailsCount: 1},
		},
	}
	if err := writeEvidence(outDir, summary); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	status, err := os.ReadFile(filepath.Join(outDir, "status"))
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if strings.TrimSpace(string(status)) != "passed" {
		t.Fatalf("status file = %q", status)
	}
	for _, file := range []string{"summary.json", "summary.md"} {
		content, err := os.ReadFile(filepath.Join(outDir, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, required := range []string{"reference-service-load-smoke.v1", "auth_failure"} {
			if !strings.Contains(string(content), required) {
				t.Fatalf("%s missing %q:\n%s", file, required, content)
			}
		}
	}
}
