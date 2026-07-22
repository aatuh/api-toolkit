package securityprofile_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v4/middleware/timeout"
	"github.com/aatuh/api-toolkit/v4/securityprofile"
)

func ExampleStreamingRouteOverride() {
	override := securityprofile.StreamingRouteOverride("/events", http.MethodGet)

	fmt.Println(override.Pattern)
	fmt.Println(override.Methods[0])

	// Output:
	// /events
	// GET
}

func ExampleRouteOverride_hardTimeoutCapabilities() {
	hardTimeout := true
	capabilities := timeout.RouteCapabilityFiniteJSON
	override := securityprofile.RouteOverride{
		Pattern:                 "/widgets",
		HardTimeout:             &hardTimeout,
		HardTimeoutCapabilities: &capabilities,
	}
	profile, err := securityprofile.New(
		securityprofile.WithRequireAuth(false),
		securityprofile.WithTimeout(time.Second),
		securityprofile.WithRouteOverrides(override),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(profile.Middlewares))

	// Output:
	// 2
}
