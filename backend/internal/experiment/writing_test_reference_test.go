package experiment

import (
	"errors"
	"fmt"
	"testing"
)

func TestWritingTestShapeRequiresOneExactReferenceKind(t *testing.T) {
	model := TestEntrantRef{SourceKind: "model", Model: ModelRef{ProviderID: "p", ModelID: "a"}}
	setting := TestEntrantRef{SourceKind: "setting", SettingKind: "template", SettingID: "a", SettingRevision: "r1"}
	prepared := TestEntrantRef{SourceKind: "authoring_candidate", AuthoringSessionID: "session", AuthoringCandidateID: "a", AuthoringRevision: 1}
	cases := []struct {
		name   string
		factor TestFactor
		ref    TestEntrantRef
		mutate func(*TestEntrantRef)
	}{
		{"model setting kind", FactorModel, model, func(r *TestEntrantRef) { r.SettingKind = "template" }},
		{"model setting revision", FactorModel, model, func(r *TestEntrantRef) { r.SettingRevision = "r1" }},
		{"model prepared candidate", FactorModel, model, func(r *TestEntrantRef) { r.AuthoringCandidateID = "private" }},
		{"model prepared revision", FactorModel, model, func(r *TestEntrantRef) { r.AuthoringRevision = 1 }},
		{"setting prepared candidate", FactorTemplate, setting, func(r *TestEntrantRef) { r.AuthoringCandidateID = "private" }},
		{"setting prepared revision", FactorTemplate, setting, func(r *TestEntrantRef) { r.AuthoringRevision = 1 }},
		{"prepared missing revision", FactorTemplate, prepared, func(r *TestEntrantRef) { r.AuthoringRevision = 0 }},
		{"prepared unrelated kind", FactorTemplate, prepared, func(r *TestEntrantRef) { r.SettingKind = "voice" }},
		{"prepared saved revision", FactorTemplate, prepared, func(r *TestEntrantRef) { r.SettingRevision = "r1" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first, second := tc.ref, tc.ref
			switch tc.ref.SourceKind {
			case "model":
				second.Model.ModelID = "b"
			case "setting":
				second.SettingID = "b"
			case "authoring_candidate":
				second.AuthoringCandidateID = "b"
			}
			start := TestStart{UserID: "owner", RequestKey: "request", Factor: tc.factor, Count: 2, Entrants: []TestEntrantRef{first, second}}
			if tc.factor == FactorModel {
				start.ModelStage = StageWrite
			}
			if err := ValidateTestShape(start); err != nil {
				t.Fatalf("valid reference rejected: %v", err)
			}
			tc.mutate(&start.Entrants[0])
			if err := ValidateTestShape(start); !errors.Is(err, ErrTestEntrant) {
				t.Fatalf("mixed reference accepted: %v", err)
			}
		})
	}
}

func TestWritingTestShapeFormatsAxesAndMixedPreparedSettings(t *testing.T) {
	for _, count := range []int{2, 4, 8, 16} {
		for _, factor := range []TestFactor{FactorModel, FactorVoice, FactorTemplate, FactorGuideline} {
			start := TestStart{UserID: "owner", RequestKey: "request", Factor: factor, Count: count}
			if factor == FactorModel {
				start.ModelStage = StageObserve
			}
			for index := 0; index < count; index++ {
				id := fmt.Sprint(index)
				switch {
				case factor == FactorModel:
					start.Entrants = append(start.Entrants, TestEntrantRef{SourceKind: "model", Model: ModelRef{ProviderID: "p", ModelID: id}})
				case index%2 == 0:
					start.Entrants = append(start.Entrants, TestEntrantRef{SourceKind: "setting", SettingKind: string(factor), SettingID: id, SettingRevision: "r1"})
				default:
					start.Entrants = append(start.Entrants, TestEntrantRef{SourceKind: "authoring_candidate", SettingKind: string(factor), AuthoringSessionID: "session", AuthoringCandidateID: id, AuthoringRevision: 1})
				}
			}
			if err := ValidateTestShape(start); err != nil {
				t.Fatalf("%s/%d: %v", factor, count, err)
			}
			start.Entrants[1] = start.Entrants[0]
			if err := ValidateTestShape(start); !errors.Is(err, ErrTestDuplicate) {
				t.Fatalf("%s/%d duplicate: %v", factor, count, err)
			}
		}
	}
	for _, count := range []int{0, 1, 3, 5, 6, 7, 9, 15, 17} {
		start := TestStart{UserID: "owner", RequestKey: "request", Factor: FactorModel, ModelStage: StageWrite, Count: count, Entrants: make([]TestEntrantRef, count)}
		if err := ValidateTestShape(start); !errors.Is(err, ErrTestCount) {
			t.Fatalf("legacy/invalid count %d accepted: %v", count, err)
		}
	}
}
