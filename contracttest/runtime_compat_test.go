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
	"github.com/aatuh/api-toolkit/v3/httpx"
	jsonmw "github.com/aatuh/api-toolkit/v3/middleware/json"
	"github.com/aatuh/api-toolkit/v3/middleware/timeout"
	"github.com/aatuh/api-toolkit/v3/ports"
	"github.com/aatuh/api-toolkit/v3/specs"
)

func TestRuntimeCompatibilityGolden(t *testing.T) {
	snapshot := map[string]any{
		"responses": map[string]responseSnapshot{
			"httpx_write_json":              runtimeCompatWriteJSON(t),
			"httpx_write_problem":           runtimeCompatWriteProblem(t),
			"json_middleware_rejection":     runtimeCompatJSONMiddlewareRejection(t),
			"version_endpoint":              runtimeCompatVersionEndpoint(t),
			"health_detailed_disabled":      runtimeCompatDetailedHealthDisabled(t),
			"hard_timeout_problem_response": runtimeCompatHardTimeout(t),
		},
		"openapi_metadata": runtimeCompatOpenAPI(t),
	}

	got := mustMarshalGolden(t, snapshot)
	goldenPath := filepath.Join("testdata", "golden", "runtime_compatibility.json")
	if os.Getenv("UPDATE_RUNTIME_COMPAT_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
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
		out[http.CanonicalHeaderKey(name)] = copied
	}
	return out
}

func mustMarshalGolden(t *testing.T, value any) []byte {
	t.Helper()

	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden: %v", err)
	}
	return append(out, '\n')
}
