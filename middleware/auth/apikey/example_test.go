package apikey_test

import (
	"context"
	"fmt"

	"github.com/aatuh/api-toolkit/v3/middleware/auth/apikey"
)

func ExampleWithPrincipal() {
	ctx := apikey.WithPrincipal(context.Background(), apikey.Principal{
		ID:       "user-1",
		TenantID: "tenant-1",
		Scopes:   []string{"widgets:read"},
	})

	principal, ok := apikey.PrincipalFromContext(ctx)
	fmt.Println(ok)
	fmt.Println(principal.HasScope("widgets:read"))

	// Output:
	// true
	// true
}
