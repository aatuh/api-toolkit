package stripe

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/ports"
	stripe "github.com/stripe/stripe-go/v79"
)

// CreateCustomer creates a Stripe customer.
func (p *Provider) CreateCustomer(ctx context.Context, in ports.CustomerInput) (ports.Customer, error) {
	params := &stripe.CustomerParams{}
	params.Context = ctx
	if strings.TrimSpace(in.Name) != "" {
		params.Name = stripe.String(strings.TrimSpace(in.Name))
	}
	if strings.TrimSpace(in.Email) != "" {
		params.Email = stripe.String(strings.TrimSpace(in.Email))
	}
	if strings.TrimSpace(in.Phone) != "" {
		params.Phone = stripe.String(strings.TrimSpace(in.Phone))
	}
	if in.Address != nil {
		addr := &stripe.AddressParams{}
		if strings.TrimSpace(in.Address.Line1) != "" {
			addr.Line1 = stripe.String(strings.TrimSpace(in.Address.Line1))
		}
		if strings.TrimSpace(in.Address.Line2) != "" {
			addr.Line2 = stripe.String(strings.TrimSpace(in.Address.Line2))
		}
		if strings.TrimSpace(in.Address.City) != "" {
			addr.City = stripe.String(strings.TrimSpace(in.Address.City))
		}
		if strings.TrimSpace(in.Address.State) != "" {
			addr.State = stripe.String(strings.TrimSpace(in.Address.State))
		}
		if strings.TrimSpace(in.Address.PostalCode) != "" {
			addr.PostalCode = stripe.String(strings.TrimSpace(in.Address.PostalCode))
		}
		if strings.TrimSpace(in.Address.Country) != "" {
			addr.Country = stripe.String(strings.TrimSpace(in.Address.Country))
		}
		params.Address = addr
	}
	if len(in.Metadata) > 0 {
		params.Metadata = in.Metadata
	}
	cust, err := p.client.Customers.New(params)
	if err != nil {
		return ports.Customer{}, normalizeStripeError(err)
	}
	return ports.Customer{ID: cust.ID}, nil
}

// UpdateCustomer updates an existing Stripe customer.
func (p *Provider) UpdateCustomer(ctx context.Context, customerID string, in ports.CustomerInput) error {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return errors.New("customer id required")
	}
	params := &stripe.CustomerParams{}
	params.Context = ctx
	if strings.TrimSpace(in.Name) != "" {
		params.Name = stripe.String(strings.TrimSpace(in.Name))
	}
	if strings.TrimSpace(in.Email) != "" {
		params.Email = stripe.String(strings.TrimSpace(in.Email))
	}
	if strings.TrimSpace(in.Phone) != "" {
		params.Phone = stripe.String(strings.TrimSpace(in.Phone))
	}
	if in.Address != nil {
		addr := &stripe.AddressParams{}
		hasAddress := false
		if strings.TrimSpace(in.Address.Line1) != "" {
			addr.Line1 = stripe.String(strings.TrimSpace(in.Address.Line1))
			hasAddress = true
		}
		if strings.TrimSpace(in.Address.Line2) != "" {
			addr.Line2 = stripe.String(strings.TrimSpace(in.Address.Line2))
			hasAddress = true
		}
		if strings.TrimSpace(in.Address.City) != "" {
			addr.City = stripe.String(strings.TrimSpace(in.Address.City))
			hasAddress = true
		}
		if strings.TrimSpace(in.Address.State) != "" {
			addr.State = stripe.String(strings.TrimSpace(in.Address.State))
			hasAddress = true
		}
		if strings.TrimSpace(in.Address.PostalCode) != "" {
			addr.PostalCode = stripe.String(strings.TrimSpace(in.Address.PostalCode))
			hasAddress = true
		}
		if strings.TrimSpace(in.Address.Country) != "" {
			addr.Country = stripe.String(strings.TrimSpace(in.Address.Country))
			hasAddress = true
		}
		if hasAddress {
			params.Address = addr
		}
	}
	if len(in.Metadata) > 0 {
		params.Metadata = in.Metadata
	}
	_, err := p.client.Customers.Update(customerID, params)
	return normalizeStripeError(err)
}

