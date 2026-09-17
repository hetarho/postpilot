package clip

import "context"

// CaptionPreviewOf hands ② every caption of the plan it is editing, drawn by the
// one place the style set lives, so the preview and the render cannot drift
// (CDS-83). It takes the draft rather than the saved plan because the editor
// must show the caption it is editing, not the one it last saved — the same
// contract PreparePreview already works on.
//
// It shares the preview's owner lock and timeout: this measures with resvg
// exactly as a preparation does, and one owner may have one measurement running.
func (s *GenerationService) CaptionPreviewOf(ctx context.Context, user, id string, revision int, draft CorrectionPlan) (CaptionPreview, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return CaptionPreview{}, err
	}
	if p.Finalized != nil {
		return CaptionPreview{}, ErrFinalized
	}
	if revision <= 0 || p.EditPlanRevision != revision {
		return CaptionPreview{}, ErrPlanConflict
	}
	cfg := s.cfg.Preview
	fragmenter, ok := s.renderer.(CaptionFragmenter)
	if !ok || cfg.Timeout <= 0 {
		return CaptionPreview{}, ErrPreviewUnavailable
	}
	if _, loaded := s.previewOwners.LoadOrStore(user, struct{}{}); loaded {
		return CaptionPreview{}, ErrPreviewBusy
	}
	defer s.previewOwners.Delete(user)
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	next, err := ApplyCorrection(s.cfg.Render, p, draft)
	if err != nil {
		return CaptionPreview{}, err
	}
	// The design selection is the PROJECT's, exactly as it is for a preview or a
	// render: the styles a caption may take and the pace it moves at (CLIP-139).
	next = next.WithDesign(p.DesignSelection())
	if next.Portable == nil {
		recipe := Recipe{}
		if p.Composition != nil && p.Composition.Snapshot.LegacyRecipe != nil {
			recipe = *p.Composition.Snapshot.LegacyRecipe
		}
		next.Portable, err = FreezeLegacyPlan(p, next, recipe, s.cfg.Render.Composition)
		if err != nil {
			return CaptionPreview{}, err
		}
	}
	sources, err := RetainedSources(p)
	if err != nil {
		return CaptionPreview{}, err
	}
	refs := make([]RenderSource, 0, len(sources))
	for _, v := range sources {
		refs = append(refs, v.RenderSource)
	}
	canvas, err := ClipCanvas(next.Ratio)
	if err != nil {
		return CaptionPreview{}, err
	}
	fragments, err := fragmenter.CaptionFragments(ctx, next, refs)
	if err != nil {
		return CaptionPreview{}, err
	}
	return CaptionPreview{Ratio: next.Ratio, Canvas: canvas, Fragments: fragments}, nil
}
