package media

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

var _ clip.CaptionFramePreparer = (*Rendering)(nil)

// A browser render draws a sequence-rendered caption from the server's own
// drawing of each output frame (CLIP-159), which is the same drawing the render
// writes to disk for its own overlay pass. The frames travel as ONE sprite
// sheet: a two second caption is sixty of them, and one request and one decode
// beat sixty of each.
//
// Nothing here starts a job, spends a credit or writes a project: it is the
// preview's own read path with a frame run in place of an element page.
func (r *Rendering) PrepareCaptionFrames(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource, instanceID string, offset int, cfg clip.PreviewConfig) (clip.CaptionFrames, error) {
	out := clip.CaptionFrames{NextOffset: -1}
	if cfg.MaxFrameCells <= 0 || cfg.MaxSheetPixels <= 0 || cfg.MaxResponseBytes <= 0 || offset < 0 || instanceID == "" {
		return out, clip.ErrPreviewTooLarge
	}
	canvas, err := clip.ClipCanvas(plan.Ratio)
	if err != nil {
		return out, err
	}
	if err := clip.ValidateEditPlan(r.cfg, compositionGeometry(plan), sources); err != nil {
		return out, err
	}
	if plan.Portable == nil {
		return out, clip.ErrInvalid
	}
	err = r.media.WithWorkspace(ctx, "clip-caption-frames", func(ws clip.MediaWorkspace) error {
		layout, err := r.layoutComposition(ctx, ws, plan)
		if err != nil {
			return err
		}
		visual, found := declaredVisual{}, false
		for _, v := range layout.visuals {
			if v.manifest.InstanceID == instanceID {
				visual, found = v, true
				break
			}
		}
		if !found || visual.manifest.Role != "caption" {
			return clip.ErrInvalid
		}
		// A static style has ONE raster, which the draft preview already serves
		// as this caption's asset; asking for its frames is a mistake rather
		// than a cheaper path (CDS-81).
		if visual.caption.Caption.Static() {
			return clip.ErrInvalid
		}
		out, err = r.captionSheet(ctx, ws, canvas, visual, offset, cfg)
		return err
	})
	if err != nil {
		return clip.CaptionFrames{NextOffset: -1}, err
	}
	return out, nil
}

// captionSheet draws one run of a caption's frames onto a single document and
// rasterises it once. The frames are the render's own: same crop, same progress
// per frame, same painter (CDS-85).
func (r *Rendering) captionSheet(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, visual declaredVisual, offset int, cfg clip.PreviewConfig) (clip.CaptionFrames, error) {
	out := clip.CaptionFrames{NextOffset: -1}
	fps := r.cfg.FPS
	startMS, endMS := visual.manifest.StartMS, visual.manifest.EndMS
	first := startMS * fps / 1000
	count := (endMS*fps+999)/1000 - first
	crop := sequenceCrop(canvas, visual.caption)
	cellW, cellH := int(crop.Width), int(crop.Height)
	if count <= 0 || cellW <= 0 || cellH <= 0 || offset >= count {
		return out, clip.ErrInvalid
	}
	// The sheet is decoded whole by a browser, so both of its sides stay inside
	// one bound and the run is cut to what fits.
	columns := max(1, min(cfg.MaxSheetPixels/cellW, cfg.MaxFrameCells))
	rows := max(1, cfg.MaxSheetPixels/cellH)
	cells := min(count-offset, min(cfg.MaxFrameCells, columns*rows))
	if cells <= 0 {
		return out, clip.ErrPreviewTooLarge
	}
	// The bytes bound the run too. A style with a wide bleed and a busy texture
	// (ember, neon) compresses badly, so a run that fits the pixels can still be
	// more than one response carries; it is then cut in proportion and drawn
	// again rather than refused, and the browser asks for the rest from
	// NextOffset as it does for any run.
	budget := sheetByteBudget(cfg)
	var data []byte
	for {
		var err error
		data, columns, err = r.drawCaptionSheet(ctx, ws, canvas, visual, crop, offset, count, cells, cfg)
		if err != nil {
			return out, err
		}
		if len(data) <= budget {
			break
		}
		if cells == 1 {
			return out, clip.ErrPreviewTooLarge
		}
		cells = max(1, min(cells-1, cells*budget/len(data)))
	}
	next := offset + cells
	if next >= count {
		next = -1
	}
	return clip.CaptionFrames{Sheet: data, CellWidth: cellW, CellHeight: cellH, Columns: columns, Cells: cells,
		X: int(math.Round(crop.X)), Y: int(math.Round(crop.Y)), FirstFrame: first, FrameOffset: offset, NextOffset: next}, nil
}

// sheetByteBudget is the most PNG one sheet may hold. The sheet travels in a
// response bounded by MaxResponseBytes, and the browser reads that response as
// JSON, where the bytes are base64: four characters for every three, plus the
// run's few other fields.
func sheetByteBudget(cfg clip.PreviewConfig) int {
	return (cfg.MaxResponseBytes - 1024) / 4 * 3
}

// drawCaptionSheet rasterises `cells` frames from `offset` onto one sheet and
// returns its PNG and its column count.
func (r *Rendering) drawCaptionSheet(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, visual declaredVisual, crop clip.Region, offset, count, cells int, cfg clip.PreviewConfig) ([]byte, int, error) {
	cellW, cellH := int(crop.Width), int(crop.Height)
	columns := min(max(1, min(cfg.MaxSheetPixels/cellW, cfg.MaxFrameCells)), cells)
	rows := (cells + columns - 1) / columns
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, columns*cellW, rows*cellH)
	var defs, body strings.Builder
	duration := visual.manifest.EndMS - visual.manifest.StartMS
	for i := range cells {
		// The same progress the render's own frame carries, so a browser-drawn
		// frame and a server-drawn one are the same picture (CDS-7).
		progress := 0.0
		if count > 1 {
			progress = float64(offset+i) / float64(count-1)
		}
		frameDefs, frameBody, ok := design.DrawCaptionFrame(captionFrame(canvas, visual.copy, visual.caption, duration, progress))
		if !ok {
			return nil, 0, elementProblem(visual.text, "invalid_design")
		}
		// Two cells of one document cannot share a filter or clip id, so each
		// cell's ids carry its own index, exactly as a fragment carries its
		// caption's (CDS-80's filters are per drawing, not per caption).
		cell := fmt.Sprintf("%s-%d", visual.manifest.InstanceID, offset+i)
		defs.WriteString(prefixIDs(cell, frameDefs))
		col, row := i%columns, i/columns
		fmt.Fprintf(&body, `<g transform="translate(%s,%s)">%s</g>`,
			number(float64(col*cellW)-crop.X), number(float64(row*cellH)-crop.Y), prefixIDs(cell, frameBody))
	}
	if defs.Len() > 0 {
		fmt.Fprintf(&b, `<defs>%s</defs>`, defs.String())
	}
	b.WriteString(body.String())
	b.WriteString(`</svg>`)
	png := filepath.Join(ws.Path, fmt.Sprintf("caption-sheet-%d.png", offset))
	defer os.Remove(png)
	if err := r.rasterizeTo(ctx, ws, clip.Region{Width: float64(columns * cellW), Height: float64(rows * cellH)}, b.String(), png); err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(png)
	return data, columns, err
}
