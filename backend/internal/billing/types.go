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
	ErrPaymentMethodRequired     = errors.New("payment method required")
	ErrChargeFailed              = errors.New("charge failed")
)

type Term string

const (
	TermMonthly Term = "monthly"
	TermAnnual  Term = "annual"
)

func (t Term) Valid() bool { return t == TermMonthly || t == TermAnnual }

type Subscription struct {
	UserID        string
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
	UserID             string
	LotID              string
	Credits            int
	USDCents           int
	KRW                int
	ProviderPaymentKey string
	OrderID            string
	ChargedAt          time.Time
	RefundedAt         *time.Time
}

type Quote struct {
	USDCents     int
	KRW          int
	RatePerUSDE4 int64
	RateDate     string
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
	Code    string
	Message string
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

// KRWFor converts an all-in USD-cent price through a KRW/USD rate stored at four decimal
// places, rounding half up to one won without introducing floating point.
func KRWFor(usdCents int, rateE4 int64) int {
	if usdCents <= 0 || rateE4 <= 0 {
		return 0
	}
	return int((int64(usdCents)*rateE4 + 500_000) / 1_000_000)
}

func AnnualPriceCents(monthlyCents int) int { return monthlyCents * 10 }

func PriceCents(tier plan.Plan, term Term) int {
	monthly := plan.MonthlyPriceCents(tier)
	if term == TermAnnual {
		return AnnualPriceCents(monthly)
	}
	return monthly
}

// TermEnd advances by one or twelve anchor windows. Asking AnchorWindow at an exclusive
// boundary advances to the next clamped month and naturally returns to the original day.
func TermEnd(anchor, start time.Time, term Term) time.Time {
	windows := 1
	if term == TermAnnual {
		windows = 12
	}
	current := start
	for range windows {
		_, current = plan.AnchorWindow(anchor, current)
	}
	return current
}
