package main

//go:generate go run ./gen -spec openapi.json -out spec_gen.go

import (
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	openapimw "github.com/aatuh/api-toolkit/contrib/v2/middleware/openapi"
)

func main() {
	router := chi.New()

	validator, err := openapimw.NewFromFile("openapi.json",
		openapimw.WithResponseValidation(openapimw.ResponseValidationOptions{
			Enabled:      true,
			MaxBodyBytes: 1 << 20,
		}),
	)
	if err != nil {
		log.Fatalf("openapi validator: %v", err)
	}
	router.Use(validator.Middleware())

	server := &Server{}
	RegisterRoutes(router, server)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Println("listening on :8080")
	log.Fatal(srv.ListenAndServe())
}
