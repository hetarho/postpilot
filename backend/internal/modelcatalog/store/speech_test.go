package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
)

func TestSpeechStorePreservesImmutableBindingsAndDoesNotTouchCompletionCatalog(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	before, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p := modelcatalog.SpeechProfile{ID: "private-profile", Revision: 1, Label: "Spoken voice", Level: modelcatalog.LevelValue, Enabled: true, CreatedAt: testNow, Binding: modelcatalog.SpeechBinding{Design: llm.ModelRef{ProviderID: "speech", ModelID: "design"}, Synthesis: llm.ModelRef{ProviderID: "speech", ModelID: "synth"}, Settings: llm.SpeechSettings{Stability: 0.5, SimilarityBoost: 0.75, Speed: 1}, OutputFormat: llm.SpeechOutputFormat, DescriptionMax: 1000, PreviewMax: 1000, SpeechMax: 1000, DesignModel: llm.SpeechModel{Ref: llm.ModelRef{ProviderID: "speech", ModelID: "design"}, Design: true, Label: "Design"}, SpeechModel: llm.SpeechModel{Ref: llm.ModelRef{ProviderID: "speech", ModelID: "synth"}, Synthesis: true, Korean: true, Label: "Synth", MaxText: 5000, CharacterCostMultiplier: "1.250001"}}, Prices: []modelcatalog.SpeechPrice{{Operation: modelcatalog.SpeechDesign, Source: "https://example.com/price", BoundsSource: "https://example.com/limit", CheckedAt: testNow, Complete: true, Charges: []modelcatalog.SpeechCharge{{Unit: llm.SpeechUnitCharacterCost, USDPerUnit: "0.000000001", Multiplier: "1.25", MaximumUnits: "1000.5"}}}}}
	if _, err := s.SaveSpeechRevision(ctx, p, 0); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSpeechRevision(ctx, p.ID, 1)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if err := s.RecordSpeechReadiness(ctx, p.ID, 1, "voice-private-evidence", true); !errors.Is(err, modelcatalog.ErrSpeechProfileConflict) {
		t.Fatal("exports qualified before voice:", err)
	}
	if err := s.RecordSpeechReadiness(ctx, p.ID, 1, "voice-private-evidence", false); err != nil {
		t.Fatal(err)
	}
	revised := p
	revised.Revision = 2
	revised.Binding.Synthesis.ModelID = "synth-v2"
	revised.Binding.SpeechModel.Ref = revised.Binding.Synthesis
	revised.Label = "Updated"
	if _, err := s.SaveSpeechRevision(ctx, revised, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveSpeechRevision(ctx, revised, 1); !errors.Is(err, modelcatalog.ErrSpeechProfileConflict) {
		t.Fatalf("stale revision write: %v", err)
	}
	if err := s.RecordSpeechReadiness(ctx, p.ID, 1, "late-old-proof", false); !errors.Is(err, modelcatalog.ErrSpeechProfileConflict) {
		t.Fatalf("late qualification accepted: %v", err)
	}
	old, err := s.GetSpeechRevision(ctx, p.ID, 1)
	if err != nil || old.Binding != p.Binding || old.VoiceEvidence != "voice-private-evidence" {
		t.Fatalf("historical binding changed: %+v %v", old, err)
	}
	profiles, err := s.ListSpeechProfiles(ctx)
	if err != nil || len(profiles) != 1 || profiles[0].Revision != 2 || profiles[0].VoiceEvidence != "" {
		t.Fatalf("current projection: %+v %v", profiles, err)
	}
	after, err := s.List(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("speech curation changed completion rows:", err)
	}
}
