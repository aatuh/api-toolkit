package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
	"github.com/aatuh/api-toolkit/v3/httpx"
)

// Server implements the generated Handlers interface.
type Server struct{}

func (s *Server) ListPets(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, []Pet{
		{ID: "pet_1", Name: "Rex"},
	})
}

func (s *Server) CreatePet(w http.ResponseWriter, r *http.Request) {
	var payload NewPet
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, CreatePetBadRequest{Detail: "invalid JSON body"})
		return
	}
	if strings.TrimSpace(payload.Name) == "" {
		writeError(w, CreatePetBadRequest{
			Detail: "validation failed",
			Errors: fielderrors.FieldErrors{{
				Field:   "name",
				Code:    "required",
				Message: "name is required",
			}},
		})
		return
	}
	if strings.EqualFold(payload.Name, "duplicate") {
		writeError(w, CreatePetConflict{Detail: "pet already exists"})
		return
	}
	pet := Pet{
		ID:   "pet_2",
		Name: payload.Name,
	}
	httpx.WriteJSON(w, http.StatusCreated, pet)
}

func writeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var se StatusError
	if errors.As(err, &se) {
		status := se.StatusCode()
		problem := httpx.Problem{
			Type:   httpx.DefaultTypeForStatus(status),
			Title:  http.StatusText(status),
			Detail: se.Error(),
		}
		var fieldProvider fielderrors.Provider
		if errors.As(err, &fieldProvider) && len(fieldProvider.FieldErrors()) > 0 {
			problem.Type = httpx.DefaultTypeURI(httpx.TypeValidation)
			httpx.WriteProblemWithFieldErrors(w, status, problem, fieldProvider.FieldErrors())
			return
		}
		httpx.WriteProblem(w, status, problem)
		return
	}
	httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
		Title:  http.StatusText(http.StatusInternalServerError),
		Detail: "internal server error",
	})
}
