// Command auth shows Clerk auth wiring.
package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v3/adapters/logzap"
	"github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/clerk"
	"github.com/aatuh/api-toolkit/v3/authorization"
	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func main() {
	log := logzap.NewDevelopment("info")
	requireIssuedAt := true
	cfg := clerk.Config{
		Enabled:           true,
		JWKSURL:           "https://example.clerk.accounts.dev/.well-known/jwks.json",
		Issuer:            "https://example.clerk.accounts.dev",
		Audience:          "example",
		AllowedAlgorithms: []string{"RS256"},
		RequiredClaims: clerk.ClaimRequirements{
			RequireIssuedAt: &requireIssuedAt,
		},
	}
	auth, err := clerk.NewMiddleware(context.Background(), cfg, log)
	if err != nil {
		log.Error("init clerk middleware", "err", err)
		return
	}
	defer auth.Close()

	router := chi.New()
	router.Use(auth.Handler)

	admins := map[string]struct{}{
		"user_admin": {},
	}
	authorizer := ports.AuthorizerFunc(func(_ context.Context, subject any, action string, _ any) error {
		subj, ok := subject.(clerk.Subject)
		if !ok {
			return httpx.ErrForbidden
		}
		if action == "admin" {
			if _, ok := admins[subj.UserID]; !ok {
				return httpx.ErrForbidden
			}
		}
		return nil
	})

	router.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		subj, ok := clerk.SubjectFromContext(r.Context())
		if !ok {
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "missing auth subject",
			})
			return
		}
		if err := authorization.Require(r.Context(), authorizer, subj, "admin", nil); err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"role": "admin"})
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
		log.Error("server failed", "err", err)
	}
}
