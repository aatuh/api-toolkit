package maxbody_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/middleware/maxbody"
)

func ExampleNew() {
	middleware, err := maxbody.New(maxbody.Options{MaxBytes: 1 << 20})
	if err != nil {
		panic(err)
	}

	fmt.Println(middleware.MaxBytes)

	// Output:
	// 1048576
}
