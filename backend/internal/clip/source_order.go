package clip

import (
	"context"
	"strings"
)

// SourceOrderStore is the one write this action needs.
type SourceOrderStore interface {
	ReorderSources(ctx context.Context, user, project, batch string, ids []string) (SourceBatch, error)
}

// Reorder records the order the owner arranged the footage in (CLIP-136). It is
// the only way that order changes: nothing a template, a model or a render does
// reaches it, and the writer reads it as the order to follow when the project
// carries no instruction saying otherwise.
func (s *SourceService) Reorder(ctx context.Context, user, project, batch string, ids []string) (SourceBatch, error) {
	if strings.TrimSpace(user) == "" || strings.TrimSpace(project) == "" || strings.TrimSpace(batch) == "" || len(ids) == 0 {
		return SourceBatch{}, ErrInvalid
	}
	store, ok := s.store.(SourceOrderStore)
	if !ok {
		return SourceBatch{}, ErrSourceState
	}
	out, err := store.ReorderSources(ctx, user, project, batch, ids)
	if err != nil {
		return SourceBatch{}, err
	}
	now := s.now()
	for i := range out.Sources {
		out.Sources[i].Availability = SourceAvailability(out, out.Sources[i], now)
	}
	return out, nil
}
