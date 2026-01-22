package httpx

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes a JSON response with a buffered marshal to avoid partial writes.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	if err := writeJSON(w, status, "application/json", v); err != nil {
		WriteProblem(w, http.StatusInternalServerError, Problem{
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: "failed to encode response",
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, contentType string, v any) error {
	if status <= 0 {
		status = http.StatusOK
	}
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, err = w.Write(append(body, '\n'))
	return err
}
