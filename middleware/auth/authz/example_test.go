package authz_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aatuh/api-toolkit/v2/middleware/auth/authz"
)

func ExampleNewRequireRoleMiddlewareChecked() {
	rolesFromContext := func(context.Context) []string {
		return []string{"admin"}
	}

	admin, err := authz.NewRequireRoleMiddlewareChecked("admin", rolesFromContext)
	if err != nil {
		panic(err)
	}
	if err := authz.ValidateRequireRoleMiddlewareRoutes([]authz.RequireRoleRouteSpec{
		{Method: http.MethodGet, Route: "/admin", Middleware: admin},
	}); err != nil {
		panic(err)
	}

	fmt.Println("authz startup checks passed")
	// Output: authz startup checks passed
}
