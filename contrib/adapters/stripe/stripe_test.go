package stripe

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	stripeapi "github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/client"
	"github.com/stripe/stripe-go/v79/form"
	stripewebhook "github.com/stripe/stripe-go/v79/webhook"

	compatbilling "github.com/aatuh/api-toolkit/v2/compat/billing"
	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestParseWebhookRequiresVerificationWhenBypassIsNotAllowed(t *testing.T) {
	payload := testWebhookPayload()

	tests := []struct {
		name string
		p    *Provider
		ctx  func() context.Context
	}{
		{
			name: "missing secret uses secure default",
			p:    New("sk_test", ""),
			ctx:  context.Background,
		},
		{
			name: "public ip is not allowed in dev mode",
			p:    New("sk_test", "", WithDevMode(true)),
			ctx: func() context.Context {
				return AllowInsecureWebhook(context.Background(), netip.MustParseAddr("203.0.113.10"))
			},
		},
		{
			name: "skip verify still requires safe dev context",
			p:    New("sk_test", "whsec_test", WithSkipVerify(true), WithDevMode(true)),
			ctx:  context.Background,
		},
		{
			name: "skip verify rejects public ip even in dev mode",
			p:    New("sk_test", "whsec_test", WithSkipVerify(true), WithDevMode(true)),
			ctx: func() context.Context {
				return AllowInsecureWebhook(context.Background(), netip.MustParseAddr("203.0.113.10"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.p.ParseWebhook(tc.ctx(), payload, "invalid")
			if err == nil {
				t.Fatal("expected verification error")
			}
			if !errors.Is(err, stripewebhook.ErrInvalidHeader) && err.Error() != "stripe webhook verification required" {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseWebhookAllowsUnsignedPayloadFromSafeDevContext(t *testing.T) {
	t.Parallel()

	p := New("sk_test", "", WithDevMode(true))
	ctx := AllowInsecureWebhook(context.Background(), netip.MustParseAddr("127.0.0.1"))

	got, err := p.ParseWebhook(ctx, testWebhookPayload(), "")
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}

	assertWebhookEvent(t, got)
}

func TestParseWebhookAllowsSkipVerifyOnlyInSafeDevContext(t *testing.T) {
	t.Parallel()

	p := New("sk_test", "whsec_test", WithSkipVerify(true), WithDevMode(true))
	ctx := AllowInsecureWebhook(context.Background(), netip.MustParseAddr("10.0.0.10"))

	got, err := p.ParseWebhook(ctx, testWebhookPayload(), "invalid")
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}

	assertWebhookEvent(t, got)
}

func TestParseWebhookAllowsInsecureDevContextFromSafeRequest(t *testing.T) {
	t.Parallel()

	p := New("sk_test", "", WithDevMode(true))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", nil)
	req.RemoteAddr = "127.0.0.1:443"
	ctx := AllowInsecureWebhookFromRequest(context.Background(), req, identity.Resolver{})

	got, err := p.ParseWebhook(ctx, testWebhookPayload(), "")
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}

	assertWebhookEvent(t, got)
}

func TestParseWebhookRejectsInsecureDevContextFromPublicRequest(t *testing.T) {
	t.Parallel()

	p := New("sk_test", "", WithDevMode(true))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", nil)
	req.RemoteAddr = "203.0.113.10:443"
	ctx := AllowInsecureWebhookFromRequest(context.Background(), req, identity.Resolver{})

	_, err := p.ParseWebhook(ctx, testWebhookPayload(), "")
	if err == nil {
		t.Fatal("expected webhook verification error")
	}
	if err.Error() != "stripe webhook verification required" {
		t.Fatalf("error = %q, want verification required", err.Error())
	}
}

func TestParseWebhookVerifiesSignedPayload(t *testing.T) {
	t.Parallel()

	payload := testWebhookPayload()
	secret := "whsec_test"
	signed := stripewebhook.GenerateTestSignedPayload(&stripewebhook.UnsignedPayload{
		Payload:   payload,
		Secret:    secret,
		Timestamp: time.Now(),
	})

	p := New("sk_test", secret)
	got, err := p.ParseWebhook(context.Background(), signed.Payload, signed.Header)
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}

	assertWebhookEvent(t, got)
}

func TestBillingValidationGuards(t *testing.T) {
	t.Parallel()

	p := New("sk_test", "")
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "checkout session requires price or amount currency",
			run: func() error {
				_, err := p.CreateCheckoutSession(context.Background(), compatbilling.CheckoutSessionRequest{})
				return err
			},
			want: "price id or amount+currency required",
		},
		{
			name: "customer default payment method requires ids",
			run: func() error {
				return p.SetCustomerDefaultPaymentMethod(context.Background(), " ", "")
			},
			want: "customer id and payment method required",
		},
		{
			name: "retrieve payment method requires id",
			run: func() error {
				_, err := p.RetrievePaymentMethod(context.Background(), "")
				return err
			},
			want: "payment method id required",
		},
		{
			name: "setup intent requires customer id",
			run: func() error {
				_, err := p.CreateSetupIntent(context.Background(), compatbilling.SetupIntentInput{})
				return err
			},
			want: "customer id required",
		},
		{
			name: "invoice item requires customer id",
			run: func() error {
				_, err := p.CreateInvoiceItem(context.Background(), compatbilling.InvoiceItemInput{})
				return err
			},
			want: "customer id required",
		},
		{
			name: "retrieve invoice item requires id",
			run: func() error {
				_, err := p.RetrieveInvoiceItem(context.Background(), " ")
				return err
			},
			want: "invoice item id required",
		},
		{
			name: "update invoice item requires id",
			run: func() error {
				_, err := p.UpdateInvoiceItem(context.Background(), " ", compatbilling.InvoiceItemUpdate{})
				return err
			},
			want: "invoice item id required",
		},
		{
			name: "invoice requires customer id",
			run: func() error {
				_, err := p.CreateInvoice(context.Background(), compatbilling.InvoiceInput{})
				return err
			},
			want: "customer id required",
		},
		{
			name: "finalize invoice requires id",
			run: func() error {
				_, err := p.FinalizeInvoice(context.Background(), " ")
				return err
			},
			want: "invoice id required",
		},
		{
			name: "pay invoice requires id",
			run: func() error {
				_, err := p.PayInvoice(context.Background(), " ")
				return err
			},
			want: "invoice id required",
		},
		{
			name: "retrieve invoice requires id",
			run: func() error {
				_, err := p.RetrieveInvoice(context.Background(), " ")
				return err
			},
			want: "invoice id required",
		},
		{
			name: "billing portal session requires customer id",
			run: func() error {
				_, err := p.CreateBillingPortalSession(context.Background(), compatbilling.BillingPortalSessionInput{})
				return err
			},
			want: "customer id required",
		},
		{
			name: "subscription primary item requires id",
			run: func() error {
				_, err := p.GetSubscriptionPrimaryItemID(context.Background(), " ")
				return err
			},
			want: "subscription id required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.run()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if err.Error() != tc.want {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestProviderNormalizesStripeResourceMissingErrors(t *testing.T) {
	t.Parallel()

	p := newProviderWithBackendError(&stripeapi.Error{
		Code: stripeapi.ErrorCodeResourceMissing,
		Msg:  "resource missing",
	})

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "create checkout session",
			run: func() error {
				_, err := p.CreateCheckoutSession(context.Background(), compatbilling.CheckoutSessionRequest{
					PriceID:    "price_123",
					SuccessURL: "https://example.com/success",
					CancelURL:  "https://example.com/cancel",
				})
				return err
			},
		},
		{
			name: "list prices",
			run: func() error {
				_, err := p.ListPrices(context.Background())
				return err
			},
		},
		{
			name: "set customer default payment method",
			run: func() error {
				return p.SetCustomerDefaultPaymentMethod(context.Background(), "cus_123", "pm_123")
			},
		},
		{
			name: "retrieve payment method",
			run: func() error {
				_, err := p.RetrievePaymentMethod(context.Background(), "pm_123")
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.run()
			if !errors.Is(err, ports.ErrResourceMissing) {
				t.Fatalf("expected ports.ErrResourceMissing, got %v", err)
			}

			var stripeErr *stripeapi.Error
			if !errors.As(err, &stripeErr) {
				t.Fatalf("expected wrapped stripe error, got %T", err)
			}
		})
	}
}

func TestNormalizeStripeErrorLeavesNonMissingErrorsUntouched(t *testing.T) {
	t.Parallel()

	err := &stripeapi.Error{
		Code: stripeapi.ErrorCodeCardDeclined,
		Msg:  "card declined",
	}

	got := normalizeStripeError(err)
	if !errors.Is(got, err) {
		t.Fatalf("expected original error, got %v", got)
	}
}

func TestBillingPortalFlowDataParams(t *testing.T) {
	t.Parallel()

	flow := &compatbilling.BillingPortalFlowData{
		Type: compatbilling.BillingPortalFlowTypeSubscriptionUpdateConfirm,
		AfterCompletion: &compatbilling.BillingPortalFlowAfterCompletion{
			Type:              compatbilling.BillingPortalFlowAfterCompletionTypeRedirect,
			RedirectReturnURL: "https://example.com/account",
		},
		SubscriptionUpdateConfirm: &compatbilling.BillingPortalFlowSubscriptionUpdateConfirm{
			SubscriptionID: "sub_123",
			Items: []compatbilling.BillingPortalFlowSubscriptionUpdateConfirmItem{
				{
					SubscriptionItemID: "si_123",
					PriceID:            "price_123",
					Quantity:           2,
				},
				{},
			},
		},
	}

	got := billingPortalFlowDataParams(flow)
	if got == nil {
		t.Fatal("expected flow params")
	}
	if got.Type == nil || *got.Type != string(compatbilling.BillingPortalFlowTypeSubscriptionUpdateConfirm) {
		t.Fatalf("flow type = %v", got.Type)
	}
	if got.AfterCompletion == nil || got.AfterCompletion.Type == nil || *got.AfterCompletion.Type != string(compatbilling.BillingPortalFlowAfterCompletionTypeRedirect) {
		t.Fatalf("after completion type = %+v", got.AfterCompletion)
	}
	if got.AfterCompletion.Redirect == nil || got.AfterCompletion.Redirect.ReturnURL == nil || *got.AfterCompletion.Redirect.ReturnURL != "https://example.com/account" {
		t.Fatalf("after completion redirect = %+v", got.AfterCompletion.Redirect)
	}
	if got.SubscriptionUpdateConfirm == nil || got.SubscriptionUpdateConfirm.Subscription == nil || *got.SubscriptionUpdateConfirm.Subscription != "sub_123" {
		t.Fatalf("subscription update confirm = %+v", got.SubscriptionUpdateConfirm)
	}
	if len(got.SubscriptionUpdateConfirm.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(got.SubscriptionUpdateConfirm.Items))
	}

	item := got.SubscriptionUpdateConfirm.Items[0]
	if item.ID == nil || *item.ID != "si_123" {
		t.Fatalf("item id = %v", item.ID)
	}
	if item.Price == nil || *item.Price != "price_123" {
		t.Fatalf("item price = %v", item.Price)
	}
	if item.Quantity == nil || *item.Quantity != 2 {
		t.Fatalf("item quantity = %v", item.Quantity)
	}
}

