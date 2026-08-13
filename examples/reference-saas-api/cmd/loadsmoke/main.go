package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ratelimitmw "github.com/aatuh/api-toolkit/v4/middleware/ratelimit"

	"example.com/reference-saas-api/internal/httpapi"
)

const (
	defaultRequests    = 240
	defaultConcurrency = 8
	defaultOutDir      = ".ci-result/reference-service-load"
	maxRequests        = 100000
	maxConcurrency     = 512
	requestTimeout     = 2 * time.Second

	loadAuthHeaderValue  = "reference-load-auth-header"
	loadAdminHeaderValue = "reference-load-admin-header"
	loadActorID          = "reference-load-actor"
)

type loadOptions struct {
	Requests    int
	Concurrency int
	OutDir      string
	Commit      string
	Profile     string
	Now         func() time.Time
}

type loadSummary struct {
	Schema                string                      `json:"schema"`
	Status                string                      `json:"status"`
	Timestamp             string                      `json:"timestamp"`
	Requests              int                         `json:"requests"`
	Concurrency           int                         `json:"concurrency"`
	DurationMS            float64                     `json:"duration_ms"`
	ThroughputRPS         float64                     `json:"throughput_rps"`
	ExpectedFailureCount  int                         `json:"expected_failure_count"`
	ExpectedFailureRate   float64                     `json:"expected_failure_rate"`
	UnexpectedStatusCount int                         `json:"unexpected_status_count"`
	UnexpectedStatusRate  float64                     `json:"unexpected_status_rate"`
	SecretLeakCount       int                         `json:"secret_leak_count"`
	LatencyMS             latencySummary              `json:"latency_ms"`
	Memory                memorySummary               `json:"memory"`
	Allocations           allocationSummary           `json:"allocations"`
	Limits                limitSummary                `json:"limits"`
	FailureBehavior       failureBehaviorSummary      `json:"failure_behavior"`
	Operations            map[string]operationSummary `json:"operations"`
	GoroutinePeak         int                         `json:"goroutine_peak"`
	GracefulShutdownMS    float64                     `json:"graceful_shutdown_ms"`
	Environment           environmentMetadata         `json:"environment"`
}

type environmentMetadata struct {
	Profile    string `json:"profile"`
	Commit     string `json:"commit"`
	GoVersion  string `json:"go_version"`
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	GOMAXPROCS int    `json:"gomaxprocs"`
}

