package billing

import (
	"context"
	"net/http"
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
}

type Provider interface {
	IssueBillingKey(ctx context.Context, authKey, customerKey string) (BillingKey, error)
	Charge(ctx context.Context, request ChargeRequest) (Payment, error)
	PaymentByOrder(ctx context.Context, orderID string) (Payment, bool, error)
	Refund(ctx context.Context, paymentKey, reason string) error
	ParseNotification(r *http.Request) (Notification, error)
}

type Rates interface {
	KRWPerUSD(ctx context.Context, date time.Time) (rateE4 int64, published bool, err error)
}

type Credits interface {
	// StartMonthlyWindow opens the window a first subscription charge paid for: the running
	// window closes with no carry-over and the tier's whole grant opens (QUOTA-42).
	StartMonthlyWindow(ctx context.Context, userID string, tier plan.Plan, start, end time.Time) error
	// OpenMonthlyLot opens a renewal's window, which is absent-only: a renewal keeps the
	// anchor it already has, so re-running one must not rewrite a window already granted.
	OpenMonthlyLot(ctx context.Context, userID string, tier plan.Plan, start, end time.Time) error
	RaiseMonthlyLot(ctx context.Context, userID string, credits int) error
	OpenPurchasedLot(ctx context.Context, userID string, credits int) (lotID string, err error)
	VoidUntouchedLot(ctx context.Context, lotID string) error
	LotUntouched(ctx context.Context, lotID string) (bool, error)
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
