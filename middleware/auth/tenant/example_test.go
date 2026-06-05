package tenant_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v3/authorization"
	"github.com/aatuh/api-toolkit/v3/middleware/auth/tenant"
)

func ExampleNew() {
	mw, err := tenant.New(tenant.Options{HeaderName: "X-Tenant-ID"})
	if err != nil {
		panic(err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope, _ := authorization.ScopeFromContext(r.Context())
		fmt.Fprint(w, scope.TenantID)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	fmt.Println(recorder.Body.String())

	// Output:
	// tenant-1
}
