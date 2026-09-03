// Package store persists the model catalog. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/modelcatalog/store/sqlc"
)

// The fixed-width UTC layout every context writes timestamps in, so string comparison
// and ORDER BY agree with chronological order.
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

func (s *Store) List(ctx context.Context) ([]modelcatalog.Model, error) {
	rows, err := s.read.ListCatalogModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("select catalog models: %w", err)
	}
	registrations, err := s.read.ListCatalogModelPurposes(ctx)
	if err != nil {
		return nil, fmt.Errorf("select catalog model purposes: %w", err)
	}
	byModel := map[string][]modelcatalog.Purpose{}
	for _, registration := range registrations {
		purposes, err := parsePurposes(registration.ModelID, []string{registration.Purpose})
		if err != nil {
			return nil, err
		}
		byModel[registration.ModelID] = append(byModel[registration.ModelID], purposes...)
	}
	out := make([]modelcatalog.Model, 0, len(rows))
	for _, row := range rows {
		model, err := toModel(sqlc.GetCatalogModelRow(row), byModel[row.ModelID])
		if err != nil {
			return nil, err
		}
		out = append(out, model)
	}
	return out, nil
}

func (s *Store) Get(ctx context.Context, modelID string) (modelcatalog.Model, error) {
	return get(ctx, s.read, modelID)
}

func get(ctx context.Context, q *sqlc.Queries, modelID string) (modelcatalog.Model, error) {
	row, err := q.GetCatalogModel(ctx, modelID)
	if errors.Is(err, sql.ErrNoRows) {
		return modelcatalog.Model{}, modelcatalog.ErrNotFound
	}
	if err != nil {
		return modelcatalog.Model{}, fmt.Errorf("select catalog model: %w", err)
	}
	stored, err := q.GetCatalogModelPurposes(ctx, modelID)
	if err != nil {
		return modelcatalog.Model{}, fmt.Errorf("select catalog model purposes: %w", err)
	}
	purposes, err := parsePurposes(modelID, stored)
	if err != nil {
		return modelcatalog.Model{}, err
	}
	return toModel(row, purposes)
}

func (s *Store) Upsert(ctx context.Context, m modelcatalog.Model) error {
	return upsert(ctx, s.write, m)
}

