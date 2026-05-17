package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	compatbilling "github.com/aatuh/api-toolkit/v3/compat/billing"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestNewAppliesOptionsAndWebhookHelpers(t *testing.T) {
	provider := New("sk_test", "", WithDevMode(true))
	if provider == nil {
		t.Fatal("expected provider")
	}

	payload, err := json.Marshal(map[string]any{
		"id":      "evt_test",
		"type":    "checkout.session.completed",
		"created": int64(1),
		"data": map[string]any{
			"object": map[string]any{"id": "cs_test"},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	safeCtx := AllowInsecureWebhookContext(context.Background(), netip.MustParseAddr("127.0.0.1"))
	event, err := provider.ParseWebhook(safeCtx, payload, "")
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}
	if event.ID != "evt_test" || event.Type != "checkout.session.completed" {
		t.Fatalf("unexpected webhook event: %#v", event)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg := LoadConfig(nil)
	if cfg.Enabled {
		t.Fatal("expected stripe integration to default disabled")
	}
	if cfg.FrontendBaseURL != "http://localhost:3000" {
		t.Fatalf("FrontendBaseURL = %q, want %q", cfg.FrontendBaseURL, "http://localhost:3000")
	}
}

func TestHealthCheckerNilAndFailureBehavior(t *testing.T) {
	if checker := HealthChecker(nil); checker != nil {
		t.Fatal("expected nil checker for nil provider")
	}

	checker := HealthChecker(fakePaymentProvider{err: errors.New("stripe unavailable")})
	if checker == nil {
		t.Fatal("expected checker for provider")
	}
	result := checker.Check(context.Background())
	if result.Status != ports.HealthStatusDegraded {
		t.Fatalf("status = %s, want degraded", result.Status)
	}
}

func TestHealthCheckerReportsPriceCount(t *testing.T) {
	checker := HealthChecker(fakePaymentProvider{
		prices: []compatbilling.Price{{ID: "price_1"}, {ID: "price_2"}},
	})
	result := checker.Check(context.Background())
	if result.Status != ports.HealthStatusHealthy {
		t.Fatalf("status = %s, want healthy: %s", result.Status, result.Message)
	}
	details, ok := result.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("details = %T, want map", result.Details)
	}
	if details["price_count"] != 2 {
		t.Fatalf("price_count = %v, want 2", details["price_count"])
	}
}

type fakePaymentProvider struct {
	prices []compatbilling.Price
	err    error
}

func (p fakePaymentProvider) CreateCheckoutSession(context.Context, compatbilling.CheckoutSessionRequest) (compatbilling.CheckoutSession, error) {
	return compatbilling.CheckoutSession{}, errors.New("unused")
}

func (p fakePaymentProvider) ParseWebhook(context.Context, []byte, string) (compatbilling.WebhookEvent, error) {
	return compatbilling.WebhookEvent{}, errors.New("unused")
}

func (p fakePaymentProvider) ListPrices(context.Context) ([]compatbilling.Price, error) {
	return p.prices, p.err
}
