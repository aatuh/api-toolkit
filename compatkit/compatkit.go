package compatkit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/contracttest"
)

const (
	defaultTimeout      = 5 * time.Second
	defaultMaxBodyBytes = int64(1 << 20)
)

// Suite describes the downstream compatibility checks to run against a target
// service.
type Suite struct {
	Target       Target
	Checks       []Check
	Timeout      time.Duration
	MaxBodyBytes int64
}

// Target identifies the service under test. Set exactly one of Handler or
// BaseURL.
type Target struct {
	Handler http.Handler
	BaseURL string
	Client  *http.Client
}

// Request describes one HTTP request issued by a compatibility check.
type Request struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// Check is one named compatibility assertion.
type Check struct {
	Name    string
	Request Request
	Expect  Expectation
}

// Response is the bounded response material passed to an expectation.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Expectation validates one response.
type Expectation func(Response) error

// Finding is a failed suite setup, request, or response expectation.
type Finding struct {
	Check   string
	Message string
}

// Result is the structured outcome from RunChecks.
type Result struct {
	Findings []Finding
}

// OK reports whether every configured compatibility check passed.
func (r Result) OK() bool {
	return len(r.Findings) == 0
}

// Error returns nil when the suite passed, otherwise an error containing every
// finding.
func (r Result) Error() error {
	if r.OK() {
		return nil
	}
	var b strings.Builder
	for i, finding := range r.Findings {
		if i > 0 {
			b.WriteByte('\n')
		}
		check := strings.TrimSpace(finding.Check)
		if check == "" {
			check = "suite"
		}
		b.WriteString(check)
		b.WriteString(": ")
		b.WriteString(finding.Message)
	}
	return errors.New(b.String())
}

// Run executes a suite and fails t when any finding is reported.
func Run(t testing.TB, suite Suite) {
	t.Helper()
	if err := RunChecks(context.Background(), suite).Error(); err != nil {
		t.Fatalf("compatibility findings:\n%s", err)
	}
}

// RunChecks executes a suite and returns structured findings instead of failing
// a test. The caller-provided context bounds the whole run; Suite.Timeout bounds
// each individual request.
func RunChecks(ctx context.Context, suite Suite) Result {
	if len(suite.Checks) == 0 {
		return Result{Findings: []Finding{{Message: "suite has no checks"}}}
	}
	runner, findings := newRunner(suite)
	if len(findings) > 0 {
		return Result{Findings: findings}
	}

	var result Result
	for _, check := range suite.Checks {
		name := checkName(check)
		response, err := runner.do(ctx, check.Request)
		if err != nil {
			result.Findings = append(result.Findings, Finding{Check: name, Message: err.Error()})
			continue
		}
		if check.Expect == nil {
			result.Findings = append(result.Findings, Finding{Check: name, Message: "check has no expectation"})
			continue
		}
		if err := check.Expect(response); err != nil {
			result.Findings = append(result.Findings, Finding{Check: name, Message: err.Error()})
		}
	}
	return result
}

// ExpectAll combines response expectations and returns the first failure.
func ExpectAll(expectations ...Expectation) Expectation {
	return func(response Response) error {
		for _, expect := range expectations {
			if expect == nil {
				continue
			}
			if err := expect(response); err != nil {
				return err
			}
		}
		return nil
	}
}

// ExpectStatus requires the response status code to match status.
func ExpectStatus(status int) Expectation {
	return func(response Response) error {
		if response.Status != status {
			return fmt.Errorf("status = %d, want %d", response.Status, status)
		}
		return nil
	}
}

// ExpectHeader requires an exact response header value.
func ExpectHeader(name, value string) Expectation {
	return func(response Response) error {
		if got := response.Header.Get(name); got != value {
			return fmt.Errorf("header %s = %q, want %q", name, got, value)
		}
		return nil
	}
}

// ExpectHeaderContains requires a response header value to contain a substring.
func ExpectHeaderContains(name, value string) Expectation {
	return func(response Response) error {
		got := response.Header.Get(name)
		if !strings.Contains(strings.ToLower(got), strings.ToLower(value)) {
			return fmt.Errorf("header %s = %q, want to contain %q", name, got, value)
		}
		return nil
	}
}

// ExpectProblemDetails requires an RFC 9457-style Problem Details response. If
// status is zero, any 4xx or 5xx response is accepted.
func ExpectProblemDetails(status int) Expectation {
	return func(response Response) error {
		if status > 0 {
			if err := ExpectStatus(status)(response); err != nil {
				return err
			}
		} else if response.Status < 400 || response.Status > 599 {
			return fmt.Errorf("status = %d, want 4xx or 5xx Problem Details", response.Status)
		}
		if err := ExpectHeaderContains("Content-Type", "application/problem+json")(response); err != nil {
			return err
		}
		var body map[string]any
		if err := json.Unmarshal(response.Body, &body); err != nil {
			return fmt.Errorf("decode Problem Details JSON: %w", err)
		}
		if strings.TrimSpace(fmt.Sprint(body["title"])) == "" {
			return errors.New("problem title is missing")
		}
		gotStatus, ok := jsonInt(body["status"])
		if !ok {
			return errors.New("problem status is missing or not numeric")
		}
		if gotStatus != response.Status {
			return fmt.Errorf("problem status = %d, want response status %d", gotStatus, response.Status)
		}
		return nil
	}
}

