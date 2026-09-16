package media

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Brightness sampling covers captions, intro/outro slots and information pairs.
//
// The three frames CDS-44 names — first, middle and last of the copy window —
// are extracted as PNGs and read in Go rather than parsed out of FFmpeg's
// signalstats output, so the luminance formula is one tested function and no
// text is scraped from a subprocess.
type Luminance struct {
	// Rec.709 relative luminance of the plate region, 0..1, and its deviation
	// across the sampled frames.
	Mean, Sigma float64
	// The mean colour under the plate, for the contrast check (V3).
	R, G, B float64
	// Per-frame luminance, in the order CDS-44 samples: first, middle, last.
	Frames []float64
}

// Scrim reports whether this ground demands one (CDS-44): too bright, or too
// busy to trust.
func (l Luminance) Scrim() bool {
	return l.Mean >= design.Luma.ScrimThreshold || l.Sigma >= design.Luma.SigmaThreshold
}

// AccentWhite reports whether an accent word has to turn white here (CDS-44).
func (l Luminance) AccentWhite() bool { return l.Mean >= design.Luma.ScrimThreshold }

// Hex is the sampled mean colour, which is the ground a text without a plate is
// actually read against.
func (l Luminance) Hex() string { return design.Hex(l.R, l.G, l.B) }

// Sampled distinguishes a measured ground from the zero value: a frame can
// legitimately average to black, and that is not the same as no sample.
func (l Luminance) Sampled() bool { return len(l.Frames) > 0 }

// Background is the effective background of one text on this ground: what the
// eye actually reads it against (V3).
//
// Stroked text is measured against stroke.dark composited over the sampled
// ground, after the scrim when present. Unstroked slots use the ground itself.
//
// The scrim is a vertical gradient, so what it contributes where the text sits
// is its own opacity at the copy's vertical centre, and zero when the copy lies
// outside the band CDS-32 fixes for that anchor.
func (l Luminance) Background(canvas clip.Canvas, style design.StyleRule, anchor string, region clip.Region) string {
	ground := l.Hex()
	if l.Scrim() {
		if s, ok := scrimFor(canvas, anchor); ok {
			paint := design.Scrim[s.Edge]
			if alpha := scrimAlpha(paint, s.Region, region); alpha > 0 {
				if washed, ok := design.Over(paint.Hex, alpha, ground); ok {
					ground = washed
				}
			}
		}
	}
	if style.Stroke == "" {
		return ground
	}
	stroke := design.Color["stroke_dark"]
	out, ok := design.Over(stroke.Hex, stroke.Alpha, ground)
	if !ok {
		return ground
	}
	return out
}

// scrimAlpha is the gradient's opacity at the vertical centre of a region, and
// zero for a region whose centre is outside the scrim's own rectangle.
func scrimAlpha(paint design.ScrimPaint, band, region clip.Region) float64 {
	if band.Height <= 0 {
		return 0
	}
	centre := region.Y + region.Height/2
	if centre < band.Y || centre > band.Y+band.Height {
		return 0
	}
	t := (centre - band.Y) / band.Height
	return paint.From + (paint.To-paint.From)*t
}

