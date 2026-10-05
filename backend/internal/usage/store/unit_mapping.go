package store

import (
	"encoding/json"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

// Only persistence records carry serialization. The version is the boundary
// between an old completion-only admission and an explicitly priced unit job.
type unitTariffRecord struct{ Unit, USDPerUnit, Multiplier, MaximumUnits, UnitsPerInputCharacter string }
type unitBudgetRecord struct {
	BoundedInput                                                              bool   `json:",omitempty"`
	TotalInputCharacters                                                      int    `json:",omitempty"`
	InputIdentityDigest                                                       string `json:",omitempty"`
	PolicyID                                                                  string
	Revision                                                                  int64
	AuthorizationID, ScopeDigest, ProviderID, ModelID, Operation, InputDigest string
	AuxiliaryCharacters                                                       int
	ParametersDigest                                                          string
	Count, InputCharacters                                                    int
	Tariffs                                                                   []unitTariffRecord
	Source, BoundsSource, CheckedAt                                           string
	Complete                                                                  bool
}
type unitBudgetsRecord struct {
	Version int
	Calls   []unitBudgetRecord
}
type unitQuoteRecord struct {
	Version                  int
	ID, UserID, Kind, Digest string
	Calls                    []unitBudgetRecord
	Source, PublicationDate  string
	ReferenceE4, AppliedE4   int64
	Temporary                bool
	MaxCredits               int
	ExpiresAt                string
}
type unitEvidenceRecord struct{ Unit, Quantity string }

func budgetsToRecord(calls []usage.UnitBudget) []unitBudgetRecord {
	out := make([]unitBudgetRecord, 0, len(calls))
	for _, c := range calls {
		r := unitBudgetRecord{BoundedInput: c.BoundedInput, TotalInputCharacters: c.TotalInputCharacters, InputIdentityDigest: c.InputIdentityDigest, PolicyID: c.PolicyID, Revision: c.Revision, AuthorizationID: c.AuthorizationID, ScopeDigest: c.ScopeDigest, ProviderID: c.Ref.ProviderID, ModelID: c.Ref.ModelID,
			Operation: c.Operation, InputDigest: c.InputDigest, Count: c.Count, InputCharacters: c.InputCharacters, AuxiliaryCharacters: c.AuxiliaryCharacters, ParametersDigest: c.ParametersDigest, Source: c.Source, BoundsSource: c.BoundsSource, CheckedAt: formatTime(c.CheckedAt), Complete: c.Complete}
		for _, t := range c.Tariffs {
			r.Tariffs = append(r.Tariffs, unitTariffRecord{string(t.Unit), t.USDPerUnit, t.Multiplier, t.MaximumUnits, t.UnitsPerInputCharacter})
		}
		out = append(out, r)
	}
	return out
}

func budgetsFromRecord(records []unitBudgetRecord) ([]usage.UnitBudget, error) {
	out := make([]usage.UnitBudget, 0, len(records))
	for _, r := range records {
		stamp, err := time.Parse(writeLayout, r.CheckedAt)
		if err != nil {
			return nil, err
		}
		c := usage.UnitBudget{BoundedInput: r.BoundedInput, TotalInputCharacters: r.TotalInputCharacters, InputIdentityDigest: r.InputIdentityDigest, PolicyID: r.PolicyID, Revision: r.Revision, AuthorizationID: r.AuthorizationID, ScopeDigest: r.ScopeDigest, Ref: llm.ModelRef{ProviderID: r.ProviderID, ModelID: r.ModelID}, Operation: r.Operation,
			InputDigest: r.InputDigest, Count: r.Count, InputCharacters: r.InputCharacters, AuxiliaryCharacters: r.AuxiliaryCharacters, ParametersDigest: r.ParametersDigest, Source: r.Source, BoundsSource: r.BoundsSource, CheckedAt: stamp, Complete: r.Complete}
		for _, t := range r.Tariffs {
			c.Tariffs = append(c.Tariffs, usage.UnitTariff{Unit: llm.SpeechUnit(t.Unit), USDPerUnit: t.USDPerUnit, Multiplier: t.Multiplier, MaximumUnits: t.MaximumUnits, UnitsPerInputCharacter: t.UnitsPerInputCharacter})
		}
		out = append(out, c)
	}
	return out, nil
}

func encodeUnitQuote(q usage.UnitQuote) (string, error) {
	r := unitQuoteRecord{Version: 1, ID: q.ID, UserID: q.UserID, Kind: q.Kind, Digest: q.Digest, Calls: budgetsToRecord(q.Calls), Source: q.Rate.Source, PublicationDate: q.Rate.PublicationDate,
		ReferenceE4: q.Rate.ReferenceE4, AppliedE4: q.Rate.AppliedE4, Temporary: q.Rate.Temporary, MaxCredits: q.MaxCredits, ExpiresAt: formatTime(q.ExpiresAt)}
	data, err := json.Marshal(r)
	return string(data), err
}

func decodeUnitQuote(data string) (usage.UnitQuote, error) {
	var r unitQuoteRecord
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return usage.UnitQuote{}, err
	}
	if r.Version != 1 {
		return usage.UnitQuote{}, usage.ErrUnitApproval
	}
	calls, err := budgetsFromRecord(r.Calls)
	if err != nil {
		return usage.UnitQuote{}, err
	}
	expires, err := parseTime(r.ExpiresAt)
	if err != nil {
		return usage.UnitQuote{}, err
	}
	return usage.UnitQuote{ID: r.ID, UserID: r.UserID, Kind: r.Kind, Digest: r.Digest, Calls: calls, MaxCredits: r.MaxCredits, ExpiresAt: expires,
		Rate: plan.RateSnapshot{Source: r.Source, PublicationDate: r.PublicationDate, ReferenceE4: r.ReferenceE4, AppliedE4: r.AppliedE4, Temporary: r.Temporary}}, nil
}
