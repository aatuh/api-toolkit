// Command pagination shows a limit/offset list endpoint with query limits.
package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/v2/fielderrors"
	listx "github.com/aatuh/api-toolkit/v2/endpoints/list"
	"github.com/aatuh/api-toolkit/v2/httpx"
	querylimits "github.com/aatuh/api-toolkit/v2/middleware/querylimits"
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
	handler, err := newRouter()
	if err != nil {
		log.Fatalf("init query limits: %v", err)
	}

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

func newRouter() (http.Handler, error) {
	r := chi.New()
	qmw, err := querylimits.New(querylimits.Options{
		MaxParams:   20,
		MaxLimit:    maxLimit,
		LimitParam:  "limit",
		ErrorWriter: writePaginationLimitError,
	})
	if err != nil {
		return nil, err
	}
	r.Use(qmw.Middleware())
	r.Get("/items", listItems)
	return r, nil
}

func listItems(w http.ResponseWriter, r *http.Request) {
	query, err := listx.ParseListQuery(r, listx.ListQueryConfig{
		DefaultLimit: defaultLimit,
		MaxLimit:     maxLimit,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	limit := query.Limit
	offset := query.Offset
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

func writePaginationLimitError(w http.ResponseWriter, status int, p httpx.Problem) {
	switch p.Detail {
	case "invalid pagination limit":
		httpx.WriteError(w, fielderrors.FieldErrors{{
			Field:   "limit",
			Code:    "invalid",
			Message: "limit must be a positive integer",
		}})
	case "pagination limit exceeds maximum":
		httpx.WriteError(w, fielderrors.FieldErrors{{
			Field:   "limit",
			Code:    "max",
			Message: "limit exceeds maximum",
		}})
	default:
		httpx.WriteProblem(w, status, p)
	}
}
