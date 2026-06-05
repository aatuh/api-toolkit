package binding_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aatuh/api-toolkit/v3/binding"
)

type exampleCreateWidget struct {
	Name     string `json:"name" required:"true"`
	Quantity int    `json:"quantity" required:"true"`
}

func ExampleDecodeJSON() {
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/widgets",
		strings.NewReader(`{"name":"starter","quantity":2}`),
	)

	widget, err := binding.DecodeJSON[exampleCreateWidget](req, binding.JSONConfig{MaxBytes: 1 << 20})
	if err != nil {
		panic(err)
	}

	fmt.Println(widget.Name, widget.Quantity)

	// Output:
	// starter 2
}
