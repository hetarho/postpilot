package usage

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

const UnitQuoteTTL = 10 * time.Minute
const UnitCancellationPolicyVersion = 1
const UnitCallsMax = 100

var (
	ErrUnitApproval = errors.New("bounded unit approval required or changed")
	ErrUnitPricing  = errors.New("documented bounded unit pricing unavailable")
	ErrUnitCall     = errors.New("unit call is outside its open admission")
	unitDecimal     = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,9})?$`)
	unitDigest      = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// UnitTariff carries independent billing evidence. The conversion bounds supplier
// units per input character; it is not a claim that the supplier consumed them.
// Seconds cannot be priced until a provider offers an enforced output ceiling.
type UnitTariff struct {
	Unit                                                         llm.SpeechUnit
	USDPerUnit, Multiplier, MaximumUnits, UnitsPerInputCharacter string
}

// UnitBudget keeps product identities opaque. Its owning context resolves the
// policy/revision and validates ownership before asking for a quote. Count is
// explicit work, never permission to retry a failed request automatically.
type UnitBudget struct {
	PolicyID               string
	Revision               int64
	AuthorizationID        string
	ScopeDigest            string
	Ref                    llm.ModelRef
	Operation              string
	InputDigest            string
	Count, InputCharacters int
	AuxiliaryCharacters    int
	ParametersDigest       string
	// BoundedInput permits a not-yet-written corpus within one frozen identity and total bound.
	BoundedInput         bool
	TotalInputCharacters int
	InputIdentityDigest  string
	Tariffs              []UnitTariff
	Source, BoundsSource string
	CheckedAt            time.Time
	Complete             bool
}

type UnitQuote struct {
	ID, UserID, Kind, Digest string
	Calls                    []UnitBudget
	Rate                     plan.RateSnapshot
	MaxCredits               int
	ExpiresAt                time.Time
	ConsumedJobID            string
}

type UnitApproval struct {
	QuoteID                   string
	ApprovedMaxCredits        *int
	CancellationPolicyVersion int
}

// UnitEvent is appended alongside one zero-token usage event. ExactUSD is a
// rational decimal sum converted only at terminal settlement; CostMicrousd is
// retained as a compatibility projection for historical operator diagnostics.
type UnitEvent struct {
	ClaimID, SupplierRequestID, BudgetFingerprint string
	Evidence                                      []llm.SpeechUnitEvidence
	ReportedUSD, ExactUSD                         string
	CostSource                                    llm.CostSource
}

type UnitClaim struct {
	ID     string
	Budget UnitBudget
	Work   Work
}

type UnitBudgetChecker interface {
	ValidateUnitBudget(context.Context, string, plan.Plan, UnitBudget) error
}

// UnitLedger is opt-in so historical completion stores remain unchanged. Every
// write is scoped by the existing ledger transaction. No provider call runs in it.
type UnitLedger interface {
	SaveUnitQuote(context.Context, UnitQuote, time.Time) error
	GetUnitQuote(context.Context, string, string) (UnitQuote, error)
	ConsumeUnitQuote(context.Context, string, string) (bool, error)
	SaveUnitAdmission(context.Context, string, string, []UnitBudget) error
	UnitAdmission(context.Context, string) (string, []UnitBudget, error)
	ClaimUnitCall(context.Context, string, string, string, int, time.Time) (bool, error)
	InsertUnitEvent(context.Context, Event, UnitEvent) error
	UnitCostForJob(context.Context, string) (string, error)
}

func decimalQuantity(s string) (*big.Rat, bool) {
	if !unitDecimal.MatchString(s) {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(s)
	return r, ok && r.Sign() >= 0
}

func evidenceURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && len(s) <= 2000 && u.Scheme == "https" && u.Host != "" && u.User == nil
}

func (b UnitBudget) MaximumUSD() (*big.Rat, error) {
	if b.PolicyID == "" || len(b.PolicyID) > 200 || b.Revision <= 0 || !unitDigest.MatchString(b.ScopeDigest) || !unitDigest.MatchString(b.InputDigest) ||
		b.Ref.ProviderID == "" || b.Ref.ModelID == "" || b.Count < 1 || b.Count > UnitCallsMax || b.InputCharacters < 0 || b.InputCharacters > llm.SpeechMaxText ||
		b.AuxiliaryCharacters < 0 || b.AuxiliaryCharacters > llm.SpeechDescriptionMax || !b.Complete || !evidenceURL(b.Source) || !evidenceURL(b.BoundsSource) || b.CheckedAt.IsZero() || len(b.Tariffs) == 0 || len(b.Tariffs) > 8 {
		return nil, ErrUnitPricing
	}
	switch b.Operation {
	case "voice_design":
		if b.InputCharacters < llm.SpeechPreviewMin {
			return nil, ErrUnitPricing
		}
	case "speech":
		if b.InputCharacters < 1 {
			return nil, ErrUnitPricing
		}
	case "voice_confirm":
		if b.InputCharacters != 0 {
			return nil, ErrUnitPricing
		}
	default:
		return nil, ErrUnitPricing
	}
	if b.BoundedInput && (b.Operation != "speech" || !unitDigest.MatchString(b.InputIdentityDigest) || b.TotalInputCharacters < 1 || b.TotalInputCharacters > b.Count*b.InputCharacters) {
		return nil, ErrUnitPricing
	}
	if !b.BoundedInput && (b.TotalInputCharacters != 0 || b.InputIdentityDigest != "") {
		return nil, ErrUnitPricing
	}
	total := new(big.Rat)
	seen := map[llm.SpeechUnit]bool{}
	variable := 0
	for _, t := range b.Tariffs {
		price, ok := decimalQuantity(t.USDPerUnit)
		if !ok {
			return nil, ErrUnitPricing
		}
		multiplier, ok := decimalQuantity(t.Multiplier)
		if !ok || multiplier.Sign() <= 0 {
			return nil, ErrUnitPricing
		}
		maximum, ok := decimalQuantity(t.MaximumUnits)
		if !ok || maximum.Sign() <= 0 || seen[t.Unit] {
			return nil, ErrUnitPricing
		}
		seen[t.Unit] = true
		units := new(big.Rat)
		switch t.Unit {
		case llm.SpeechUnitRequests:
			units.SetInt64(1)
		case llm.SpeechUnitCharacters, llm.SpeechUnitCredits, llm.SpeechUnitCharacterCost:
			if price.Sign() > 0 {
				conversion, valid := decimalQuantity(t.UnitsPerInputCharacter)
				if !valid || conversion.Sign() <= 0 || b.InputCharacters == 0 {
					return nil, ErrUnitPricing
				}
				units.Mul(conversion, new(big.Rat).SetInt64(int64(b.InputCharacters)))
				variable++
			}
		case llm.SpeechUnitSeconds:
			if price.Sign() > 0 {
				return nil, ErrUnitPricing
			} // decoding a long response cannot bound its supplier bill
		default:
			return nil, ErrUnitPricing
		}
		if units.Cmp(maximum) > 0 || variable > 1 {
			return nil, ErrUnitPricing
		}
		cost := new(big.Rat).Mul(new(big.Rat).Mul(price, multiplier), units)
		if b.BoundedInput && t.UnitsPerInputCharacter != "" && t.Unit != llm.SpeechUnitRequests {
			cost.Mul(cost, new(big.Rat).SetFrac64(int64(b.TotalInputCharacters), int64(b.InputCharacters)))
		} else {
			cost.Mul(cost, new(big.Rat).SetInt64(int64(b.Count)))
		}
		total.Add(total, cost)
	}
	return total, nil
}

func (b UnitBudget) Fingerprint() string {
	parts := []string{"unit-budget-v1", b.PolicyID, strconv.FormatInt(b.Revision, 10), b.AuthorizationID, b.ScopeDigest, b.Ref.String(), b.Operation, b.InputDigest,
		strconv.Itoa(b.Count), strconv.Itoa(b.InputCharacters), strconv.Itoa(b.AuxiliaryCharacters), b.ParametersDigest, b.Source, b.BoundsSource, b.CheckedAt.UTC().Format(time.RFC3339Nano), strconv.FormatBool(b.Complete)}
	if b.BoundedInput {
		parts = append(parts, "bounded-corpus-v1", strconv.Itoa(b.TotalInputCharacters), b.InputIdentityDigest)
	}
	for _, t := range b.Tariffs {
		parts = append(parts, string(t.Unit), t.USDPerUnit, t.Multiplier, t.MaximumUnits, t.UnitsPerInputCharacter)
	}
	return UnitDigest(parts...)
}

// UnitDigest is a length-delimited SHA-256 for caller-owned canonical identities.
func UnitDigest(parts ...string) string {
	h := sha256.New()
	var n [8]byte
	for _, p := range parts {
		binary.BigEndian.PutUint64(n[:], uint64(len(p)))
		h.Write(n[:])
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func unitBudgetTotal(calls []UnitBudget) (*big.Rat, error) {
	if len(calls) == 0 || len(calls) > UnitCallsMax {
		return nil, ErrUnitPricing
	}
	total := new(big.Rat)
	count := 0
	seen := map[string]bool{}
	for _, b := range calls {
		cost, err := b.MaximumUSD()
		if err != nil {
			return nil, err
		}
		fp := b.Fingerprint()
		if seen[fp] {
			return nil, ErrUnitPricing
		}
		seen[fp] = true
		count += b.Count
		if count > UnitCallsMax {
			return nil, ErrUnitPricing
		}
		total.Add(total, cost)
	}
	return total, nil
}

func unitCallsDigest(calls []UnitBudget) string {
	parts := []string{"unit-calls-v1"}
	for _, b := range calls {
		parts = append(parts, b.Fingerprint())
	}
	return UnitDigest(parts...)
}

func exactCredits(usd *big.Rat, rate plan.RateSnapshot) (int, error) {
	if usd.Sign() == 0 {
		return 0, nil
	}
	if usd.Sign() < 0 || !rate.Valid() {
		return 0, ErrUnitPricing
	}
	c := new(big.Rat).Mul(usd, new(big.Rat).SetFrac(big.NewInt(rate.AppliedE4), big.NewInt(10_000)))
	n := new(big.Int).Quo(c.Num(), c.Denom())
	if new(big.Int).Mod(c.Num(), c.Denom()).Sign() != 0 {
		n.Add(n, big.NewInt(1))
	}
	if !n.IsInt64() || n.Int64() > math.MaxInt32 {
		return 0, ErrUnitPricing
	}
	return int(n.Int64()), nil
}

// UnitCost uses only supplied evidence. Missing any nonzero applicable tariff
// leaves the whole call unavailable, rather than partially fabricating a bill.
func UnitCost(b UnitBudget, e llm.SpeechEvidence) (string, llm.CostSource) {
	if e.ReportedUSD != "" {
		if cost, ok := decimalQuantity(e.ReportedUSD); ok {
			return cost.RatString(), llm.CostReported
		}
		return "", llm.CostUnavailable
	}
	quantities := map[llm.SpeechUnit]*big.Rat{}
	for _, u := range e.Units {
		n, ok := decimalQuantity(u.Quantity)
		if !ok || quantities[u.Unit] != nil {
			return "", llm.CostUnavailable
		}
		quantities[u.Unit] = n
	}
	total := new(big.Rat)
	for _, t := range b.Tariffs {
		price, ok := decimalQuantity(t.USDPerUnit)
		if !ok {
			return "", llm.CostUnavailable
		}
		if price.Sign() == 0 {
			continue
		}
		q := quantities[t.Unit]
		if q == nil {
			return "", llm.CostUnavailable
		}
		m, ok := decimalQuantity(t.Multiplier)
		if !ok {
			return "", llm.CostUnavailable
		}
		total.Add(total, new(big.Rat).Mul(new(big.Rat).Mul(price, m), q))
	}
	return total.RatString(), llm.CostEstimated
}

func unitStore(tx Storage) (UnitLedger, error) {
	u, ok := tx.(UnitLedger)
	if !ok {
		return nil, fmt.Errorf("unit ledger not wired")
	}
	return u, nil
}
