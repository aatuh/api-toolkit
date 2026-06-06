package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v3/binding"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	"github.com/aatuh/api-toolkit/v3/fielderrors"
	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/middleware/maxbody"
	"github.com/aatuh/api-toolkit/v3/middleware/timeout"
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
	plain := plainNetHTTP()
	_ = plain

	server := &http.Server{
		Addr:              ":8080",
		Handler:           toolkitNetHTTP(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func plainNetHTTP() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		writePlainJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /widgets", createWidgetPlain)

	return mux
}

func createWidgetPlain(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req createWidgetRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" || req.Quantity < 1 {
		http.Error(w, "name and positive quantity are required", http.StatusBadRequest)
		return
	}

	writePlainJSON(w, http.StatusCreated, createWidget(req))
}

func writePlainJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func toolkitNetHTTP() http.Handler {
	mux := http.NewServeMux()
	health.NewBasicHandler().RegisterPublicRoutesTo(serveMuxGetRouter{mux: mux})
	mux.HandleFunc("POST /widgets", createWidgetToolkit)

	return toolkitMiddleware(mux, defaultTimeout)
}

func createWidgetToolkit(w http.ResponseWriter, r *http.Request) {
	req, err := binding.DecodeJSON[createWidgetRequest](r, binding.JSONConfig{
		MaxBytes:      maxRequestBodyBytes,
		RequireObject: true,
	})
	if err != nil {
		binding.WriteValidationProblem(w, err)
		return
	}
	if req.Quantity < 1 {
		binding.WriteValidationProblem(w, fielderrors.FieldErrors{{
			Field:   "quantity",
			Code:    "minimum",
			Message: "quantity must be at least 1",
		}})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		binding.WriteValidationProblem(w, fielderrors.FieldErrors{{
			Field:   "name",
			Code:    "required",
			Message: "name is required",
		}})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, createWidget(req))
}

func toolkitMiddleware(next http.Handler, requestTimeout time.Duration) http.Handler {
	hardTimeout, err := timeout.NewHard(timeout.Options{
		Timeout:         requestTimeout,
		MaxCaptureBytes: 64 << 10,
	})
	if err != nil {
		panic(err)
	}
	bodyLimit, err := maxbody.New(maxbody.Options{MaxBytes: maxRequestBodyBytes})
	if err != nil {
		panic(err)
	}

	return bodyLimit.Handler(hardTimeout.Handler(next))
}

func createWidget(req createWidgetRequest) widgetResponse {
	return widgetResponse{
		ID:       "widget-123",
		Name:     strings.TrimSpace(req.Name),
		Quantity: req.Quantity,
	}
}

type serveMuxGetRouter struct {
	mux *http.ServeMux
}

func (r serveMuxGetRouter) Get(pattern string, h http.HandlerFunc) {
	r.mux.HandleFunc("GET "+pattern, h)
}