func upsert(ctx context.Context, q *sqlc.Queries, m modelcatalog.Model) error {
	err := q.UpsertCatalogModel(ctx, sqlc.UpsertCatalogModelParams{
		ModelID:             m.ModelID,
		ProviderSlug:        m.ProviderSlug,
		Label:               m.Label,
		Vision:              boolToInt(m.Vision),
		StructuredOutput:    boolToInt(m.StructuredOutput),
		ImageOutput:         boolToInt(m.ImageOutput),
		VideoOutput:         boolToInt(m.VideoOutput),
		ContextTokens:       nullInt(m.ContextTokens),
		InputUsdPerMillion:  nullString(m.InputUSDPerMillion),
		OutputUsdPerMillion: nullString(m.OutputUSDPerMillion),
		PricingCheckedAt:    nullString(m.PricingCheckedAt),
		ReasoningEffort:     nullString(string(m.Reasoning)),
		Listed:              boolToInt(m.Listed),
		LastSeenAt:          nullTime(m.LastSeenAt),
		CreatedAt:           formatTime(m.CreatedAt),
		UpdatedAt:           formatTime(m.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("upsert catalog model: %w", err)
	}
	return nil
}

// Patch reads, applies and writes back in one transaction. The merge happens in Go rather
// than in the SQL because "leave this field alone" and "clear this field" are different
// requests, and a COALESCE cannot tell them apart.
func (s *Store) Patch(ctx context.Context, modelID string, patch modelcatalog.Patch, updatedAt time.Time) (modelcatalog.Model, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return modelcatalog.Model{}, fmt.Errorf("begin patch catalog model: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)

	current, err := get(ctx, q, modelID)
	if err != nil {
		return modelcatalog.Model{}, err
	}
	if patch.Reasoning != nil {
		current.Reasoning = *patch.Reasoning
	}
	current.UpdatedAt = updatedAt

	n, err := q.UpdateCatalogModelCuration(ctx, sqlc.UpdateCatalogModelCurationParams{
		ReasoningEffort: nullString(string(current.Reasoning)),
		UpdatedAt:       formatTime(current.UpdatedAt),
		ModelID:         modelID,
	})
	if err != nil {
		return modelcatalog.Model{}, fmt.Errorf("update catalog model: %w", err)
	}
	if n == 0 {
		return modelcatalog.Model{}, modelcatalog.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return modelcatalog.Model{}, fmt.Errorf("commit patch catalog model: %w", err)
	}
	return current, nil
}

// RegisterPurpose writes the row snapshot and the registration together. One transaction,
// so an error between the two writes cannot leave a curated row whose registration
// silently did not stick. Idempotent: re-checking an already-checked box only refreshes
// the snapshot, and the original registration keeps its created_at.
func (s *Store) RegisterPurpose(ctx context.Context, m modelcatalog.Model, purpose modelcatalog.Purpose) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin register model purpose: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := upsert(ctx, q, m); err != nil {
		return err
	}
	err = q.AddCatalogModelPurpose(ctx, sqlc.AddCatalogModelPurposeParams{
		ModelID: m.ModelID, Purpose: string(purpose), CreatedAt: formatTime(m.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("add catalog model purpose: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit register model purpose: %w", err)
	}
	return nil
}

// DeregisterPurpose removes one registration and stamps updated_at in the same
// transaction — a deregistration is a curation edit. The catalog row itself stays, so the
// reasoning override survives a full deregistration the way it survived `enabled = 0`.
func (s *Store) DeregisterPurpose(ctx context.Context, modelID string, purpose modelcatalog.Purpose, at time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin deregister model purpose: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	err = q.RemoveCatalogModelPurpose(ctx, sqlc.RemoveCatalogModelPurposeParams{
		ModelID: modelID, Purpose: string(purpose),
	})
	if err != nil {
		return fmt.Errorf("remove catalog model purpose: %w", err)
	}
	if err := q.TouchCatalogModelCuration(ctx, sqlc.TouchCatalogModelCurationParams{
		UpdatedAt: formatTime(at), ModelID: modelID,
	}); err != nil {
		return fmt.Errorf("stamp catalog model curation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit deregister model purpose: %w", err)
	}
	return nil
}

// RefreshAvailability records what one successful upstream read saw.
//
// Everything is unlisted first and the seen models are put back in the same transaction,
// so no reader ever observes the gap and a model the provider dropped needs no separate
// delete pass. Only rows that already exist are touched: the catalog stores what an
// operator curated, not the four hundred models they did not.
func (s *Store) RefreshAvailability(ctx context.Context, seen []modelcatalog.Candidate, at time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catalog refresh: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)

	if err := q.UnlistAllCatalogModels(ctx); err != nil {
		return fmt.Errorf("unlist catalog models: %w", err)
	}
	stamp := formatTime(at)
	pricingDate := at.UTC().Format(time.DateOnly)
	for _, candidate := range seen {
		err := q.MarkCatalogModelSeen(ctx, sqlc.MarkCatalogModelSeenParams{
			ProviderSlug:        candidate.ProviderSlug,
			Label:               candidate.Label,
			Vision:              boolToInt(candidate.Vision),
			StructuredOutput:    boolToInt(candidate.StructuredOutput),
			ImageOutput:         boolToInt(candidate.ImageOutput),
			VideoOutput:         boolToInt(candidate.VideoOutput),
			ContextTokens:       nullInt(candidate.ContextTokens),
			InputUsdPerMillion:  nullString(candidate.InputUSDPerMillion),
			OutputUsdPerMillion: nullString(candidate.OutputUSDPerMillion),
			PricingCheckedAt:    nullString(pricingDate),
			LastSeenAt:          nullString(stamp),
			ModelID:             candidate.ModelID,
		})
		if err != nil {
			return fmt.Errorf("mark catalog model seen: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit catalog refresh: %w", err)
	}
	return nil
}

func toModel(row sqlc.GetCatalogModelRow, purposes []modelcatalog.Purpose) (modelcatalog.Model, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return modelcatalog.Model{}, fmt.Errorf("parse catalog model created_at: %w", err)
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return modelcatalog.Model{}, fmt.Errorf("parse catalog model updated_at: %w", err)
	}
	var lastSeen time.Time
	if row.LastSeenAt.Valid {
		lastSeen, err = parseTime(row.LastSeenAt.String)
		if err != nil {
			return modelcatalog.Model{}, fmt.Errorf("parse catalog model last_seen_at: %w", err)
		}
	}
	// The column CHECK already refuses anything else, so a failure here means the row was
	// written by something other than this code.
	reasoning := llm.ReasoningEffort(row.ReasoningEffort.String)
	if !row.ReasoningEffort.Valid {
		reasoning = llm.ReasoningUnspecified
	}
	if !reasoning.Valid() {
		return modelcatalog.Model{}, fmt.Errorf("catalog model %s: unknown reasoning_effort %q", row.ModelID, row.ReasoningEffort.String)
	}
	// The join rows arrive in column order; the domain order is the display order.
	modelcatalog.SortPurposes(purposes)
	return modelcatalog.Model{
		ModelID:             row.ModelID,
		ProviderSlug:        row.ProviderSlug,
		Label:               row.Label,
		Vision:              row.Vision == 1,
		StructuredOutput:    row.StructuredOutput == 1,
		ImageOutput:         row.ImageOutput == 1,
		VideoOutput:         row.VideoOutput == 1,
		ContextTokens:       row.ContextTokens.Int64,
		InputUSDPerMillion:  row.InputUsdPerMillion.String,
		OutputUSDPerMillion: row.OutputUsdPerMillion.String,
		PricingCheckedAt:    row.PricingCheckedAt.String,
		Reasoning:           reasoning,
		Purposes:            purposes,
		Listed:              row.Listed == 1,
		LastSeenAt:          lastSeen,
		CreatedAt:           created,
		UpdatedAt:           updated,
	}, nil
}

// parsePurposes maps stored purpose slugs into the domain type. The column CHECK refuses
// anything else, so a failure means the row was written by something other than this code.
func parsePurposes(modelID string, values []string) ([]modelcatalog.Purpose, error) {
	purposes := make([]modelcatalog.Purpose, 0, len(values))
	for _, value := range values {
		purpose, err := modelcatalog.ParsePurpose(value)
		if err != nil {
			return nil, fmt.Errorf("catalog model %s: %w", modelID, err)
		}
		purposes = append(purposes, purpose)
	}
	return purposes, nil
}

func boolToInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullInt(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: value != 0}
}

func nullTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(value), Valid: true}
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

var _ modelcatalog.Store = (*Store)(nil)