func TestBillingPortalFlowDataParamsReturnsNilForEmptyFlow(t *testing.T) {
	t.Parallel()

	if got := billingPortalFlowDataParams(&compatbilling.BillingPortalFlowData{}); got != nil {
		t.Fatalf("expected nil flow params, got %+v", got)
	}
}

func TestInvoiceFromStripe(t *testing.T) {
	t.Parallel()

	got := invoiceFromStripe(&stripeapi.Invoice{
		ID:               "in_123",
		Status:           stripeapi.InvoiceStatusOpen,
		Currency:         stripeapi.CurrencyUSD,
		AmountDue:        0,
		Total:            1250,
		AmountPaid:       500,
		HostedInvoiceURL: "https://billing.example/in_123",
		Created:          1710000000,
		DueDate:          1710003600,
		StatusTransitions: &stripeapi.InvoiceStatusTransitions{
			FinalizedAt: 1710001800,
			PaidAt:      1710005400,
		},
	})

	if got.ID != "in_123" {
		t.Fatalf("invoice id = %q", got.ID)
	}
	if got.Status != string(stripeapi.InvoiceStatusOpen) {
		t.Fatalf("status = %q", got.Status)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency = %q", got.Currency)
	}
	if got.AmountDue != 1250 {
		t.Fatalf("amount due = %d", got.AmountDue)
	}
	if got.AmountPaid != 500 {
		t.Fatalf("amount paid = %d", got.AmountPaid)
	}
	if got.HostedInvoiceURL != "https://billing.example/in_123" {
		t.Fatalf("hosted invoice url = %q", got.HostedInvoiceURL)
	}
	if !got.CreatedAt.Equal(time.Unix(1710000000, 0)) {
		t.Fatalf("created at = %s", got.CreatedAt)
	}
	if got.DueDate == nil || !got.DueDate.Equal(time.Unix(1710003600, 0)) {
		t.Fatalf("due date = %v", got.DueDate)
	}
	if got.FinalizedAt == nil || !got.FinalizedAt.Equal(time.Unix(1710001800, 0)) {
		t.Fatalf("finalized at = %v", got.FinalizedAt)
	}
	if got.PaidAt == nil || !got.PaidAt.Equal(time.Unix(1710005400, 0)) {
		t.Fatalf("paid at = %v", got.PaidAt)
	}
}

