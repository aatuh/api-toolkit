//lint:file-ignore SA1019 tests intentionally assert assignability against the deprecated ports billing surface during v2 migration.

package billing

import (
	"testing"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestCheckoutSessionRequestAliasMatchesPorts(t *testing.T) {
	req := CheckoutSessionRequest{
		Amount:   42,
		Currency: "eur",
	}

	var portsReq ports.CheckoutSessionRequest = req //nolint:staticcheck // Intentional v2 compatibility assertion.
	if portsReq.Amount != req.Amount {
		t.Fatalf("amount = %d, want %d", portsReq.Amount, req.Amount)
	}
	if portsReq.Currency != req.Currency {
		t.Fatalf("currency = %q, want %q", portsReq.Currency, req.Currency)
	}
}

func TestPaymentProviderAliasMatchesPorts(t *testing.T) {
	var compatProvider PaymentProvider
	var portsProvider ports.PaymentProvider = compatProvider //nolint:staticcheck // Intentional v2 compatibility assertion.
	if portsProvider != nil {
		t.Fatal("expected zero-value payment provider to stay nil across aliases")
	}
}
