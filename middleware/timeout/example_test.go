package timeout_test

import (
	"fmt"
	"time"

	"github.com/aatuh/api-toolkit/v4/middleware/timeout"
)

func ExampleNewPropagator() {
	middleware, err := timeout.NewPropagator(timeout.Options{Timeout: time.Second})
	if err != nil {
		panic(err)
	}

	fmt.Println(middleware.Timeout)

	// Output:
	// 1s
}
