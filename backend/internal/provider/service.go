package provider

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Service is the catalog's use-cases.
type Service struct {
	store   Store
	catalog Catalog
	now     func() time.Time
}

// NewService wires the context.
func NewService(store Store, catalog Catalog) *Service {
	return &Service{store: store, catalog: catalog, now: time.Now}
}

// ListModels is the registry snapshot, flags included.
func (s *Service) ListModels() []llm.ModelInfo {
	return s.catalog.Models()
}

// GetSelections returns the user's per-stage choices. A choice whose model is no longer
// registered is reported `Missing` and cleared here (PRD §7: 마지막 선택 초기화), so the
// user sees the greyed entry once and then must choose again.
func (s *Service) GetSelections(ctx context.Context, userID string) ([]Selection, error) {
	selections, err := s.store.ListSelections(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list selections: %w", err)
	}
	for i := range selections {
		// A model that lost `vision` in the yaml is as gone for observe as one deleted:
		// the dropdown no longer lists it, so the choice is cleared the same way.
		if info, ok := s.catalog.Lookup(selections[i].Ref); ok && Suitable(selections[i].Stage, info) {
			continue
		}
		selections[i].Missing = true
		// Best effort: a failed clear only means the user is told again next time.
		if err := s.store.DeleteSelection(ctx, userID, selections[i]); err != nil {
			slog.Warn("clear vanished selection failed", "user", userID, "stage", selections[i].Stage, "err", err)
		}
	}
	return selections, nil
}

// SaveSelection records a choice. Only a registered, enabled model can be chosen — the
// same rule the dropdown shows, enforced where it can be trusted.
func (s *Service) SaveSelection(ctx context.Context, userID string, stage Stage, ref llm.ModelRef) (Selection, error) {
	if _, err := ParseStage(string(stage)); err != nil {
		return Selection{}, err
	}
	info, ok := s.catalog.Lookup(ref)
	if !ok {
		return Selection{}, fmt.Errorf("%w: %s", ErrModelNotRegistered, ref)
	}
	if info.Disabled {
		return Selection{}, fmt.Errorf("%w: %s (%s)", ErrModelDisabled, ref, info.DisabledReason)
	}
	if !Suitable(stage, info) {
		return Selection{}, fmt.Errorf("%w: %s has no vision, %s needs it", ErrModelUnsuitable, ref, stage)
	}
	selection := Selection{Stage: stage, Ref: ref, UpdatedAt: s.now()}
	if err := s.store.UpsertSelection(ctx, userID, selection); err != nil {
		return Selection{}, fmt.Errorf("save selection: %w", err)
	}
	return selection, nil
}
