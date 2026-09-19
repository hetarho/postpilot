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
	RepresentativeFrame                bool
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

type PlanLayouter interface {
	Layout(context.Context, EditPlan, []RenderSource) (EditPlan, Manifest, error)
}

type RenderPlanValidator interface {
	ValidateRenderPlan(context.Context, EditPlan, []RenderSource) (EditPlan, error)
}