func assertWebhookEvent(t *testing.T, got compatbilling.WebhookEvent) {
	t.Helper()

	if got.ID != "evt_123" {
		t.Fatalf("event id = %q", got.ID)
	}
	if got.Type != "checkout.session.completed" {
		t.Fatalf("event type = %q", got.Type)
	}
	if !got.CreatedAt.Equal(time.Unix(1710000000, 0)) {
		t.Fatalf("created at = %s", got.CreatedAt)
	}
	if string(got.Payload) != `{"id":"cs_test_123"}` {
		t.Fatalf("payload = %s", string(got.Payload))
	}
}

func testWebhookPayload() []byte {
	return []byte(`{"id":"evt_123","type":"checkout.session.completed","api_version":"` + stripeapi.APIVersion + `","created":1710000000,"data":{"object":{"id":"cs_test_123"}}}`)
}

func newProviderWithBackendError(err error) *Provider {
	c := &client.API{}
	backend := &stubStripeBackend{err: err}
	c.Init("sk_test", &stripeapi.Backends{
		API:     backend,
		Connect: backend,
		Uploads: backend,
	})
	return &Provider{client: c}
}

type stubStripeBackend struct {
	err error
}

func (b *stubStripeBackend) Call(method, path, key string, params stripeapi.ParamsContainer, v stripeapi.LastResponseSetter) error {
	return b.err
}

func (b *stubStripeBackend) CallStreaming(method, path, key string, params stripeapi.ParamsContainer, v stripeapi.StreamingLastResponseSetter) error {
	return b.err
}

func (b *stubStripeBackend) CallRaw(method, path, key string, body *form.Values, params *stripeapi.Params, v stripeapi.LastResponseSetter) error {
	return b.err
}

func (b *stubStripeBackend) CallMultipart(method, path, key, boundary string, body *bytes.Buffer, params *stripeapi.Params, v stripeapi.LastResponseSetter) error {
	return b.err
}

func (b *stubStripeBackend) SetMaxNetworkRetries(maxNetworkRetries int64) {}
