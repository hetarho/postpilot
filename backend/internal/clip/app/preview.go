package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) PreparePreview(ctx context.Context, user, id string, revision int, hash string, draft clip.CorrectionPlan, ids []string, offset int) (clip.PreparedPreview, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	if p.Finalized != nil {
		return clip.PreparedPreview{}, clip.ErrFinalized
	}
	if revision <= 0 || p.EditPlanRevision != revision {
		return clip.PreparedPreview{}, clip.ErrPlanConflict
	}
	cfg := s.cfg.Preview
	renderer, ok := s.renderer.(clip.PreviewPreparer)
	if !ok || cfg.Timeout <= 0 || cfg.MaxAssets <= 0 {
		return clip.PreparedPreview{}, clip.ErrPreviewUnavailable
	}
	if offset < 0 || len(ids) > cfg.MaxAssets {
		return clip.PreparedPreview{}, clip.ErrPreviewTooLarge
	}
	// The owner lock covers measurement as well as rasterization. No waiter can
	// build an unbounded queue, and every error/cancellation releases admission.
	if _, loaded := s.previewOwners.LoadOrStore(user, struct{}{}); loaded {
		return clip.PreparedPreview{}, clip.ErrPreviewBusy
	}
	defer s.previewOwners.Delete(user)
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	next, err := clip.ApplyCorrection(s.cfg.Render, p, draft)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	// The pace and the accent are render inputs read from the PROJECT, the way
	// the disclosure and the facts are (CLIP-139).
	next = next.WithDesign(p.DesignSelection())
	if next.Portable == nil {
		recipe := clip.Recipe{}
		if p.Composition != nil && p.Composition.Snapshot.LegacyRecipe != nil {
			recipe = *p.Composition.Snapshot.LegacyRecipe
		}
		next.Portable, err = clip.FreezeLegacyPlan(p, next, recipe, s.cfg.Render.Composition)
		if err != nil {
			return clip.PreparedPreview{}, err
		}
	}
	sources, err := clip.RetainedSources(p)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	refs := make([]clip.RenderSource, 0, len(sources))
	for _, v := range sources {
		refs = append(refs, v.RenderSource)
	}
	out, err := renderer.PreparePreview(ctx, next, refs, ids, offset, cfg)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	// A concurrent save invalidates this preparation; it never updates the saved
	// draft, retention, result, job, observations or accounting.
	current, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	if current.Finalized != nil {
		return clip.PreparedPreview{}, clip.ErrFinalized
	}
	if current.EditPlanRevision != revision {
		return clip.PreparedPreview{}, clip.ErrPlanConflict
	}
	out.DraftHash = hash
	return out, nil
}

func (s *GenerationService) PreviewResponseLimit() int { return s.cfg.Preview.MaxResponseBytes }
