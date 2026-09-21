package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) PreparePreview(ctx context.Context, user, id string, revision int, hash string, draft clip.CorrectionPlan, ids []string, offset int) (clip.PreparedPreview, error) {
	p, err := s.admitPreview(ctx, user, id, revision)
	if err != nil {
		return clip.PreparedPreview{}, err
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
	next, refs, err := s.correctedPlan(p, draft)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	out, err := renderer.PreparePreview(ctx, next, refs, ids, offset, cfg)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	if _, err := s.admitPreview(ctx, user, id, revision); err != nil {
		return clip.PreparedPreview{}, err
	}
	out.DraftHash = hash
	return out, nil
}

// admitPreview is what a read of a draft is allowed against: the owner's own
// unfinalized project at exactly the revision the caller was looking at. It runs
// before the work and again after it, because a concurrent save invalidates a
// preparation — and it updates no draft, retention, result, job, observation or
// accounting either time.
func (s *GenerationService) admitPreview(ctx context.Context, user, id string, revision int) (clip.Project, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.Project{}, err
	}
	if p.Finalized != nil {
		return clip.Project{}, clip.ErrFinalized
	}
	if revision <= 0 || p.EditPlanRevision != revision {
		return clip.Project{}, clip.ErrPlanConflict
	}
	return p, nil
}

// correctedPlan is the plan a draft read draws: the owner's correction applied
// to the saved project, carrying the design the PROJECT holds — the pace and the
// accent are render inputs read from there, the way the disclosure and the facts
// are (CLIP-139) — and the sources it cites.
func (s *GenerationService) correctedPlan(p clip.Project, draft clip.CorrectionPlan) (clip.EditPlan, []clip.RenderSource, error) {
	next, err := clip.ApplyCorrection(s.cfg.Render, p, draft)
	if err != nil {
		return clip.EditPlan{}, nil, err
	}
	next = next.WithDesign(p.DesignSelection())
	if next.Portable == nil {
		recipe := clip.Recipe{}
		if p.Composition != nil && p.Composition.Snapshot.LegacyRecipe != nil {
			recipe = *p.Composition.Snapshot.LegacyRecipe
		}
		next.Portable, err = clip.FreezeLegacyPlan(p, next, recipe, s.cfg.Render.Composition)
		if err != nil {
			return clip.EditPlan{}, nil, err
		}
	}
	sources, err := clip.RetainedSources(p)
	if err != nil {
		return clip.EditPlan{}, nil, err
	}
	refs := make([]clip.RenderSource, 0, len(sources))
	for _, v := range sources {
		refs = append(refs, v.RenderSource)
	}
	return next, refs, nil
}

// PrepareCaptionFrames serves one run of a sequence-rendered caption's own
// frames to a browser render (CLIP-159). It is admitted exactly as a draft
// preview is, spends no credit and changes nothing.
func (s *GenerationService) PrepareCaptionFrames(ctx context.Context, user, id string, revision int, hash string, draft clip.CorrectionPlan, instanceID string, offset int) (clip.CaptionFrames, error) {
	p, err := s.admitPreview(ctx, user, id, revision)
	if err != nil {
		return clip.CaptionFrames{}, err
	}
	cfg := s.cfg.Preview
	renderer, ok := s.renderer.(clip.CaptionFramePreparer)
	if !ok || cfg.Timeout <= 0 || cfg.MaxFrameCells <= 0 {
		return clip.CaptionFrames{}, clip.ErrPreviewUnavailable
	}
	if _, loaded := s.previewOwners.LoadOrStore(user, struct{}{}); loaded {
		return clip.CaptionFrames{}, clip.ErrPreviewBusy
	}
	defer s.previewOwners.Delete(user)
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	next, refs, err := s.correctedPlan(p, draft)
	if err != nil {
		return clip.CaptionFrames{}, err
	}
	out, err := renderer.PrepareCaptionFrames(ctx, next, refs, instanceID, offset, cfg)
	if err != nil {
		return clip.CaptionFrames{}, err
	}
	if _, err := s.admitPreview(ctx, user, id, revision); err != nil {
		return clip.CaptionFrames{}, err
	}
	out.DraftHash = hash
	return out, nil
}

func (s *GenerationService) PreviewResponseLimit() int { return s.cfg.Preview.MaxResponseBytes }
