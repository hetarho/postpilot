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
	DueSubscriptions(ctx context.Context, at time.Time) ([]Subscription, error)
	TierAt(ctx context.Context, userID, coverageID string, at time.Time) (plan.Plan, error)
	InsertTierTransition(ctx context.Context, userID, coverageID string, at time.Time, tier plan.Plan, correlationID string) error
	SupportCoverage(ctx context.Context, userID string) (SupportCoverage, bool, error)
	UpsertSupportCoverage(ctx context.Context, coverage SupportCoverage) error
	DeleteSupportCoverage(ctx context.Context, userID string) error
}

// IntentStore is the fixed-KRW checkout journal. The legacy Store remains
// compatible with historical tests while production installs this extension.
type IntentStore interface {
	PutQuote(context.Context, QuoteRecord) error
	Quote(context.Context, string) (QuoteRecord, bool, error)
	InsertIntent(context.Context, Intent) error
	Intent(context.Context, string) (Intent, bool, error)
	PendingIntent(context.Context, string) (Intent, bool, error)
	DueIntents(context.Context) ([]Intent, error)
	MarkIntent(context.Context, string, string, string, string, time.Time) (bool, error)
}

type Provider interface {
	IssueBillingKey(ctx context.Context, authKey, customerKey string) (BillingKey, error)
	Charge(ctx context.Context, request ChargeRequest) (Payment, error)
	PaymentByOrder(ctx context.Context, orderID string) (Payment, bool, error)
	Refund(ctx context.Context, paymentKey, reason string) error
	// ParseNotification reads one provider notification out of the POSTed body. The
	// transport is unwrapped by the http adapter (ARCH-7): a domain port takes bytes, not a
	// `*http.Request`.
	ParseNotification(body []byte) (Notification, error)
}

type Rates interface {
	KRWPerUSD(ctx context.Context, date time.Time) (rateE4 int64, published bool, err error)
}

type Credits interface {
	OpenCoverage(ctx context.Context, userID string, coverage Coverage, at time.Time, correlationID string) error
	AddUpgradeBonus(ctx context.Context, userID string, coverage Coverage, at time.Time, credits, exportDelta int, correlationID string) error
	// StartMonthlyWindow opens the window a first subscription charge paid for: the running
	// window closes with no carry-over and the tier's whole grant opens (QUOTA-42).
	StartMonthlyWindow(ctx context.Context, userID string, tier plan.Plan, start, end time.Time) error
	// OpenMonthlyLot opens a renewal's window, which is absent-only: a renewal keeps the
	// anchor it already has, so re-running one must not rewrite a window already granted.
	OpenMonthlyLot(ctx context.Context, userID string, tier plan.Plan, start, end time.Time) error
	RaiseMonthlyLot(ctx context.Context, userID string, credits int) error
	OpenPurchasedLot(ctx context.Context, userID string, credits int) (lotID string, err error)
	// VoidUntouchedLot reports ErrLotTouched when the lot is no longer whole.
	VoidUntouchedLot(ctx context.Context, lotID string) error
	// UntouchedLots answers "is this purchase still whole" for a screenful of purchases in
	// one read-pool query. Billing consumes only the plural read — the single writer-bound
	// one is for an answer about to decide a write, which billing reaches through
	// VoidUntouchedLot instead. Billing still learns nothing about credit_lots (ARCH-7).
	UntouchedLots(ctx context.Context, lotIDs []string) (map[string]bool, error)
	RestoreLot(ctx context.Context, lotID string, credits int) error
	GrantBonusOnce(ctx context.Context, id, userID string, credits int) (created bool, err error)
}

type Plans interface {
	AssignTier(ctx context.Context, userID string, tier plan.Plan) error
	TierOf(ctx context.Context, userID string) (plan.Plan, error)
}

type Accounts interface {
	VerifiedEmail(ctx context.Context, userID string) (email string, ok bool, err error)
}

type Mailer interface {
	Send(ctx context.Context, to, subject, text string) error
}