// CreateSetupIntent creates a Stripe setup intent for saving payment methods.
func (p *Provider) CreateSetupIntent(ctx context.Context, in ports.SetupIntentInput) (ports.SetupIntent, error) {
	if strings.TrimSpace(in.CustomerID) == "" {
		return ports.SetupIntent{}, errors.New("customer id required")
	}
	params := &stripe.SetupIntentParams{}
	params.Context = ctx
	params.Customer = stripe.String(strings.TrimSpace(in.CustomerID))
	params.Usage = stripe.String(defaultString(in.Usage, string(stripe.SetupIntentUsageOffSession)))
	params.PaymentMethodTypes = []*string{stripe.String("card")}
	if len(in.Metadata) > 0 {
		params.Metadata = in.Metadata
	}
	intent, err := p.client.SetupIntents.New(params)
	if err != nil {
		return ports.SetupIntent{}, normalizeStripeError(err)
	}
	out := ports.SetupIntent{
		ID:           intent.ID,
		ClientSecret: intent.ClientSecret,
		CustomerID:   in.CustomerID,
	}
	if intent.Customer != nil && strings.TrimSpace(intent.Customer.ID) != "" {
		out.CustomerID = intent.Customer.ID
	}
	if intent.PaymentMethod != nil && strings.TrimSpace(intent.PaymentMethod.ID) != "" {
		out.PaymentMethodID = intent.PaymentMethod.ID
	}
	return out, nil
}

// SetCustomerDefaultPaymentMethod updates the customer's default invoice payment method.
func (p *Provider) SetCustomerDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	customerID = strings.TrimSpace(customerID)
	paymentMethodID = strings.TrimSpace(paymentMethodID)
	if customerID == "" || paymentMethodID == "" {
		return errors.New("customer id and payment method required")
	}
	params := &stripe.CustomerParams{}
	params.Context = ctx
	params.InvoiceSettings = &stripe.CustomerInvoiceSettingsParams{
		DefaultPaymentMethod: stripe.String(paymentMethodID),
	}
	_, err := p.client.Customers.Update(customerID, params)
	return err
}

// RetrievePaymentMethod fetches a stored payment method details.
func (p *Provider) RetrievePaymentMethod(ctx context.Context, paymentMethodID string) (ports.PaymentMethod, error) {
	paymentMethodID = strings.TrimSpace(paymentMethodID)
	if paymentMethodID == "" {
		return ports.PaymentMethod{}, errors.New("payment method id required")
	}
	params := &stripe.PaymentMethodParams{}
	params.Context = ctx
	pm, err := p.client.PaymentMethods.Get(paymentMethodID, params)
	if err != nil {
		return ports.PaymentMethod{}, err
	}
	out := ports.PaymentMethod{ID: pm.ID}
	if pm.Card != nil {
		out.Brand = string(pm.Card.Brand)
		out.Last4 = pm.Card.Last4
		out.ExpMonth = int(pm.Card.ExpMonth)
		out.ExpYear = int(pm.Card.ExpYear)
	}
	return out, nil
}

// CreateInvoiceItem creates a pending invoice item.
func (p *Provider) CreateInvoiceItem(ctx context.Context, in ports.InvoiceItemInput) (ports.InvoiceItem, error) {
	if strings.TrimSpace(in.CustomerID) == "" {
		return ports.InvoiceItem{}, errors.New("customer id required")
	}
	params := &stripe.InvoiceItemParams{}
	params.Context = ctx
	params.Customer = stripe.String(strings.TrimSpace(in.CustomerID))
	params.Currency = stripe.String(strings.ToLower(strings.TrimSpace(in.Currency)))
	params.Amount = stripe.Int64(in.Amount)
	if strings.TrimSpace(in.Description) != "" {
		params.Description = stripe.String(strings.TrimSpace(in.Description))
	}
	if strings.TrimSpace(in.TaxBehavior) != "" {
		params.TaxBehavior = stripe.String(strings.TrimSpace(in.TaxBehavior))
	}
	if len(in.Metadata) > 0 {
		params.Metadata = in.Metadata
	}
	if strings.TrimSpace(in.IdempotencyKey) != "" {
		params.SetIdempotencyKey(strings.TrimSpace(in.IdempotencyKey))
	}
	item, err := p.client.InvoiceItems.New(params)
	if err != nil {
		return ports.InvoiceItem{}, normalizeStripeError(err)
	}
	out := ports.InvoiceItem{ID: item.ID}
	if item.Invoice != nil && strings.TrimSpace(item.Invoice.ID) != "" {
		out.InvoiceID = item.Invoice.ID
	}
	return out, nil
}

