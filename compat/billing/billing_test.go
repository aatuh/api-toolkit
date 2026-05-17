package billing

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestHostedBillingContractsRemainInCompatPackage(t *testing.T) {
	req := CheckoutSessionRequest{
		Amount:        42,
		Currency:      "eur",
		PriceID:       "price_123",
		SuccessURL:    "https://example.com/success",
		CancelURL:     "https://example.com/cancel",
		Metadata:      map[string]string{"tenant": "tenant_123"},
		Mode:          "subscription",
		CustomerID:    "cus_123",
		CustomerEmail: "owner@example.com",
	}
	provider := fakePaymentProvider{}

	session, err := provider.CreateCheckoutSession(context.Background(), req)
	if err != nil {
		t.Fatalf("create checkout session: %v", err)
	}
	if session.ID == "" || session.URL == "" {
		t.Fatalf("expected hosted checkout session, got %+v", session)
	}

	event, err := provider.ParseWebhook(context.Background(), []byte(`{"ok":true}`), "sig")
	if err != nil {
		t.Fatalf("parse webhook: %v", err)
	}
	if event.Type != "checkout.session.completed" || !json.Valid(event.Payload) {
		t.Fatalf("unexpected webhook event: %+v", event)
	}
}

func TestBillingProviderContractCoversPortalAndInvoices(t *testing.T) {
	provider := fakeBillingProvider{}
	portal, err := provider.CreateBillingPortalSession(context.Background(), BillingPortalSessionInput{
		CustomerID: "cus_123",
		ReturnURL:  "https://example.com/billing",
		Flow: &BillingPortalFlowData{
			Type: BillingPortalFlowTypeSubscriptionUpdateConfirm,
			AfterCompletion: &BillingPortalFlowAfterCompletion{
				Type:              BillingPortalFlowAfterCompletionTypeRedirect,
				RedirectReturnURL: "https://example.com/done",
			},
		},
	})
	if err != nil {
		t.Fatalf("create billing portal session: %v", err)
	}
	if portal.URL == "" {
		t.Fatalf("expected portal url, got %+v", portal)
	}
}

type fakePaymentProvider struct{}

func (fakePaymentProvider) CreateCheckoutSession(context.Context, CheckoutSessionRequest) (CheckoutSession, error) {
	return CheckoutSession{ID: "cs_123", URL: "https://checkout.example/cs_123"}, nil
}

func (fakePaymentProvider) ParseWebhook(_ context.Context, payload []byte, _ string) (WebhookEvent, error) {
	return WebhookEvent{
		ID:        "evt_123",
		Type:      "checkout.session.completed",
		CreatedAt: time.Unix(1, 0),
		Payload:   append(json.RawMessage(nil), payload...),
	}, nil
}

func (fakePaymentProvider) ListPrices(context.Context) ([]Price, error) {
	return []Price{{ID: "price_123", Currency: "eur", UnitAmount: 4200, Active: true}}, nil
}

type fakeBillingProvider struct{}

func (fakeBillingProvider) CreateCustomer(context.Context, CustomerInput) (Customer, error) {
	return Customer{ID: "cus_123"}, nil
}

func (fakeBillingProvider) UpdateCustomer(context.Context, string, CustomerInput) error {
	return nil
}

func (fakeBillingProvider) CreateSetupIntent(context.Context, SetupIntentInput) (SetupIntent, error) {
	return SetupIntent{ID: "seti_123", ClientSecret: "secret"}, nil
}

func (fakeBillingProvider) SetCustomerDefaultPaymentMethod(context.Context, string, string) error {
	return nil
}

func (fakeBillingProvider) RetrievePaymentMethod(context.Context, string) (PaymentMethod, error) {
	return PaymentMethod{ID: "pm_123", Brand: "visa", Last4: "4242"}, nil
}

func (fakeBillingProvider) CreateInvoiceItem(context.Context, InvoiceItemInput) (InvoiceItem, error) {
	return InvoiceItem{ID: "ii_123"}, nil
}

func (fakeBillingProvider) RetrieveInvoiceItem(context.Context, string) (InvoiceItem, error) {
	return InvoiceItem{ID: "ii_123"}, nil
}

func (fakeBillingProvider) UpdateInvoiceItem(context.Context, string, InvoiceItemUpdate) (InvoiceItem, error) {
	return InvoiceItem{ID: "ii_123"}, nil
}

func (fakeBillingProvider) CreateInvoice(context.Context, InvoiceInput) (Invoice, error) {
	return Invoice{ID: "in_123", Status: "draft"}, nil
}

func (fakeBillingProvider) FinalizeInvoice(context.Context, string) (Invoice, error) {
	return Invoice{ID: "in_123", Status: "open"}, nil
}

func (fakeBillingProvider) PayInvoice(context.Context, string) (Invoice, error) {
	return Invoice{ID: "in_123", Status: "paid"}, nil
}

func (fakeBillingProvider) RetrieveInvoice(context.Context, string) (Invoice, error) {
	return Invoice{ID: "in_123", Status: "paid"}, nil
}

func (fakeBillingProvider) CreateBillingPortalSession(context.Context, BillingPortalSessionInput) (BillingPortalSession, error) {
	return BillingPortalSession{ID: "bps_123", URL: "https://billing.example/session"}, nil
}

var _ PaymentProvider = fakePaymentProvider{}
var _ BillingProvider = fakeBillingProvider{}
