package binding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type requiredPresenceInt struct {
	Count int `json:"count" query:"count" path:"count" required:"true"`
}

type requiredPresenceBool struct {
	Enabled bool `json:"enabled" query:"enabled" path:"enabled" required:"true"`
}

type requiredPresenceString struct {
	Name string `json:"name" query:"name" path:"name" required:"true"`
}

type requiredPresenceSlice struct {
	Tags []string `json:"tags" required:"true"`
}

type requiredPresenceObject struct {
	Metadata map[string]string `json:"metadata" required:"true"`
}

type requiredPresencePointer struct {
	Count *int `json:"count" required:"true"`
}

func TestDecodeJSONRequiredModePresentAcceptsPresentZeroValues(t *testing.T) {
	tests := []struct {
		name   string
		decode func() error
	}{
		{
			name: "zero integer",
			decode: func() error {
				_, err := DecodeJSON[requiredPresenceInt](newPresenceJSONRequest(`{"count":0}`), JSONConfig{RequiredMode: RequiredModePresent})
				return err
			},
		},
		{
			name: "false boolean",
			decode: func() error {
				_, err := DecodeJSON[requiredPresenceBool](newPresenceJSONRequest(`{"enabled":false}`), JSONConfig{RequiredMode: RequiredModePresent})
				return err
			},
		},
		{
			name: "empty string",
			decode: func() error {
				_, err := DecodeJSON[requiredPresenceString](newPresenceJSONRequest(`{"name":""}`), JSONConfig{RequiredMode: RequiredModePresent})
				return err
			},
		},
		{
			name: "empty array",
			decode: func() error {
				_, err := DecodeJSON[requiredPresenceSlice](newPresenceJSONRequest(`{"tags":[]}`), JSONConfig{RequiredMode: RequiredModePresent})
				return err
			},
		},
		{
			name: "empty object",
			decode: func() error {
				_, err := DecodeJSON[requiredPresenceObject](newPresenceJSONRequest(`{"metadata":{}}`), JSONConfig{RequiredMode: RequiredModePresent})
				return err
			},
		},
		{
			name: "null",
			decode: func() error {
				_, err := DecodeJSON[requiredPresenceInt](newPresenceJSONRequest(`{"count":null}`), JSONConfig{RequiredMode: RequiredModePresent})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.decode(); err != nil {
				t.Fatalf("DecodeJSON() error = %v", err)
			}
		})
	}

	decoded, err := DecodeJSON[requiredPresencePointer](newPresenceJSONRequest(`{"count":0}`), JSONConfig{RequiredMode: RequiredModePresent})
	if err != nil {
		t.Fatalf("DecodeJSON() pointer error = %v", err)
	}
	if decoded.Count == nil || *decoded.Count != 0 {
		t.Fatalf("decoded pointer = %#v, want present pointer to zero", decoded.Count)
	}
}

