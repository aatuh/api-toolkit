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

func TestBillingCompatibilityAliasesCoverDeprecatedPortsSurface(t *testing.T) {
	var checkout CheckoutSessionRequest
	var portsCheckout ports.CheckoutSessionRequest = checkout //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsCheckout

	var session CheckoutSession
	var portsSession ports.CheckoutSession = session //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsSession

	var event WebhookEvent
	var portsEvent ports.WebhookEvent = event //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsEvent

	var price Price
	var portsPrice ports.Price = price //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsPrice

	var customerInput CustomerInput
	var portsCustomerInput ports.CustomerInput = customerInput //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsCustomerInput

	var address CustomerAddress
	var portsAddress ports.CustomerAddress = address //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsAddress

	var customer Customer
	var portsCustomer ports.Customer = customer //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsCustomer

	var setupInput SetupIntentInput
	var portsSetupInput ports.SetupIntentInput = setupInput //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsSetupInput

	var setup SetupIntent
	var portsSetup ports.SetupIntent = setup //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsSetup

	var paymentMethod PaymentMethod
	var portsPaymentMethod ports.PaymentMethod = paymentMethod //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsPaymentMethod

	var invoiceItemInput InvoiceItemInput
	var portsInvoiceItemInput ports.InvoiceItemInput = invoiceItemInput //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsInvoiceItemInput

	var invoiceItem InvoiceItem
	var portsInvoiceItem ports.InvoiceItem = invoiceItem //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsInvoiceItem

	var invoiceItemUpdate InvoiceItemUpdate
	var portsInvoiceItemUpdate ports.InvoiceItemUpdate = invoiceItemUpdate //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsInvoiceItemUpdate

	var invoiceInput InvoiceInput
	var portsInvoiceInput ports.InvoiceInput = invoiceInput //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsInvoiceInput

	var invoice Invoice
	var portsInvoice ports.Invoice = invoice //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsInvoice

	var flowType BillingPortalFlowType
	var portsFlowType ports.BillingPortalFlowType = flowType //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsFlowType

	var afterType BillingPortalFlowAfterCompletionType
	var portsAfterType ports.BillingPortalFlowAfterCompletionType = afterType //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsAfterType

	var after BillingPortalFlowAfterCompletion
	var portsAfter ports.BillingPortalFlowAfterCompletion = after //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsAfter

	var item BillingPortalFlowSubscriptionUpdateConfirmItem
	var portsItem ports.BillingPortalFlowSubscriptionUpdateConfirmItem = item //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsItem

	var confirm BillingPortalFlowSubscriptionUpdateConfirm
	var portsConfirm ports.BillingPortalFlowSubscriptionUpdateConfirm = confirm //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsConfirm

	var flow BillingPortalFlowData
	var portsFlow ports.BillingPortalFlowData = flow //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsFlow

	var portalInput BillingPortalSessionInput
	var portsPortalInput ports.BillingPortalSessionInput = portalInput //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsPortalInput

	var portal BillingPortalSession
	var portsPortal ports.BillingPortalSession = portal //nolint:staticcheck // Intentional v2 compatibility assertion.
	_ = portsPortal

	var compatBillingProvider BillingProvider
	var portsBillingProvider ports.BillingProvider = compatBillingProvider //nolint:staticcheck // Intentional v2 compatibility assertion.
	if portsBillingProvider != nil {
		t.Fatal("expected zero-value billing provider to stay nil across aliases")
	}

	if BillingPortalFlowTypeSubscriptionUpdateConfirm != ports.BillingPortalFlowTypeSubscriptionUpdateConfirm { //nolint:staticcheck // Intentional v2 compatibility assertion.
		t.Fatal("subscription update confirm constant drifted from ports alias")
	}
	if BillingPortalFlowAfterCompletionTypeRedirect != ports.BillingPortalFlowAfterCompletionTypeRedirect { //nolint:staticcheck // Intentional v2 compatibility assertion.
		t.Fatal("redirect constant drifted from ports alias")
	}
}
