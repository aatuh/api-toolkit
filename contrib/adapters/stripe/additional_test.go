package stripe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/healthchecktest"
	compatbilling "github.com/aatuh/api-toolkit/v3/compat/billing"
	"github.com/aatuh/api-toolkit/v3/httpx/identity"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestProviderOptionsAndWebhookContextHelpers(t *testing.T) {
	p := New("sk_test", "whsec_test", WithSkipVerify(true), WithDevMode(true))
	if !p.skipVerify || !p.devMode || p.webhookSecret != "whsec_test" || p.client == nil {
		t.Fatalf("provider = %#v", p)
	}
	if defaultString(" value ", "fallback") != " value " || defaultString(" ", "fallback") != "fallback" {
		t.Fatal("defaultString did not preserve non-blank or apply fallback")
	}
	if isSafeWebhookIP(netip.Addr{}) || !isSafeWebhookIP(netip.MustParseAddr("127.0.0.1")) || !isSafeWebhookIP(netip.MustParseAddr("10.0.0.1")) {
		t.Fatal("safe webhook IP classification mismatch")
	}
	var nilCtx context.Context
	ctx := AllowInsecureWebhook(nilCtx, netip.MustParseAddr("127.0.0.1"))
	if !p.allowInsecureWebhook(ctx) {
		t.Fatal("expected safe dev context to allow insecure webhook")
	}
	if p.allowInsecureWebhook(AllowInsecureWebhook(context.Background(), netip.MustParseAddr("203.0.113.10"))) {
		t.Fatal("public IP should not allow insecure webhook")
	}
}

func TestWebhookRequestContextAndMalformedPayloads(t *testing.T) {
	p := New("sk_test", "", WithDevMode(true))
	if got := AllowInsecureWebhookFromRequest(context.Background(), nil, identity.Resolver{}); got == nil {
		t.Fatal("nil request should return a context")
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", nil)
	req.RemoteAddr = "bad-remote-addr"
	ctx := AllowInsecureWebhookFromRequest(context.Background(), req, identity.Resolver{})
	if p.allowInsecureWebhook(ctx) {
		t.Fatal("invalid remote addr should not allow insecure webhook")
	}
	ctx = AllowInsecureWebhook(context.Background(), netip.MustParseAddr("127.0.0.1"))
	if _, err := p.ParseWebhook(ctx, []byte(`{bad`), ""); err == nil {
		t.Fatal("expected malformed dev webhook payload error")
	}
}

func TestHealthCheckerNilProvider(t *testing.T) {
	if HealthChecker(nil) != nil {
		t.Fatal("nil provider health checker should be nil")
	}
}

func TestHealthCheckerContract(t *testing.T) {
	t.Parallel()

	healthchecktest.AssertCheckerContract(t, HealthChecker(healthCheckPaymentProvider{}), "stripe", ports.HealthStatusHealthy)
}

type healthCheckPaymentProvider struct{}

func (healthCheckPaymentProvider) CreateCheckoutSession(context.Context, compatbilling.CheckoutSessionRequest) (compatbilling.CheckoutSession, error) {
	return compatbilling.CheckoutSession{}, errors.New("unused")
}

func (healthCheckPaymentProvider) ParseWebhook(context.Context, []byte, string) (compatbilling.WebhookEvent, error) {
	return compatbilling.WebhookEvent{}, errors.New("unused")
}

func (healthCheckPaymentProvider) ListPrices(context.Context) ([]compatbilling.Price, error) {
	return []compatbilling.Price{{ID: "price_contract"}}, nil
}
