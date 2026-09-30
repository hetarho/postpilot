package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/provider/store/sqlc"
)

// ListRecommendationSets reads every set with its slots, in the operator's order.
func (s *Store) ListRecommendationSets(ctx context.Context) ([]provider.RecommendationSet, error) {
	return listRecommendationSets(ctx, s.read)
}

func listRecommendationSets(ctx context.Context, queries *sqlc.Queries) ([]provider.RecommendationSet, error) {
	rows, err := queries.ListRecommendationSets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recommendation sets: %w", err)
	}
	slots, err := queries.ListRecommendationSetSlots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recommendation set slots: %w", err)
	}
	bySet := map[string]map[provider.Stage]*provider.RecommendationStageSelection{}
	for _, row := range slots {
		stage, err := provider.ParseStage(row.Stage)
		if err != nil {
			// A stage this binary does not know is skipped, as model_selections does.
			continue
		}
		stages := bySet[row.SetID]
		if stages == nil {
			stages = map[provider.Stage]*provider.RecommendationStageSelection{}
			bySet[row.SetID] = stages
		}
		selection := stages[stage]
		if selection == nil {
			selection = &provider.RecommendationStageSelection{Stage: stage}
			stages[stage] = selection
		}
		ref := llm.ModelRef{ProviderID: row.ProviderID, ModelID: row.ModelID}
		switch provider.SelectionSlot(row.Slot) {
		case provider.SlotActive:
			selection.Active = ref
		case provider.SlotCandidateA:
			selection.CandidateA = ref
		case provider.SlotCandidateB:
			selection.CandidateB = ref
		}
	}
	out := make([]provider.RecommendationSet, 0, len(rows))
	for _, row := range rows {
		set := provider.RecommendationSet{ID: row.ID, Label: row.Label}
		// Stage order is the product's display order, not the table's alphabetical one.
		for _, stage := range []provider.Stage{provider.StageObserve, provider.StageAnalyze, provider.StageWrite} {
			if selection := bySet[row.ID][stage]; selection != nil {
				set.Selections = append(set.Selections, *selection)
			}
		}
		out = append(out, set)
	}
	return out, nil
}

func (s *Store) CreateRecommendationSet(ctx context.Context, set provider.RecommendationSet, limit int, at time.Time) error {
	return s.inTx(ctx, func(queries *sqlc.Queries) error {
		count, err := queries.CountRecommendationSets(ctx)
		if err != nil {
			return fmt.Errorf("count recommendation sets: %w", err)
		}
		if count >= int64(limit) {
			return provider.ErrRecommendationLimit
		}
		position, err := queries.NextRecommendationSetPosition(ctx)
		if err != nil {
			return fmt.Errorf("next recommendation set position: %w", err)
		}
		stamp := at.UTC().Format(writeLayout)
		if err := queries.InsertRecommendationSet(ctx, sqlc.InsertRecommendationSetParams{
			ID: set.ID, Label: set.Label, Position: position, CreatedAt: stamp, UpdatedAt: stamp,
		}); err != nil {
			return fmt.Errorf("insert recommendation set: %w", err)
		}
		return insertRecommendationSlots(ctx, queries, set)
	})
}

func (s *Store) ReplaceRecommendationSet(ctx context.Context, set provider.RecommendationSet, at time.Time) error {
	return s.inTx(ctx, func(queries *sqlc.Queries) error {
		updated, err := queries.UpdateRecommendationSetLabel(ctx, sqlc.UpdateRecommendationSetLabelParams{
			Label: set.Label, UpdatedAt: at.UTC().Format(writeLayout), ID: set.ID,
		})
		if err != nil {
			return fmt.Errorf("update recommendation set: %w", err)
		}
		if updated == 0 {
			return provider.ErrRecommendationNotFound
		}
		if err := queries.DeleteRecommendationSetSlots(ctx, set.ID); err != nil {
			return fmt.Errorf("clear recommendation set slots: %w", err)
		}
		return insertRecommendationSlots(ctx, queries, set)
	})
}

func (s *Store) DeleteRecommendationSet(ctx context.Context, id string) error {
	// The slots go with the set by ON DELETE CASCADE.
	deleted, err := s.write.DeleteRecommendationSet(ctx, id)
	if err != nil {
		return fmt.Errorf("delete recommendation set: %w", err)
	}
	if deleted == 0 {
		return provider.ErrRecommendationNotFound
	}
	return nil
}

