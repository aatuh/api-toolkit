//lint:file-ignore SA1019 compatibility aliases intentionally re-export the deprecated ports billing surface during v2 migration.

package billing

import "github.com/aatuh/api-toolkit/v2/ports"

type CheckoutSessionRequest = ports.CheckoutSessionRequest                                                 //nolint:staticcheck // Intentional v2 compatibility alias.
type CheckoutSession = ports.CheckoutSession                                                               //nolint:staticcheck // Intentional v2 compatibility alias.
type WebhookEvent = ports.WebhookEvent                                                                     //nolint:staticcheck // Intentional v2 compatibility alias.
type Price = ports.Price                                                                                   //nolint:staticcheck // Intentional v2 compatibility alias.
type PaymentProvider = ports.PaymentProvider                                                               //nolint:staticcheck // Intentional v2 compatibility alias.
type CustomerInput = ports.CustomerInput                                                                   //nolint:staticcheck // Intentional v2 compatibility alias.
type CustomerAddress = ports.CustomerAddress                                                               //nolint:staticcheck // Intentional v2 compatibility alias.
type Customer = ports.Customer                                                                             //nolint:staticcheck // Intentional v2 compatibility alias.
type SetupIntentInput = ports.SetupIntentInput                                                             //nolint:staticcheck // Intentional v2 compatibility alias.
type SetupIntent = ports.SetupIntent                                                                       //nolint:staticcheck // Intentional v2 compatibility alias.
type PaymentMethod = ports.PaymentMethod                                                                   //nolint:staticcheck // Intentional v2 compatibility alias.
type InvoiceItemInput = ports.InvoiceItemInput                                                             //nolint:staticcheck // Intentional v2 compatibility alias.
type InvoiceItem = ports.InvoiceItem                                                                       //nolint:staticcheck // Intentional v2 compatibility alias.
type InvoiceItemUpdate = ports.InvoiceItemUpdate                                                           //nolint:staticcheck // Intentional v2 compatibility alias.
type InvoiceInput = ports.InvoiceInput                                                                     //nolint:staticcheck // Intentional v2 compatibility alias.
type Invoice = ports.Invoice                                                                               //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingPortalFlowType = ports.BillingPortalFlowType                                                   //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingPortalFlowAfterCompletionType = ports.BillingPortalFlowAfterCompletionType                     //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingPortalFlowAfterCompletion = ports.BillingPortalFlowAfterCompletion                             //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingPortalFlowSubscriptionUpdateConfirmItem = ports.BillingPortalFlowSubscriptionUpdateConfirmItem //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingPortalFlowSubscriptionUpdateConfirm = ports.BillingPortalFlowSubscriptionUpdateConfirm         //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingPortalFlowData = ports.BillingPortalFlowData                                                   //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingPortalSessionInput = ports.BillingPortalSessionInput                                           //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingPortalSession = ports.BillingPortalSession                                                     //nolint:staticcheck // Intentional v2 compatibility alias.
type BillingProvider = ports.BillingProvider                                                               //nolint:staticcheck // Intentional v2 compatibility alias.

const (
	BillingPortalFlowTypeSubscriptionUpdateConfirm = ports.BillingPortalFlowTypeSubscriptionUpdateConfirm //nolint:staticcheck // Intentional v2 compatibility alias.
	BillingPortalFlowAfterCompletionTypeRedirect   = ports.BillingPortalFlowAfterCompletionTypeRedirect   //nolint:staticcheck // Intentional v2 compatibility alias.
)
