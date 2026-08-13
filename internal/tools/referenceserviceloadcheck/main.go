// Command referenceserviceloadcheck compares bounded local reference-service
// load evidence with the reviewed seed baseline. It does not publish evidence
// or make portability claims for a different deployment environment.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type config struct{ root, baseline, summary, out string }

type baseline struct {
	Scenario, Mode, FailureScenario                                                                   string
	Evidence                                                                                          string `json:"-"`
	Requests, Concurrency, ExpectedFailureStatus, Unexpected, SecretLeaks                             int
	Throughput, P50, P95, P99, Max, HeapDelta, TotalAlloc, Mallocs, AllocsPerRequest, BytesPerRequest float64
}

type loadSummary struct {
	Schema                string  `json:"schema"`
	Status                string  `json:"status"`
	ThroughputRPS         float64 `json:"throughput_rps"`
	UnexpectedStatusCount int     `json:"unexpected_status_count"`
	SecretLeakCount       int     `json:"secret_leak_count"`
	LatencyMS             struct {
		P50 float64 `json:"p50"`
		P95 float64 `json:"p95"`
		P99 float64 `json:"p99"`
		Max float64 `json:"max"`
	} `json:"latency_ms"`
	Memory struct {
		HeapAllocDeltaBytes  int64 `json:"heap_alloc_delta_bytes"`
		TotalAllocDeltaBytes int64 `json:"total_alloc_delta_bytes"`
	} `json:"memory"`
	Allocations struct {
		MallocsDelta     uint64  `json:"mallocs_delta"`
		AllocsPerRequest float64 `json:"allocs_per_request"`
		BytesPerRequest  float64 `json:"bytes_per_request"`
	} `json:"allocations"`
	Limits struct {
		RateLimitResponses int `json:"rate_limit_responses"`
		Timeouts           int `json:"timeouts"`
	} `json:"limits"`
	FailureBehavior struct {
		Scenario              string `json:"scenario"`
		ExpectedStatus        int    `json:"expected_status"`
		UnexpectedStatusCount int    `json:"unexpected_status_count"`
		SecretLeakCount       int    `json:"secret_leak_count"`
	} `json:"failure_behavior"`
	GoroutinePeak      int     `json:"goroutine_peak"`
	GracefulShutdownMS float64 `json:"graceful_shutdown_ms"`
	Environment        struct {
		Profile    string `json:"profile"`
		Commit     string `json:"commit"`
		GoVersion  string `json:"go_version"`
		GOOS       string `json:"goos"`
		GOARCH     string `json:"goarch"`
		GOMAXPROCS int    `json:"gomaxprocs"`
	} `json:"environment"`
}

type check struct {
	Name   string `json:"name"`
	Actual string `json:"actual"`
	Limit  string `json:"limit"`
	Passed bool   `json:"passed"`
}

type comparison struct {
	Schema   string   `json:"schema"`
	Status   string   `json:"status"`
	Baseline baseline `json:"baseline"`
	Checks   []check  `json:"checks"`
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseConfig(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	b, err := readBaseline(cfg.baseline)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	s, err := readSummary(cfg.summary)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	c := compare(b, s)
	payload, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(cfg.out), 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.WriteFile(cfg.out, payload, 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "reference-service load comparison %s; report=%s\n", c.Status, cfg.out)
	if c.Status != "passed" {
		return 1
	}
	return 0
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("reference-service-load-check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository root")
	baselinePath := fs.String("baseline", "docs/reference-service-load-baseline.tsv", "reviewed baseline")
	summaryPath := fs.String("summary", ".ci-result/reference-service-load/summary.json", "load summary")
	outPath := fs.String("out", ".ci-result/reference-service-load/comparison.json", "comparison output")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return config{}, err
	}
	resolve := func(value string) (string, error) {
		if filepath.IsAbs(value) {
			return "", errors.New("paths must be repository-relative")
		}
		clean := filepath.Clean(value)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", errors.New("paths must stay below the repository root")
		}
		return filepath.Join(absRoot, clean), nil
	}
	b, err := resolve(*baselinePath)
	if err != nil {
		return config{}, err
	}
	s, err := resolve(*summaryPath)
	if err != nil {
		return config{}, err
	}
	o, err := resolve(*outPath)
	if err != nil {
		return config{}, err
	}
	return config{root: absRoot, baseline: b, summary: s, out: o}, nil
}

