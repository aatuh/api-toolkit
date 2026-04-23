package ports

import (
	"context"
	"encoding/json"
	"time"
)

// CheckoutSessionRequest describes a hosted checkout request.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.CheckoutSessionRequest.
//
// This billing surface is compatibility-sensitive in v2 and currently models a
// Stripe-like hosted checkout flow rather than a provider-neutral abstraction.
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
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.CheckoutSession.
type CheckoutSession struct {
	ID  string
	URL string
}

// WebhookEvent is a generic payment provider webhook payload wrapper.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.WebhookEvent.
type WebhookEvent struct {
	ID        string
	Type      string
	CreatedAt time.Time
	Payload   json.RawMessage
}

// Price represents a payment provider price.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.Price.
type Price struct {
	ID         string
	ProductID  string
	Currency   string
	UnitAmount int64
	Nickname   string
	Metadata   map[string]string
	Active     bool
}

// PaymentProvider defines a hosted checkout + webhook contract.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.PaymentProvider.
//
// This is a compatibility-sensitive v2 surface. It intentionally reflects the
// current hosted-checkout provider model and should not be presented as a
// universal billing abstraction.
type PaymentProvider interface {
	CreateCheckoutSession(ctx context.Context, req CheckoutSessionRequest) (CheckoutSession, error)
	ParseWebhook(ctx context.Context, payload []byte, sigHeader string) (WebhookEvent, error)
	ListPrices(ctx context.Context) ([]Price, error)
}

// CustomerInput describes a new billing customer.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.CustomerInput.
type CustomerInput struct {
	Name     string
	Email    string
	Phone    string
	Address  *CustomerAddress
	Metadata map[string]string
}

// CustomerAddress describes a customer's billing address.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.CustomerAddress.
type CustomerAddress struct {
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
}

// Customer represents a billing customer.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.Customer.
type Customer struct {
	ID string
}

// SetupIntentInput describes a payment method setup intent request.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.SetupIntentInput.
type SetupIntentInput struct {
	CustomerID string
	Usage      string
	Metadata   map[string]string
}

// SetupIntent represents a setup intent response.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.SetupIntent.
type SetupIntent struct {
	ID              string
	ClientSecret    string
	CustomerID      string
	PaymentMethodID string
}

// PaymentMethod describes a stored payment method.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.PaymentMethod.
type PaymentMethod struct {
	ID       string
	Brand    string
	Last4    string
	ExpMonth int
	ExpYear  int
}

// InvoiceItemInput describes a pending invoice item.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.InvoiceItemInput.
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
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.InvoiceItem.
type InvoiceItem struct {
	ID        string
	InvoiceID string
}

// InvoiceItemUpdate describes a patch for an invoice item.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.InvoiceItemUpdate.
type InvoiceItemUpdate struct {
	TaxBehavior string
}

// InvoiceInput describes a draft invoice creation request.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.InvoiceInput.
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
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.Invoice.
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
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalFlowType.
type BillingPortalFlowType string

const (
	// BillingPortalFlowTypeSubscriptionUpdateConfirm opens a confirmation flow for a specific plan update.
	//
	// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalFlowTypeSubscriptionUpdateConfirm.
	BillingPortalFlowTypeSubscriptionUpdateConfirm BillingPortalFlowType = "subscription_update_confirm"
)

// BillingPortalFlowAfterCompletionType describes behavior once a deep-link flow completes.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalFlowAfterCompletionType.
type BillingPortalFlowAfterCompletionType string

const (
	// BillingPortalFlowAfterCompletionTypeRedirect redirects customer to provided URL after completion.
	//
	// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalFlowAfterCompletionTypeRedirect.
	BillingPortalFlowAfterCompletionTypeRedirect BillingPortalFlowAfterCompletionType = "redirect"
)

// BillingPortalFlowAfterCompletion configures the post-flow behavior.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalFlowAfterCompletion.
type BillingPortalFlowAfterCompletion struct {
	Type              BillingPortalFlowAfterCompletionType
	RedirectReturnURL string
}

// BillingPortalFlowSubscriptionUpdateConfirmItem describes one item update inside confirm flow.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalFlowSubscriptionUpdateConfirmItem.
type BillingPortalFlowSubscriptionUpdateConfirmItem struct {
	SubscriptionItemID string
	PriceID            string
	Quantity           int64
}

// BillingPortalFlowSubscriptionUpdateConfirm configures a targeted subscription update confirmation.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalFlowSubscriptionUpdateConfirm.
type BillingPortalFlowSubscriptionUpdateConfirm struct {
	SubscriptionID string
	Items          []BillingPortalFlowSubscriptionUpdateConfirmItem
}

// BillingPortalFlowData configures a deep-link flow in billing portal session creation.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalFlowData.
type BillingPortalFlowData struct {
	Type                      BillingPortalFlowType
	AfterCompletion           *BillingPortalFlowAfterCompletion
	SubscriptionUpdateConfirm *BillingPortalFlowSubscriptionUpdateConfirm
}

// BillingPortalSessionInput describes a customer portal session request.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalSessionInput.
type BillingPortalSessionInput struct {
	CustomerID string
	ReturnURL  string
	Locale     string
	Flow       *BillingPortalFlowData
}

// BillingPortalSession represents a customer portal session.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingPortalSession.
type BillingPortalSession struct {
	ID  string
	URL string
}

// BillingProvider defines billing-related operations for hosted invoicing.
//
// Deprecated: use github.com/aatuh/api-toolkit/v2/compat/billing.BillingProvider.
//
// This is a compatibility-sensitive v2 surface. It is currently shaped around
// provider-specific customer, invoicing, and billing-portal workflows and is a
// candidate for extraction in v3.
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
