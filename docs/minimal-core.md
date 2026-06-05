# Minimal Core Dependency Path

Audience: existing-service users who want the smallest practical adoption path.

This path uses only the root module and four stable packages:

- `httpx`
- `binding`
- `middleware/maxbody`
- `middleware/timeout`

It does not use contrib, generated scaffolds, provider adapters, or root
business ports.

## Install

```sh
go get github.com/aatuh/api-toolkit/v3
```

## Example

```go
package main

import (
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v3/binding"
	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/middleware/maxbody"
	"github.com/aatuh/api-toolkit/v3/middleware/timeout"
)

type createWidgetRequest struct {
	Name string `json:"name" required:"true"`
}

func createWidget(w http.ResponseWriter, r *http.Request) {
	req, err := binding.DecodeJSON[createWidgetRequest](r, binding.JSONConfig{
		MaxBytes:      64 << 10,
		RequireObject: true,
	})
	if err != nil {
		binding.WriteValidationProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func main() {
	bodyLimit, err := maxbody.New(maxbody.Options{MaxBytes: 1 << 20})
	if err != nil {
		panic(err)
	}
	deadline, err := timeout.NewPropagator(timeout.Options{Timeout: 2 * time.Second})
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /widgets", createWidget)

	handler := deadline.Handler(bodyLimit.Handler(mux))
	if err := http.ListenAndServe(":8080", handler); err != nil {
		panic(err)
	}
}
```

## What this path proves

- The app keeps its existing router and deployment model.
- Request bodies are bounded before JSON decoding.
- Validation failures return Problem Details.
- Successful responses use buffered JSON writes.
- Deadlines propagate through `context.Context`; handlers still need to observe
  `r.Context().Done()` for long-running work.

Add packages only when the next concrete need appears. For example, add
`middleware/json` for stricter content-type handling, `queryparams` for
collection parsing, `endpoints/health` for probes, or `routecontracts` when you
want OpenAPI metadata.

