package clip

import (
	"context"
	"time"
)

type BrowserUploadStore interface {
	BrowserRenderStore
	ReserveBrowserRenderUpload(context.Context, string, string, int64, time.Time) error
	CompleteBrowserRender(context.Context, string, string, time.Time) (string, error)
	CancelBrowserRender(context.Context, string, string, time.Time) (bool, error)
}

func (s *GenerationService) CancelBrowserRender(ctx context.Context, user, id string) (bool, error) {
	store, ok := s.store.(BrowserUploadStore)
	if !ok {
		return false, ErrRenderUnavailable
	}
	return store.CancelBrowserRender(ctx, user, id, s.now())
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
	// Encoding has no time ceiling. The orphan grace period belongs to the
	// stored object's modification time, not the admitted render's start.
	if err := store.ReserveBrowserRenderUpload(ctx, user, id, bytes, s.now()); err != nil {
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
	if r.CancelledAt != nil {
		return Project{}, ErrSourceState
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
	project, err := store.CompleteBrowserRender(ctx, user, id, s.now())
	if err != nil {
		return Project{}, err
	}
	return s.projects.GetProject(ctx, user, project)
}
