// Package billing owns subscriptions, payment methods and the append-only money ledger.
// Its domain stays transport- and persistence-free; provider and database shapes are mapped
// at the package edges.
package billing

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

var (
	ErrEmailVerificationRequired = errors.New("verified email required")
	ErrCustomerKeyMismatch       = errors.New("customer key mismatch")
	ErrSubscriptionNeedsMethod   = errors.New("active renewing subscription needs a payment method")
	ErrTierNotSubscribable       = errors.New("tier is not subscribable")
	ErrSubscriptionExists        = errors.New("active subscription already exists")
	ErrSubscriptionRequired      = errors.New("active subscription required")
	ErrNoChange                  = errors.New("subscription already has the requested tier and term")
	ErrNoScheduledChange         = errors.New("subscription has no scheduled change")
	ErrChangeUnsupported         = errors.New("changing tier and term together is unsupported")
	ErrPaymentMethodRequired     = errors.New("payment method required")
	ErrChargeFailed              = errors.New("charge failed")
	ErrPurchaseTooSmall          = errors.New("purchase must be at least one dollar")
	ErrPurchaseNotFound          = errors.New("purchase not found")
	ErrRefundWindowClosed        = errors.New("refund window closed")
	ErrPurchaseSpent             = errors.New("purchased credits were spent")
	ErrRefundFailed              = errors.New("refund failed")
	// ErrLotTouched is what the credits port reports when a purchased lot is no longer
	// whole. It is billing's own sentinel, translated from whatever the ledger says by the
	// adapter that wires the two (ARCH-7): billing knows a lot can be spent, not how the
	// ledger names that.
	ErrLotTouched     = errors.New("purchased credit lot has already been touched")
	ErrPaymentPending = errors.New("payment outcome is pending")
	ErrStaleQuote     = errors.New("billing quote no longer matches subscription")
	ErrInvalidPack    = errors.New("unknown fixed credit pack")
	// ErrMasterAccount refuses every payment a master account would start (BILL-20): the
	// operator tier is not sold, and a paid tier it bought would demote it.
	ErrMasterAccount = errors.New("a master account starts no payment")
)

type Term string

const (
	TermMonthly Term = "monthly"
	TermAnnual  Term = "annual"
)

func (t Term) Valid() bool { return t == TermMonthly || t == TermAnnual }

type Subscription struct {
	UserID        string
	CoverageID    string
	Tier          plan.Plan
	Term          Term
	AnchorAt      time.Time
	TermStart     time.Time
	TermEnd       time.Time
	NextGrantAt   time.Time
	AutoRenew     bool
	ScheduledTier *plan.Plan
	ScheduledTerm *Term
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Coverage struct {
	ID        string
	Anchor    time.Time
	End       time.Time
	Tier      plan.Plan
	DailyTier plan.Plan
}

type SupportCoverage struct {
	UserID    string
	ID        string
	Tier      plan.Plan
	Anchor    time.Time
	UpdatedAt time.Time
}

type PaymentMethod struct {
	UserID       string
	Provider     string
	BillingKey   string
	CustomerKey  string
	CardLabel    string
	RegisteredAt time.Time
}

type Event struct {
	ID                 int64
	UserID             string
	Kind               string
	Tier               *plan.Plan
	Term               *Term
	Credits            *int
	USDCents           *int
	KRWPerUSDE4        *int64
	RateDate           *string
	KRW                *int
	ProviderPaymentKey *string
	OrderID            *string
	Note               *string
	CreatedAt          time.Time
}

type Purchase struct {
	ID                 string
	PackID             string
	UserID             string
	LotID              string
	Credits            int
	USDCents           int
	KRW                int
	RatePerUSDE4       int64
	RateDate           string
	ProviderPaymentKey string
	OrderID            string
	ChargedAt          time.Time
	RefundedAt         *time.Time
	Refundable         bool
}

// Quote is a fixed KRW amount payable (BILL-2), with the id a change confirms it by.
type Quote struct {
	ID  string
	KRW int
}

type ChangeQuote struct {
	Quote
	AppliedNow  bool
	EffectiveAt time.Time
}

type PurchaseQuote struct {
	Quote
	Credits int
	PackID  string
}

type QuoteRecord struct {
	ID, UserID                                              string
	Tier                                                    plan.Plan
	Term                                                    Term
	KRW                                                     int
	AppliedNow                                              bool
	EffectiveAt, SubscriptionUpdatedAt, QuotedAt, ExpiresAt time.Time
}

type Intent struct {
	OrderID, UserID, Kind, PackID, QuoteID       string
	BillingKey, CustomerKey                      string
	Tier                                         plan.Plan
	Term                                         Term
	KRW                                          int
	QuotedAt, SubscriptionUpdatedAt, EffectiveAt time.Time
	Status, ProviderStatus, PaymentKey           string
	CreatedAt, UpdatedAt                         time.Time
}

type AccountBilling struct {
	Subscription  *Subscription
	PaymentMethod *PaymentMethod
	History       []Event
	Purchases     []Purchase
	CustomerKey   string
}

type PaymentMethodRegistration struct {
	PaymentMethod PaymentMethod
	BonusGranted  bool
}

type BillingKey struct {
	Value       string
	CardLabel   string
	CustomerKey string
}

type ChargeRequest struct {
	BillingKey  string
	CustomerKey string
	OrderID     string
	KRW         int
	Name        string
}

type Payment struct {
	PaymentKey string
	OrderID    string
	Status     string
	AmountKRW  int
	BalanceKRW int
	Currency   string
	Cancels    []PaymentCancel
}

type PaymentCancel struct {
	TransactionKey string
	AmountKRW      int
	Status         string
}

type Notification struct {
	EventType  string
	PaymentKey string
	OrderID    string
	Status     string
	Raw        []byte
}

type ProviderNotification struct {
	Provider   string
	EventType  string
	PaymentKey string
	OrderID    string
	Status     string
	Payload    string
	ReceivedAt time.Time
}

type ProviderError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *ProviderError) Error() string { return e.Code + ": " + e.Message }

// CustomerKey is opaque, stable, and contains no account identifier Toss could expose.
func CustomerKey(userID string) string {
	sum := sha256.Sum256([]byte("postpilot:" + userID))
	// Toss's current JS SDK caps customerKey at 50 characters and requires at least one
	// permitted special character. The prefix guarantees `_`; raw base64url keeps the full
	// 256-bit digest while fitting in 46 characters.
	return "pp_" + base64.RawURLEncoding.EncodeToString(sum[:])
}

// TermEnd is where a paid term that starts at start ends: plan.CoverageEnd, one or twelve
// anchored months on.
func TermEnd(anchor, start time.Time, term Term) time.Time {
	return plan.CoverageEnd(anchor, start, term == TermAnnual)
}
