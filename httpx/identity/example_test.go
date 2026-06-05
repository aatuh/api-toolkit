package identity_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v3/httpx/identity"
)

func ExampleResolver_ClientIPString() {
	proxies, err := identity.ParseTrustedProxies([]string{"10.0.0.1"})
	if err != nil {
		panic(err)
	}
	resolver := identity.Resolver{
		TrustedProxies: proxies,
		HeaderPolicy:   identity.HeaderPolicyXForwarded,
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	fmt.Println(resolver.ClientIPString(req))

	// Output:
	// 203.0.113.7
}
