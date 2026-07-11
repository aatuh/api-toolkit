package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/client"
	"github.com/stripe/stripe-go/v79/webhook"

	compatbilling "github.com/aatuh/api-toolkit/v4/compat/billing"
	"github.com/aatuh/api-toolkit/v4/httpx/identity"
)

// Provider implements compatbilling.PaymentProvider using Stripe Checkout + webhooks.
type Provider struct {
	client        *client.API
	webhookSecret string
	skipVerify    bool
	devMode       bool
}

// Option customizes Provider behavior.
type Option func(*Provider)

// WithSkipVerify allows skipping webhook signature verification (dev only).
func WithSkipVerify(skip bool) Option {
	return func(p *Provider) {
		p.skipVerify = skip
	}
}

// WithDevMode enables development-only webhook behavior (e.g., skipping verification).
func WithDevMode(dev bool) Option {
	return func(p *Provider) {
		p.devMode = dev
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
func (p *Provider) CreateCheckoutSession(ctx context.Context, req compatbilling.CheckoutSessionRequest) (compatbilling.CheckoutSession, error) {
	if req.PriceID == "" && (req.Amount <= 0 || req.Currency == "") {
		return compatbilling.CheckoutSession{}, errors.New("price id or amount+currency required")
	}

	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(defaultString(req.Mode, string(stripe.CheckoutSessionModePayment))),
		SuccessURL: stripe.String(req.SuccessURL),
		CancelURL:  stripe.String(req.CancelURL),
	}
	params.Context = ctx
	if req.Locale != "" {
		params.Locale = stripe.String(req.Locale)
	}
	if len(req.Metadata) > 0 {
		params.Metadata = req.Metadata
	}
	if strings.TrimSpace(req.CustomerID) != "" {
		params.Customer = stripe.String(strings.TrimSpace(req.CustomerID))
	}
	if strings.TrimSpace(req.CustomerEmail) != "" {
		params.CustomerEmail = stripe.String(strings.TrimSpace(req.CustomerEmail))
	}
	if strings.TrimSpace(req.ClientReferenceID) != "" {
		params.ClientReferenceID = stripe.String(strings.TrimSpace(req.ClientReferenceID))
	}
	if len(req.SubscriptionMetadata) > 0 && strings.TrimSpace(req.Mode) == string(stripe.CheckoutSessionModeSubscription) {
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: req.SubscriptionMetadata,
		}
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
		return compatbilling.CheckoutSession{}, normalizeStripeError(err)
	}
	return compatbilling.CheckoutSession{ID: session.ID, URL: session.URL}, nil
}

// ParseWebhook verifies and parses the Stripe webhook payload.
func (p *Provider) ParseWebhook(ctx context.Context, payload []byte, sigHeader string) (compatbilling.WebhookEvent, error) {
	secret := strings.TrimSpace(p.webhookSecret)
	allowInsecure := p.allowInsecureWebhook(ctx)
	if secret == "" {
		if !allowInsecure {
			return compatbilling.WebhookEvent{}, errors.New("stripe webhook verification required")
		}
		var event stripe.Event
		if err := json.Unmarshal(payload, &event); err != nil {
			return compatbilling.WebhookEvent{}, err
		}
		return compatbilling.WebhookEvent{
			ID:        event.ID,
			Type:      string(event.Type),
			CreatedAt: time.Unix(event.Created, 0),
			Payload:   event.Data.Raw,
		}, nil
	}
	if p.skipVerify && allowInsecure {
		var event stripe.Event
		if err := json.Unmarshal(payload, &event); err != nil {
			return compatbilling.WebhookEvent{}, err
		}
		return compatbilling.WebhookEvent{
			ID:        event.ID,
			Type:      string(event.Type),
			CreatedAt: time.Unix(event.Created, 0),
			Payload:   event.Data.Raw,
		}, nil
	}
	event, err := webhook.ConstructEvent(payload, sigHeader, secret)
	if err != nil {
		return compatbilling.WebhookEvent{}, err
	}
	return compatbilling.WebhookEvent{
		ID:        event.ID,
		Type:      string(event.Type),
		CreatedAt: time.Unix(event.Created, 0),
		Payload:   event.Data.Raw,
	}, nil
}

type webhookCtxKey struct{}

type webhookContext struct {
	allowed bool
	ip      netip.Addr
}

// AllowInsecureWebhook marks the request as safe for skipping verification in dev mode.
// It only enables skipping when the source IP is private or loopback.
func AllowInsecureWebhook(ctx context.Context, ip netip.Addr) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if !isSafeWebhookIP(ip) {
		return ctx
	}
	return context.WithValue(ctx, webhookCtxKey{}, webhookContext{allowed: true, ip: ip})
}

// AllowInsecureWebhookFromRequest infers source IP from a request and marks the context accordingly.
func AllowInsecureWebhookFromRequest(ctx context.Context, r *http.Request, resolver identity.Resolver) context.Context {
	if r == nil {
		return ctx
	}
	ip, ok := resolver.ClientIP(r)
	if !ok {
		return ctx
	}
	return AllowInsecureWebhook(ctx, ip)
}

func (p *Provider) allowInsecureWebhook(ctx context.Context) bool {
	if !p.devMode {
		return false
	}
	if ctx == nil {
		return false
	}
	info, ok := ctx.Value(webhookCtxKey{}).(webhookContext)
	if !ok || !info.allowed {
		return false
	}
	return isSafeWebhookIP(info.ip)
}

func isSafeWebhookIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// ListPrices fetches active prices from Stripe.
func (p *Provider) ListPrices(ctx context.Context) ([]compatbilling.Price, error) {
	params := &stripe.PriceListParams{}
	params.Context = ctx
	params.Filters.AddFilter("active", "", "true")

	iter := p.client.Prices.List(params)
	var out []compatbilling.Price
	for iter.Next() {
		price := iter.Price()
		if price == nil {
			continue
		}
		out = append(out, compatbilling.Price{
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
		return nil, normalizeStripeError(err)
	}
	return out, nil
}

func defaultString(val, def string) string {
	if strings.TrimSpace(val) == "" {
		return def
	}
	return val
}
