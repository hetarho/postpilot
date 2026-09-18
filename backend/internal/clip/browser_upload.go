package clip

import (
	"context"
	"time"
)

type BrowserUploadStore interface {
	BrowserRenderStore
	ReserveBrowserRenderUpload(context.Context, string, string, int64, time.Time, time.Time) error
	CompleteBrowserRender(context.Context, string, string, time.Time, time.Time) (string, error)
}

func (s *GenerationService) PrepareBrowserUpload(ctx context.Context, user, id string, bytes int64) (SignedSourcePut, error) {
	store, ok := s.store.(BrowserUploadStore)
	if !ok || s.sources == nil {
		return SignedSourcePut{}, ErrRenderUnavailable
	}
	if bytes <= 0 || bytes > s.sources.config.MaxFileBytes {
		return SignedSourcePut{}, ErrInvalid
	}
	r, err := store.GetBrowserRender(ctx, user, id)
	if err != nil {
		return SignedSourcePut{}, err
	}
	// An orphan cannot be promoted after the sweeper is allowed to remove it.
	deadline := r.CreatedAt.Add(s.cfg.OrphanMinAge)
	now := s.now()
	if !now.Add(s.sources.config.PutTTL).Before(deadline) {
		return SignedSourcePut{}, ErrSourceState
	}
	if err := store.ReserveBrowserRenderUpload(ctx, user, id, bytes, now, deadline); err != nil {
		return SignedSourcePut{}, err
	}
	return s.sources.objects.PresignSource(ctx, r.ResultKey(), "video/mp4", s.sources.config.PutTTL)
}

func (s *GenerationService) CompleteBrowserUpload(ctx context.Context, user, id string) (Project, error) {
	store, ok := s.store.(BrowserUploadStore)
	if !ok || s.sources == nil {
		return Project{}, ErrRenderUnavailable
	}
	r, err := store.GetBrowserRender(ctx, user, id)
	if err != nil {
		return Project{}, err
	}
	if r.StoredAt == nil {
		if r.UploadBytes <= 0 || r.Verdict == nil || !r.Verdict.Passed {
			return Project{}, ErrSourceState
		}
		info, err := s.sources.objects.HeadSource(ctx, r.ResultKey())
		if err != nil {
			return Project{}, err
		}
		if info.Bytes != r.UploadBytes || info.ContentType != "video/mp4" {
			return Project{}, ErrInvalid
		}
	}
	project, err := store.CompleteBrowserRender(ctx, user, id, s.now(), r.CreatedAt.Add(s.cfg.OrphanMinAge))
	if err != nil {
		return Project{}, err
	}
	return s.projects.GetProject(ctx, user, project)
}
