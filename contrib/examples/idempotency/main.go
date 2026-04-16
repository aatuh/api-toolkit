// Command idempotency shows idempotent endpoint wiring.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/idempotency"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/validation"
	"github.com/aatuh/api-toolkit/v2/httpx"
	idempotencymw "github.com/aatuh/api-toolkit/v2/middleware/idempotency"
	"github.com/aatuh/api-toolkit/v2/ports"
)

type checkoutRequest struct {
	Amount   int64  `json:"amount" validate:"min=1"`
	Currency string `json:"currency" validate:"required"`
}

type fakeProvider struct{}

//nolint:unparam
func (fakeProvider) CreateCheckoutSession(_ context.Context, _ ports.CheckoutSessionRequest) (ports.CheckoutSession, error) {
	return ports.CheckoutSession{
		ID:  "cs_test_123",
		URL: "https://checkout.example.test/session/cs_test_123",
	}, nil
}

func (fakeProvider) ParseWebhook(_ context.Context, _ []byte, _ string) (ports.WebhookEvent, error) {
	return ports.WebhookEvent{}, nil
}

func (fakeProvider) ListPrices(_ context.Context) ([]ports.Price, error) {
	return nil, nil
}

func main() {
	router := chi.New()
	store := idempotency.NewMemoryStore()
	idem, err := idempotencymw.New(idempotencymw.Options{
		Store: store,
	})
	if err != nil {
		log.Fatalf("init idempotency middleware: %v", err)
	}
	// This example is unauthenticated. In authenticated APIs, apply auth and
	// tenant middleware before idempotency so the default request hash includes
	// caller scope.
	router.Use(idem.Middleware())

	validator := validation.New()
	provider := fakeProvider{}

	router.Post("/checkout", func(w http.ResponseWriter, r *http.Request) {
		var req checkoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "invalid json body",
			})
			return
		}
		if err := validator.ValidateStruct(r.Context(), req); err != nil {
			httpx.WriteError(w, err)
			return
		}
		session, err := provider.CreateCheckoutSession(r.Context(), ports.CheckoutSessionRequest{
			Amount:   req.Amount,
			Currency: req.Currency,
		})
		if err != nil {
			httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
				Title:  http.StatusText(http.StatusInternalServerError),
				Detail: "payment provider error",
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, session)
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