// RetrieveInvoiceItem fetches an invoice item to see if it's already attached to an invoice.
func (p *Provider) RetrieveInvoiceItem(ctx context.Context, invoiceItemID string) (ports.InvoiceItem, error) {
	invoiceItemID = strings.TrimSpace(invoiceItemID)
	if invoiceItemID == "" {
		return ports.InvoiceItem{}, errors.New("invoice item id required")
	}
	params := &stripe.InvoiceItemParams{}
	params.Context = ctx
	item, err := p.client.InvoiceItems.Get(invoiceItemID, params)
	if err != nil {
		return ports.InvoiceItem{}, normalizeStripeError(err)
	}
	out := ports.InvoiceItem{ID: item.ID}
	if item.Invoice != nil && strings.TrimSpace(item.Invoice.ID) != "" {
		out.InvoiceID = item.Invoice.ID
	}
	return out, nil
}

// UpdateInvoiceItem updates fields on a draft invoice item.
func (p *Provider) UpdateInvoiceItem(ctx context.Context, invoiceItemID string, in ports.InvoiceItemUpdate) (ports.InvoiceItem, error) {
	invoiceItemID = strings.TrimSpace(invoiceItemID)
	if invoiceItemID == "" {
		return ports.InvoiceItem{}, errors.New("invoice item id required")
	}
	params := &stripe.InvoiceItemParams{}
	params.Context = ctx
	if strings.TrimSpace(in.TaxBehavior) != "" {
		params.TaxBehavior = stripe.String(strings.TrimSpace(in.TaxBehavior))
	}
	item, err := p.client.InvoiceItems.Update(invoiceItemID, params)
	if err != nil {
		return ports.InvoiceItem{}, normalizeStripeError(err)
	}
	out := ports.InvoiceItem{ID: item.ID}
	if item.Invoice != nil && strings.TrimSpace(item.Invoice.ID) != "" {
		out.InvoiceID = item.Invoice.ID
	}
	return out, nil
}

// CreateInvoice creates a draft invoice for the customer.
func (p *Provider) CreateInvoice(ctx context.Context, in ports.InvoiceInput) (ports.Invoice, error) {
	if strings.TrimSpace(in.CustomerID) == "" {
		return ports.Invoice{}, errors.New("customer id required")
	}
	params := &stripe.InvoiceParams{}
	params.Context = ctx
	params.Customer = stripe.String(strings.TrimSpace(in.CustomerID))
	params.AutoAdvance = stripe.Bool(in.AutoAdvance)
	if in.AutomaticTax {
		params.AutomaticTax = &stripe.InvoiceAutomaticTaxParams{Enabled: stripe.Bool(true)}
	}
	if strings.TrimSpace(in.CollectionMethod) != "" {
		params.CollectionMethod = stripe.String(strings.TrimSpace(in.CollectionMethod))
	}
	if strings.TrimSpace(in.PendingInvoiceItemsBehavior) != "" {
		params.PendingInvoiceItemsBehavior = stripe.String(strings.TrimSpace(in.PendingInvoiceItemsBehavior))
	}
	if in.DueDays != nil && *in.DueDays > 0 {
		params.DaysUntilDue = stripe.Int64(int64(*in.DueDays))
	}
	if len(in.Metadata) > 0 {
		params.Metadata = in.Metadata
	}
	if strings.TrimSpace(in.IdempotencyKey) != "" {
		params.SetIdempotencyKey(strings.TrimSpace(in.IdempotencyKey))
	}
	inv, err := p.client.Invoices.New(params)
	if err != nil {
		return ports.Invoice{}, normalizeStripeError(err)
	}
	return invoiceFromStripe(inv), nil
}

