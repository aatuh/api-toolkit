package swagstub_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/swagstub"
)

func ExampleRegister() {
	swagstub.Register("example-widget", &swagstub.Spec{Title: "Widget API"})

	fmt.Println(swagstub.Get("example-widget").Title)

	// Output:
	// Widget API
}
