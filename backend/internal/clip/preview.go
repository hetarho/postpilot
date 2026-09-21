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
	// How many of a sequence-rendered caption's frames one sheet may carry, and
	// how large the sheet it draws them on may be in either direction: a browser
	// has to decode the sheet whole and blit cells out of it (CLIP-159).
	MaxFrameCells, MaxSheetPixels int
	Timeout                       time.Duration
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

// CaptionFrames is one run of a sequence-rendered caption's own frames, drawn
// the way a server render draws them and laid out as one sprite sheet: a two
// second caption is sixty frames, and one request and one decode beat sixty of
// each (CLIP-159, CDS-85).
type CaptionFrames struct {
	DraftHash string
	// One PNG holding the cells in order, left to right and top to bottom.
	Sheet                   []byte
	CellWidth, CellHeight   int
	Columns, Cells          int
	X, Y                    int
	FirstFrame, FrameOffset int
	// The next frame offset to ask for, or -1 once the caption has no more.
	NextOffset int
}

type CaptionFramePreparer interface {
	PrepareCaptionFrames(context.Context, EditPlan, []RenderSource, string, int, PreviewConfig) (CaptionFrames, error)
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
