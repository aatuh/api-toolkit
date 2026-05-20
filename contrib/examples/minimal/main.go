// Command minimal shows a basic API wiring example.
package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v3/adapters/validation"
	"github.com/aatuh/api-toolkit/v3/fielderrors"
	"github.com/aatuh/api-toolkit/v3/httpx"
	jsonmw "github.com/aatuh/api-toolkit/v3/middleware/json"
)

type createWidgetRequest struct {
	Name     string `json:"name" validate:"string;required"`
	Quantity int    `json:"quantity" validate:"int;min=1"`
}

func main() {
	router := chi.New()
	validator := validation.New()

	router.Post("/widgets", func(w http.ResponseWriter, r *http.Request) {
		dec, err := jsonmw.StrictDecoder(r)
		if err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "invalid json body",
			})
			return
		}
		var req createWidgetRequest
		if err := dec.Decode(&req); err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "invalid json body",
			})
			return
		}
		if err := validator.ValidateStruct(r.Context(), req); err != nil {
			var provider fielderrors.Provider
			if errors.As(err, &provider) {
				httpx.WriteProblemWithFieldErrors(w, http.StatusBadRequest, httpx.Problem{
					Title:  http.StatusText(http.StatusBadRequest),
					Detail: "validation failed",
				}, provider.FieldErrors())
				return
			}
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "validation failed",
			})
			return
		}

		httpx.WriteJSON(w, http.StatusCreated, map[string]any{
			"id":       "widget_123",
			"name":     req.Name,
			"quantity": req.Quantity,
		})
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
