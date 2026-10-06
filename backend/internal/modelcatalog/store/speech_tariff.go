package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/modelcatalog/store/sqlc"
)

func (s *Store) GetSpeechCombination(ctx context.Context, d, v llm.ModelRef) (string, error) {
	id, err := s.read.GetSpeechCombination(ctx, sqlc.GetSpeechCombinationParams{ProviderID: d.ProviderID, DesignModelID: d.ModelID, SpeechModelID: v.ModelID})
	if errors.Is(err, sql.ErrNoRows) {
		return "", modelcatalog.ErrNotFound
	}
	return id, err
}

func (s *Store) GetSpeechTariff(ctx context.Context) (modelcatalog.SpeechAccountTariff, error) {
	r, err := s.read.GetSpeechTariff(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return modelcatalog.SpeechAccountTariff{}, nil
	}
	if err != nil {
		return modelcatalog.SpeechAccountTariff{}, err
	}
	at, err := parseTime(r.CheckedAt)
	return modelcatalog.SpeechAccountTariff{Revision: r.Revision, ConnectionScope: r.ConnectionScope, DesignUSDPerUnit: r.DesignUsdPerUnit, SpeechUSDPerUnit: r.SpeechUsdPerUnit, ConfirmationUSD: r.ConfirmationUsd, Source: r.Source, Complete: r.Complete == 1, CheckedAt: at}, err
}

func (s *Store) SaveSpeechTariff(ctx context.Context, t modelcatalog.SpeechAccountTariff, expected int64, updates []modelcatalog.SpeechProfile) error {
	if t.Revision != expected+1 {
		return modelcatalog.ErrSpeechProfileConflict
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	current, err := q.GetSpeechTariff(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if current.Revision != expected {
		return modelcatalog.ErrSpeechProfileConflict
	}
	rows, err := q.ListSpeechProfiles(ctx)
	if err != nil {
		return err
	}
	managed := map[string]int64{}
	for _, r := range rows {
		owner, err := q.GetSpeechCombination(ctx, sqlc.GetSpeechCombinationParams{ProviderID: r.ProviderID, DesignModelID: r.DesignModelID, SpeechModelID: r.SpeechModelID})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if owner == r.ProfileID {
			managed[r.ProfileID] = r.Revision
		}
	}
	if len(managed) != len(updates) {
		return modelcatalog.ErrSpeechProfileConflict
	}
	for _, p := range updates {
		if !p.CatalogManaged || managed[p.ID] != p.Revision-1 || p.TariffRevision != t.Revision {
			return modelcatalog.ErrSpeechProfileConflict
		}
		delete(managed, p.ID)
	}
	if err := q.InsertSpeechTariff(ctx, sqlc.InsertSpeechTariffParams{Revision: t.Revision, ConnectionScope: t.ConnectionScope, DesignUsdPerUnit: t.DesignUSDPerUnit, SpeechUsdPerUnit: t.SpeechUSDPerUnit, ConfirmationUsd: t.ConfirmationUSD, Source: t.Source, Complete: boolToInt(t.Complete), CheckedAt: formatTime(t.CheckedAt)}); err != nil {
		return err
	}
	if expected == 0 {
		if err := q.CreateSpeechTariffPointer(ctx, t.Revision); err != nil {
			return err
		}
	} else {
		n, err := q.AdvanceSpeechTariffPointer(ctx, sqlc.AdvanceSpeechTariffPointerParams{Revision: t.Revision, ExpectedRevision: expected})
		if err != nil {
			return err
		}
		if n != 1 {
			return modelcatalog.ErrSpeechProfileConflict
		}
	}
	for _, p := range updates {
		if err := saveSpeechRevision(ctx, q, p, p.Revision-1); err != nil {
			return err
		}
	}
	return tx.Commit()
}
