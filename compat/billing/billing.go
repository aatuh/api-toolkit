package billing

import (
	"context"
	"encoding/json"
	"time"
)

// CheckoutSessionRequest describes a hosted checkout request.
type CheckoutSessionRequest struct {
	Amount               int64             // Minor units (cents)
	Currency             string            // ISO currency, e.g. "eur"
	PriceID              string            // Provider-specific price ID (preferred)
	SuccessURL           string            // Where to redirect on success
	CancelURL            string            // Where to redirect on cancellation
	Metadata             map[string]string // Arbitrary key/value for reconciliation
	Mode                 string            // "payment" | "subscription"
	Locale               string            // Optional locale code, provider-specific
	CustomerID           string            // Optional provider customer ID
	CustomerEmail        string            // Optional customer email address
	ClientReferenceID    string            // Optional client-side correlation ID
	SubscriptionMetadata map[string]string // Metadata for subscription objects
}

// CheckoutSession represents a provider-created hosted checkout session.
type CheckoutSession struct {
	ID  string
	URL string
}

// WebhookEvent is a generic payment provider webhook payload wrapper.
type WebhookEvent struct {
	ID        string
	Type      string
	CreatedAt time.Time
	Payload   json.RawMessage
}

// Price represents a payment provider price.
type Price struct {
	ID         string
	ProductID  string
	Currency   string
	UnitAmount int64
	Nickname   string
	Metadata   map[string]string
	Active     bool
}

// PaymentProvider defines a hosted checkout and webhook contract.
type PaymentProvider interface {
	CreateCheckoutSession(ctx context.Context, req CheckoutSessionRequest) (CheckoutSession, error)
	ParseWebhook(ctx context.Context, payload []byte, sigHeader string) (WebhookEvent, error)
	ListPrices(ctx context.Context) ([]Price, error)
}

// CustomerInput describes a new billing customer.
type CustomerInput struct {
	Name     string
	Email    string
	Phone    string
	Address  *CustomerAddress
	Metadata map[string]string
}

// CustomerAddress describes a customer's billing address.
type CustomerAddress struct {
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
}

// Customer represents a billing customer.
type Customer struct {
	ID string
}

// SetupIntentInput describes a payment method setup intent request.
type SetupIntentInput struct {
	CustomerID string
	Usage      string
	Metadata   map[string]string
}

// SetupIntent represents a setup intent response.
type SetupIntent struct {
	ID              string
	ClientSecret    string
	CustomerID      string
	PaymentMethodID string
}

// PaymentMethod describes a stored payment method.
type PaymentMethod struct {
	ID       string
	Brand    string
	Last4    string
	ExpMonth int
	ExpYear  int
}

// InvoiceItemInput describes a pending invoice item.
type InvoiceItemInput struct {
	CustomerID     string
	Amount         int64
	Currency       string
	Description    string
	TaxBehavior    string
	Metadata       map[string]string
	IdempotencyKey string
}

// InvoiceItem represents a created invoice item.
type InvoiceItem struct {
	ID        string
	InvoiceID string
}

// InvoiceItemUpdate describes a patch for an invoice item.
type InvoiceItemUpdate struct {
	TaxBehavior string
}

// InvoiceInput describes a draft invoice creation request.
type InvoiceInput struct {
	CustomerID                  string
	AutoAdvance                 bool
	AutomaticTax                bool
	CollectionMethod            string
	DueDays                     *int
	PendingInvoiceItemsBehavior string
	Metadata                    map[string]string
	IdempotencyKey              string
}

// Invoice represents a billing invoice.
type Invoice struct {
	ID               string
	Status           string
	Currency         string
	AmountDue        int64
	AmountPaid       int64
	HostedInvoiceURL string
	CreatedAt        time.Time
	DueDate          *time.Time
	FinalizedAt      *time.Time
	PaidAt           *time.Time
}

// BillingPortalFlowType describes customer portal deep-link flow types.
type BillingPortalFlowType string

const (
	// BillingPortalFlowTypeSubscriptionUpdateConfirm opens a confirmation flow for a specific plan update.
	BillingPortalFlowTypeSubscriptionUpdateConfirm BillingPortalFlowType = "subscription_update_confirm"
)

// BillingPortalFlowAfterCompletionType describes behavior once a deep-link flow completes.
type BillingPortalFlowAfterCompletionType string

const (
	// BillingPortalFlowAfterCompletionTypeRedirect redirects customer to provided URL after completion.
	BillingPortalFlowAfterCompletionTypeRedirect BillingPortalFlowAfterCompletionType = "redirect"
)

// BillingPortalFlowAfterCompletion configures the post-flow behavior.
type BillingPortalFlowAfterCompletion struct {
	Type              BillingPortalFlowAfterCompletionType
	RedirectReturnURL string
}

// BillingPortalFlowSubscriptionUpdateConfirmItem describes one item update inside confirm flow.
type BillingPortalFlowSubscriptionUpdateConfirmItem struct {
	SubscriptionItemID string
	PriceID            string
	Quantity           int64
}

// BillingPortalFlowSubscriptionUpdateConfirm configures a targeted subscription update confirmation.
type BillingPortalFlowSubscriptionUpdateConfirm struct {
	SubscriptionID string
	Items          []BillingPortalFlowSubscriptionUpdateConfirmItem
}

// BillingPortalFlowData configures a deep-link flow in billing portal session creation.
type BillingPortalFlowData struct {
	Type                      BillingPortalFlowType
	AfterCompletion           *BillingPortalFlowAfterCompletion
	SubscriptionUpdateConfirm *BillingPortalFlowSubscriptionUpdateConfirm
}

// BillingPortalSessionInput describes a customer portal session request.
type BillingPortalSessionInput struct {
	CustomerID string
	ReturnURL  string
	Locale     string
	Flow       *BillingPortalFlowData
}

// BillingPortalSession represents a customer portal session.
type BillingPortalSession struct {
	ID  string
	URL string
}

// BillingProvider defines billing-related operations for hosted invoicing.
type BillingProvider interface {
	CreateCustomer(ctx context.Context, in CustomerInput) (Customer, error)
	UpdateCustomer(ctx context.Context, customerID string, in CustomerInput) error
	CreateSetupIntent(ctx context.Context, in SetupIntentInput) (SetupIntent, error)
	SetCustomerDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) error
	RetrievePaymentMethod(ctx context.Context, paymentMethodID string) (PaymentMethod, error)
	CreateInvoiceItem(ctx context.Context, in InvoiceItemInput) (InvoiceItem, error)
	RetrieveInvoiceItem(ctx context.Context, invoiceItemID string) (InvoiceItem, error)
	UpdateInvoiceItem(ctx context.Context, invoiceItemID string, in InvoiceItemUpdate) (InvoiceItem, error)
	CreateInvoice(ctx context.Context, in InvoiceInput) (Invoice, error)
	FinalizeInvoice(ctx context.Context, invoiceID string) (Invoice, error)
	PayInvoice(ctx context.Context, invoiceID string) (Invoice, error)
	RetrieveInvoice(ctx context.Context, invoiceID string) (Invoice, error)
	CreateBillingPortalSession(ctx context.Context, in BillingPortalSessionInput) (BillingPortalSession, error)
}
