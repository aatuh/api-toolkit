package authorization_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/authorization"
)

func ExampleApplyScope() {
	filters := authorization.ApplyScope(
		map[string]any{"status": "active"},
		authorization.Scope{TenantID: "tenant-1", UserID: "user-1"},
	)

	fmt.Println(filters["status"])
	fmt.Println(filters["tenant_id"])
	fmt.Println(filters["user_id"])

	// Output:
	// active
	// tenant-1
	// user-1
}
