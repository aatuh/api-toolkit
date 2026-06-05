package securityprofile_test

import (
	"fmt"
	"net/http"

	"github.com/aatuh/api-toolkit/v3/securityprofile"
)

func ExampleStreamingRouteOverride() {
	override := securityprofile.StreamingRouteOverride("/events", http.MethodGet)

	fmt.Println(override.Pattern)
	fmt.Println(override.Methods[0])

	// Output:
	// /events
	// GET
}
