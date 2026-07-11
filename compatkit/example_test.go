package compatkit_test

import (
	"net/http"
	"testing"

	"github.com/aatuh/api-toolkit/v4/compatkit"
	"github.com/aatuh/api-toolkit/v4/httpx"
)

func ExampleRun() {
	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/problem", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: "Bad Request"})
	})

	var t testing.T
	compatkit.Run(&t, compatkit.Suite{
		Target: compatkit.Target{Handler: mux},
		Checks: compatkit.StableHTTPChecks(compatkit.StableHTTPConfig{
			ReadinessPath: "/readyz",
			ProblemRequest: compatkit.Request{
				Method: http.MethodGet,
				Path:   "/problem",
			},
			ProblemStatus: http.StatusBadRequest,
		}),
	})
}