// MoveRecommendationSet rewrites every position as 1..n in the new order. There are at most
// ten sets, and renumbering the whole list means a tie the app never wrote cannot turn a
// swap into a no-op.
func (s *Store) MoveRecommendationSet(ctx context.Context, id string, earlier bool) error {
	return s.inTx(ctx, func(queries *sqlc.Queries) error {
		rows, err := queries.ListRecommendationSets(ctx)
		if err != nil {
			return fmt.Errorf("list recommendation sets: %w", err)
		}
		index := -1
		for i, row := range rows {
			if row.ID == id {
				index = i
				break
			}
		}
		if index < 0 {
			return provider.ErrRecommendationNotFound
		}
		target := index + 1
		if earlier {
			target = index - 1
		}
		if target < 0 || target >= len(rows) {
			return nil
		}
		rows[index], rows[target] = rows[target], rows[index]
		for i, row := range rows {
			if err := queries.SetRecommendationSetPosition(ctx, sqlc.SetRecommendationSetPositionParams{
				Position: int64(i + 1), ID: row.ID,
			}); err != nil {
				return fmt.Errorf("reorder recommendation sets: %w", err)
			}
		}
		return nil
	})
}

func insertRecommendationSlots(ctx context.Context, queries *sqlc.Queries, set provider.RecommendationSet) error {
	for _, selection := range set.Selections {
		for _, entry := range selection.Slots() {
			if err := queries.InsertRecommendationSetSlot(ctx, sqlc.InsertRecommendationSetSlotParams{
				SetID: set.ID, Stage: string(selection.Stage), Slot: string(entry.Slot),
				ProviderID: entry.Ref.ProviderID, ModelID: entry.Ref.ModelID,
			}); err != nil {
				return fmt.Errorf("insert recommendation set slot %s/%s: %w", selection.Stage, entry.Slot, err)
			}
		}
	}
	return nil
}

// TxStore is the recommendation-set store bound to a caller's transaction. The models
// document writes registrations and the set list together (MODEL-73), and a use-case that
// commits across two contexts' tables runs over transaction-scoped ports (ARCH-6), so the
// composition root hands the catalog this view of the caller's transaction.
type TxStore struct{ queries *sqlc.Queries }

// NewTx binds the recommendation-set queries to tx.
func NewTx(tx *sql.Tx) *TxStore { return &TxStore{queries: sqlc.New(tx)} }

// ListRecommendationSets reads every set inside the transaction, in the operator's order.
func (s *TxStore) ListRecommendationSets(ctx context.Context) ([]provider.RecommendationSet, error) {
	return listRecommendationSets(ctx, s.queries)
}

// ReplaceRecommendationSets makes the stored sets exactly `sets`, in their order: a listed id
// that is stored keeps its row and creation time with the label, slots and position
// rewritten, a stored set the list does not name is deleted, and a listed id that is not
// stored is inserted. Nothing here reaches model_selections (MODEL-71).
func (s *TxStore) ReplaceRecommendationSets(ctx context.Context, sets []provider.RecommendationSet, at time.Time) error {
	rows, err := s.queries.ListRecommendationSets(ctx)
	if err != nil {
		return fmt.Errorf("list recommendation sets: %w", err)
	}
	listed := make(map[string]bool, len(sets))
	for _, set := range sets {
		listed[set.ID] = true
	}
	stored := make(map[string]bool, len(rows))
	for _, row := range rows {
		stored[row.ID] = true
		if listed[row.ID] {
			continue
		}
		if _, err := s.queries.DeleteRecommendationSet(ctx, row.ID); err != nil {
			return fmt.Errorf("delete recommendation set: %w", err)
		}
	}
	stamp := at.UTC().Format(writeLayout)
	for index, set := range sets {
		position := int64(index + 1)
		if stored[set.ID] {
			if _, err := s.queries.UpdateRecommendationSetLabel(ctx, sqlc.UpdateRecommendationSetLabelParams{
				Label: set.Label, UpdatedAt: stamp, ID: set.ID,
			}); err != nil {
				return fmt.Errorf("update recommendation set: %w", err)
			}
			if err := s.queries.SetRecommendationSetPosition(ctx, sqlc.SetRecommendationSetPositionParams{
				Position: position, ID: set.ID,
			}); err != nil {
				return fmt.Errorf("reorder recommendation set: %w", err)
			}
			if err := s.queries.DeleteRecommendationSetSlots(ctx, set.ID); err != nil {
				return fmt.Errorf("clear recommendation set slots: %w", err)
			}
		} else if err := s.queries.InsertRecommendationSet(ctx, sqlc.InsertRecommendationSetParams{
			ID: set.ID, Label: set.Label, Position: position, CreatedAt: stamp, UpdatedAt: stamp,
		}); err != nil {
			return fmt.Errorf("insert recommendation set: %w", err)
		}
		if err := insertRecommendationSlots(ctx, s.queries, set); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) inTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()
	if err := fn(sqlc.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
