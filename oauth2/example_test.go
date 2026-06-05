package oauth2_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/oauth2"
)

func ExampleRequireScopes() {
	claims := oauth2.TokenClaims{Scopes: []string{"widgets:read widgets:write"}}

	err := oauth2.RequireScopes(claims, "widgets:read")
	fmt.Println(err == nil)

	// Output:
	// true
}
