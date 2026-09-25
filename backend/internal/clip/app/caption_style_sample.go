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

// RegionPresetSamples draws every intro and outro preset on the project's ratio
// with its slots numbered by the caller's label (CLIP-165), so ① shows which
// entry lands where by the renderer's own drawing. Like the style samples it
// asks the project for its ratio and owner only, and shares the preview's owner
// lock and timeout.
func (s *GenerationService) RegionPresetSamples(ctx context.Context, user, id, label string) (clip.RegionPresetSamples, error) {
	if !clip.ValidSlotLabel(label) {
		return clip.RegionPresetSamples{}, clip.ErrInvalid
	}
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.RegionPresetSamples{}, err
	}
	cfg := s.cfg.Preview
	sampler, ok := s.renderer.(clip.RegionPresetSampler)
	if !ok || cfg.Timeout <= 0 {
		return clip.RegionPresetSamples{}, clip.ErrPreviewUnavailable
	}
	canvas, err := clip.ClipCanvas(p.Ratio)
	if err != nil {
		return clip.RegionPresetSamples{}, err
	}
	if _, loaded := s.previewOwners.LoadOrStore(user, struct{}{}); loaded {
		return clip.RegionPresetSamples{}, clip.ErrPreviewBusy
	}
	defer s.previewOwners.Delete(user)
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	intro, outro, err := sampler.RegionPresetSamples(ctx, p.Ratio, label)
	if err != nil {
		return clip.RegionPresetSamples{}, err
	}
	return clip.RegionPresetSamples{Ratio: p.Ratio, Canvas: canvas, Intro: intro, Outro: outro}, nil
}
