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

// A sequence-rendered caption draws one layer per output frame (CDS-80), so it
// cannot be one looped plate. The frames go to disk and enter the overlay chain
// through `image2` rather than `image2pipe`: the chain already takes several
// inputs, feeding pipes concurrently makes scheduling and partial-failure retry
// much harder, and files let one failed caption re-render alone.
//
// The frames are written under the attempt's own workspace, are counted against
// CLIP-33's temporary-disk budget exactly as every other intermediate is, and
// are deleted as soon as the overlay pass that reads them finishes.
type captionSequence struct {
	// The printf pattern ffmpeg is given, e.g. …/copy-0003/%05d.png.
	Pattern string
	// The directory holding them, removed whole once the pass is done.
	Dir string
	// Where the cropped layer sits on the canvas, and how many frames it holds.
	Origin clip.Region
	Frames int
	// The output frame this caption's first frame belongs to.
	StartFrame int
}

// sequenceCrop is the box a sequence layer is drawn and rasterised in: the
// caption's own region grown by the style's bleed and clamped to the canvas. A
// full-canvas layer per frame would reserve ten megabytes of workspace budget
// for every frame of every caption, which CLIP-33 cannot carry.
func sequenceCrop(canvas clip.Canvas, l copyLayout) clip.Region {
	bleed := l.Caption.Bleed()
	p := l.Region
	x, y := math.Max(0, math.Floor(p.X-bleed)), math.Max(0, math.Floor(p.Y-bleed))
	right := math.Min(float64(canvas.Width), math.Ceil(p.X+p.Width+bleed))
	bottom := math.Min(float64(canvas.Height), math.Ceil(p.Y+p.Height+bleed))
	return clip.Region{X: x, Y: y, Width: right - x, Height: bottom - y}
}

// captionFrame is everything one sequence style is handed to draw one instant:
// already measured, already placed, with only the progress differing between
// frames (CDS-83 — the preview is handed exactly this).
func captionFrame(canvas clip.Canvas, c clip.Copy, l copyLayout, durationMS int, progress float64) design.CaptionFrame {
	accent := design.Accent[c.Accent]
	if !l.Caption.Paint.Accent {
		accent = ""
	}
	left, right, vertical := copyInsets(l.Style)
	inner, top := l.Region.Width-left-right, l.Region.Y+vertical
	frame := design.CaptionFrame{
		Canvas: design.Size{Width: canvas.Width, Height: canvas.Height}, Style: l.Caption,
		Family: design.FontFamily(l.Caption.Face), Size: l.FontSize, Tracking: l.Role.Tracking,
		Region: design.Bounds(l.Region), Accent: accent, Progress: progress, DurationMS: durationMS,
	}
	for i, text := range l.Lines {
		bounds := l.Bounds[i]
		x := l.Region.X + left + (inner-bounds.Width)/2
		line := design.CaptionLine{Text: text, X: x - bounds.X, Y: top - bounds.Y, Top: top, Width: bounds.Width, Height: bounds.Height}
		if l.Keyword.Present && l.Keyword.Line == i {
			line.Keyword, line.KeywordX, line.KeywordWidth = l.Keyword.Text, x+l.Keyword.Offset, l.Keyword.Width
		}
		if i < len(l.Words) {
			for _, w := range l.Words[i] {
				line.Words = append(line.Words, design.CaptionWord{Text: w.Text, X: x + w.Offset, Width: w.Width})
			}
		}
		frame.Lines = append(frame.Lines, line)
		top += bounds.Height + l.FontSize*(l.Role.LineHeight-1)
	}
	return frame
}

// sequenceSVG is one frame's whole document, cropped to the layer's own box so
// the PNG carries only the pixels the style paints.
func sequenceSVG(crop clip.Region, frame design.CaptionFrame) (string, bool) {
	defs, body, ok := design.DrawCaptionFrame(frame)
	if !ok {
		return "", false
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="%.0f %.0f %.0f %.0f">`,
		crop.Width, crop.Height, crop.X, crop.Y, crop.Width, crop.Height)
	if defs != "" {
		fmt.Fprintf(&b, `<defs>%s</defs>`, defs)
	}
	b.WriteString(body)
	b.WriteString(`</svg>`)
	return b.String(), true
}

// captionSequenceFrames rasterises one caption's whole interval, one PNG per
// output frame, and returns what the overlay chain needs to read them back.
func (r *Rendering) captionSequence(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c clip.Copy, l copyLayout, startMS, endMS, index int) (captionSequence, error) {
	crop := sequenceCrop(canvas, l)
	fps := r.cfg.FPS
	first := startMS * fps / 1000
	last := (endMS*fps + 999) / 1000
	count := last - first
	if count <= 0 || crop.Width <= 0 || crop.Height <= 0 {
		return captionSequence{}, clip.ErrInvalid
	}
	dir := filepath.Join(ws.Path, fmt.Sprintf("seq-%04d", index))
	if err := os.Mkdir(dir, 0700); err != nil {
		return captionSequence{}, err
	}
	out := captionSequence{Pattern: filepath.Join(dir, "%05d.png"), Dir: dir,
		Origin: crop, Frames: count, StartFrame: first}
	duration := endMS - startMS
	for i := range count {
		// The progress is the frame's own position in the interval, so the
		// same plan draws the same frame on every host (CDS-7).
		progress := 0.0
		if count > 1 {
			progress = float64(i) / float64(count-1)
		}
		svg, ok := sequenceSVG(crop, captionFrame(canvas, c, l, duration, progress))
		if !ok {
			return captionSequence{}, clip.ErrInvalid
		}
		name := filepath.Join(dir, fmt.Sprintf("%05d.png", i+1))
		if err := r.rasterizeTo(ctx, ws, crop, svg, name); err != nil {
			return captionSequence{}, err
		}
	}
	return out, nil
}
