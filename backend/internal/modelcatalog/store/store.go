// Package store persists the model catalog. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	byModel := map[string][]registration{}
	for _, row := range registrations {
		byModel[row.ModelID] = append(byModel[row.ModelID], registration{
			Purpose: row.Purpose, ReasoningEffort: row.ReasoningEffort,
		})
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

// registration is one join row as both read paths carry it: the purpose and the effort the
// operator set for THAT purpose.
type registration struct {
	Purpose         string
	ReasoningEffort sql.NullString
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
	registrations := make([]registration, 0, len(stored))
	for _, item := range stored {
		registrations = append(registrations, registration{
			Purpose: item.Purpose, ReasoningEffort: item.ReasoningEffort,
		})
	}
	return toModel(row, registrations)
}

func (s *Store) Upsert(ctx context.Context, m modelcatalog.Model) error {
	return upsert(ctx, s.write, m)
}

func upsert(ctx context.Context, q *sqlc.Queries, m modelcatalog.Model) error {
	err := q.UpsertCatalogModel(ctx, sqlc.UpsertCatalogModelParams{
		ModelID:                m.ModelID,
		ProviderSlug:           m.ProviderSlug,
		Label:                  m.Label,
		Vision:                 boolToInt(m.Vision),
		StructuredOutput:       boolToInt(m.StructuredOutput),
		ImageOutput:            boolToInt(m.ImageOutput),
		VideoOutput:            boolToInt(m.VideoOutput),
		Reasons:                boolToInt(m.Reasons),
		ReasoningEfforts:       joinEfforts(m.Efforts),
		ReasoningDefaultEffort: m.DefaultEffort,
		ReasoningMandatory:     boolToInt(m.Mandatory),
		ReasoningNativeEffort:  boolToInt(m.NativeEffort),
		ReasoningMaxTokens:     boolToInt(m.MaxTokens),
		ContextTokens:          nullInt(m.ContextTokens),
		InputUsdPerMillion:     nullString(m.InputUSDPerMillion),
		OutputUsdPerMillion:    nullString(m.OutputUSDPerMillion),
		PricingCheckedAt:       nullString(m.PricingCheckedAt),
		Listed:                 boolToInt(m.Listed),
		LastSeenAt:             nullTime(m.LastSeenAt),
		CreatedAt:              formatTime(m.CreatedAt),
		UpdatedAt:              formatTime(m.UpdatedAt),
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
		// An empty effort CLEARS the override rather than storing one: the column goes NULL
		// and the map loses the key, so "no override" is one state in memory and on disk
		// instead of two that read the same only by accident.
		if *patch.Reasoning == llm.ReasoningUnspecified {
			delete(current.Reasoning, patch.Purpose)
		} else {
			if current.Reasoning == nil {
				current.Reasoning = map[modelcatalog.Purpose]llm.ReasoningEffort{}
			}
			current.Reasoning[patch.Purpose] = *patch.Reasoning
		}
	}
	current.UpdatedAt = updatedAt

	// The UPDATE matching zero rows is the refusal: the join row exists only for a purpose
	// this model is registered to, so an effort cannot be stored where it would never be read.
	n, err := q.UpdateCatalogModelPurposeReasoning(ctx, sqlc.UpdateCatalogModelPurposeReasoningParams{
		ReasoningEffort: nullString(string(current.Reasoning[patch.Purpose])),
		ModelID:         modelID,
		Purpose:         string(patch.Purpose),
	})
	if err != nil {
		return modelcatalog.Model{}, fmt.Errorf("update catalog model purpose reasoning: %w", err)
	}
	if n == 0 {
		return modelcatalog.Model{}, fmt.Errorf("%w: %s for %s", modelcatalog.ErrPurposeNotRegistered, modelID, patch.Purpose)
	}
	// A curation edit stamps the row, the same way a (de)registration does.
	if err := q.TouchCatalogModelCuration(ctx, sqlc.TouchCatalogModelCurationParams{
		UpdatedAt: formatTime(current.UpdatedAt), ModelID: modelID,
	}); err != nil {
		return modelcatalog.Model{}, fmt.Errorf("touch catalog model: %w", err)
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
			ProviderSlug:           candidate.ProviderSlug,
			Label:                  candidate.Label,
			Vision:                 boolToInt(candidate.Vision),
			StructuredOutput:       boolToInt(candidate.StructuredOutput),
			ImageOutput:            boolToInt(candidate.ImageOutput),
			VideoOutput:            boolToInt(candidate.VideoOutput),
			Reasons:                boolToInt(candidate.Reasons),
			ReasoningEfforts:       joinEfforts(candidate.Efforts),
			ReasoningDefaultEffort: candidate.DefaultEffort,
			ReasoningMandatory:     boolToInt(candidate.Mandatory),
			ReasoningNativeEffort:  boolToInt(candidate.NativeEffort),
			ReasoningMaxTokens:     boolToInt(candidate.MaxTokens),
			ContextTokens:          nullInt(candidate.ContextTokens),
			InputUsdPerMillion:     nullString(candidate.InputUSDPerMillion),
			OutputUsdPerMillion:    nullString(candidate.OutputUSDPerMillion),
			PricingCheckedAt:       nullString(pricingDate),
			LastSeenAt:             nullString(stamp),
			ModelID:                candidate.ModelID,
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

func toModel(row sqlc.GetCatalogModelRow, registrations []registration) (modelcatalog.Model, error) {
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
	purposes, reasoning, err := parseRegistrations(row.ModelID, registrations)
	if err != nil {
		return modelcatalog.Model{}, err
	}
	return modelcatalog.Model{
		ModelID:          row.ModelID,
		ProviderSlug:     row.ProviderSlug,
		Label:            row.Label,
		Vision:           row.Vision == 1,
		StructuredOutput: row.StructuredOutput == 1,
		ImageOutput:      row.ImageOutput == 1,
		VideoOutput:      row.VideoOutput == 1,
		ReasoningCapability: modelcatalog.ReasoningCapability{
			Reasons:       row.Reasons == 1,
			Efforts:       splitEfforts(row.ReasoningEfforts),
			DefaultEffort: row.ReasoningDefaultEffort,
			Mandatory:     row.ReasoningMandatory == 1,
			NativeEffort:  row.ReasoningNativeEffort == 1,
			MaxTokens:     row.ReasoningMaxTokens == 1,
		},
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

// parseRegistrations maps the join rows into the domain's two halves: which purposes the
// model serves, and the effort the operator set for each. Both column CHECKs refuse anything
// else, so a failure means the row was written by something other than this code.
func parseRegistrations(modelID string, rows []registration) ([]modelcatalog.Purpose, map[modelcatalog.Purpose]llm.ReasoningEffort, error) {
	purposes := make([]modelcatalog.Purpose, 0, len(rows))
	var reasoning map[modelcatalog.Purpose]llm.ReasoningEffort
	for _, row := range rows {
		purpose, err := modelcatalog.ParsePurpose(row.Purpose)
		if err != nil {
			return nil, nil, fmt.Errorf("catalog model %s: %w", modelID, err)
		}
		purposes = append(purposes, purpose)
		if !row.ReasoningEffort.Valid {
			continue
		}
		effort := llm.ReasoningEffort(row.ReasoningEffort.String)
		if !effort.Valid() {
			return nil, nil, fmt.Errorf("catalog model %s %s: unknown reasoning_effort %q", modelID, purpose, row.ReasoningEffort.String)
		}
		if reasoning == nil {
			reasoning = map[modelcatalog.Purpose]llm.ReasoningEffort{}
		}
		reasoning[purpose] = effort
	}
	// The join rows arrive in column order; the domain order is the display order.
	modelcatalog.SortPurposes(purposes)
	return purposes, reasoning, nil
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

// joinEfforts / splitEfforts are the whole storage form of the effort list: read whole,
// written whole, never joined and never queried by element (see the query file's header).
//
// JSON rather than a delimiter, because the column's entire claim is that it holds what the
// source published VERBATIM: a delimiter would silently split any future value containing it,
// and the admin would then offer a value the source never listed. The order survives, because
// that order is the order a selector offers the values in.
//
// An empty list is the empty string, not "[]", so the column's DEFAULT ” already reads as
// UNKNOWN for every row written before this data existed.
func joinEfforts(efforts []string) string {
	if len(efforts) == 0 {
		return ""
	}
	encoded, err := json.Marshal(efforts)
	if err != nil {
		// []string cannot fail to marshal; a failure here would be a corrupted heap.
		slog.Error("encode reasoning efforts failed", "err", err)
		return ""
	}
	return string(encoded)
}

// splitEfforts is deliberately forgiving: an unparseable value reads as UNKNOWN rather than
// failing the whole catalog read, which would take the admin screen down over one column.
func splitEfforts(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var efforts []string
	if err := json.Unmarshal([]byte(value), &efforts); err != nil {
		slog.Warn("unreadable reasoning efforts column", "value", value, "err", err)
		return nil
	}
	return efforts
}
