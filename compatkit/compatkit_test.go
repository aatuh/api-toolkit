package compatkit

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

const compatibleOpenAPI = `{
  "openapi": "3.1.0",
  "info": {"title": "Widget API", "version": "1.0.0"},
  "paths": {
    "/widgets": {
      "get": {
        "operationId": "listWidgets",
        "responses": {
          "200": {
            "description": "OK",
            "content": {"application/json": {"schema": {"type": "object"}}}
          }
        }
      }
    }
  }
}`

func TestRunChecksAgainstHandlerUsesStableHTTPProfile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"version": "1.2.3"})
	})
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(compatibleOpenAPI))
	})
	mux.HandleFunc("/problem", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: "Bad Request"})
	})

	result := RunChecks(context.Background(), Suite{
		Target: Target{Handler: mux},
		Checks: StableHTTPChecks(StableHTTPConfig{
			ReadinessPath:   "/readyz",
			VersionPath:     "/version",
			OpenAPIPath:     "/openapi.json",
			PreviousOpenAPI: []byte(compatibleOpenAPI),
			ProblemRequest: Request{
				Method: http.MethodGet,
				Path:   "/problem",
			},
			ProblemStatus: http.StatusBadRequest,
		}),
	})

	if err := result.Error(); err != nil {
		t.Fatalf("compatibility checks failed: %v", err)
	}
}

func TestRunChecksAgainstBaseURL(t *testing.T) {
	result := RunChecks(context.Background(), Suite{
		Target: Target{
			BaseURL: "https://service.test",
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://service.test/jobs" {
					t.Fatalf("request URL = %s, want https://service.test/jobs", req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(`{"status":"accepted"}`)),
				}, nil
			})},
		},
		Checks: []Check{{
			Name: "accepted",
			Request: Request{
				Method: http.MethodPost,
				Path:   "/jobs",
			},
			Expect: ExpectStatus(http.StatusAccepted),
		}},
	})

	if err := result.Error(); err != nil {
		t.Fatalf("compatibility checks failed: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRunChecksReportsExpectationFailures(t *testing.T) {
	result := RunChecks(context.Background(), Suite{
		Target: Target{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusInternalServerError)
		})},
		Checks: []Check{{
			Name: "health",
			Request: Request{
				Method: http.MethodGet,
				Path:   "/healthz",
			},
			Expect: ExpectStatus(http.StatusOK),
		}},
	})

	if result.OK() {
		t.Fatal("result OK = true, want findings")
	}
	err := result.Error()
	if err == nil || !strings.Contains(err.Error(), "health: status = 500, want 200") {
		t.Fatalf("unexpected result error: %v", err)
	}
}

func TestRunChecksRejectsAbsoluteCheckURL(t *testing.T) {
	result := RunChecks(context.Background(), Suite{
		Target: Target{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not receive absolute check URL")
		})},
		Checks: []Check{{
			Name: "absolute-url",
			Request: Request{
				Method: http.MethodGet,
				Path:   "https://example.com/healthz",
			},
			Expect: ExpectStatus(http.StatusOK),
		}},
	})

	if result.OK() {
		t.Fatal("result OK = true, want request path finding")
	}
	if got := result.Error().Error(); !strings.Contains(got, "request path must be relative to the suite target") {
		t.Fatalf("unexpected result error: %s", got)
	}
}

func TestRunChecksLimitsResponseBody(t *testing.T) {
	result := RunChecks(context.Background(), Suite{
		Target: Target{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("too large"))
		})},
		MaxBodyBytes: 3,
		Checks: []Check{{
			Name: "bounded",
			Request: Request{
				Method: http.MethodGet,
				Path:   "/large",
			},
			Expect: ExpectStatus(http.StatusOK),
		}},
	})

	if result.OK() {
		t.Fatal("result OK = true, want response body limit finding")
	}
	if got := result.Error().Error(); !strings.Contains(got, "response body exceeds MaxBodyBytes=3") {
		t.Fatalf("unexpected result error: %s", got)
	}
}

func TestExpectOpenAPICompatibleReportsFindings(t *testing.T) {
	response := Response{
		Status: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{
		  "openapi": "3.1.0",
		  "info": {"title": "Widget API", "version": "1.0.0"},
		  "paths": {}
		}`),
	}

	err := ExpectOpenAPICompatible([]byte(compatibleOpenAPI))(response)
	if err == nil {
		t.Fatal("ExpectOpenAPICompatible error = nil, want finding")
	}
	if !strings.Contains(err.Error(), "operation_removed GET /widgets") {
		t.Fatalf("unexpected error: %v", err)
	}
}