// ExpectOpenAPIDocument requires a successful OpenAPI JSON document with an
// openapi version and paths object.
func ExpectOpenAPIDocument() Expectation {
	return func(response Response) error {
		if err := ExpectStatus(http.StatusOK)(response); err != nil {
			return err
		}
		var body map[string]any
		if err := json.Unmarshal(response.Body, &body); err != nil {
			return fmt.Errorf("decode OpenAPI JSON: %w", err)
		}
		if strings.TrimSpace(fmt.Sprint(body["openapi"])) == "" {
			return errors.New("OpenAPI document is missing openapi version")
		}
		if _, ok := body["paths"].(map[string]any); !ok {
			return errors.New("OpenAPI document is missing paths object")
		}
		return nil
	}
}

// ExpectOpenAPICompatible requires the response body to be compatible with base
// according to contracttest.OpenAPICompatibilityFindings.
func ExpectOpenAPICompatible(base []byte) Expectation {
	base = append([]byte(nil), base...)
	return func(response Response) error {
		if len(bytes.TrimSpace(base)) == 0 {
			return errors.New("base OpenAPI document is required")
		}
		if err := ExpectOpenAPIDocument()(response); err != nil {
			return err
		}
		findings, err := contracttest.OpenAPICompatibilityFindings(base, response.Body)
		if err != nil {
			return fmt.Errorf("OpenAPI compatibility parse error: %w", err)
		}
		if len(findings) > 0 {
			return fmt.Errorf("OpenAPI compatibility findings: %s", strings.Join(findings, "; "))
		}
		return nil
	}
}

// ExpectOpenAPIGolden requires the response body to match golden after
// deterministic OpenAPI JSON normalization.
func ExpectOpenAPIGolden(golden []byte) Expectation {
	golden = append([]byte(nil), golden...)
	return func(response Response) error {
		if err := ExpectOpenAPIDocument()(response); err != nil {
			return err
		}
		got, err := contracttest.NormalizeOpenAPI(response.Body)
		if err != nil {
			return fmt.Errorf("normalize OpenAPI response: %w", err)
		}
		want, err := contracttest.NormalizeOpenAPI(golden)
		if err != nil {
			return fmt.Errorf("normalize golden OpenAPI: %w", err)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("OpenAPI golden mismatch\nwant:\n%s\ngot:\n%s", want, got)
		}
		return nil
	}
}

// StableHTTPConfig configures the built-in downstream HTTP compatibility
// profile. Empty paths are skipped so services can opt into only the contracts
// they expose.
type StableHTTPConfig struct {
	ReadinessPath   string
	VersionPath     string
	OpenAPIPath     string
	PreviousOpenAPI []byte
	ProblemRequest  Request
	ProblemStatus   int
}

// StableHTTPChecks returns checks for common api-toolkit service contracts:
// readiness, version, OpenAPI drift, and Problem Details responses.
func StableHTTPChecks(cfg StableHTTPConfig) []Check {
	var checks []Check
	if strings.TrimSpace(cfg.ReadinessPath) != "" {
		checks = append(checks, Check{
			Name: "readiness",
			Request: Request{
				Method: http.MethodGet,
				Path:   cfg.ReadinessPath,
			},
			Expect: ExpectAll(
				ExpectStatus(http.StatusOK),
				ExpectHeaderContains("Content-Type", "application/json"),
			),
		})
	}
	if strings.TrimSpace(cfg.VersionPath) != "" {
		checks = append(checks, Check{
			Name: "version",
			Request: Request{
				Method: http.MethodGet,
				Path:   cfg.VersionPath,
			},
			Expect: ExpectAll(
				ExpectStatus(http.StatusOK),
				ExpectHeaderContains("Content-Type", "application/json"),
			),
		})
	}
	if strings.TrimSpace(cfg.OpenAPIPath) != "" {
		expect := ExpectOpenAPIDocument()
		if len(bytes.TrimSpace(cfg.PreviousOpenAPI)) > 0 {
			expect = ExpectOpenAPICompatible(cfg.PreviousOpenAPI)
		}
		checks = append(checks, Check{
			Name: "openapi",
			Request: Request{
				Method: http.MethodGet,
				Path:   cfg.OpenAPIPath,
			},
			Expect: ExpectAll(
				ExpectHeaderContains("Content-Type", "application/json"),
				expect,
			),
		})
	}
	if strings.TrimSpace(cfg.ProblemRequest.Path) != "" {
		request := cfg.ProblemRequest
		if strings.TrimSpace(request.Method) == "" {
			request.Method = http.MethodGet
		}
		checks = append(checks, Check{
			Name:    "problem-details",
			Request: request,
			Expect:  ExpectProblemDetails(cfg.ProblemStatus),
		})
	}
	return checks
}

