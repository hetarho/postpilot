package app

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
)

func (s *SourceService) ConfirmImport(ctx context.Context, user, batch, source string) (clip.SourceBatch, error) {
	port, ok := s.store.(clip.SourceImportStore)
	if !ok {
		return clip.SourceBatch{}, clip.ErrSourceState
	}
	b, err := s.store.GetSourceBatch(ctx, user, batch)
	if err != nil {
		return b, err
	}
	if _, err = port.ImportSourceBatch(ctx, user, b.ProjectID, batch, s.now()); err != nil {
		return clip.SourceBatch{}, err
	}
	return s.confirm(ctx, user, batch, source, true)
}
func (s *SourceService) CompleteImport(ctx context.Context, user, project, batch string) (*clip.SourceBatch, error) {
	port, ok := s.store.(clip.SourceImportStore)
	if !ok {
		return nil, clip.ErrSourceState
	}
	result, err := port.CompleteSourceImport(ctx, user, project, batch, clip.CompletedSourceImportID(user, project, batch), s.now())
	if err != nil {
		return nil, err
	}
	// Only pending leases remain in the original batch after a partial completion.
	// The existing cleanup path owns expiry and late PUT handling.
	if result == nil || result.ID != batch {
		old, e := s.store.GetSourceBatch(ctx, user, batch)
		if e == nil && old.State == "cleanup_pending" {
			s.cleanupBestEffort(ctx, old)
		}
	}
	if result != nil {
		found, e := s.store.GetSourceBatch(ctx, user, result.ID)
		if e != nil {
			return nil, e
		}
		result = &found
	}
	return result, nil
}
