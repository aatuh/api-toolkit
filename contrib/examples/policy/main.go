// Command policy shows policy-engine authorization with Cedar.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	cedarcore "github.com/cedar-policy/cedar-go"

	cedarad "github.com/aatuh/api-toolkit/contrib/v2/adapters/cedar"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/v2/authorization"
	"github.com/aatuh/api-toolkit/v2/httpx"
)

const policyText = `permit (
	principal,
	action == Action::"view",
	resource
) when { context.tenant == "acme" };`

func main() {
	var policy cedarcore.Policy
	if err := policy.UnmarshalCedar([]byte(policyText)); err != nil {
		log.Fatalf("parse policy: %v", err)
	}
	policies := cedarcore.NewPolicySet()
	policies.Add("policy0", &policy)

	engine, err := cedarad.New(cedarad.Config{Policies: policies})
	if err != nil {
		log.Fatalf("init policy engine: %v", err)
	}

	authorizer := authorization.NewPolicyAuthorizer(engine, authorization.PolicyAuthorizerOptions{
		ContextProvider: func(_ context.Context) any {
			return map[string]any{"tenant": "acme"}
		},
	})

	router := chi.New()
	router.Get("/docs/{id}", func(w http.ResponseWriter, r *http.Request) {
		subj := cedarcore.NewEntityUID("User", "user_123")
		resource := cedarcore.NewEntityUID("Document", cedarcore.String(chi.URLParam(r, "id")))
		if err := authorization.Require(r.Context(), authorizer, subj, "view", resource); err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"id": chi.URLParam(r, "id")})
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
