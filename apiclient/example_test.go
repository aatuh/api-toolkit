package apiclient_test

import (
	"fmt"
	"time"

	"github.com/aatuh/api-toolkit/v4/apiclient"
)

func ExamplePreconditionHeaders() {
	lastModified := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	headers := apiclient.PreconditionHeaders(`"widgets-v1"`, lastModified)

	fmt.Println(headers.Get("If-Match"))
	fmt.Println(headers.Get("If-Unmodified-Since"))

	// Output:
	// "widgets-v1"
	// Fri, 02 Jan 2026 03:04:05 GMT
}