type latencySummary struct {
	Min  float64 `json:"min"`
	Mean float64 `json:"mean"`
	P50  float64 `json:"p50"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	Max  float64 `json:"max"`
}

type memorySummary struct {
	HeapAllocBeforeBytes int64 `json:"heap_alloc_before_bytes"`
	HeapAllocAfterBytes  int64 `json:"heap_alloc_after_bytes"`
	HeapAllocDeltaBytes  int64 `json:"heap_alloc_delta_bytes"`
	TotalAllocDeltaBytes int64 `json:"total_alloc_delta_bytes"`
}

type allocationSummary struct {
	MallocsDelta     uint64  `json:"mallocs_delta"`
	AllocsPerRequest float64 `json:"allocs_per_request"`
	BytesPerRequest  float64 `json:"bytes_per_request"`
}

type limitSummary struct {
	RateLimitResponses int `json:"rate_limit_responses"`
	Timeouts           int `json:"timeouts"`
}

type failureBehaviorSummary struct {
	Scenario              string  `json:"scenario"`
	ExpectedStatus        int     `json:"expected_status"`
	ExpectedFailureCount  int     `json:"expected_failure_count"`
	ProblemDetailsCount   int     `json:"problem_details_count"`
	UnexpectedStatusCount int     `json:"unexpected_status_count"`
	SecretLeakCount       int     `json:"secret_leak_count"`
	FailureRate           float64 `json:"failure_rate"`
}

type operationSummary struct {
	Count                 int            `json:"count"`
	ExpectedStatus        int            `json:"expected_status"`
	ExpectedFailure       bool           `json:"expected_failure"`
	StatusCounts          map[string]int `json:"status_counts"`
	ProblemDetailsCount   int            `json:"problem_details_count"`
	UnexpectedStatusCount int            `json:"unexpected_status_count"`
	SecretLeakCount       int            `json:"secret_leak_count"`
	Timeouts              int            `json:"timeouts"`
}

type requestSpec struct {
	Name            string
	Method          string
	Path            string
	Body            func(int) string
	ExpectedStatus  int
	ExpectedFailure bool
	WithAPIKey      bool
	TenantID        string
	ActorID         string
	IdempotencyKey  func(int) string
}

type requestResult struct {
	Operation       string
	ExpectedStatus  int
	ExpectedFailure bool
	Status          int
	Duration        time.Duration
	ProblemDetails  bool
	SecretLeak      bool
	Timeout         bool
	Body            string
}

func main() {
	opts := loadOptions{Now: time.Now}
	flag.IntVar(&opts.Requests, "requests", defaultRequests, "number of in-process HTTP requests to issue")
	flag.IntVar(&opts.Concurrency, "concurrency", defaultConcurrency, "number of concurrent workers")
	flag.StringVar(&opts.OutDir, "out", defaultOutDir, "directory for status, summary.json, and summary.md")
	flag.StringVar(&opts.Commit, "commit", os.Getenv("REFERENCE_SERVICE_LOAD_COMMIT"), "source commit identifier or unknown")
	flag.StringVar(&opts.Profile, "profile", os.Getenv("REFERENCE_SERVICE_LOAD_PROFILE"), "controlled-runner profile identifier")
	flag.Parse()

	summary, err := runLoadSmoke(context.Background(), opts)
	if summary.Status != "" {
		fmt.Printf("reference-service load smoke %s; summary=%s\n", summary.Status, filepath.Join(opts.OutDir, "summary.json"))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "reference-service load smoke failed: %v\n", err)
		os.Exit(1)
	}
}

func runLoadSmoke(ctx context.Context, opts loadOptions) (loadSummary, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Commit == "" {
		opts.Commit = "unknown"
	}
	if opts.Profile == "" {
		opts.Profile = "local-in-process"
	}
	if err := validateOptions(opts); err != nil {
		return loadSummary{}, err
	}
	restoreEnv := configureLoadSmokeEnv()
	defer restoreEnv()

	handler, err := newLoadRouter(opts.Requests)
	if err != nil {
		return loadSummary{}, err
	}
	tenantID, err := seedTenant(handler)
	if err != nil {
		return loadSummary{}, err
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	results, goroutinePeak, shutdownDuration := executeRequests(ctx, handler, tenantID, opts)
	duration := time.Since(started)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	summary := buildSummary(opts, opts.Now().UTC(), duration, before, after, results, goroutinePeak, shutdownDuration)
	if err := writeEvidence(opts.OutDir, summary); err != nil {
		return summary, err
	}
	if summary.Status != "passed" {
		return summary, fmt.Errorf("%d unexpected statuses, %d secret leaks, %d timeouts",
			summary.UnexpectedStatusCount, summary.SecretLeakCount, summary.Limits.Timeouts)
	}
	return summary, nil
}

func validateOptions(opts loadOptions) error {
	if opts.Requests < 1 || opts.Requests > maxRequests {
		return fmt.Errorf("requests must be between 1 and %d", maxRequests)
	}
	if opts.Concurrency < 1 || opts.Concurrency > maxConcurrency {
		return fmt.Errorf("concurrency must be between 1 and %d", maxConcurrency)
	}
	if opts.Concurrency > opts.Requests {
		return errors.New("concurrency cannot exceed requests")
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return errors.New("out directory is required")
	}
	if opts.Commit != "unknown" && !isCommitID(opts.Commit) {
		return errors.New("commit must be a lowercase hexadecimal identifier or unknown")
	}
	if !isMetadataIdentifier(opts.Profile) {
		return errors.New("profile must be a non-empty controlled-runner identifier")
	}
	return nil
}

func isCommitID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func isMetadataIdentifier(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

func configureLoadSmokeEnv() func() {
	previousENV, hadENV := os.LookupEnv("ENV")
	previousActor, hadActor := os.LookupEnv("API_ACTOR_ID")
	_ = os.Setenv("ENV", "test")
	_ = os.Unsetenv("API_ACTOR_ID")
	return func() {
		restoreEnv("ENV", previousENV, hadENV)
		restoreEnv("API_ACTOR_ID", previousActor, hadActor)
	}
}

func restoreEnv(name, value string, hadValue bool) {
	if hadValue {
		_ = os.Setenv(name, value)
		return
	}
	_ = os.Unsetenv(name)
}

func newLoadRouter(requests int) (http.Handler, error) {
	capacity := float64(requests + 8)
	if capacity < 32 {
		capacity = 32
	}
	rateLimit, err := ratelimitmw.New(ratelimitmw.Options{
		Capacity:     capacity,
		RefillRate:   capacity,
		RetryAfter:   time.Second,
		Key:          func(*http.Request) string { return "reference-service-load-smoke" },
		HeaderConfig: ratelimitmw.DefaultHeaderConfig(),
	})
	if err != nil {
		return nil, err
	}
	return httpapi.NewRouter(httpapi.RouterConfig{
		APIKey:    loadAuthHeaderValue,
		AdminKey:  loadAdminHeaderValue,
		RateLimit: rateLimit,
	}), nil
}

func seedTenant(handler http.Handler) (string, error) {
	result := performRequest(handler, requestSpec{
		Name:           "seed_organization",
		Method:         http.MethodPost,
		Path:           "/organizations",
		Body:           func(int) string { return `{"name":"Reference Load Baseline"}` },
		ExpectedStatus: http.StatusCreated,
		WithAPIKey:     true,
		ActorID:        loadActorID,
		IdempotencyKey: func(int) string { return "reference-load-seed-organization" },
	}, -1)
	if result.Status != http.StatusCreated {
		return "", fmt.Errorf("seed organization status=%d body=%s", result.Status, result.Body)
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(result.Body), &body); err != nil {
		return "", fmt.Errorf("decode seed organization: %w", err)
	}
	if strings.TrimSpace(body.ID) == "" {
		return "", errors.New("seed organization response missing id")
	}
	return body.ID, nil
}

func executeRequests(ctx context.Context, handler http.Handler, tenantID string, opts loadOptions) ([]requestResult, int, time.Duration) {
	jobs := make(chan int)
	results := make(chan requestResult, opts.Requests)
	var wg sync.WaitGroup
	goroutinePeak := runtime.NumGoroutine()
	for worker := 0; worker < opts.Concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				select {
				case <-ctx.Done():
					results <- requestResult{
						Operation:       "cancelled",
						ExpectedStatus:  http.StatusOK,
						ExpectedFailure: false,
						Status:          0,
						Timeout:         true,
					}
				default:
					results <- performRequest(handler, operationFor(index, tenantID), index)
				}
			}
		}()
		if current := runtime.NumGoroutine(); current > goroutinePeak {
			goroutinePeak = current
		}
	}
	for index := 0; index < opts.Requests; index++ {
		jobs <- index
	}
	shutdownStarted := time.Now()
	close(jobs)
	wg.Wait()
	shutdownDuration := time.Since(shutdownStarted)
	close(results)

	out := make([]requestResult, 0, opts.Requests)
	for result := range results {
		out = append(out, result)
	}
	return out, goroutinePeak, shutdownDuration
}

func operationFor(index int, tenantID string) requestSpec {
	switch {
	case index%10 == 0:
		return requestSpec{
			Name:            "auth_failure",
			Method:          http.MethodGet,
			Path:            "/widgets",
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedFailure: true,
			TenantID:        tenantID,
		}
	case index%4 == 0:
		return requestSpec{
			Name:           "readiness",
			Method:         http.MethodGet,
			Path:           "/readyz",
			ExpectedStatus: http.StatusOK,
		}
	case index%3 == 0:
		return requestSpec{
			Name:           "create_widget",
			Method:         http.MethodPost,
			Path:           "/widgets",
			Body:           func(i int) string { return fmt.Sprintf(`{"name":"load-widget-%06d"}`, i) },
			ExpectedStatus: http.StatusCreated,
			WithAPIKey:     true,
			TenantID:       tenantID,
			ActorID:        loadActorID,
			IdempotencyKey: func(i int) string { return fmt.Sprintf("reference-load-create-widget-%06d", i) },
		}
	default:
		return requestSpec{
			Name:           "list_widgets",
			Method:         http.MethodGet,
			Path:           "/widgets",
			ExpectedStatus: http.StatusOK,
			WithAPIKey:     true,
			TenantID:       tenantID,
			ActorID:        loadActorID,
		}
	}
}

func performRequest(handler http.Handler, spec requestSpec, index int) requestResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	body := ""
	if spec.Body != nil {
		body = spec.Body(index)
	}
	req := httptest.NewRequest(spec.Method, spec.Path, strings.NewReader(body)).WithContext(ctx)
	req.RemoteAddr = "127.0.0.1:12345"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if spec.WithAPIKey {
		req.Header.Set("X-API-Key", loadAuthHeaderValue)
	}
	if spec.TenantID != "" {
		req.Header.Set("X-Tenant-ID", spec.TenantID)
	}
	if spec.ActorID != "" {
		req.Header.Set("X-Actor-ID", spec.ActorID)
	}
	if spec.IdempotencyKey != nil {
		req.Header.Set("Idempotency-Key", spec.IdempotencyKey(index))
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	response := rec.Result()
	defer response.Body.Close()

	responseBody := rec.Body.String()
	contentType := response.Header.Get("Content-Type")
	return requestResult{
		Operation:       spec.Name,
		ExpectedStatus:  spec.ExpectedStatus,
		ExpectedFailure: spec.ExpectedFailure,
		Status:          rec.Code,
		Duration:        time.Since(started),
		ProblemDetails:  strings.Contains(contentType, "application/problem+json"),
		SecretLeak:      leaksSyntheticSecret(responseBody),
		Timeout:         ctx.Err() == context.DeadlineExceeded,
		Body:            responseBody,
	}
}

func leaksSyntheticSecret(body string) bool {
	for _, secret := range []string{loadAuthHeaderValue, loadAdminHeaderValue} {
		if secret != "" && strings.Contains(body, secret) {
			return true
		}
	}
	return false
}

func buildSummary(opts loadOptions, timestamp time.Time, duration time.Duration, before, after runtime.MemStats, results []requestResult, goroutinePeak int, shutdownDuration time.Duration) loadSummary {
	operations := make(map[string]operationSummary)
	durations := make([]time.Duration, 0, len(results))
	var expectedFailures, unexpected, secretLeaks, rateLimited, timeouts int
	for _, result := range results {
		durations = append(durations, result.Duration)
		op := operations[result.Operation]
		if op.StatusCounts == nil {
			op.StatusCounts = map[string]int{}
		}
		op.Count++
		op.ExpectedStatus = result.ExpectedStatus
		op.ExpectedFailure = result.ExpectedFailure
		op.StatusCounts[strconv.Itoa(result.Status)]++
		if result.ProblemDetails {
			op.ProblemDetailsCount++
		}
		if result.ExpectedFailure {
			expectedFailures++
		}
		if result.Status != result.ExpectedStatus || result.Timeout {
			op.UnexpectedStatusCount++
			unexpected++
		}
		if result.SecretLeak {
			op.SecretLeakCount++
			secretLeaks++
		}
		if result.Status == http.StatusTooManyRequests {
			rateLimited++
		}
		if result.Timeout {
			op.Timeouts++
			timeouts++
		}
		operations[result.Operation] = op
	}
	if len(results) != opts.Requests {
		unexpected += opts.Requests - len(results)
	}

	totalAllocDelta := uint64(0)
	if after.TotalAlloc >= before.TotalAlloc {
		totalAllocDelta = after.TotalAlloc - before.TotalAlloc
	}
	mallocsDelta := uint64(0)
	if after.Mallocs >= before.Mallocs {
		mallocsDelta = after.Mallocs - before.Mallocs
	}
	requestCount := float64(len(results))
	if requestCount == 0 {
		requestCount = 1
	}
	durationSeconds := duration.Seconds()
	if durationSeconds <= 0 {
		durationSeconds = 0.000001
	}

	status := "passed"
	if unexpected > 0 || secretLeaks > 0 || timeouts > 0 {
		status = "failed"
	}
	authFailure := operations["auth_failure"]

	return loadSummary{
		Schema:                "reference-service-load-smoke.v1",
		Status:                status,
		Timestamp:             timestamp.Format(time.RFC3339),
		Requests:              opts.Requests,
		Concurrency:           opts.Concurrency,
		DurationMS:            milliseconds(duration),
		ThroughputRPS:         float64(len(results)) / durationSeconds,
		ExpectedFailureCount:  expectedFailures,
		ExpectedFailureRate:   float64(expectedFailures) / requestCount,
		UnexpectedStatusCount: unexpected,
		UnexpectedStatusRate:  float64(unexpected) / requestCount,
		SecretLeakCount:       secretLeaks,
		LatencyMS:             summarizeLatency(durations),
		Memory: memorySummary{
			HeapAllocBeforeBytes: int64(before.HeapAlloc),
			HeapAllocAfterBytes:  int64(after.HeapAlloc),
			HeapAllocDeltaBytes:  int64(after.HeapAlloc) - int64(before.HeapAlloc),
			TotalAllocDeltaBytes: int64(totalAllocDelta),
		},
		Allocations: allocationSummary{
			MallocsDelta:     mallocsDelta,
			AllocsPerRequest: float64(mallocsDelta) / requestCount,
			BytesPerRequest:  float64(totalAllocDelta) / requestCount,
		},
		Limits: limitSummary{
			RateLimitResponses: rateLimited,
			Timeouts:           timeouts,
		},
		FailureBehavior: failureBehaviorSummary{
			Scenario:              "missing API key on GET /widgets",
			ExpectedStatus:        http.StatusUnauthorized,
			ExpectedFailureCount:  authFailure.Count,
			ProblemDetailsCount:   authFailure.ProblemDetailsCount,
			UnexpectedStatusCount: authFailure.UnexpectedStatusCount,
			SecretLeakCount:       authFailure.SecretLeakCount,
			FailureRate:           float64(authFailure.Count) / requestCount,
		},
		Operations:         operations,
		GoroutinePeak:      goroutinePeak,
		GracefulShutdownMS: milliseconds(shutdownDuration),
		Environment: environmentMetadata{
			Profile:    opts.Profile,
			Commit:     opts.Commit,
			GoVersion:  runtime.Version(),
			GOOS:       runtime.GOOS,
			GOARCH:     runtime.GOARCH,
			GOMAXPROCS: runtime.GOMAXPROCS(0),
		},
	}
}

func summarizeLatency(durations []time.Duration) latencySummary {
	if len(durations) == 0 {
		return latencySummary{}
	}
	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total time.Duration
	for _, duration := range sorted {
		total += duration
	}
	return latencySummary{
		Min:  milliseconds(sorted[0]),
		Mean: milliseconds(total / time.Duration(len(sorted))),
		P50:  percentile(sorted, 50),
		P95:  percentile(sorted, 95),
		P99:  percentile(sorted, 99),
		Max:  milliseconds(sorted[len(sorted)-1]),
	}
}

func percentile(sorted []time.Duration, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil((p / 100) * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return milliseconds(sorted[rank-1])
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration.Nanoseconds()) / float64(time.Millisecond)
}

func writeEvidence(outDir string, summary loadSummary) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	status := summary.Status
	if status == "" {
		status = "failed"
	}
	if err := os.WriteFile(filepath.Join(outDir, "status"), []byte(status+"\n"), 0o644); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(filepath.Join(outDir, "summary.json"), payload, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "summary.md"), []byte(renderMarkdown(summary)), 0o644)
}

func renderMarkdown(summary loadSummary) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Reference Service Load Smoke")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Schema: %s\n", summary.Schema)
	fmt.Fprintf(&b, "- Status: %s\n", summary.Status)
	fmt.Fprintf(&b, "- Requests: %d\n", summary.Requests)
	fmt.Fprintf(&b, "- Concurrency: %d\n", summary.Concurrency)
	fmt.Fprintf(&b, "- Duration: %.2f ms\n", summary.DurationMS)
	fmt.Fprintf(&b, "- Throughput: %.2f requests/second\n", summary.ThroughputRPS)
	fmt.Fprintf(&b, "- Expected failure rate: %.2f%%\n", summary.ExpectedFailureRate*100)
	fmt.Fprintf(&b, "- Unexpected status rate: %.2f%%\n", summary.UnexpectedStatusRate*100)
	fmt.Fprintf(&b, "- Rate-limit responses: %d\n", summary.Limits.RateLimitResponses)
	fmt.Fprintf(&b, "- Timeouts: %d\n", summary.Limits.Timeouts)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Latency")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "| Min | Mean | P50 | P95 | P99 | Max |\n")
	fmt.Fprintf(&b, "| ---: | ---: | ---: | ---: | ---: | ---: |\n")
	fmt.Fprintf(&b, "| %.3f ms | %.3f ms | %.3f ms | %.3f ms | %.3f ms | %.3f ms |\n",
		summary.LatencyMS.Min, summary.LatencyMS.Mean, summary.LatencyMS.P50, summary.LatencyMS.P95, summary.LatencyMS.P99, summary.LatencyMS.Max)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Memory And Allocations")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Heap allocation delta: %d bytes\n", summary.Memory.HeapAllocDeltaBytes)
	fmt.Fprintf(&b, "- Total allocation delta: %d bytes\n", summary.Memory.TotalAllocDeltaBytes)
	fmt.Fprintf(&b, "- Mallocs delta: %d\n", summary.Allocations.MallocsDelta)
	fmt.Fprintf(&b, "- Allocations per request: %.2f\n", summary.Allocations.AllocsPerRequest)
	fmt.Fprintf(&b, "- Bytes per request: %.2f\n", summary.Allocations.BytesPerRequest)
	fmt.Fprintf(&b, "- Goroutine peak: %d\n", summary.GoroutinePeak)
	fmt.Fprintf(&b, "- Graceful worker shutdown: %.3f ms\n", summary.GracefulShutdownMS)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Environment")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Profile: %s\n", summary.Environment.Profile)
	fmt.Fprintf(&b, "- Commit: %s\n", summary.Environment.Commit)
	fmt.Fprintf(&b, "- Go: %s\n", summary.Environment.GoVersion)
	fmt.Fprintf(&b, "- Platform: %s/%s, GOMAXPROCS=%d\n", summary.Environment.GOOS, summary.Environment.GOARCH, summary.Environment.GOMAXPROCS)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Failure Behavior")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Scenario: %s\n", summary.FailureBehavior.Scenario)
	fmt.Fprintf(&b, "- Expected status: %d\n", summary.FailureBehavior.ExpectedStatus)
	fmt.Fprintf(&b, "- Expected failures: %d\n", summary.FailureBehavior.ExpectedFailureCount)
	fmt.Fprintf(&b, "- Problem Details responses: %d\n", summary.FailureBehavior.ProblemDetailsCount)
	fmt.Fprintf(&b, "- Unexpected failure statuses: %d\n", summary.FailureBehavior.UnexpectedStatusCount)
	fmt.Fprintf(&b, "- Secret leaks: %d\n", summary.FailureBehavior.SecretLeakCount)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Operations")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Operation | Count | Expected status | Status counts | Unexpected | Problem Details |")
	fmt.Fprintln(&b, "| --- | ---: | ---: | --- | ---: | ---: |")
	for _, name := range sortedOperationNames(summary.Operations) {
		op := summary.Operations[name]
		fmt.Fprintf(&b, "| %s | %d | %d | %s | %d | %d |\n",
			name, op.Count, op.ExpectedStatus, formatStatusCounts(op.StatusCounts), op.UnexpectedStatusCount, op.ProblemDetailsCount)
	}
	return b.String()
}

func sortedOperationNames(operations map[string]operationSummary) []string {
	names := make([]string, 0, len(operations))
	for name := range operations {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func formatStatusCounts(counts map[string]int) string {
	statuses := make([]string, 0, len(counts))
	for status := range counts {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	parts := make([]string, 0, len(statuses))
	for _, status := range statuses {
		parts = append(parts, status+":"+strconv.Itoa(counts[status]))
	}
	return strings.Join(parts, ", ")
}
