package app

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice/spoken"
	"testing"
	"time"
)

func TestQualificationAccountingRefusesUnknownChangedOrUnsettledEvidence(t *testing.T) {
	b := usage.UnitBudget{PolicyID: "profile", Revision: 1, AuthorizationID: "session", ScopeDigest: usage.UnitDigest("scope"), Ref: llm.ModelRef{ProviderID: "speech", ModelID: "tts"}, Operation: "speech", InputDigest: usage.UnitDigest("input"), ParametersDigest: usage.UnitDigest("parameters"), InputCharacters: 10, Count: 1, Tariffs: []usage.UnitTariff{{Unit: llm.SpeechUnitCharacterCost, USDPerUnit: "0.0001", Multiplier: "1", MaximumUnits: "1000", UnitsPerInputCharacter: "1"}}, Source: "https://example.com/prices", BoundsSource: "https://example.com/bounds", CheckedAt: time.Now().Add(-time.Minute), Complete: true}
	o := spoken.Operation{Kind: spoken.JobKindProbe, QualificationSessionID: b.AuthorizationID, ScopeDigest: b.ScopeDigest, Profile: spoken.Profile{ID: b.PolicyID, Revision: b.Revision, Synthesis: b.Ref}, SpeechInputs: [2]string{b.InputDigest, ""}, Evidence: []llm.SpeechEvidence{{RequestID: "real-request", Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: "10"}}}}}
	j := QualificationJob{Budgets: []usage.UnitBudget{b}, ActualUSD: "1/1000", Settled: true, Settlement: usage.Settlement{Reason: usage.OutcomeSucceeded}}
	if e := verifyQualifiedJob(o, j); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		change func(*spoken.Operation, *QualificationJob)
	}{
		{"unknown units", func(o *spoken.Operation, _ *QualificationJob) {
			o.Evidence = []llm.SpeechEvidence{{RequestID: "request"}}
		}},
		{"reported dollars without units", func(o *spoken.Operation, _ *QualificationJob) {
			o.Evidence = []llm.SpeechEvidence{{RequestID: "request", ReportedUSD: "0.001"}}
		}},
		{"missing request", func(o *spoken.Operation, _ *QualificationJob) { o.Evidence[0].RequestID = "" }},
		{"changed input", func(o *spoken.Operation, _ *QualificationJob) { o.SpeechInputs[0] = usage.UnitDigest("different") }},
		{"changed profile", func(o *spoken.Operation, _ *QualificationJob) { o.Profile.Revision++ }},
		{"changed scope", func(o *spoken.Operation, _ *QualificationJob) { o.ScopeDigest = usage.UnitDigest("different") }},
		{"ledger differs", func(_ *spoken.Operation, j *QualificationJob) { j.ActualUSD = "0" }},
		{"unsettled", func(_ *spoken.Operation, j *QualificationJob) { j.Settled = false }},
		{"cancelled", func(_ *spoken.Operation, j *QualificationJob) { j.Settlement.Reason = usage.OutcomeCancelled }},
		{"over ceiling", func(o *spoken.Operation, _ *QualificationJob) {
			o.Evidence[0].Units = []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: "11"}}
		}},
		{"reported zero hides excess units", func(o *spoken.Operation, j *QualificationJob) {
			o.Evidence[0].Units = []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: "11"}}
			o.Evidence[0].ReportedUSD = "0"
			j.ActualUSD = "0"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := o
			changed.Evidence = append([]llm.SpeechEvidence(nil), o.Evidence...)
			job := j
			tc.change(&changed, &job)
			if e := verifyQualifiedJob(changed, job); e == nil {
				t.Fatal("incomplete evidence qualified")
			}
		})
	}
}
func TestQualificationFlagOrHumanCheckboxAloneCannotQualify(t *testing.T) {
	h := &QualificationHarness{}
	for _, live := range []bool{false, true} {
		if _, e := h.Run(context.Background(), QualificationPlan{Digest: "exact"}, live, "changed"); !errors.Is(e, ErrQualificationEvidence) {
			t.Fatal(e)
		}
	}
	if e := h.Publish(context.Background(), QualificationReport{Review: QualificationReview{Reviewer: "listener", ReviewedAt: time.Now(), AuditionAccepted: true, KoreanAccepted: true, ContinuityAccepted: true}}, "report"); !errors.Is(e, ErrQualificationEvidence) {
		t.Fatal(e)
	}
}
