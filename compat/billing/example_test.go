package billing_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v4/compat/billing"
)

func ExampleCheckoutSessionRequest() {
	request := billing.CheckoutSessionRequest{
		PriceID:    "price_basic",
		SuccessURL: "https://app.example.test/billing/success",
		CancelURL:  "https://app.example.test/billing/cancel",
		Mode:       "subscription",
		Metadata:   map[string]string{"tenant_id": "tenant-1"},
	}

	fmt.Println(request.PriceID)
	fmt.Println(request.Metadata["tenant_id"])

	// Output:
	// price_basic
	// tenant-1
}
