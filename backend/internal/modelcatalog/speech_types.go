package modelcatalog

import (
	"context"
	"errors"
	"math/big"
	"net/url"
	"regexp"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

const (
	SpeechProfileLabelMax    = 100
	SpeechPriceComponentsMax = 8
	SpeechQualificationTTL   = 24 * time.Hour
)

var (
	ErrSpeechProfileInvalid       = errors.New("invalid speech profile")
	ErrSpeechProfileConflict      = errors.New("speech profile revision changed")
	ErrSpeechProfileUnavailable   = errors.New("speech profile unavailable")
	ErrSpeechQualificationInvalid = errors.New("invalid speech qualification session")
	decimalUSD                    = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,9})?$`)
)

type SpeechOperation string

const (
	SpeechDesign     SpeechOperation = "voice_design"
	SpeechConfirm    SpeechOperation = "voice_confirm"
	SpeechSynthesize SpeechOperation = "speech"
)

var SpeechOperations = []SpeechOperation{SpeechDesign, SpeechConfirm, SpeechSynthesize}

// MaximumUnits is a documented per-request ceiling for this bounded input/path,
// not a measured quantity or a cap inferred from returned audio. Components are
// additive; an operator must not add the same tariff in two different units.
type SpeechCharge struct {
	Unit         llm.SpeechUnit
	USDPerUnit   string
	Multiplier   string
	MaximumUnits string
	// Verified upper conversion at BoundsSource. Missing conversion may be
	// curated as a draft, but cannot authorize a variable-unit paid call.
	UnitsPerInputCharacter string
}
type SpeechPrice struct {
	Operation    SpeechOperation
	Charges      []SpeechCharge
	Source       string
	BoundsSource string
	CheckedAt    time.Time
	// Explicit evidence that this is the complete applicable account schedule.
	Complete bool
}

type SpeechBinding struct {
	ConnectionScope string
	Design          llm.ModelRef
	Synthesis       llm.ModelRef
	Settings        llm.SpeechSettings
	OutputFormat    string
	DescriptionMax  int
	PreviewMax      int
	SpeechMax       int
	// Source snapshots detect capability and rate-factor drift before admission.
	DesignModel llm.SpeechModel
	SpeechModel llm.SpeechModel
}

type SpeechProfile struct {
	ID             string
	Revision       int64
	Label          string
	Level          Level
	Enabled        bool
	Binding        SpeechBinding
	Prices         []SpeechPrice
	VoiceEvidence  string
	ExportEvidence string
	CreatedAt      time.Time
	CatalogManaged bool
	TariffRevision int64
}

// The customer projection deliberately cannot contain supplier evidence or
// prices, even when its caller is master. No selection is stored or inferred.
type SpeechChoice struct {
	ID                string
	Revision          int64
	Label             string
	Design            llm.ModelRef
	DesignLabel       string
	Synthesis         llm.ModelRef
	SynthesisLabel    string
	Level             Level
	RequiredPlan      plan.Plan
	Entitled          bool
	Available         bool
	UnavailableReason string
	DescriptionMax    int
	PreviewMin        int
	PreviewMax        int
	SpeechMax         int
	VoiceReady        bool
	ExportReady       bool
}

type SpeechAdminBrowse struct {
	Profiles     []SpeechProfile
	Candidates   []llm.SpeechModel
	Choices      []SpeechChoice
	FetchError   string
	Combinations []SpeechProfile
	Tariff       SpeechAccountTariff
}

type SpeechAccountTariff struct {
	Revision         int64
	ConnectionScope  string
	DesignUSDPerUnit string
	SpeechUSDPerUnit string
	ConfirmationUSD  string
	Source           string
	Complete         bool
	CheckedAt        time.Time
}

type SpeechRegistration struct {
	ID                string
	ExpectedRevision  int64
	Design, Synthesis llm.ModelRef
	Enabled           bool
	Level             Level
	Adjustments       *llm.SpeechSettings
}

type SpeechQualificationSession struct {
	ID         string
	OwnerID    string
	ProfileID  string
	Revision   int64
	MaximumUSD string
	ExpiresAt  time.Time
}

// Readiness is published by the live qualification harness, never by a profile
// capability checkbox or the customer query string. Evidence remains private.
type SpeechQualificationEvidence struct {
	SessionID          string
	OwnerID            string
	ReportID           string
	DesignRequestID    string
	ConfirmRequestID   string
	SpeechRequestIDs   []string
	ConfirmedVoiceID   string
	AuditionAccepted   bool
	KoreanAccepted     bool
	ContinuityAccepted bool
	UsageVerified      bool
}

type SpeechProfileStore interface {
	ListSpeechProfiles(context.Context) ([]SpeechProfile, error)
	GetSpeechRevision(context.Context, string, int64) (SpeechProfile, error)
	SaveSpeechRevision(context.Context, SpeechProfile, int64) (SpeechProfile, error)
	RecordSpeechReadiness(context.Context, string, int64, string, bool) error
	CreateSpeechQualification(context.Context, SpeechQualificationSession) error
	GetSpeechQualification(context.Context, string, string) (SpeechQualificationSession, error)
	GetSpeechCombination(context.Context, llm.ModelRef, llm.ModelRef) (string, error)
	GetSpeechTariff(context.Context) (SpeechAccountTariff, error)
	SaveSpeechTariff(context.Context, SpeechAccountTariff, int64, []SpeechProfile) error
}
type SpeechCatalogSource interface {
	llm.SpeechCatalogReader
	SpeechConnection() llm.SpeechConnection
}

func SpeechDecimal(value string) (*big.Rat, bool) {
	if !decimalUSD.MatchString(value) {
		return nil, false
	}
	n, ok := new(big.Rat).SetString(value)
	return n, ok && n.Sign() >= 0
}

func validSpeechSource(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && len(raw) <= 2000
}

func (p SpeechPrice) Valid(now time.Time) bool {
	if !p.Complete || !validSpeechSource(p.Source) || !validSpeechSource(p.BoundsSource) || p.CheckedAt.IsZero() || p.CheckedAt.After(now) || len(p.Charges) == 0 || len(p.Charges) > SpeechPriceComponentsMax {
		return false
	}
	seen := map[llm.SpeechUnit]bool{}
	variableCharges := 0
	for _, c := range p.Charges {
		switch c.Unit {
		case llm.SpeechUnitCharacters, llm.SpeechUnitCredits, llm.SpeechUnitRequests, llm.SpeechUnitSeconds, llm.SpeechUnitCharacterCost:
		default:
			return false
		}
		price, ok := SpeechDecimal(c.USDPerUnit)
		if !ok {
			return false
		}
		if c.Unit != llm.SpeechUnitRequests && price.Sign() > 0 {
			variableCharges++
			if variableCharges > 1 {
				return false
			}
		}
		multiplier, ok := SpeechDecimal(c.Multiplier)
		if !ok || multiplier.Sign() <= 0 {
			return false
		}
		maximum, ok := SpeechDecimal(c.MaximumUnits)
		if !ok || maximum.Sign() <= 0 || seen[c.Unit] {
			return false
		}
		seen[c.Unit] = true
		if c.UnitsPerInputCharacter != "" {
			conversion, ok := SpeechDecimal(c.UnitsPerInputCharacter)
			if !ok || conversion.Sign() <= 0 {
				return false
			}
		}
	}
	return true
}

func (p SpeechProfile) PricingReady(now time.Time) bool {
	seen := map[SpeechOperation]bool{}
	for _, price := range p.Prices {
		if !price.Valid(now) || seen[price.Operation] {
			return false
		}
		seen[price.Operation] = true
	}
	return len(p.Prices) == len(SpeechOperations) && seen[SpeechDesign] && seen[SpeechConfirm] && seen[SpeechSynthesize]
}

func (p SpeechProfile) ZeroPriced(now time.Time) bool {
	if !p.PricingReady(now) {
		return false
	}
	for _, price := range p.Prices {
		for _, c := range price.Charges {
			v, ok := SpeechDecimal(c.USDPerUnit)
			if !ok || v.Sign() != 0 {
				return false
			}
		}
	}
	return true
}
