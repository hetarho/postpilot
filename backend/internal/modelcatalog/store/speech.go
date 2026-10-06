package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/modelcatalog/store/sqlc"
)

func (s *Store) ListSpeechProfiles(ctx context.Context) ([]modelcatalog.SpeechProfile, error) {
	rows, err := s.read.ListSpeechProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list speech profiles: %w", err)
	}
	out := make([]modelcatalog.SpeechProfile, 0, len(rows))
	for _, row := range rows {
		p, err := speechFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) GetSpeechRevision(ctx context.Context, id string, rev int64) (modelcatalog.SpeechProfile, error) {
	row, err := s.read.GetSpeechRevision(ctx, sqlc.GetSpeechRevisionParams{ProfileID: id, Revision: rev})
	if errors.Is(err, sql.ErrNoRows) {
		return modelcatalog.SpeechProfile{}, modelcatalog.ErrNotFound
	}
	if err != nil {
		return modelcatalog.SpeechProfile{}, fmt.Errorf("get speech revision: %w", err)
	}
	return speechFromRow(row)
}

func (s *Store) SaveSpeechRevision(ctx context.Context, p modelcatalog.SpeechProfile, expected int64) (modelcatalog.SpeechProfile, error) {
	if p.Revision != expected+1 || expected < 0 {
		return modelcatalog.SpeechProfile{}, modelcatalog.ErrSpeechProfileConflict
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return modelcatalog.SpeechProfile{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if p.CatalogManaged {
		tariff, err := q.GetSpeechTariff(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return modelcatalog.SpeechProfile{}, err
		}
		if tariff.Revision != p.TariffRevision {
			return modelcatalog.SpeechProfile{}, modelcatalog.ErrSpeechProfileConflict
		}
	}
	if err := saveSpeechRevision(ctx, q, p, expected); err != nil {
		return modelcatalog.SpeechProfile{}, err
	}
	return p, tx.Commit()
}

func saveSpeechRevision(ctx context.Context, q *sqlc.Queries, p modelcatalog.SpeechProfile, expected int64) error {
	binding, prices, err := encodeSpeech(p)
	if err != nil {
		return err
	}
	stamp := formatTime(p.CreatedAt)
	if expected == 0 {
		if err := q.CreateSpeechProfile(ctx, sqlc.CreateSpeechProfileParams{ID: p.ID, CurrentRevision: p.Revision, CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
			return err
		}
	} else {
		n, err := q.AdvanceSpeechProfile(ctx, sqlc.AdvanceSpeechProfileParams{ID: p.ID, Revision: p.Revision, Stamp: stamp, ExpectedRevision: expected})
		if err != nil {
			return err
		}
		if n != 1 {
			return modelcatalog.ErrSpeechProfileConflict
		}
	}
	if p.CatalogManaged {
		n, err := q.ClaimSpeechCombination(ctx, sqlc.ClaimSpeechCombinationParams{ProviderID: p.Binding.Design.ProviderID, DesignModelID: p.Binding.Design.ModelID, SpeechModelID: p.Binding.Synthesis.ModelID, ProfileID: p.ID})
		if err != nil {
			return err
		}
		if n != 1 {
			id, err := q.GetSpeechCombination(ctx, sqlc.GetSpeechCombinationParams{ProviderID: p.Binding.Design.ProviderID, DesignModelID: p.Binding.Design.ModelID, SpeechModelID: p.Binding.Synthesis.ModelID})
			if err != nil {
				return err
			}
			if id != p.ID {
				return modelcatalog.ErrSpeechProfileConflict
			}
		}
	}
	level := string(p.Level)
	if level == "" {
		level = string(modelcatalog.LevelValue)
	}
	return q.InsertSpeechRevision(ctx, sqlc.InsertSpeechRevisionParams{ProfileID: p.ID, Revision: p.Revision, ProviderID: p.Binding.Design.ProviderID, DesignModelID: p.Binding.Design.ModelID, SpeechModelID: p.Binding.Synthesis.ModelID, Label: p.Label, Level: level, Enabled: boolToInt(p.Enabled), BindingJson: binding, PricesJson: prices, CreatedAt: stamp, CatalogGrade: sql.NullString{String: string(p.Level), Valid: p.CatalogManaged}, TariffRevision: p.TariffRevision})
}

func (s *Store) RecordSpeechReadiness(ctx context.Context, id string, rev int64, evidence string, export bool) error {
	if evidence == "" {
		return modelcatalog.ErrSpeechQualificationInvalid
	}
	var n int64
	var err error
	if export {
		n, err = s.write.RecordSpeechExportReadiness(ctx, sqlc.RecordSpeechExportReadinessParams{ProfileID: id, Revision: rev, Evidence: evidence})
	} else {
		n, err = s.write.RecordSpeechVoiceReadiness(ctx, sqlc.RecordSpeechVoiceReadinessParams{ProfileID: id, Revision: rev, Evidence: evidence})
	}
	if err != nil {
		return err
	}
	if n != 1 {
		return modelcatalog.ErrSpeechProfileConflict
	}
	return nil
}

func (s *Store) CreateSpeechQualification(ctx context.Context, q modelcatalog.SpeechQualificationSession) error {
	return s.write.CreateSpeechQualification(ctx, sqlc.CreateSpeechQualificationParams{ID: q.ID, OwnerID: q.OwnerID, ProfileID: q.ProfileID, Revision: q.Revision, MaximumUsd: q.MaximumUSD, ExpiresAt: formatTime(q.ExpiresAt)})
}

func (s *Store) GetSpeechQualification(ctx context.Context, owner, id string) (modelcatalog.SpeechQualificationSession, error) {
	row, err := s.read.GetSpeechQualification(ctx, sqlc.GetSpeechQualificationParams{OwnerID: owner, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return modelcatalog.SpeechQualificationSession{}, modelcatalog.ErrSpeechQualificationInvalid
	}
	if err != nil {
		return modelcatalog.SpeechQualificationSession{}, err
	}
	at, err := parseTime(row.ExpiresAt)
	if err != nil {
		return modelcatalog.SpeechQualificationSession{}, err
	}
	return modelcatalog.SpeechQualificationSession{ID: row.ID, OwnerID: row.OwnerID, ProfileID: row.ProfileID, Revision: row.Revision, MaximumUSD: row.MaximumUsd, ExpiresAt: at}, nil
}

func speechFromRow(row sqlc.SpeechProfileRevision) (modelcatalog.SpeechProfile, error) {
	binding, prices, err := decodeSpeech(row.BindingJson, row.PricesJson)
	if err != nil {
		return modelcatalog.SpeechProfile{}, err
	}
	if binding.Design.ProviderID != row.ProviderID || binding.Synthesis.ProviderID != row.ProviderID || binding.Design.ModelID != row.DesignModelID || binding.Synthesis.ModelID != row.SpeechModelID {
		return modelcatalog.SpeechProfile{}, errors.New("speech binding columns disagree")
	}
	grade := row.Level
	if row.CatalogGrade.Valid {
		grade = row.CatalogGrade.String
	}
	level, err := modelcatalog.ParseLevel(grade)
	if err != nil {
		return modelcatalog.SpeechProfile{}, err
	}
	at, err := parseTime(row.CreatedAt)
	if err != nil {
		return modelcatalog.SpeechProfile{}, err
	}
	return modelcatalog.SpeechProfile{ID: row.ProfileID, Revision: row.Revision, Label: row.Label, Level: level, Enabled: row.Enabled == 1, Binding: binding, Prices: prices, VoiceEvidence: row.VoiceEvidence, ExportEvidence: row.ExportEvidence, CreatedAt: at, CatalogManaged: row.CatalogGrade.Valid, TariffRevision: row.TariffRevision}, nil
}

var _ modelcatalog.SpeechProfileStore = (*Store)(nil)

// JSON is an opaque versioned snapshot at this edge, never a transport/domain tag.
type speechBindingRecord struct {
	ConnectionScope string               `json:"connection_scope"`
	Version         int                  `json:"version"`
	Provider        string               `json:"provider"`
	Design          string               `json:"design"`
	Synthesis       string               `json:"synthesis"`
	Settings        speechSettingsRecord `json:"settings"`
	Format          string               `json:"format"`
	DescriptionMax  int                  `json:"description_max"`
	PreviewMax      int                  `json:"preview_max"`
	SpeechMax       int                  `json:"speech_max"`
	DesignModel     speechModelRecord    `json:"design_model"`
	SpeechModel     speechModelRecord    `json:"speech_model"`
}
type speechSettingsRecord struct {
	Stability    float64 `json:"stability"`
	Similarity   float64 `json:"similarity"`
	Style        float64 `json:"style"`
	SpeakerBoost bool    `json:"speaker_boost"`
	Speed        float64 `json:"speed"`
}
type speechModelRecord struct {
	Label        string `json:"label"`
	Design       bool   `json:"design"`
	Synthesis    bool   `json:"synthesis"`
	Korean       bool   `json:"korean"`
	Style        bool   `json:"style"`
	SpeakerBoost bool   `json:"speaker_boost"`
	Alpha        bool   `json:"alpha"`
	Max          int    `json:"max"`
	Factor       string `json:"factor"`
	Character    string `json:"character"`
	Discount     string `json:"discount"`
}
type speechPricesRecord struct {
	Version int                 `json:"version"`
	Prices  []speechPriceRecord `json:"prices"`
}
type speechPriceRecord struct {
	Operation    string               `json:"operation"`
	Charges      []speechChargeRecord `json:"charges"`
	Source       string               `json:"source"`
	BoundsSource string               `json:"bounds_source"`
	CheckedAt    string               `json:"checked_at"`
	Complete     bool                 `json:"complete"`
}
type speechChargeRecord struct {
	Unit         string `json:"unit"`
	USD          string `json:"usd"`
	Multiplier   string `json:"multiplier"`
	Maximum      string `json:"maximum"`
	PerCharacter string `json:"per_character,omitempty"`
}
