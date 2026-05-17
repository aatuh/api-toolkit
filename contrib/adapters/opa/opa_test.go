package opa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestEvaluateAllowsBooleanResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		input, ok := payload["input"].(map[string]any)
		if !ok {
			t.Fatalf("expected input object, got %T", payload["input"])
		}
		if input["action"] != "read" {
			t.Fatalf("expected action read, got %v", input["action"])
		}
		_, _ = w.Write([]byte(`{"result": true}`))
	}))
	defer server.Close()

	client, err := New(Config{DecisionURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	decision, err := client.Evaluate(context.Background(), ports.PolicyRequest{
		Subject:  map[string]any{"id": "user"},
		Action:   "read",
		Resource: map[string]any{"id": "doc"},
		Context:  map[string]any{"tenant": "acme"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !decision.Allow {
		t.Fatal("expected allow")
	}
}

func TestEvaluateResultKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result": {"allow": true}}`))
	}))
	defer server.Close()

	client, err := New(Config{
		DecisionURL: server.URL,
		ResultKey:   "allow",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	decision, err := client.Evaluate(context.Background(), ports.PolicyRequest{Action: "read"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !decision.Allow {
		t.Fatal("expected allow")
	}
}

func TestEvaluateNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("deny"))
	}))
	defer server.Close()

	client, err := New(Config{DecisionURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.Evaluate(context.Background(), ports.PolicyRequest{Action: "read"}); err == nil {
		t.Fatal("expected error")
	}
}
