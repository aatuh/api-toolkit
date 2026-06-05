package negotiation_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/negotiation"
)

func ExampleNegotiate() {
	mediaType, ok := negotiation.Negotiate(
		"application/json",
		[]negotiation.MediaType{"application/json", "application/problem+json"},
	)

	fmt.Println(mediaType)
	fmt.Println(ok)

	// Output:
	// application/json
	// true
}
