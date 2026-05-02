# Getting Started (10 minutes)

Audience: new users who want one minimal API, one run command, and one test that
exercises the same toolkit wiring used by the server.

## 1) Create a module

```sh
go mod init example.com/my-api
go get github.com/aatuh/api-toolkit/v2
go get github.com/aatuh/api-toolkit/contrib/v2
```

## 2) Create `main.go`

```go
package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v2/adapters/logzap"
	"github.com/aatuh/api-toolkit/contrib/v2/bootstrap"
	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/ports"
)

type widget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func newRouter(log ports.Logger) (http.Handler, error) {
	if log == nil {
		log = ports.NopLogger{}
	}
	r := chi.New()
	profile, err := bootstrap.ProfileStrictAPI(log)
	if err != nil {
		return nil, err
	}
	profile.Apply(r)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Post("/widgets", func(w http.ResponseWriter, r *http.Request) {
		widget := widget{ID: "w_123", Name: "starter"}
		httpx.WriteJSON(w, http.StatusCreated, widget)
	})
	return r, nil
}

func main() {
	log := logzap.NewProduction()
	handler, err := newRouter(log)
	if err != nil {
		log.Error("router init failed", "err", err)
		return
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
		log.Error("server failed", "err", err)
	}
}
```

Expected result: the application starts an HTTP server on `:8080` with the strict
API profile applied to the same router that owns the routes.

## 3) Run it

```sh
go run ./main.go
```

Try the health and write endpoints from another shell:

```sh
curl -s http://localhost:8080/health
curl -s -X POST http://localhost:8080/widgets
```

Expected responses:

```json
{"status":"ok"}
```

```json
{"id":"w_123","name":"starter"}
```

## 4) Add a tiny test

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestHealthUsesTutorialRouter(t *testing.T) {
	handler, err := newRouter(ports.NopLogger{})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON response, got %q", got)
	}
}
```

Run tests:

```sh
go test ./...
```

Next: use [cookbook.md](cookbook.md) for common patterns and
[../contrib/examples/README.md](../contrib/examples/README.md) for runnable
example applications.
