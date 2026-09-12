package clip

import (
	"context"
	"errors"
	"time"
)

var ErrPreviewBusy = errors.New("clip preview preparation busy")
var ErrPreviewTooLarge = errors.New("clip preview preparation limit")
var ErrPreviewUnavailable = errors.New("clip preview preparation unavailable")

type PreviewConfig struct {
	MaxAssets, MaxAssetBytes, MaxResponseBytes int
	Timeout                                    time.Duration
}
type PreviewAsset struct {
	Key, InstanceID                    string
	PNG                                []byte
	X, Y, Width, Height                int
	StartMS, EndMS, InMS, OutMS, Layer int
	DY                                 float64
}
type PreviewParity string

const (
	PreviewSourceContrast     PreviewParity = "source_contrast_final_only"
	PreviewAudioNormalization PreviewParity = "audio_normalization_final_only"
	PreviewFrameTiming        PreviewParity = "browser_frame_timing"
)

type PreparedPreview struct {
	DraftHash  string
	Canvas     Canvas
	Assets     []PreviewAsset
	NextOffset int
	Parity     []PreviewParity
}
type PreviewPreparer interface {
	PreparePreview(context.Context, EditPlan, []RenderSource, []string, int, PreviewConfig) (PreparedPreview, error)
}
type CompositionLayouter interface {
	LayoutComposition(context.Context, EditPlan, []RenderSource) (EditPlan, []CompositionElement, error)
}

func (s *GenerationService) PreparePreview(ctx context.Context, user, id string, revision int, hash string, draft CorrectionPlan, ids []string, offset int) (PreparedPreview, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return PreparedPreview{}, err
	}
	if revision <= 0 || p.EditPlanRevision != revision {
		return PreparedPreview{}, ErrPlanConflict
	}
	cfg := s.cfg.Preview
	renderer, ok := s.renderer.(PreviewPreparer)
	if !ok || cfg.Timeout <= 0 || cfg.MaxAssets <= 0 {
		return PreparedPreview{}, ErrPreviewUnavailable
	}
	if offset < 0 || len(ids) > cfg.MaxAssets {
		return PreparedPreview{}, ErrPreviewTooLarge
	}
	// The owner lock covers measurement as well as rasterization. No waiter can
	// build an unbounded queue, and every error/cancellation releases admission.
	if _, loaded := s.previewOwners.LoadOrStore(user, struct{}{}); loaded {
		return PreparedPreview{}, ErrPreviewBusy
	}
	defer s.previewOwners.Delete(user)
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	next, styles, err := ApplyCorrection(s.cfg.Render, p, draft)
	if err != nil {
		return PreparedPreview{}, err
	}
	if next.Portable == nil {
		recipe := Recipe{CopyStyles: styles}
		if p.Composition != nil && p.Composition.Snapshot.LegacyRecipe != nil {
			recipe = *p.Composition.Snapshot.LegacyRecipe
		}
		next.Portable, err = FreezeLegacyPlan(p, next, recipe, s.cfg.Render.Composition)
		if err != nil {
			return PreparedPreview{}, err
		}
	}
	sources, err := RetainedSources(p)
	if err != nil {
		return PreparedPreview{}, err
	}
	refs := make([]RenderSource, 0, len(sources))
	for _, v := range sources {
		refs = append(refs, v.RenderSource)
	}
	out, err := renderer.PreparePreview(ctx, next, refs, ids, offset, cfg)
	if err != nil {
		return PreparedPreview{}, err
	}
	// A concurrent save invalidates this preparation; it never updates the saved
	// draft, retention, result, job, observations or accounting.
	current, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return PreparedPreview{}, err
	}
	if current.EditPlanRevision != revision {
		return PreparedPreview{}, ErrPlanConflict
	}
	out.DraftHash = hash
	return out, nil
}

func (s *GenerationService) PreviewResponseLimit() int { return s.cfg.Preview.MaxResponseBytes }