// FinalizeInvoice finalizes a draft invoice.
func (p *Provider) FinalizeInvoice(ctx context.Context, invoiceID string) (ports.Invoice, error) {
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return ports.Invoice{}, errors.New("invoice id required")
	}
	params := &stripe.InvoiceFinalizeInvoiceParams{}
	params.Context = ctx
	inv, err := p.client.Invoices.FinalizeInvoice(invoiceID, params)
	if err != nil {
		return ports.Invoice{}, normalizeStripeError(err)
	}
	return invoiceFromStripe(inv), nil
}

// PayInvoice attempts to pay an open invoice immediately using the customer's default payment method.
func (p *Provider) PayInvoice(ctx context.Context, invoiceID string) (ports.Invoice, error) {
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return ports.Invoice{}, errors.New("invoice id required")
	}
	params := &stripe.InvoicePayParams{}
	params.Context = ctx
	inv, err := p.client.Invoices.Pay(invoiceID, params)
	if err != nil {
		return ports.Invoice{}, normalizeStripeError(err)
	}
	return invoiceFromStripe(inv), nil
}

// RetrieveInvoice fetches an invoice by ID.
func (p *Provider) RetrieveInvoice(ctx context.Context, invoiceID string) (ports.Invoice, error) {
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return ports.Invoice{}, errors.New("invoice id required")
	}
	params := &stripe.InvoiceParams{}
	params.Context = ctx
	inv, err := p.client.Invoices.Get(invoiceID, params)
	if err != nil {
		return ports.Invoice{}, normalizeStripeError(err)
	}
	return invoiceFromStripe(inv), nil
}

// CreateBillingPortalSession creates a customer portal session.
func (p *Provider) CreateBillingPortalSession(ctx context.Context, in ports.BillingPortalSessionInput) (ports.BillingPortalSession, error) {
	customerID := strings.TrimSpace(in.CustomerID)
	if customerID == "" {
		return ports.BillingPortalSession{}, errors.New("customer id required")
	}
	params := &stripe.BillingPortalSessionParams{}
	params.Context = ctx
	params.Customer = stripe.String(customerID)
	if strings.TrimSpace(in.ReturnURL) != "" {
		params.ReturnURL = stripe.String(strings.TrimSpace(in.ReturnURL))
	}
	if strings.TrimSpace(in.Locale) != "" {
		params.Locale = stripe.String(strings.TrimSpace(in.Locale))
	}
	session, err := p.client.BillingPortalSessions.New(params)
	if err != nil {
		return ports.BillingPortalSession{}, normalizeStripeError(err)
	}
	return ports.BillingPortalSession{ID: session.ID, URL: session.URL}, nil
}

func normalizeStripeError(err error) error {
	if err == nil {
		return nil
	}
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) && stripeErr.Code == stripe.ErrorCodeResourceMissing {
		return errors.Join(ports.ErrResourceMissing, err)
	}
	return err
}

func invoiceFromStripe(inv *stripe.Invoice) ports.Invoice {
	if inv == nil {
		return ports.Invoice{}
	}
	amountDue := inv.AmountDue
	if amountDue == 0 && inv.Total > 0 {
		amountDue = inv.Total
	}
	out := ports.Invoice{
		ID:               inv.ID,
		Status:           string(inv.Status),
		Currency:         strings.ToUpper(string(inv.Currency)),
		AmountDue:        amountDue,
		AmountPaid:       inv.AmountPaid,
		HostedInvoiceURL: inv.HostedInvoiceURL,
		CreatedAt:        time.Unix(inv.Created, 0),
	}
	if inv.DueDate > 0 {
		val := time.Unix(inv.DueDate, 0)
		out.DueDate = &val
	}
	if inv.StatusTransitions != nil {
		if inv.StatusTransitions.FinalizedAt > 0 {
			val := time.Unix(inv.StatusTransitions.FinalizedAt, 0)
			out.FinalizedAt = &val
		}
		if inv.StatusTransitions.PaidAt > 0 {
			val := time.Unix(inv.StatusTransitions.PaidAt, 0)
			out.PaidAt = &val
		}
	}
	return out
}
