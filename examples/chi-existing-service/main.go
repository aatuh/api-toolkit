package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/aatuh/api-toolkit/v4/binding"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
	"github.com/aatuh/api-toolkit/v4/fielderrors"
	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/middleware/maxbody"
	"github.com/aatuh/api-toolkit/v4/middleware/timeout"
)

const (
	maxRequestBodyBytes = int64(1 << 20)
	defaultTimeout      = 2 * time.Second
)

type createWidgetRequest struct {
	Name     string `json:"name" required:"true"`
	Quantity int    `json:"quantity" required:"true"`
}

type widgetResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

func main() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           newRouter(defaultTimeout),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

func newRouter(requestTimeout time.Duration) http.Handler {
	router := chi.NewRouter()
	router.Use(mustBodyLimit().Handler)
	router.Use(mustHardTimeout(requestTimeout).Handler)

	health.NewBasicHandler().RegisterPublicRoutesTo(chiGetRouter{router: router})
	router.Post("/widgets", createWidget)
	router.Get("/widgets/{id}", getWidget)

	return router
}

func createWidget(w http.ResponseWriter, r *http.Request) {
	req, err := binding.DecodeJSON[createWidgetRequest](r, binding.JSONConfig{
		MaxBytes:      maxRequestBodyBytes,
		RequireObject: true,
	})
	if err != nil {
		binding.WriteValidationProblem(w, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		binding.WriteValidationProblem(w, fielderrors.FieldErrors{{
			Field:   "name",
			Code:    "required",
			Message: "name is required",
			Public:  true,
		}})
		return
	}
	if req.Quantity < 1 {
		binding.WriteValidationProblem(w, fielderrors.FieldErrors{{
			Field:   "quantity",
			Code:    "minimum",
			Message: "quantity must be at least 1",
			Public:  true,
		}})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, widgetResponse{
		ID:       "widget-123",
		Name:     strings.TrimSpace(req.Name),
		Quantity: req.Quantity,
	})
}

func getWidget(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeBadRequest),
			Title:  http.StatusText(http.StatusBadRequest),
			Detail: "widget id is required",
		})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, widgetResponse{
		ID:       id,
		Name:     "demo",
		Quantity: 1,
	})
}

func mustBodyLimit() *maxbody.Middleware {
	bodyLimit, err := maxbody.New(maxbody.Options{MaxBytes: maxRequestBodyBytes})
	if err != nil {
		panic(err)
	}
	return bodyLimit
}

func mustHardTimeout(requestTimeout time.Duration) *timeout.HardTimeout {
	hardTimeout, err := timeout.NewHard(timeout.Options{
		Timeout:         requestTimeout,
		MaxCaptureBytes: 64 << 10,
	})
	if err != nil {
		panic(err)
	}
	return hardTimeout
}

type chiGetRouter struct {
	router chi.Router
}

func (r chiGetRouter) Get(pattern string, h http.HandlerFunc) {
	r.router.Get(pattern, h)
}
