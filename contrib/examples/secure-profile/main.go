// Command secure-profile shows a hardened profile wiring example.
package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/logzap"
	"github.com/aatuh/api-toolkit/contrib/v2/bootstrap"
	oteltrace "github.com/aatuh/api-toolkit/contrib/v2/middleware/oteltrace"
	"github.com/aatuh/api-toolkit/v2/httpx"
	securemw "github.com/aatuh/api-toolkit/v2/middleware/secure"
)

func main() {
	log := logzap.NewProduction()
	router := chi.New()

	profile, err := bootstrap.ProfileStrictAPI(log,
		bootstrap.WithSecureOptions(securemw.WithCrossOriginIsolation()),
		bootstrap.WithRequestTimeout(5*time.Second),
		bootstrap.WithOTelOptions(oteltrace.Options{TracerName: "example-api"}),
	)
	if err != nil {
		log.Error("profile init failed", "err", err)
		return
	}
	profile.Apply(router)

	router.Get("/hello", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
