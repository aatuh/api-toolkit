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

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/middleware/auth/apikey"
	"github.com/aatuh/api-toolkit/v2/specs"
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
	registry.Register(specs.Operation{
		Method:  http.MethodGet,
		Path:    "/admin",
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
		},
		Security: []specs.SecurityRequirement{
			{Name: "ApiKeyAuth", Scopes: []string{"admin:read"}},
		},
		Scopes: []string{"admin:read"},
		Responses: map[int]specs.Response{
			http.StatusOK: {
				Description:  "Authenticated admin principal.",
				ContentTypes: []string{"application/json"},
			},
			http.StatusUnauthorized: {
				Description:  "Missing or invalid API key.",
				ContentTypes: []string{"application/problem+json"},
			},
			http.StatusForbidden: {
				Description:  "The API key is missing the required scope.",
				ContentTypes: []string{"application/problem+json"},
			},
		},
		Extensions: map[string]any{
			"x-auth-scheme": "ApiKey",
		},
	})

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, _ := apikey.PrincipalFromContext(r.Context())
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"principal_id": principal.ID,
			"scopes":       principal.Scopes,
		})
	})
	router.Get("/admin", auth.Handler(apikey.RequireScopeMiddleware("admin:read")(adminHandler)).ServeHTTP)
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
