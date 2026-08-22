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

func ExampleRequiredMode() {
	type update struct {
		Enabled bool `json:"enabled" required:"true"`
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/widgets/42", strings.NewReader(`{"enabled":false}`))
	decoded, err := binding.DecodeJSON[update](req, binding.JSONConfig{
		RequiredMode: binding.RequiredModePresent,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(decoded.Enabled)

	// Output:
	// false
}

func ExamplePathConfig_HasParam() {
	type routeParameter struct {
		Label string `path:"label" required:"true"`
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/labels/", nil)
	decoded, err := binding.DecodePath[routeParameter](req, binding.PathConfig{
		Param: func(*http.Request, string) string { return "" },
		HasParam: func(*http.Request, string) bool {
			return true
		},
		RequiredMode: binding.RequiredModePresent,
	})

	fmt.Println(err == nil, decoded.Label == "")
	// Output:
	// true true
}

func ExampleRequiredModeNonZero() {
	type update struct {
		Retries int `json:"retries" required:"true"`
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/widgets/42", strings.NewReader(`{"retries":0}`))
	_, err := binding.DecodeJSON[update](req, binding.JSONConfig{
		RequiredMode: binding.RequiredModeNonZero,
	})

	fmt.Println(err != nil)

	// Output:
	// true
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