type runner struct {
	handler      http.Handler
	baseURL      string
	client       *http.Client
	timeout      time.Duration
	maxBodyBytes int64
}

func newRunner(suite Suite) (*runner, []Finding) {
	target := suite.Target
	hasHandler := target.Handler != nil
	baseURL := strings.TrimSpace(target.BaseURL)
	if hasHandler && baseURL != "" {
		return nil, []Finding{{Message: "set exactly one target: Handler or BaseURL"}}
	}
	if !hasHandler && baseURL == "" {
		return nil, []Finding{{Message: "target Handler or BaseURL is required"}}
	}
	timeout := suite.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxBodyBytes := suite.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	r := &runner{timeout: timeout, maxBodyBytes: maxBodyBytes}
	if hasHandler {
		r.handler = target.Handler
		return r, nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, []Finding{{Message: "BaseURL must be an absolute http or https URL"}}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, []Finding{{Message: "BaseURL must use http or https"}}
	}
	r.baseURL = strings.TrimRight(baseURL, "/")
	r.client = target.Client
	if r.client == nil {
		r.client = &http.Client{Timeout: timeout}
	}
	return r, nil
}

func (r *runner) do(ctx context.Context, request Request) (Response, error) {
	method := strings.TrimSpace(request.Method)
	if method == "" {
		method = http.MethodGet
	}
	if r.handler != nil {
		return r.doHandler(ctx, method, request)
	}
	targetURL, err := joinBasePath(r.baseURL, request.Path)
	if err != nil {
		return Response{}, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, targetURL, bytes.NewReader(request.Body))
	if err != nil {
		return Response{}, err
	}
	req.Header = request.Header.Clone()
	resp, err := r.client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, r.maxBodyBytes+1))
	if err != nil {
		return Response{}, err
	}
	if int64(len(body)) > r.maxBodyBytes {
		return Response{}, fmt.Errorf("response body exceeds MaxBodyBytes=%d", r.maxBodyBytes)
	}
	return Response{
		Status: resp.StatusCode,
		Header: resp.Header.Clone(),
		Body:   body,
	}, nil
}

func (r *runner) doHandler(ctx context.Context, method string, request Request) (Response, error) {
	path, err := cleanRelativePath(request.Path)
	if err != nil {
		return Response{}, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	req := httptest.NewRequestWithContext(reqCtx, method, "http://compatkit.local"+path, bytes.NewReader(request.Body))
	req.Header = request.Header.Clone()
	recorder := newCaptureResponseWriter(r.maxBodyBytes)
	r.handler.ServeHTTP(recorder, req)
	if recorder.exceeded {
		return Response{}, fmt.Errorf("response body exceeds MaxBodyBytes=%d", r.maxBodyBytes)
	}
	return Response{
		Status: recorder.statusCode(),
		Header: recorder.header.Clone(),
		Body:   recorder.body.Bytes(),
	}, nil
}

type captureResponseWriter struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	limit    int64
	exceeded bool
}

func newCaptureResponseWriter(limit int64) *captureResponseWriter {
	return &captureResponseWriter{
		header: make(http.Header),
		limit:  limit,
	}
}

func (w *captureResponseWriter) Header() http.Header {
	return w.header
}

func (w *captureResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
}

func (w *captureResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	remaining := w.limit - int64(w.body.Len())
	if remaining > 0 {
		if int64(len(p)) <= remaining {
			_, _ = w.body.Write(p)
		} else {
			_, _ = w.body.Write(p[:int(remaining)])
			w.exceeded = true
		}
	} else {
		w.exceeded = true
	}
	return len(p), nil
}

func (w *captureResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func joinBasePath(baseURL, path string) (string, error) {
	cleaned, err := cleanRelativePath(path)
	if err != nil {
		return "", err
	}
	base, err := url.Parse(baseURL + "/")
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	parsed, _ := url.Parse(cleaned)
	return base.ResolveReference(parsed).String(), nil
}

func cleanRelativePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("request path is required")
	}
	parsed, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse request path: %w", err)
	}
	if parsed.IsAbs() || parsed.Host != "" {
		return "", errors.New("request path must be relative to the suite target")
	}
	if !strings.HasPrefix(parsed.Path, "/") {
		parsed.Path = "/" + parsed.Path
	}
	return parsed.String(), nil
}

func checkName(check Check) string {
	if name := strings.TrimSpace(check.Name); name != "" {
		return name
	}
	method := strings.TrimSpace(check.Request.Method)
	if method == "" {
		method = http.MethodGet
	}
	return method + " " + strings.TrimSpace(check.Request.Path)
}

func jsonInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), v == float64(int(v))
	case int:
		return v, true
	case json.Number:
		i, err := strconv.Atoi(v.String())
		return i, err == nil
	default:
		return 0, false
	}
}
