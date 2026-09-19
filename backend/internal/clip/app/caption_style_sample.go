package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// CaptionStyleSamples draws every approved caption style the way the render
// draws a caption, so ① offers the set by its own look rather than by a picture
// of it that could go stale the moment a style changes (CDS-80, CDS-83). It
// asks the project only for its ratio and its owner: the samples are the same
// for every project of that ratio, and nothing about this project's own plan,
// selection or footage reaches them.
//
// It shares the preview's owner lock and timeout for the same reason ②'s
// fragments do: this measures with resvg, and one owner measures once at a time.
func (s *GenerationService) CaptionStyleSamples(ctx context.Context, user, id string) (clip.CaptionPreview, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	cfg := s.cfg.Preview
	fragmenter, ok := s.renderer.(clip.CaptionFragmenter)
	if !ok || cfg.Timeout <= 0 {
		return clip.CaptionPreview{}, clip.ErrPreviewUnavailable
	}
	styles := make([]string, 0, len(design.CaptionStyles()))
	for _, style := range design.CaptionStyles() {
		styles = append(styles, style.ID)
	}
	plan, sources, err := clip.CaptionStyleSamplePlan(p.Ratio, styles)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	canvas, err := clip.ClipCanvas(plan.Ratio)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	if _, loaded := s.previewOwners.LoadOrStore(user, struct{}{}); loaded {
		return clip.CaptionPreview{}, clip.ErrPreviewBusy
	}
	defer s.previewOwners.Delete(user)
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	fragments, err := fragmenter.CaptionFragments(ctx, plan, sources)
	if err != nil {
		return clip.CaptionPreview{}, err
	}
	return clip.CaptionPreview{Ratio: plan.Ratio, Canvas: canvas, Fragments: fragments}, nil
}
