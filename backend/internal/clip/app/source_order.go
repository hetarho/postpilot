package app

import (
	"context"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// Reorder records the order the owner arranged the footage in (CLIP-136). It is
// the only way that order changes: nothing a template, a model or a render does
// reaches it, and the writer reads it as the order to follow when the project
// carries no instruction saying otherwise.
func (s *SourceService) Reorder(ctx context.Context, user, project, batch string, ids []string) (clip.SourceBatch, error) {
	if strings.TrimSpace(user) == "" || strings.TrimSpace(project) == "" || strings.TrimSpace(batch) == "" || len(ids) == 0 {
		return clip.SourceBatch{}, clip.ErrInvalid
	}
	store, ok := s.store.(clip.SourceOrderStore)
	if !ok {
		return clip.SourceBatch{}, clip.ErrSourceState
	}
	out, err := store.ReorderSources(ctx, user, project, batch, ids)
	if err != nil {
		return clip.SourceBatch{}, err
	}
	now := s.now()
	for i := range out.Sources {
		out.Sources[i].Availability = clip.SourceAvailability(out, out.Sources[i], now)
	}
	return out, nil
}
