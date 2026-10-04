package billing

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

type Store interface {
	InWriteTx(ctx context.Context, fn func(Store, Credits, Plans) error) error
	Subscription(ctx context.Context, userID string) (Subscription, bool, error)
	PaymentMethod(ctx context.Context, userID string) (PaymentMethod, bool, error)
	Events(ctx context.Context, userID string, limit int) ([]Event, error)
	Purchases(ctx context.Context, userID string) ([]Purchase, error)
	Purchase(ctx context.Context, userID, purchaseID string) (Purchase, bool, error)
	InsertProviderNotification(ctx context.Context, notification ProviderNotification) error
	UpsertPaymentMethod(ctx context.Context, method PaymentMethod) error
	DeletePaymentMethod(ctx context.Context, userID string) error
	InsertEvent(ctx context.Context, event Event) error
	InsertPurchase(ctx context.Context, purchase Purchase) error
	MarkPurchaseRefunded(ctx context.Context, userID, purchaseID string, at time.Time) (bool, error)
	UpsertSubscription(ctx context.Context, subscription Subscription) error
	// AdvanceNextGrant moves next_grant_at alone, and only while it still holds `from`; false
	// means another writer moved the row since the caller read it.
	AdvanceNextGrant(ctx context.Context, userID string, from, to time.Time) (bool, error)
	DueSubscriptions(ctx context.Context, at time.Time) ([]Subscription, error)
	TierAt(ctx context.Context, userID, coverageID string, at time.Time) (plan.Plan, error)
	InsertTierTransition(ctx context.Context, userID, coverageID string, at time.Time, tier plan.Plan, correlationID string) error
	SupportCoverage(ctx context.Context, userID string) (SupportCoverage, bool, error)
	UpsertSupportCoverage(ctx context.Context, coverage SupportCoverage) error
	DeleteSupportCoverage(ctx context.Context, userID string) error

	// The checkout journal every payment runs through: a change quote, then one order per
	// charge that leaves pending (or review) once.
	PutQuote(context.Context, QuoteRecord) error
	Quote(context.Context, string) (QuoteRecord, bool, error)
	// PurgeExpiredQuotes deletes the quotes that expired before expiredBefore.
	PurgeExpiredQuotes(ctx context.Context, expiredBefore time.Time) (int, error)
	InsertIntent(context.Context, Intent) error
	Intent(context.Context, string) (Intent, bool, error)
	PendingIntent(context.Context, string) (Intent, bool, error)
	// DueIntents lists pending orders created at or before createdBefore.
	DueIntents(ctx context.Context, createdBefore time.Time) ([]Intent, error)
	MarkIntent(context.Context, string, string, string, string, time.Time) (bool, error)
	// ReviewIntents lists up to limit orders in review, oldest first: payments the provider
	// captured that the product could not apply.
	ReviewIntents(ctx context.Context, limit int) ([]Intent, error)
	// FailReviewIntent moves an order from review to failed; false means it had already left.
	FailReviewIntent(ctx context.Context, orderID, providerStatus string, at time.Time) (bool, error)
}

type Provider interface {
	IssueBillingKey(ctx context.Context, authKey, customerKey string) (BillingKey, error)
	Charge(ctx context.Context, request ChargeRequest) (Payment, error)
	PaymentByOrder(ctx context.Context, orderID string) (Payment, bool, error)
	// ParseNotification reads one provider notification out of the POSTed body. The
	// transport is unwrapped by the http adapter (ARCH-7): a domain port takes bytes, not a
	// `*http.Request`.
	ParseNotification(body []byte) (Notification, error)
}

// RefundProvider is the provider's reviewed partial-cancel and reconciliation
// capability. It is optional so older charge adapters remain isolated.
type RefundProvider interface {
	CancelPayment(ctx context.Context, paymentKey string, amountKRW int, reason, idempotencyKey string) (Payment, error)
}

type Credits interface {
	OpenCoverage(ctx context.Context, userID string, coverage Coverage, at time.Time, correlationID string) error
	AddUpgradeBonus(ctx context.Context, userID string, coverage Coverage, at time.Time, credits, exportDelta int, correlationID string) error
	OpenPurchasedLot(ctx context.Context, userID string, credits int) (lotID string, err error)
	// UntouchedLots answers "is this purchase still whole" for a screenful of purchases in
	// one read-pool query. Billing still learns nothing about credit_lots (ARCH-7).
	UntouchedLots(ctx context.Context, lotIDs []string) (map[string]bool, error)
	GrantBonusOnce(ctx context.Context, id, userID string, credits int) (created bool, err error)
}

// Plans is the account tier as billing writes it. AssignTier leaves a master account on
// master (QUOTA-63) and is what every payment-driven write uses; ReassignTier is the support
// path's write, which another master drives and which may move a master.
type Plans interface {
	AssignTier(ctx context.Context, userID string, tier plan.Plan) error
	ReassignTier(ctx context.Context, userID string, tier plan.Plan) error
	TierOf(ctx context.Context, userID string) (plan.Plan, error)
}

type Accounts interface {
	VerifiedEmail(ctx context.Context, userID string) (email string, ok bool, err error)
}

type Mailer interface {
	Send(ctx context.Context, to, subject, text string) error
}
