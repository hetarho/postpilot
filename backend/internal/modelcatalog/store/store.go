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
	out := make([]modelcatalog.Model, 0, len(rows))
	for _, row := range rows {
		model, err := toModel(row)
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
	return toModel(row)
}

func (s *Store) Upsert(ctx context.Context, m modelcatalog.Model) error {
	err := s.write.UpsertCatalogModel(ctx, sqlc.UpsertCatalogModelParams{
		ModelID:             m.ModelID,
		ProviderSlug:        m.ProviderSlug,
		Label:               m.Label,
		Vision:              boolToInt(m.Vision),
		StructuredOutput:    boolToInt(m.StructuredOutput),
		ContextTokens:       nullInt(m.ContextTokens),
		InputUsdPerMillion:  nullString(m.InputUSDPerMillion),
		OutputUsdPerMillion: nullString(m.OutputUSDPerMillion),
		PricingCheckedAt:    nullString(m.PricingCheckedAt),
		ReasoningEffort:     nullString(string(m.Reasoning)),
		Enabled:             boolToInt(m.Enabled),
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
	if patch.Enabled != nil {
		current.Enabled = *patch.Enabled
	}
	if patch.Reasoning != nil {
		current.Reasoning = *patch.Reasoning
	}
	current.UpdatedAt = updatedAt

	n, err := q.UpdateCatalogModelCuration(ctx, sqlc.UpdateCatalogModelCurationParams{
		Enabled:         boolToInt(current.Enabled),
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

func toModel(row sqlc.CatalogModel) (modelcatalog.Model, error) {
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
	return modelcatalog.Model{
		ModelID:             row.ModelID,
		ProviderSlug:        row.ProviderSlug,
		Label:               row.Label,
		Vision:              row.Vision == 1,
		StructuredOutput:    row.StructuredOutput == 1,
		ContextTokens:       row.ContextTokens.Int64,
		InputUSDPerMillion:  row.InputUsdPerMillion.String,
		OutputUSDPerMillion: row.OutputUsdPerMillion.String,
		PricingCheckedAt:    row.PricingCheckedAt.String,
		Reasoning:           reasoning,
		Enabled:             row.Enabled == 1,
		Listed:              row.Listed == 1,
		LastSeenAt:          lastSeen,
		CreatedAt:           created,
		UpdatedAt:           updated,
	}, nil
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
