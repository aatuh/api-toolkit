package secure_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v4/middleware/secure"
)

func ExampleCSPPolicy() {
	fmt.Println(secure.CSPPolicy(secure.CSPProfileAPI))

	// Output:
	// default-src 'none'; frame-ancestors 'none'
}
