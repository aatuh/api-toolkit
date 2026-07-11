package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/middleware/maxbody"
)

func main() {
	bodyLimit, err := maxbody.New(maxbody.Options{MaxBytes: 1 << 20})
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           bodyLimit.Handler(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}
