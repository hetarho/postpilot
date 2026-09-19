package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
)

// CaptionPreviewOf hands ② every caption of the plan it is editing, drawn by the
// one place the style set lives, so the preview and the render cannot drift
// (CDS-83). It takes the draft rather than the saved plan because the editor
// must show the caption it is editing, not the one it last saved — the same
// contract PreparePreview already works on.
//
// It shares the preview's owner lock and timeout: this measures with resvg
// exactly as a preparation does, and one owner may have one measurement running.
func (s *GenerationService) CaptionPreviewOf(ctx context.Context, user, id string, revision int, draft clip.CorrectionPlan) (clip.CaptionPreview, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	if p.Finalized != nil {
		return clip.CaptionPreview{}, clip.ErrFinalized
	}
	if revision <= 0 || p.EditPlanRevision != revision {
		return clip.CaptionPreview{}, clip.ErrPlanConflict
	}
	cfg := s.cfg.Preview
	fragmenter, ok := s.renderer.(clip.CaptionFragmenter)
	if !ok || cfg.Timeout <= 0 {
		return clip.CaptionPreview{}, clip.ErrPreviewUnavailable
	}
	if _, loaded := s.previewOwners.LoadOrStore(user, struct{}{}); loaded {
		return clip.CaptionPreview{}, clip.ErrPreviewBusy
	}
	defer s.previewOwners.Delete(user)
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	next, err := clip.ApplyCorrection(s.cfg.Render, p, draft)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	// The design selection is the PROJECT's, exactly as it is for a preview or a
	// render: the styles a caption may take and the pace it moves at (CLIP-139).
	next = next.WithDesign(p.DesignSelection())
	if next.Portable == nil {
		recipe := clip.Recipe{}
		if p.Composition != nil && p.Composition.Snapshot.LegacyRecipe != nil {
			recipe = *p.Composition.Snapshot.LegacyRecipe
		}
		next.Portable, err = clip.FreezeLegacyPlan(p, next, recipe, s.cfg.Render.Composition)
		if err != nil {
			return clip.CaptionPreview{}, err
		}
	}
	sources, err := clip.RetainedSources(p)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	refs := make([]clip.RenderSource, 0, len(sources))
	for _, v := range sources {
		refs = append(refs, v.RenderSource)
	}
	canvas, err := clip.ClipCanvas(next.Ratio)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	fragments, err := fragmenter.CaptionFragments(ctx, next, refs)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	return clip.CaptionPreview{Ratio: next.Ratio, Canvas: canvas, Fragments: fragments}, nil
}