func readBaseline(path string) (baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return baseline{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 19 {
			return baseline{}, fmt.Errorf("baseline must have 19 columns")
		}
		ints := []int{2, 3, 15, 16, 17}
		floats := []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
		for _, i := range ints {
			if _, err := strconv.Atoi(cols[i]); err != nil {
				return baseline{}, fmt.Errorf("invalid baseline integer %q", cols[i])
			}
		}
		for _, i := range floats {
			if _, err := strconv.ParseFloat(cols[i], 64); err != nil {
				return baseline{}, fmt.Errorf("invalid baseline number %q", cols[i])
			}
		}
		requests, _ := strconv.Atoi(cols[2])
		concurrency, _ := strconv.Atoi(cols[3])
		expected, _ := strconv.Atoi(cols[15])
		unexpected, _ := strconv.Atoi(cols[16])
		leaks, _ := strconv.Atoi(cols[17])
		parse := func(i int) float64 { v, _ := strconv.ParseFloat(cols[i], 64); return v }
		return baseline{Scenario: cols[0], Mode: cols[1], Requests: requests, Concurrency: concurrency, Throughput: parse(4), P50: parse(5), P95: parse(6), P99: parse(7), Max: parse(8), HeapDelta: parse(9), TotalAlloc: parse(10), Mallocs: parse(11), AllocsPerRequest: parse(12), BytesPerRequest: parse(13), FailureScenario: cols[14], ExpectedFailureStatus: expected, Unexpected: unexpected, SecretLeaks: leaks, Evidence: cols[18]}, nil
	}
	return baseline{}, errors.New("baseline has no data row")
}

func readSummary(path string) (loadSummary, error) {
	var s loadSummary
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if s.Schema != "reference-service-load-smoke.v1" {
		return s, errors.New("unexpected load summary schema")
	}
	return s, nil
}

func compare(b baseline, s loadSummary) comparison {
	checks := []check{}
	add := func(name string, passed bool, actual, limit any) {
		checks = append(checks, check{Name: name, Passed: passed, Actual: fmt.Sprint(actual), Limit: fmt.Sprint(limit)})
	}
	add("smoke-status", s.Status == "passed", s.Status, "passed")
	add("expected-failure-status", s.FailureBehavior.ExpectedStatus == b.ExpectedFailureStatus, s.FailureBehavior.ExpectedStatus, b.ExpectedFailureStatus)
	add("unexpected-statuses", s.UnexpectedStatusCount <= b.Unexpected && s.FailureBehavior.UnexpectedStatusCount <= b.Unexpected, s.UnexpectedStatusCount, b.Unexpected)
	add("secret-leaks", s.SecretLeakCount <= b.SecretLeaks && s.FailureBehavior.SecretLeakCount <= b.SecretLeaks, s.SecretLeakCount, b.SecretLeaks)
	add("timeouts", s.Limits.Timeouts == 0, s.Limits.Timeouts, 0)
	add("goroutine-peak", s.GoroutinePeak > 0 && s.GoroutinePeak <= maxInt(64, b.Concurrency*8), s.GoroutinePeak, maxInt(64, b.Concurrency*8))
	add("graceful-shutdown-ms", s.GracefulShutdownMS >= 0 && s.GracefulShutdownMS <= 1000, s.GracefulShutdownMS, 1000)
	add("environment-metadata", hasEnvironmentMetadata(s), environmentLabel(s), "profile, commit, Go version, platform, GOMAXPROCS")
	add("throughput", s.ThroughputRPS >= b.Throughput*0.10, s.ThroughputRPS, b.Throughput*0.10)
	add("p95-latency-ms", s.LatencyMS.P95 <= b.P95*5, s.LatencyMS.P95, b.P95*5)
	add("total-allocation-bytes", float64(s.Memory.TotalAllocDeltaBytes) <= b.TotalAlloc*3, s.Memory.TotalAllocDeltaBytes, b.TotalAlloc*3)
	add("allocations-per-request", s.Allocations.AllocsPerRequest <= b.AllocsPerRequest*3, s.Allocations.AllocsPerRequest, b.AllocsPerRequest*3)
	add("bytes-per-request", s.Allocations.BytesPerRequest <= b.BytesPerRequest*3, s.Allocations.BytesPerRequest, b.BytesPerRequest*3)
	status := "passed"
	for _, c := range checks {
		if !c.Passed {
			status = "failed"
			break
		}
	}
	return comparison{Schema: "reference-service-load-comparison.v1", Status: status, Baseline: b, Checks: checks}
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func hasEnvironmentMetadata(s loadSummary) bool {
	return s.Environment.Profile != "" && s.Environment.Commit != "" && s.Environment.GoVersion != "" &&
		s.Environment.GOOS != "" && s.Environment.GOARCH != "" && s.Environment.GOMAXPROCS > 0
}

func environmentLabel(s loadSummary) string {
	return strings.Join([]string{s.Environment.Profile, s.Environment.Commit, s.Environment.GoVersion, s.Environment.GOOS + "/" + s.Environment.GOARCH, strconv.Itoa(s.Environment.GOMAXPROCS)}, ";")
}
