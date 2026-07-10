package contracttest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	"github.com/aatuh/api-toolkit/v3/endpoints/version"
	"github.com/aatuh/api-toolkit/v3/fielderrors"
	"github.com/aatuh/api-toolkit/v3/httpcache"
	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/idempotent"
	"github.com/aatuh/api-toolkit/v3/middleware/auth/apikey"
	"github.com/aatuh/api-toolkit/v3/middleware/deprecation"
	jsonmw "github.com/aatuh/api-toolkit/v3/middleware/json"
	"github.com/aatuh/api-toolkit/v3/middleware/timeout"
	"github.com/aatuh/api-toolkit/v3/ports"
	"github.com/aatuh/api-toolkit/v3/specs"
)

func TestRuntimeCompatibilityGolden(t *testing.T) {
	snapshot := map[string]any{
		"responses": map[string]responseSnapshot{
			"api_key_auth_failure":          runtimeCompatAPIKeyAuthFailure(t),
			"cache_not_modified":            runtimeCompatCacheNotModified(t),
			"cache_precondition_failed":     runtimeCompatCachePreconditionFailed(t),
			"deprecation_headers":           runtimeCompatDeprecationHeaders(t),
			"health_readiness":              runtimeCompatReadiness(t),
			"httpx_write_json":              runtimeCompatWriteJSON(t),
			"httpx_write_problem":           runtimeCompatWriteProblem(t),
			"idempotency_conflict":          runtimeCompatIdempotencyConflict(t),
			"json_middleware_rejection":     runtimeCompatJSONMiddlewareRejection(t),
			"version_endpoint":              runtimeCompatVersionEndpoint(t),
			"health_detailed_disabled":      runtimeCompatDetailedHealthDisabled(t),
			"hard_timeout_problem_response": runtimeCompatHardTimeout(t),
			"validation_problem":            runtimeCompatValidationProblem(t),
		},
		"openapi_metadata": runtimeCompatOpenAPI(t),
	}

	got := mustMarshalGolden(t, snapshot)
	goldenPath := filepath.Join("testdata", "golden", "runtime_compatibility.json")
	if os.Getenv("UPDATE_RUNTIME_COMPAT_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		//nolint:gosec // This opt-in update writes a public checked-in compatibility fixture.
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read runtime compatibility golden: %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Fatalf("runtime compatibility golden drifted\nupdate with UPDATE_RUNTIME_COMPAT_GOLDEN=1 go test ./contracttest -run TestRuntimeCompatibilityGolden\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

type responseSnapshot struct {
	Status  int                 `json:"status"`
	Header  map[string][]string `json:"header"`
	JSON    any                 `json:"json,omitempty"`
	Body    string              `json:"body,omitempty"`
	Trailer map[string][]string `json:"trailer,omitempty"`
}

func runtimeCompatWriteJSON(t *testing.T) responseSnapshot {
	t.Helper()

	rec := httptest.NewRecorder()
	httpx.WriteJSON(rec, http.StatusAccepted, map[string]string{"status": "accepted"})
	return snapshotResponse(t, rec)
}

func runtimeCompatWriteProblem(t *testing.T) responseSnapshot {
	t.Helper()

	rec := httptest.NewRecorder()
	httpx.WriteProblem(rec, http.StatusBadRequest, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeValidation),
		Title:  http.StatusText(http.StatusBadRequest),
		Detail: "validation failed",
	})
	return snapshotResponse(t, rec)
}

func runtimeCompatValidationProblem(t *testing.T) responseSnapshot {
	t.Helper()

	rec := httptest.NewRecorder()
	httpx.WriteProblemWithFieldErrors(rec, http.StatusBadRequest, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeValidation),
		Title:  http.StatusText(http.StatusBadRequest),
		Detail: "validation failed",
	}, fielderrors.FieldErrors{
		{Field: "name", Code: "required", Message: "name is required"},
		{Field: "quantity", Code: "min", Message: "quantity must be at least 1"},
	})
	return snapshotResponse(t, rec)
}

