# Getting Started (10 minutes)

This guide shows a minimal API with hardened defaults, a simple handler, and a test.

## 1) Create a module

```sh
go mod init example.com/my-api
go get github.com/aatuh/api-toolkit
go get github.com/aatuh/api-toolkit-contrib
```

## 2) Create `main.go`

```go
package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit-contrib/adapters/chi"
	"github.com/aatuh/api-toolkit-contrib/adapters/logzap"
	"github.com/aatuh/api-toolkit-contrib/bootstrap"
	"github.com/aatuh/api-toolkit/httpx"
)

type widget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	log := logzap.NewProduction()
	r := chi.New()

	profile, err := bootstrap.ProfileStrictAPI(log)
	if err != nil {
		log.Error("profile init failed", "err", err)
		return
	}
	profile.Apply(r)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Post("/widgets", func(w http.ResponseWriter, r *http.Request) {
		widget := widget{ID: "w_123", Name: "starter"}
		httpx.WriteJSON(w, http.StatusCreated, widget)
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server failed", "err", err)
	}
}
```

## 3) Run it

```sh
go run ./main.go
```

Test quickly:

```sh
curl -s http://localhost:8080/health
curl -s -X POST http://localhost:8080/widgets
```

## 4) Add a tiny test

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
```

Run tests:

```sh
go test ./...
```

Next: see the cookbook in `docs/cookbook.md` for common patterns.
