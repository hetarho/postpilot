// Package store persists the provider context (model_selections). sqlc rows stop here;
// only domain types travel inward (ARCHITECTURE §2.2).
package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/provider/store/sqlc"
)

// writeLayout matches the other contexts: fixed-width RFC3339 UTC.
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// Store implements provider.Store over SQLite.
type Store struct {
	writer *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

// New builds the store over the process's writer and reader pools.
func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

func (s *Store) UpsertSelection(ctx context.Context, userID string, sel provider.Selection) error {
	if sel.Slot == "" {
		sel.Slot = provider.SlotActive
	}
	if sel.Slot != provider.SlotActive {
		return s.upsertSlot(ctx, s.write, userID, sel)
	}
	err := s.write.UpsertSelection(ctx, sqlc.UpsertSelectionParams{
		UserID:     userID,
		Stage:      string(sel.Stage),
		ProviderID: sel.Ref.ProviderID,
		ModelID:    sel.Ref.ModelID,
		UpdatedAt:  sel.UpdatedAt.UTC().Format(writeLayout),
	})
	if err != nil {
		return fmt.Errorf("upsert selection: %w", err)
	}
	return nil
}

func (s *Store) upsertSlot(ctx context.Context, queries *sqlc.Queries, userID string, sel provider.Selection) error {
	err := queries.UpsertSelectionSlot(ctx, sqlc.UpsertSelectionSlotParams{
		UserID: userID, Stage: string(sel.Stage), Slot: string(sel.Slot),
		ProviderID: sel.Ref.ProviderID, ModelID: sel.Ref.ModelID,
		UpdatedAt: sel.UpdatedAt.UTC().Format(writeLayout),
	})
	if err != nil {
		return fmt.Errorf("upsert selection slot: %w", err)
	}
	return nil
}

func (s *Store) ListSelections(ctx context.Context, userID string) ([]provider.Selection, error) {
	rows, err := s.read.ListSelections(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list selections: %w", err)
	}
	out := make([]provider.Selection, 0, len(rows))
	for _, row := range rows {
		stage, err := provider.ParseStage(row.Stage)
		if err != nil {
			// A stage this binary does not know (a downgrade after a newer one wrote it)
			// is not the user's problem; it is skipped, not fatal.
			continue
		}
		// RFC3339Nano reads any fraction width, so a row written by hand or by an older
		// layout still loads; a value that is not a time at all is reported, not zeroed
		// in silence.
		updated, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
		if err != nil {
			slog.Warn("model_selections: unreadable updated_at", "user", userID, "stage", row.Stage, "value", row.UpdatedAt)
		}
		out = append(out, provider.Selection{
			Stage:     stage,
			Slot:      provider.SlotActive,
			Ref:       llm.ModelRef{ProviderID: row.ProviderID, ModelID: row.ModelID},
			UpdatedAt: updated,
		})
	}
	return out, nil
}

func (s *Store) ListSelectionSlots(ctx context.Context, userID string) ([]provider.Selection, error) {
	rows, err := s.read.ListSelectionSlots(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list selection slots: %w", err)
	}
	out := make([]provider.Selection, 0, len(rows))
	for _, row := range rows {
		stage, err := provider.ParseStage(row.Stage)
		if err != nil {
			continue
		}
		updated, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("selection %s/%s updated_at: %w", row.Stage, row.Slot, err)
		}
		out = append(out, provider.Selection{
			Stage: stage, Slot: provider.SelectionSlot(row.Slot),
			Ref: llm.ModelRef{ProviderID: row.ProviderID, ModelID: row.ModelID}, UpdatedAt: updated,
		})
	}
	return out, nil
}

func (s *Store) SaveSelections(ctx context.Context, userID string, selections []provider.Selection) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin selections: %w", err)
	}
	defer tx.Rollback()
	queries := sqlc.New(tx)
	for _, selection := range selections {
		if err := s.upsertSlot(ctx, queries, userID, selection); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit selections: %w", err)
	}
	return nil
}

func (s *Store) DeleteSelection(ctx context.Context, userID string, sel provider.Selection) error {
	if sel.Slot == "" {
		sel.Slot = provider.SlotActive
	}
	err := s.write.DeleteSelectionIfRef(ctx, sqlc.DeleteSelectionIfRefParams{
		UserID:     userID,
		Stage:      string(sel.Stage),
		Slot:       string(sel.Slot),
		ProviderID: sel.Ref.ProviderID,
		ModelID:    sel.Ref.ModelID,
	})
	if err != nil {
		return fmt.Errorf("delete selection: %w", err)
	}
	return nil
}
