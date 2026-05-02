// Command webhook shows a minimal webhook receiver with signature verification.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/v2/webhooks"
)

const (
	signatureHeader = "X-Signature"
	maxBodyBytes    = 1 << 20
	sharedSecret    = "demo-secret"
)

type webhookEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func main() {
	r := chi.New()
	verifier, err := webhooks.NewHMACSHA256Verifier(webhooks.HMACConfig{
		Secret:     []byte(sharedSecret),
		HeaderName: signatureHeader,
	})
	if err != nil {
		log.Fatalf("webhook verifier: %v", err)
	}
	receiver := webhooks.Receiver[webhookEvent]{Config: webhooks.ReceiverConfig[webhookEvent]{
		Verifier:     verifier,
		MaxBodyBytes: maxBodyBytes,
		Handle: func(ctx context.Context, event webhooks.Event[webhookEvent]) error {
			log.Printf("accepted webhook event id=%s type=%s", event.Payload.ID, event.Payload.Type)
			return nil
		},
	}}
	r.Post("/webhooks/payment", receiver.ServeHTTP)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}
