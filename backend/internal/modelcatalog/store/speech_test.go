package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/modelcatalog/store"
	"github.com/postpilot/backend/internal/platform/db"
)

func TestSpeechStorePreservesImmutableBindingsAndDoesNotTouchCompletionCatalog(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	before, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p := modelcatalog.SpeechProfile{ID: "private-profile", Revision: 1, Label: "Spoken voice", Level: modelcatalog.LevelValue, Enabled: true, CreatedAt: testNow, Binding: modelcatalog.SpeechBinding{ConnectionScope: "private-connection-scope", Design: llm.ModelRef{ProviderID: "speech", ModelID: "design"}, Synthesis: llm.ModelRef{ProviderID: "speech", ModelID: "synth"}, Settings: llm.SpeechSettings{Stability: 0.5, SimilarityBoost: 0.75, Speed: 1}, OutputFormat: llm.SpeechOutputFormat, DescriptionMax: 1000, PreviewMax: 1000, SpeechMax: 1000, DesignModel: llm.SpeechModel{Ref: llm.ModelRef{ProviderID: "speech", ModelID: "design"}, Design: true, Label: "Design"}, SpeechModel: llm.SpeechModel{Ref: llm.ModelRef{ProviderID: "speech", ModelID: "synth"}, Synthesis: true, Korean: true, Label: "Synth", MaxText: 5000, CharacterCostMultiplier: "1.250001"}}, Prices: []modelcatalog.SpeechPrice{{Operation: modelcatalog.SpeechDesign, Source: "https://example.com/price", BoundsSource: "https://example.com/limit", CheckedAt: testNow, Complete: true, Charges: []modelcatalog.SpeechCharge{{Unit: llm.SpeechUnitCharacterCost, USDPerUnit: "0.000000001", Multiplier: "1.25", MaximumUnits: "1000.5"}}}}}
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

func catalogProfile(id, synth string) modelcatalog.SpeechProfile {
	d := llm.ModelRef{ProviderID: "speech", ModelID: "design"}
	v := llm.ModelRef{ProviderID: "speech", ModelID: synth}
	return modelcatalog.SpeechProfile{ID: id, Revision: 1, Label: "Catalog pair", Enabled: true, CatalogManaged: true, CreatedAt: testNow,
		Binding: modelcatalog.SpeechBinding{ConnectionScope: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Design: d, Synthesis: v,
			Settings: llm.SpeechSettings{Stability: .5, SimilarityBoost: .75, Speed: 1}, OutputFormat: llm.SpeechOutputFormat, DescriptionMax: 1000, PreviewMax: 1000, SpeechMax: 1000,
			DesignModel: llm.SpeechModel{Ref: d, Label: "Design", Design: true}, SpeechModel: llm.SpeechModel{Ref: v, Label: "Synth", Synthesis: true, Korean: true, MaxText: 5000}}}
}
func TestSpeechCatalogPairUniquenessAndUnclassifiedHistory(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	p := catalogProfile("first", "synth")
	if _, err := s.SaveSpeechRevision(ctx, p, 0); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSpeechRevision(ctx, p.ID, 1)
	if err != nil || !got.CatalogManaged || got.Level != "" {
		t.Fatal(got, err)
	}
	duplicate := p
	duplicate.ID = "duplicate"
	if _, err := s.SaveSpeechRevision(ctx, duplicate, 0); !errors.Is(err, modelcatalog.ErrSpeechProfileConflict) {
		t.Fatal("duplicate pair admitted", err)
	}
	rows, _ := s.ListSpeechProfiles(ctx)
	if len(rows) != 1 {
		t.Fatal("failed claim left orphan profile", rows)
	}
	owner, err := s.GetSpeechCombination(ctx, p.Binding.Design, p.Binding.Synthesis)
	if err != nil || owner != p.ID {
		t.Fatal(owner, err)
	}
}
func TestSpeechTariffCASAndProfileRefreshCommitTogether(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	p := catalogProfile("first", "synth")
	s.SaveSpeechRevision(ctx, p, 0)
	tariff := modelcatalog.SpeechAccountTariff{Revision: 1, ConnectionScope: p.Binding.ConnectionScope, DesignUSDPerUnit: "0.000100001", SpeechUSDPerUnit: "0.000000001", ConfirmationUSD: "0", Source: "https://example.com/account", Complete: true, CheckedAt: testNow}
	update := p
	update.Revision = 2
	update.TariffRevision = 1
	invalid := update
	invalid.Revision = 3
	if err := s.SaveSpeechTariff(ctx, tariff, 0, []modelcatalog.SpeechProfile{invalid}); !errors.Is(err, modelcatalog.ErrSpeechProfileConflict) {
		t.Fatal(err)
	}
	current, err := s.GetSpeechTariff(ctx)
	if err != nil || current.Revision != 0 {
		t.Fatal("partial tariff published", current, err)
	}
	if err := s.SaveSpeechTariff(ctx, tariff, 0, []modelcatalog.SpeechProfile{update}); err != nil {
		t.Fatal(err)
	}
	current, err = s.GetSpeechTariff(ctx)
	if err != nil || !reflect.DeepEqual(current, tariff) {
		t.Fatal("decimal terms changed", current, err)
	}
	if err := s.SaveSpeechTariff(ctx, tariff, 0, []modelcatalog.SpeechProfile{update}); !errors.Is(err, modelcatalog.ErrSpeechProfileConflict) {
		t.Fatal("stale tariff accepted", err)
	}
	old, _ := s.GetSpeechRevision(ctx, p.ID, 1)
	if old.TariffRevision != 0 || old.Binding != p.Binding {
		t.Fatal("old binding overwritten")
	}
	stale := update
	stale.Revision = 3
	stale.TariffRevision = 0
	if _, err := s.SaveSpeechRevision(ctx, stale, 2); !errors.Is(err, modelcatalog.ErrSpeechProfileConflict) {
		t.Fatal("stale tariff reused", err)
	}
}

func TestConcurrentSpeechCatalogClaimsPublishOneRegistration(t *testing.T) {
	s := newStore(t)
	results := make(chan error, 4)
	for i := range 4 {
		go func(i int) {
			p := catalogProfile(fmt.Sprintf("claim-%d", i), "same-synth")
			_, err := s.SaveSpeechRevision(t.Context(), p, 0)
			results <- err
		}(i)
	}
	winners := 0
	for range 4 {
		err := <-results
		if err == nil {
			winners++
		} else if !errors.Is(err, modelcatalog.ErrSpeechProfileConflict) {
			t.Fatal(err)
		}
	}
	rows, err := s.ListSpeechProfiles(t.Context())
	if err != nil || winners != 1 || len(rows) != 1 {
		t.Fatalf("winners=%d rows=%d err=%v", winners, len(rows), err)
	}
}

func TestSpeechCatalogUpgradePreservesReferencedLegacyRevision(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := t.Context()
	if _, err = handle.Writer.ExecContext(ctx, "CREATE TABLE users(id TEXT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	oldSchema, err := os.ReadFile("../../platform/db/migrations/0133_speech_profiles.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = handle.Writer.ExecContext(ctx, strings.Split(string(oldSchema), "-- +goose Down")[0]); err != nil {
		t.Fatal(err)
	}
	binding := `{"version":1,"connection_scope":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","provider":"speech","design":"design","synthesis":"synth","settings":{"stability":0.3,"similarity":0.8,"style":0.1,"speaker_boost":true,"speed":1},"format":"mp3_44100_128","description_max":800,"preview_max":700,"speech_max":600}`
	prices := `{"version":1,"prices":[]}`
	stamp := testNow.Format("2006-01-02T15:04:05.000000000Z07:00")
	if _, err = handle.Writer.ExecContext(ctx, "INSERT INTO speech_profiles VALUES ('legacy',1,?,?)", stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err = handle.Writer.ExecContext(ctx, "INSERT INTO speech_profile_revisions(profile_id,revision,provider_id,design_model_id,speech_model_id,label,level,enabled,binding_json,prices_json,created_at) VALUES ('legacy',1,'speech','design','synth','Existing voice','value',1,?,?,?)", binding, prices, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err = handle.Writer.ExecContext(ctx, "CREATE TABLE owned_audio_probe(profile_id TEXT,revision INTEGER,FOREIGN KEY(profile_id,revision) REFERENCES speech_profile_revisions(profile_id,revision)); INSERT INTO owned_audio_probe VALUES ('legacy',1)"); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../platform/db/migrations/0143_speech_catalog_tariff.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = handle.Writer.ExecContext(ctx, strings.Split(string(migration), "-- +goose Down")[0]); err != nil {
		t.Fatal(err)
	}
	s := store.New(handle.Writer, handle.Reader)
	old, err := s.GetSpeechRevision(ctx, "legacy", 1)
	if err != nil || old.CatalogManaged || old.Binding.Settings.Stability != .3 || !old.Binding.Settings.SpeakerBoost {
		t.Fatal(old, err)
	}
	owner, err := s.GetSpeechCombination(ctx, old.Binding.Design, old.Binding.Synthesis)
	if err != nil || owner != "legacy" {
		t.Fatal(owner, err)
	}
	update := old
	update.Revision = 2
	update.CatalogManaged = true
	update.TariffRevision = 1
	tariff := modelcatalog.SpeechAccountTariff{Revision: 1, ConnectionScope: old.Binding.ConnectionScope, DesignUSDPerUnit: "0.0001", SpeechUSDPerUnit: "0.0001", ConfirmationUSD: "0", Source: "https://example.com/account", Complete: true, CheckedAt: testNow}
	if err = s.SaveSpeechTariff(ctx, tariff, 0, []modelcatalog.SpeechProfile{update}); err != nil {
		t.Fatal(err)
	}
	retained, err := s.GetSpeechRevision(ctx, "legacy", 1)
	if err != nil || retained.Binding != old.Binding || retained.Level != old.Level {
		t.Fatal("legacy revision changed", retained, err)
	}
	var count int
	if err = handle.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM owned_audio_probe WHERE profile_id='legacy' AND revision=1").Scan(&count); err != nil || count != 1 {
		t.Fatal("owned reference lost", count, err)
	}
	rows, err := handle.Reader.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("migration broke a foreign key")
	}
}
