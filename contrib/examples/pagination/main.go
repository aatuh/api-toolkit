// Command pagination shows a limit/offset list endpoint with query limits.
package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/aatuh/api-toolkit-contrib/adapters/chi"
	"github.com/aatuh/api-toolkit/httpx"
	querylimits "github.com/aatuh/api-toolkit/middleware/querylimits"
)

const (
	defaultLimit = 10
	maxLimit     = 50
)

var items = []string{
	"alpha", "bravo", "charlie", "delta", "echo",
	"foxtrot", "golf", "hotel", "india", "juliet",
	"kilo", "lima", "mike", "november", "oscar",
	"papa", "quebec", "romeo", "sierra", "tango",
}

type listResponse struct {
	Items      []string `json:"items"`
	NextOffset *int     `json:"next_offset,omitempty"`
}

func main() {
	r := chi.New()
	qmw, err := querylimits.New(querylimits.Options{
		MaxParams:  20,
		MaxLimit:   maxLimit,
		LimitParam: "limit",
	})
	if err != nil {
		log.Fatalf("init query limits: %v", err)
	}
	r.Use(qmw.Middleware())
	r.Get("/items", listItems)

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

func listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := defaultLimit
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "invalid limit",
			})
			return
		}
		limit = parsed
	}
	offset := 0
	if raw := q.Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "invalid offset",
			})
			return
		}
		offset = parsed
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	resp := listResponse{
		Items: items[offset:end],
	}
	if end < len(items) {
		next := end
		resp.NextOffset = &next
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}
