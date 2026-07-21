package binding_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aatuh/api-toolkit/v4/binding"
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

type examplePublicError string

func (e examplePublicError) Error() string         { return "internal validation failure" }
func (e examplePublicError) PublicMessage() string { return string(e) }

func ExamplePublicError() {
	var err error = examplePublicError("postal code is invalid")
	var public binding.PublicError
	ok := errors.As(err, &public)

	fmt.Println(ok)
	fmt.Println(public.PublicMessage())

	// Output:
	// true
	// postal code is invalid
}
