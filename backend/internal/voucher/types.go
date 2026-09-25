// Package voucher is the operator's vouchers (GIFT): credits that expire a set number of days
// after redemption, sold by bank transfer or given, each delivered as a one-time gift link
// anyone can open and a signed-in account redeems.
package voucher

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/plan"
)

// Product rules, not deployment settings.
const (
	// LinkLifetime is how long an unredeemed gift link stays redeemable (GIFT-7).
	LinkLifetime = 90 * 24 * time.Hour
	// PresetDays is the validity every issue preset carries (GIFT-3).
	PresetDays = 30

	MaxCredits      = 100_000
	MaxDays         = 365
	MaxMessageRunes = 80
	MaxPayerRunes   = 40
	MaxSaleKRW      = 10_000_000
)

var (
	ErrInvalid  = errors.New("voucher: issue is outside the allowed bounds")
	ErrNotFound = errors.New("voucher: not found")
	ErrRedeemed = errors.New("voucher: already redeemed")
	ErrExpired  = errors.New("voucher: link has expired")
	ErrRevoked  = errors.New("voucher: revoked")
)

// State is where a voucher stands at an instant. It is derived, never stored.
type State string

const (
	StateRedeemable State = "redeemable"
	StateRedeemed   State = "redeemed"
	StateExpired    State = "expired"
	StateRevoked    State = "revoked"
)

// Sale is what a sold voucher was paid, as the operator entered it (GIFT-4).
type Sale struct {
	AmountKRW int64
	Payer     string
}

// Voucher is one issued voucher.
type Voucher struct {
	ID string
	// Token is the gift link's secret. It never reaches a log.
	Token         string
	Credits       int
	ValidityDays  int
	Sale          *Sale
	Message       string
	IssuedBy      string
	IssuedAt      time.Time
	LinkExpiresAt time.Time
	RedeemedBy    string
	RedeemedAt    *time.Time
	LotID         string
	RevokedAt     *time.Time
}

// State derives where the voucher stands at `at`: a revocation wins, then a redemption, then
// an unredeemed link past its expiry.
func (v Voucher) State(at time.Time) State {
	switch {
	case v.RevokedAt != nil:
		return StateRevoked
	case v.RedeemedAt != nil:
		return StateRedeemed
	case !at.Before(v.LinkExpiresAt):
		return StateExpired
	default:
		return StateRedeemable
	}
}

// refusal is the error a redemption meets in a state other than redeemable.
func (s State) refusal() error {
	switch s {
	case StateRedeemed:
		return ErrRedeemed
	case StateExpired:
		return ErrExpired
	case StateRevoked:
		return ErrRevoked
	default:
		return nil
	}
}

// Issue is what the operator asks for.
type Issue struct {
	Credits      int
	ValidityDays int
	Sale         *Sale
	Message      string
}

// normalized trims the free text and refuses anything outside the product's bounds.
func (i Issue) normalized() (Issue, error) {
	out := i
	out.Message = strings.TrimSpace(i.Message)
	if out.Credits < 1 || out.Credits > MaxCredits ||
		out.ValidityDays < 1 || out.ValidityDays > MaxDays ||
		utf8.RuneCountInString(out.Message) > MaxMessageRunes {
		return Issue{}, ErrInvalid
	}
	if i.Sale != nil {
		sale := Sale{AmountKRW: i.Sale.AmountKRW, Payer: strings.TrimSpace(i.Sale.Payer)}
		payer := utf8.RuneCountInString(sale.Payer)
		if sale.AmountKRW < 1 || sale.AmountKRW > MaxSaleKRW || payer < 1 || payer > MaxPayerRunes {
			return Issue{}, ErrInvalid
		}
		out.Sale = &sale
	}
	return out, nil
}

// Listed is a voucher as the operator's list shows it.
type Listed struct {
	Voucher
	State State
	// RemainingCredits is what the redeemed lot still holds: zero before redemption, after a
	// revocation, and once the credits have lapsed.
	RemainingCredits int
	// CreditsExpireAt is when the redeemed lot expires or expired; zero before redemption.
	CreditsExpireAt time.Time
}

// PublicView is everything the public gift page may know about a voucher (GIFT-8).
type PublicView struct {
	Credits       int
	ValidityDays  int
	Message       string
	State         State
	LinkExpiresAt time.Time
}

// Redemption is what a redeem handed the account.
type Redemption struct {
	Credits         int
	CreditsExpireAt time.Time
}

// Preset is one issue shortcut: a paid rung's monthly grant for PresetDays (GIFT-3).
type Preset struct {
	Plan    plan.Plan
	Credits int
	Days    int
}

// Presets reads the paid rungs from the code-owned ladder, so the shortcuts move with it.
func Presets() []Preset {
	rungs := []plan.Plan{plan.Basic, plan.Pro, plan.Max}
	presets := make([]Preset, 0, len(rungs))
	for _, rung := range rungs {
		presets = append(presets, Preset{Plan: rung, Credits: plan.MonthlyCredits(rung), Days: PresetDays})
	}
	return presets
}

// LotStanding is where a redeemed voucher's credit lot stands at an instant.
type LotStanding struct {
	Remaining int
	ExpiresAt time.Time
}