func runtimeCompatJSONMiddlewareRejection(t *testing.T) responseSnapshot {
	t.Helper()

	mw, err := jsonmw.New(jsonmw.Options{RequireJSON: true})
	if err != nil {
		t.Fatalf("new JSON middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run for unsupported media type")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", bytes.NewBufferString("payload"))
	req.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(rec, req)
	return snapshotResponse(t, rec)
}

func runtimeCompatVersionEndpoint(t *testing.T) responseSnapshot {
	t.Helper()

	handler := version.NewHandler(version.Config{
		Info: ports.VersionInfo{
			Version: "1.2.3",
			Commit:  "abc123",
			Date:    "2026-01-02T03:04:05Z",
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/version", nil)
	handler.Handler().ServeHTTP(rec, req)
	return snapshotResponse(t, rec)
}

func runtimeCompatDetailedHealthDisabled(t *testing.T) responseSnapshot {
	t.Helper()

	handler := health.NewDefaultHandler(nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/detailed", nil)
	handler.DetailedHealthHandler(rec, req)
	return snapshotResponse(t, rec)
}

func runtimeCompatReadiness(t *testing.T) responseSnapshot {
	t.Helper()

	handler := health.NewHandler(runtimeCompatHealthManager{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	handler.ReadinessHandler(rec, req)
	return snapshotResponse(t, rec)
}

func runtimeCompatHardTimeout(t *testing.T) responseSnapshot {
	t.Helper()

	mw, err := timeout.NewHard(timeout.Options{Timeout: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("new hard timeout: %v", err)
	}
	writeErr := make(chan error, 1)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.Header().Set("X-Late", "true")
		_, err := w.Write([]byte("late"))
		writeErr <- err
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/slow", nil)
	handler.ServeHTTP(rec, req)
	if err := <-writeErr; err == nil {
		t.Fatal("late timeout write unexpectedly succeeded")
	}
	return snapshotResponse(t, rec)
}

func runtimeCompatIdempotencyConflict(t *testing.T) responseSnapshot {
	t.Helper()

	rec := httptest.NewRecorder()
	idempotent.WriteConflict(rec, "idempotency key was reused with a different request")
	return snapshotResponse(t, rec)
}

func runtimeCompatAPIKeyAuthFailure(t *testing.T) responseSnapshot {
	t.Helper()

	mw, err := apikey.NewMiddleware(apikey.Config{
		Verifier: apikey.VerifierFunc(func(context.Context, apikey.PresentedKey) (apikey.Principal, error) {
			return apikey.Principal{}, nil
		}),
	})
	if err != nil {
		t.Fatalf("new API key middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run for invalid API key")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/secure", nil)
	req.Header.Set("X-API-Key", "invalid")
	handler.ServeHTTP(rec, req)
	return snapshotResponse(t, rec)
}

func runtimeCompatDeprecationHeaders(t *testing.T) responseSnapshot {
	t.Helper()

	mw, err := deprecation.New(deprecation.Config{
		DeprecatedAt: time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC),
		SunsetAt:     time.Date(2026, 7, 2, 8, 0, 0, 0, time.UTC),
		Links: []deprecation.Link{{
			URL:   "https://docs.example.test/deprecations/widgets",
			Rel:   "deprecation",
			Type:  "text/html",
			Title: "Widget deprecation",
		}},
	})
	if err != nil {
		t.Fatalf("new deprecation middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/deprecated", nil)
	handler.ServeHTTP(rec, req)
	return snapshotResponse(t, rec)
}

func runtimeCompatCacheNotModified(t *testing.T) responseSnapshot {
	t.Helper()

	rec := httptest.NewRecorder()
	httpcache.WriteNotModified(rec, runtimeCompatCacheValidators())
	return snapshotResponse(t, rec)
}

func runtimeCompatCachePreconditionFailed(t *testing.T) responseSnapshot {
	t.Helper()

	rec := httptest.NewRecorder()
	httpcache.WritePreconditionFailed(rec)
	return snapshotResponse(t, rec)
}

func runtimeCompatCacheValidators() httpcache.Validators {
	return httpcache.Validators{
		ETag:         httpcache.StrongETag("widgets-v1"),
		LastModified: time.Date(2026, 5, 2, 8, 30, 15, 900, time.UTC),
	}
}

func runtimeCompatOpenAPI(t *testing.T) any {
	t.Helper()

	registry := specs.NewRegistry(specs.Info{
		Title:       "Runtime Compatibility API",
		Description: "Golden-tested stable metadata",
		Version:     "1.2.3",
	})
	registry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{
		Type: "apiKey",
		Name: "X-API-Key",
		In:   "header",
	})
	specs.RegisterProblemCatalog(registry, httpx.DefaultProblemCatalog())
	registry.Register(specs.Operation{
		OperationID: "createWidget",
		Method:      http.MethodPost,
		Path:        "/widgets",
		Summary:     "Create widget",
		Description: "Creates a widget.",
		Tags:        []string{"widgets"},
		Security: []specs.SecurityRequirement{{
			Name:   "ApiKeyAuth",
			Scopes: []string{"widgets:write"},
		}},
		RequestBody: &specs.RequestBody{
			Description:  "Widget payload",
			Required:     true,
			ContentTypes: []string{"application/json"},
		},
		Responses: map[int]specs.Response{
			http.StatusCreated: {
				Description: "Widget created",
				Content: map[string]specs.MediaType{
					"application/json": {SchemaRef: "#/components/schemas/Widget"},
				},
			},
			http.StatusBadRequest: {
				Ref: "#/components/responses/ValidationProblemResponse",
			},
		},
	})

	doc, err := registry.OpenAPI()
	if err != nil {
		t.Fatalf("openapi: %v", err)
	}
	var out any
	if err := json.Unmarshal(doc, &out); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}
	return out
}

func snapshotResponse(t *testing.T, rec *httptest.ResponseRecorder) responseSnapshot {
	t.Helper()

	snapshot := responseSnapshot{
		Status: rec.Code,
		Header: selectedHeaders(rec.Header(),
			"Content-Type",
			"Deprecation",
			"ETag",
			"Last-Modified",
			"Link",
			"Sunset",
			"X-Content-Type-Options",
			"X-Frame-Options",
			"X-Late",
		),
	}
	body := bytes.TrimSpace(rec.Body.Bytes())
	if len(body) == 0 {
		return snapshot
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err == nil {
		snapshot.JSON = decoded
		return snapshot
	}
	snapshot.Body = string(body)
	return snapshot
}

func selectedHeaders(header http.Header, names ...string) map[string][]string {
	out := map[string][]string{}
	for _, name := range names {
		values := header.Values(name)
		if len(values) == 0 {
			continue
		}
		copied := append([]string(nil), values...)
		out[name] = copied
	}
	return out
}

type runtimeCompatHealthManager struct{}

func (runtimeCompatHealthManager) RegisterChecker(ports.HealthChecker) {}

func (runtimeCompatHealthManager) RegisterCheckers(...ports.HealthChecker) {}

func (runtimeCompatHealthManager) GetLiveness(context.Context) ports.HealthResult {
	return ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   "live",
		Timestamp: runtimeCompatHealthTimestamp(),
	}
}

func (runtimeCompatHealthManager) GetReadiness(context.Context) ports.HealthResult {
	return ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   "ready",
		Timestamp: runtimeCompatHealthTimestamp(),
	}
}

func (runtimeCompatHealthManager) GetHealth(context.Context) ports.HealthResponse {
	return ports.HealthResponse{
		Status:    ports.HealthStatusHealthy,
		Message:   "healthy",
		Timestamp: runtimeCompatHealthTimestamp(),
	}
}

func (runtimeCompatHealthManager) GetDetailedHealth(context.Context) ports.DetailedHealthResponse {
	check := ports.HealthResult{
		Status:    ports.HealthStatusHealthy,
		Message:   "ready",
		Timestamp: runtimeCompatHealthTimestamp(),
	}
	return ports.DetailedHealthResponse{
		Status:    ports.HealthStatusHealthy,
		Timestamp: runtimeCompatHealthTimestamp(),
		Checks:    map[string]ports.HealthResult{"readiness": check},
		Summary: ports.HealthSummary{
			Total:   1,
			Healthy: 1,
		},
	}
}

func runtimeCompatHealthTimestamp() time.Time {
	return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
}

func mustMarshalGolden(t *testing.T, value any) []byte {
	t.Helper()

	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden: %v", err)
	}
	return append(out, '\n')
}