func TestDecodeJSONRequiredModePresentRejectsMissingDuplicateAndUnknownFields(t *testing.T) {
	if _, err := DecodeJSON[requiredPresenceInt](newPresenceJSONRequest(`{}`), JSONConfig{RequiredMode: RequiredModePresent}); !hasFieldError(err, "count", "required") {
		t.Fatalf("missing required field error = %v", err)
	}
	if _, err := DecodeJSON[requiredPresenceInt](newPresenceJSONRequest(`{"count":0,"count":1}`), JSONConfig{RequiredMode: RequiredModePresent}); !hasFieldError(err, "body", "invalid_json") {
		t.Fatalf("duplicate member error = %v", err)
	}
	if _, err := DecodeJSON[requiredPresenceInt](newPresenceJSONRequest(`{"count":0,"unknown":true}`), JSONConfig{RequiredMode: RequiredModePresent}); !hasFieldError(err, "body", "invalid_json") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestDecodeQueryRequiredModesPreserveLegacyAndAllowExplicitPresence(t *testing.T) {
	type query struct {
		Count   int      `query:"count" required:"true"`
		Enabled bool     `query:"enabled" required:"true"`
		Name    string   `query:"name" required:"true"`
		Tags    []string `query:"tag" required:"true"`
	}

	present := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?count=0&enabled=false&name=&tag=&tag=", nil)
	got, err := DecodeQuery[query](present, QueryConfig{RequiredMode: RequiredModePresent})
	if err != nil {
		t.Fatalf("DecodeQuery() presence error = %v", err)
	}
	if got.Count != 0 || got.Enabled || got.Name != "" || len(got.Tags) != 0 {
		t.Fatalf("DecodeQuery() presence decoded = %#v", got)
	}

	if _, err := DecodeQuery[query](present, QueryConfig{}); !hasFieldError(err, "name", "required") {
		t.Fatalf("legacy query behavior error = %v", err)
	}

	nonZero := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?count=0&enabled=false&name=ok&tag=one", nil)
	if _, err := DecodeQuery[query](nonZero, QueryConfig{RequiredMode: RequiredModeNonZero}); !hasFieldError(err, "count", "required") || !hasFieldError(err, "enabled", "required") {
		t.Fatalf("non-zero query error = %v", err)
	}

	repeated := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?count=1&enabled=true&name=widget&tag=first&tag=second", nil)
	got, err = DecodeQuery[query](repeated, QueryConfig{RequiredMode: RequiredModePresent})
	if err != nil {
		t.Fatalf("DecodeQuery() repeated values error = %v", err)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "first" || got.Tags[1] != "second" {
		t.Fatalf("DecodeQuery() repeated tags = %#v, want [first second]", got.Tags)
	}
}

func TestDecodePathRequiredModePresentTracksConfiguredPresence(t *testing.T) {
	type path struct {
		Count   int    `path:"count" required:"true"`
		Enabled bool   `path:"enabled" required:"true"`
		Name    string `path:"name" required:"true"`
	}

	values := map[string]string{"count": "0", "enabled": "false", "name": ""}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	config := PathConfig{
		Param: func(_ *http.Request, name string) string {
			return values[name]
		},
		HasParam: func(_ *http.Request, name string) bool {
			_, ok := values[name]
			return ok
		},
		RequiredMode: RequiredModePresent,
	}
	got, err := DecodePath[path](req, config)
	if err != nil {
		t.Fatalf("DecodePath() presence error = %v", err)
	}
	if got.Count != 0 || got.Enabled || got.Name != "" {
		t.Fatalf("DecodePath() presence decoded = %#v", got)
	}

	config.RequiredMode = RequiredModeNonZero
	if _, err := DecodePath[path](req, config); !hasFieldError(err, "count", "required") || !hasFieldError(err, "enabled", "required") || !hasFieldError(err, "name", "required") {
		t.Fatalf("non-zero path error = %v", err)
	}
}

func TestRequiredModesRejectUnknownConfiguration(t *testing.T) {
	type request struct {
		Count int `json:"count" query:"count" path:"count" required:"true"`
	}

	unknown := RequiredMode("unexpected")
	if _, err := DecodeJSON[request](newPresenceJSONRequest(`{"count":1}`), JSONConfig{RequiredMode: unknown}); !hasFieldError(err, "body", "invalid_required_mode") {
		t.Fatalf("DecodeJSON() unknown mode error = %v", err)
	}
	query := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?count=1", nil)
	if _, err := DecodeQuery[request](query, QueryConfig{RequiredMode: unknown}); !hasFieldError(err, "query", "invalid_required_mode") {
		t.Fatalf("DecodeQuery() unknown mode error = %v", err)
	}
	if _, err := DecodePath[request](query, PathConfig{
		Param:        func(*http.Request, string) string { return "1" },
		RequiredMode: unknown,
	}); !hasFieldError(err, "path", "invalid_required_mode") {
		t.Fatalf("DecodePath() unknown mode error = %v", err)
	}
}

func newPresenceJSONRequest(body string) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
}
