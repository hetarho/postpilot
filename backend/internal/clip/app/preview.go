package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) PreparePreview(ctx context.Context, user, id string, revision int, hash string, draft clip.CorrectionPlan, ids []string, offset int) (clip.PreparedPreview, error) {
	return s.preparePreview(ctx, user, "", id, revision, hash, draft, ids, offset)
}

// PrepareRenderPreview draws a browser render's assets with the grounds its
// sampling job kept (CLIP-192), admitted as a draft preview is plus the render.
func (s *GenerationService) PrepareRenderPreview(ctx context.Context, user, render, id string, revision int, hash string, draft clip.CorrectionPlan, ids []string, offset int) (clip.PreparedPreview, error) {
	if render == "" {
		return clip.PreparedPreview{}, clip.ErrNotFound
	}
	return s.preparePreview(ctx, user, render, id, revision, hash, draft, ids, offset)
}

func (s *GenerationService) preparePreview(ctx context.Context, user, render, id string, revision int, hash string, draft clip.CorrectionPlan, ids []string, offset int) (clip.PreparedPreview, error) {
	p, err := s.admitPreview(ctx, user, id, revision)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	grounds, err := s.renderGrounds(ctx, user, render, id, revision)
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	cfg := s.cfg.Preview
	renderer, ok := s.renderer.(clip.PreviewPreparer)
	grounded, groundsDrawn := s.renderer.(clip.GroundedPreviewPreparer)
	if !ok || render != "" && !groundsDrawn || cfg.Timeout <= 0 || cfg.MaxAssets <= 0 {
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
	var out clip.PreparedPreview
	if render != "" {
		out, err = grounded.PrepareGroundedPreview(ctx, next, refs, grounds, ids, offset, cfg)
	} else {
		out, err = renderer.PreparePreview(ctx, next, refs, ids, offset, cfg)
	}
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	if _, err := s.admitPreview(ctx, user, id, revision); err != nil {
		return clip.PreparedPreview{}, err
	}
	out.DraftHash = hash
	return out, nil
}

// renderGrounds is what a browser render's assets are drawn on: the grounds its
// own sampling job kept, for the owner's live render of this project at this
// revision. No render asks for none, which is the editing preview's no ground.
func (s *GenerationService) renderGrounds(ctx context.Context, user, render, id string, revision int) ([]clip.SampledGround, error) {
	if render == "" {
		return nil, nil
	}
	store, ok := s.store.(clip.BrowserRenderStore)
	if !ok {
		return nil, clip.ErrRenderUnavailable
	}
	r, err := store.GetBrowserRender(ctx, user, render)
	if err != nil {
		return nil, err
	}
	switch {
	case r.ProjectID != id || r.CancelledAt != nil:
		return nil, clip.ErrNotFound
	case r.Revision != revision:
		return nil, clip.ErrPlanConflict
	case r.SampledAt == nil:
		return nil, clip.ErrRenderNotSampled
	}
	if r.Grounds == nil {
		return []clip.SampledGround{}, nil
	}
	return r.Grounds, nil
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
	// A draft is drawn from the composition it was written into; a plan
	// with none cannot be drawn.
	if next.Portable == nil {
		return clip.EditPlan{}, nil, clip.ErrCompositionUnavailable
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
	return s.prepareCaptionFrames(ctx, user, "", id, revision, hash, draft, instanceID, offset)
}

// PrepareRenderCaptionFrames is the same run for a browser render, laid out on
// the grounds its sampling job kept (CLIP-192).
func (s *GenerationService) PrepareRenderCaptionFrames(ctx context.Context, user, render, id string, revision int, hash string, draft clip.CorrectionPlan, instanceID string, offset int) (clip.CaptionFrames, error) {
	if render == "" {
		return clip.CaptionFrames{}, clip.ErrNotFound
	}
	return s.prepareCaptionFrames(ctx, user, render, id, revision, hash, draft, instanceID, offset)
}

func (s *GenerationService) prepareCaptionFrames(ctx context.Context, user, render, id string, revision int, hash string, draft clip.CorrectionPlan, instanceID string, offset int) (clip.CaptionFrames, error) {
	p, err := s.admitPreview(ctx, user, id, revision)
	if err != nil {
		return clip.CaptionFrames{}, err
	}
	grounds, err := s.renderGrounds(ctx, user, render, id, revision)
	if err != nil {
		return clip.CaptionFrames{}, err
	}
	cfg := s.cfg.Preview
	renderer, ok := s.renderer.(clip.CaptionFramePreparer)
	grounded, groundsDrawn := s.renderer.(clip.GroundedPreviewPreparer)
	if !ok || render != "" && !groundsDrawn || cfg.FrameTimeout <= 0 || cfg.MaxFrameCells <= 0 {
		return clip.CaptionFrames{}, clip.ErrPreviewUnavailable
	}
	if _, loaded := s.previewOwners.LoadOrStore(user, struct{}{}); loaded {
		return clip.CaptionFrames{}, clip.ErrPreviewBusy
	}
	defer s.previewOwners.Delete(user)
	ctx, cancel := context.WithTimeout(ctx, cfg.FrameTimeout)
	defer cancel()
	next, refs, err := s.correctedPlan(p, draft)
	if err != nil {
		return clip.CaptionFrames{}, err
	}
	var out clip.CaptionFrames
	if render != "" {
		out, err = grounded.PrepareGroundedCaptionFrames(ctx, next, refs, grounds, instanceID, offset, cfg)
	} else {
		out, err = renderer.PrepareCaptionFrames(ctx, next, refs, instanceID, offset, cfg)
	}
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
