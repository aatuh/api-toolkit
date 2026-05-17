// Command api-key shows API key authentication with scoped routes.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/chi"
	"github.com/aatuh/api-toolkit/v3/binding"
	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/middleware/auth/apikey"
	"github.com/aatuh/api-toolkit/v3/negotiation"
	"github.com/aatuh/api-toolkit/v3/routecontracts"
	"github.com/aatuh/api-toolkit/v3/specs"
)

const demoKey = "demo-admin-key"

func main() {
	router := chi.New()
	auth, err := apikey.NewMiddleware(apikey.Config{
		Verifier: newDemoVerifier([]byte("local-demo-secret"), demoKey),
	})
	if err != nil {
		log.Fatalf("api key middleware: %v", err)
	}
	registry := specs.NewRegistry(specs.Info{
		Title:   "API key example",
		Version: "local",
	})
	if err := specs.RegisterSchemaFrom[adminResponse](registry, "AdminResponse", specs.SchemaOptions{}); err != nil {
		log.Fatalf("admin response schema: %v", err)
	}
	registry.RegisterSchema("Problem", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":   map[string]any{"type": "string"},
			"title":  map[string]any{"type": "string"},
			"status": map[string]any{"type": "integer"},
			"detail": map[string]any{"type": "string"},
		},
	})
	registry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{
		Type: "apiKey",
		Name: "X-API-Key",
		In:   "header",
	})
	registry.RegisterResponse("Problem", specs.Response{
		Description: "Problem Details",
		Content: map[string]specs.MediaType{
			"application/problem+json": {SchemaRef: "#/components/schemas/Problem"},
		},
	})

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query, err := binding.DecodeQuery[adminQuery](r, binding.QueryConfig{})
		if err != nil {
			problem, status := httpx.ProblemFromErrorWithCatalog(err, nil, httpx.ErrorOptions{})
			httpx.WriteProblem(w, status, problem)
			return
		}
		principal, _ := apikey.PrincipalFromContext(r.Context())
		response := adminResponse{
			PrincipalID: principal.ID,
			Scopes:      principal.Scopes,
		}
		if query.Verbose {
			response.Name = principal.Name
		}
		httpx.WriteJSON(w, http.StatusOK, response)
	})
	contracts := routecontracts.NewRegistry(router, registry)
	if err := contracts.Get("/admin", specs.Operation{
		Summary: "Read admin principal",
		Tags:    []string{"admin"},
		Parameters: []specs.Parameter{
			{
				Name:        "Authorization",
				In:          "header",
				Description: "Use ApiKey credentials, for example: ApiKey demo-admin-key.",
				Required:    true,
				Schema:      map[string]any{"type": "string"},
			},
			{
				Name:        "verbose",
				In:          "query",
				Description: "Include optional display metadata.",
				Schema:      map[string]any{"type": "boolean"},
			},
		},
		Security: []specs.SecurityRequirement{
			{Name: "ApiKeyAuth", Scopes: []string{"admin:read"}},
		},
		Scopes: []string{"admin:read"},
		Responses: map[int]specs.Response{
			http.StatusOK: {
				Description: "Authenticated admin principal.",
				Content: map[string]specs.MediaType{
					"application/json": {SchemaRef: "#/components/schemas/AdminResponse"},
				},
			},
			http.StatusBadRequest:    {Ref: "#/components/responses/Problem"},
			http.StatusUnauthorized:  {Ref: "#/components/responses/Problem"},
			http.StatusForbidden:     {Ref: "#/components/responses/Problem"},
			http.StatusNotAcceptable: {Ref: "#/components/responses/Problem"},
		},
		Extensions: map[string]any{
			"x-auth-scheme": "ApiKey",
		},
	}, adminHandler, auth.Handler, apikey.RequireScopeMiddleware("admin:read"), negotiation.RequireAccept("application/json")); err != nil {
		log.Fatalf("admin route contract: %v", err)
	}
	router.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		doc, err := registry.OpenAPI()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(doc)
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

type demoVerifier struct {
	secret []byte
	keys   map[string]apikey.Principal
}

type adminQuery struct {
	Verbose bool `query:"verbose"`
}

type adminResponse struct {
	PrincipalID string   `json:"principal_id" required:"true"`
	Name        string   `json:"name,omitempty"`
	Scopes      []string `json:"scopes" required:"true"`
}

func newDemoVerifier(secret []byte, rawKey string) *demoVerifier {
	return &demoVerifier{
		secret: secret,
		keys: map[string]apikey.Principal{
			hashKey(secret, rawKey): {
				ID:     "demo-key",
				Name:   "Local demo key",
				Scopes: []string{"admin:read"},
			},
		},
	}
}

func (v *demoVerifier) VerifyAPIKey(ctx context.Context, key apikey.PresentedKey) (apikey.Principal, error) {
	principal, ok := v.keys[hashKey(v.secret, key.Value)]
	if !ok {
		return apikey.Principal{}, errors.New("api key not found")
	}
	return principal, nil
}

func hashKey(secret []byte, raw string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}
