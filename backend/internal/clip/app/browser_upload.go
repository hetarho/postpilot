package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) CancelBrowserRender(ctx context.Context, user, id string) (bool, error) {
	store, ok := s.store.(clip.BrowserUploadStore)
	if !ok {
		return false, clip.ErrRenderUnavailable
	}
	return store.CancelBrowserRender(ctx, user, id, s.now())
}

func (s *GenerationService) PrepareBrowserUpload(ctx context.Context, user, id string, bytes int64) (clip.SignedSourcePut, error) {
	store, ok := s.store.(clip.BrowserUploadStore)
	if !ok || s.sources == nil {
		return clip.SignedSourcePut{}, clip.ErrRenderUnavailable
	}
	if bytes <= 0 || bytes > s.sources.config.MaxFileBytes {
		return clip.SignedSourcePut{}, clip.ErrInvalid
	}
	r, err := store.GetBrowserRender(ctx, user, id)
	if err != nil {
		return clip.SignedSourcePut{}, err
	}
	// Encoding has no time ceiling. The orphan grace period belongs to the
	// stored object's modification time, not the admitted render's start.
	if err := store.ReserveBrowserRenderUpload(ctx, user, id, bytes, s.now()); err != nil {
		return clip.SignedSourcePut{}, err
	}
	return s.sources.objects.PresignSource(ctx, r.ResultKey(), "video/mp4", s.sources.config.PutTTL)
}

func (s *GenerationService) CompleteBrowserUpload(ctx context.Context, user, id string) (clip.Project, error) {
	store, ok := s.store.(clip.BrowserUploadStore)
	if !ok || s.sources == nil {
		return clip.Project{}, clip.ErrRenderUnavailable
	}
	r, err := store.GetBrowserRender(ctx, user, id)
	if err != nil {
		return clip.Project{}, err
	}
	if r.CancelledAt != nil {
		return clip.Project{}, clip.ErrSourceState
	}
	if r.StoredAt == nil {
		if r.UploadBytes <= 0 || r.Verdict == nil || !r.Verdict.Passed {
			return clip.Project{}, clip.ErrSourceState
		}
		info, err := s.sources.objects.HeadSource(ctx, r.ResultKey())
		if err != nil {
			return clip.Project{}, err
		}
		if info.Bytes != r.UploadBytes || info.ContentType != "video/mp4" {
			return clip.Project{}, clip.ErrInvalid
		}
	}
	project, err := store.CompleteBrowserRender(ctx, user, id, s.now())
	if err != nil {
		return clip.Project{}, err
	}
	return s.projects.GetProject(ctx, user, project)
}
