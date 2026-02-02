// Command webhook shows a minimal webhook receiver with signature verification.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/v2/httpx"
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
	r.Post("/webhooks/payment", handleWebhook)

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

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteProblem(w, http.StatusRequestEntityTooLarge, httpx.Problem{
			Title:  http.StatusText(http.StatusRequestEntityTooLarge),
			Detail: "payload too large",
		})
		return
	}
	if !verifySignature(body, r.Header.Get(signatureHeader), sharedSecret) {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
			Title:  http.StatusText(http.StatusUnauthorized),
			Detail: "invalid webhook signature",
		})
		return
	}
	var evt webhookEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
			Title:  http.StatusText(http.StatusBadRequest),
			Detail: "invalid json payload",
		})
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{
		"status":   "accepted",
		"event_id": evt.ID,
	})
}

func verifySignature(body []byte, signature, secret string) bool {
	trimmed := strings.TrimSpace(signature)
	if trimmed == "" || secret == "" {
		return false
	}
	got, err := hex.DecodeString(trimmed)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, got)
}