// relativeLuminance is WCAG 2.1's: sRGB channels linearised, then weighted.
func relativeLuminance(r, g, b float64) float64 {
	channel := func(c float64) float64 {
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

// regionLuminance averages one frame over the plate region and returns its
// relative luminance and its mean colour.
func regionLuminance(frame image.Image, region clip.Region) (float64, float64, float64, float64) {
	bounds := frame.Bounds()
	x0, y0 := max(bounds.Min.X, int(region.X)), max(bounds.Min.Y, int(region.Y))
	x1, y1 := min(bounds.Max.X, int(region.X+region.Width)), min(bounds.Max.Y, int(region.Y+region.Height))
	if x1 <= x0 || y1 <= y0 {
		return 0, 0, 0, 0
	}
	var sr, sg, sb float64
	n := 0.0
	// Every second pixel in each direction: a quarter of the reads for a mean
	// that moves in the fourth decimal.
	for y := y0; y < y1; y += 2 {
		for x := x0; x < x1; x += 2 {
			r, g, b, _ := frame.At(x, y).RGBA()
			sr, sg, sb = sr+float64(r)/0xffff, sg+float64(g)/0xffff, sb+float64(b)/0xffff
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0, 0
	}
	sr, sg, sb = sr/n, sg/n, sb/n
	return relativeLuminance(sr, sg, sb), sr, sg, sb
}

// readFrame decodes one extracted PNG frame.
func readFrame(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}

// scrim is the rectangle an anchor's scrim occupies, and which of the two
// gradients it uses.
type scrim struct {
	Edge   string
	Region clip.Region
}

// scrimFor is the scrim an unplated copy at this anchor would take. TOP and
// UPPER_MID take the top scrim, BOTTOM and LOWER_MID the bottom one; CDS-32
// states both rectangles per ratio.
func scrimFor(canvas clip.Canvas, anchor string) (scrim, bool) {
	ratio, ok := ratioOf(canvas)
	if !ok {
		return scrim{}, false
	}
	l, _ := design.Layout(ratio)
	switch anchor {
	case "top", "upper_mid":
		return scrim{"top", clip.Region(l.ScrimTop)}, true
	case "bottom", "lower_mid":
		return scrim{"bottom", clip.Region(l.ScrimBottom)}, true
	}
	return scrim{}, false
}

// ratioOf recovers a canvas's ratio id from its own dimensions, so a drawing
// function needs nothing but the canvas it is drawing on.
func ratioOf(canvas clip.Canvas) (string, bool) {
	for ratio, layout := range design.Ratios {
		if layout.Canvas.Width == canvas.Width && layout.Canvas.Height == canvas.Height {
			return ratio, true
		}
	}
	return "", false
}

// sampleOffsets is the first, middle and last frame of a window, in the SOURCE
// time the seek needs. The window is CUT-RELATIVE OUTPUT time, so each offset
// is converted back through the cut's own rate; a 1x cut — and the synthetic
// cut the composition path uses over already-transformed footage — converts to
// itself (CDS-62). The last frame is the one BEFORE the window closes, because
// asking for the closing instant itself can land past the cut.
func sampleOffsets(cut clip.EditCut, window [2]int) []int {
	offsets := []int{window[0], (window[0] + window[1]) / 2, max(window[0], window[1]-1)}
	for i, at := range offsets {
		offsets[i] = cut.StartMS + at*cut.Rate()/clip.RateUnitPermille
	}
	return offsets
}

// sample measures one element's plate region on its own three frames. It runs
// inside the source callback that renders the cut, so no second download
// happens (CLIP-33).
func (r *Rendering) sample(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, source clip.MediaSource, cut clip.EditCut, window [2]int, region clip.Region, index int) (Luminance, error) {
	frames, err := r.sampleFrames(ctx, ws, canvas, source, cut.Focal, sampleOffsets(cut, window), index)
	if err != nil {
		return Luminance{}, err
	}
	return measureFrames(frames, []clip.Region{region, region, region}), nil
}

// sampleFrames extracts every requested timestamp in ONE read of the footage.
// A per-frame command seeks and decodes the whole intermediate again each time;
// output-side seeks share a single decode and select the very same frames
// (CLIP-124, CLIP-125). The batch is bounded because each output carries its own
// scale/crop chain.
func (r *Rendering) sampleFrames(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, source clip.MediaSource, focal clip.Point, offsets []int, index int) ([]image.Image, error) {
	frames := make([]image.Image, 0, len(offsets))
	for start := 0; start < len(offsets); start += r.cfg.SampleBatch {
		batch := offsets[start:min(len(offsets), start+r.cfg.SampleBatch)]
		if err := r.media.capacity(ws, int64(len(batch))*4*1024*1024); err != nil {
			return nil, err
		}
		args := append(r.baseArgs(), "-threads", strconv.Itoa(r.media.cfg.DecodeThreads), "-protocol_whitelist", "file,pipe", "-i", source.Path)
		paths := make([]string, len(batch))
		for i, at := range batch {
			paths[i] = filepath.Join(ws.Path, "sample-"+strconv.Itoa(index)+"-"+strconv.Itoa(start+i)+".png")
			args = append(args, "-ss", seconds(at), "-frames:v", "1", "-vf", coverChain(canvas, focal), "-c:v", "png", "-threads", strconv.Itoa(r.media.cfg.EncodeThreads), paths[i])
		}
		_, err := r.media.run(ctx, ws, r.media.cfg.FFmpegPath, args...)
		for _, path := range paths {
			if err != nil {
				_ = os.Remove(path)
				continue
			}
			frame, e := readFrame(path)
			_ = os.Remove(path)
			if e != nil {
				err = e
				continue
			}
			frames = append(frames, frame)
		}
		if err != nil {
			return nil, err
		}
	}
	return frames, nil
}

// measureFrames is CDS-44's measurement over frames already in memory: one
// region per frame, so a batch shared between elements measures each element on
// its own region.
func measureFrames(frames []image.Image, regions []clip.Region) Luminance {
	values, colours := make([]float64, 0, len(frames)), make([][3]float64, 0, len(frames))
	for i, frame := range frames {
		value, cr, cg, cb := regionLuminance(frame, regions[i])
		values, colours = append(values, value), append(colours, [3]float64{cr, cg, cb})
	}
	return summarize(values, colours)
}

// summarize is CDS-44's L and σ over the sampled frames.
func summarize(values []float64, colours [][3]float64) Luminance {
	if len(values) == 0 {
		return Luminance{}
	}
	mean := 0.0
	var out Luminance
	for i, v := range values {
		mean += v
		out.R, out.G, out.B = out.R+colours[i][0], out.G+colours[i][1], out.B+colours[i][2]
	}
	n := float64(len(values))
	mean, out.R, out.G, out.B = mean/n, out.R/n, out.G/n, out.B/n
	out.Frames = values
	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	out.Mean, out.Sigma = mean, math.Sqrt(variance/n)
	return out
}
