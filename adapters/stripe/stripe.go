package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/ports"
	stripe "github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/client"
	"github.com/stripe/stripe-go/v79/webhook"
)

// Provider implements ports.PaymentProvider using Stripe Checkout + webhooks.
type Provider struct {
	client        *client.API
	webhookSecret string
	skipVerify    bool
}

// Option customizes Provider behavior.
type Option func(*Provider)

// WithSkipVerify allows skipping webhook signature verification (dev only).
func WithSkipVerify(skip bool) Option {
	return func(p *Provider) {
		p.skipVerify = skip
	}
}

// New returns a Stripe-backed payment provider.
func New(secretKey, webhookSecret string, opts ...Option) *Provider {
	c := &client.API{}
	c.Init(secretKey, nil)
	p := &Provider{client: c, webhookSecret: webhookSecret}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// CreateCheckoutSession creates a hosted checkout session for a single line item.
func (p *Provider) CreateCheckoutSession(ctx context.Context, req ports.CheckoutSessionRequest) (ports.CheckoutSession, error) {
	if req.PriceID == "" && (req.Amount <= 0 || req.Currency == "") {
		return ports.CheckoutSession{}, errors.New("price id or amount+currency required")
	}

	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(defaultString(req.Mode, string(stripe.CheckoutSessionModePayment))),
		SuccessURL: stripe.String(req.SuccessURL),
		CancelURL:  stripe.String(req.CancelURL),
	}
	if req.Locale != "" {
		params.Locale = stripe.String(req.Locale)
	}
	if len(req.Metadata) > 0 {
		params.Metadata = req.Metadata
	}
	params.PaymentMethodTypes = []*string{stripe.String("card")}
	params.AllowPromotionCodes = stripe.Bool(true)

	if req.PriceID != "" {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(req.PriceID),
				Quantity: stripe.Int64(1),
			},
		}
	} else {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(req.Currency),
					UnitAmount: stripe.Int64(req.Amount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Token pack"),
					},
				},
			},
		}
	}

	session, err := p.client.CheckoutSessions.New(params)
	if err != nil {
		return ports.CheckoutSession{}, err
	}
	return ports.CheckoutSession{ID: session.ID, URL: session.URL}, nil
}

// ParseWebhook verifies and parses the Stripe webhook payload.
func (p *Provider) ParseWebhook(ctx context.Context, payload []byte, sigHeader string) (ports.WebhookEvent, error) {
	if p.skipVerify || strings.TrimSpace(p.webhookSecret) == "" {
		var event stripe.Event
		if err := json.Unmarshal(payload, &event); err != nil {
			return ports.WebhookEvent{}, err
		}
		return ports.WebhookEvent{
			ID:        event.ID,
			Type:      string(event.Type),
			CreatedAt: time.Unix(event.Created, 0),
			Payload:   event.Data.Raw,
		}, nil
	}
	event, err := webhook.ConstructEvent(payload, sigHeader, p.webhookSecret)
	if err != nil {
		return ports.WebhookEvent{}, err
	}
	return ports.WebhookEvent{
		ID:        event.ID,
		Type:      string(event.Type),
		CreatedAt: time.Unix(event.Created, 0),
		Payload:   event.Data.Raw,
	}, nil
}

// ListPrices fetches active prices from Stripe.
func (p *Provider) ListPrices(ctx context.Context) ([]ports.Price, error) {
	params := &stripe.PriceListParams{}
	params.Context = ctx
	params.Filters.AddFilter("active", "", "true")

	iter := p.client.Prices.List(params)
	var out []ports.Price
	for iter.Next() {
		price := iter.Price()
		if price == nil {
			continue
		}
		out = append(out, ports.Price{
			ID:         price.ID,
			ProductID:  price.Product.ID,
			Currency:   strings.ToLower(string(price.Currency)),
			UnitAmount: price.UnitAmount,
			Nickname:   price.Nickname,
			Metadata:   price.Metadata,
			Active:     price.Active,
		})
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func defaultString(val, def string) string {
	if strings.TrimSpace(val) == "" {
		return def
	}
	return val
}
