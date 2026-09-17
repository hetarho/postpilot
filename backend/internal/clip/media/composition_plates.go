package media

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
)

// sampleDeclaredGrounds measures every element's ground BEFORE any plate is
// built, so the frames CDS-44 names for the whole plan come from as few reads of
// the composed footage as the batch allows rather than one read per frame
// (CLIP-124). Each element is still measured on exactly its own three frames
// and its own region.
func (r *Rendering) sampleDeclaredGrounds(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, source clip.MediaSource, visuals []declaredVisual) error {
	cut := clip.Cut{EndMS: source.Info.DurationMS, Focal: clip.Point{X: .5, Y: .5}}
	var wanted []int
	var regions []clip.Region
	var sampled []int
	for i := range visuals {
		switch visuals[i].manifest.Role {
		case "caption", "info", "hook", "ending":
		default:
			continue
		}
		bounds := regionBounds(visuals[i])
		if bounds.Width <= 0 || bounds.Height <= 0 {
			continue
		}
		window := declaredSampleWindow(visuals[i].manifest.StartMS, visuals[i].manifest.EndMS, source.Info.DurationMS, r.cfg.FPS)
		offsets := sampleOffsets(cut, window)
		sampled = append(sampled, i)
		wanted = append(wanted, offsets...)
		for range offsets {
			regions = append(regions, bounds)
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	frames, err := r.sampleFrames(ctx, ws, canvas, source, cut.Focal, wanted, 0)
	if err != nil {
		return err
	}
	if len(frames) != len(wanted) {
		return clip.ErrInvalidMedia
	}
	for at, index := range sampled {
		from := at * 3
		visuals[index].ground = measureFrames(frames[from:from+3], regions[from:from+3])
		applyDeclaredGround(canvas, &visuals[index])
	}
	return nil
}

// One overlay input: a static caption's single plate, which the chain loops for
// the caption's whole interval, or a sequence style's own PNG per output frame
// (CDS-80, CDS-81).
type captionLayer struct {
	Plate    string
	Sequence *captionSequence
}

// paths are what the pass that reads this layer has to delete once it is done.
func (l captionLayer) paths() []string {
	if l.Sequence != nil {
		return []string{l.Sequence.Dir}
	}
	return []string{l.Plate}
}

func (r *Rendering) declaredLayer(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, visual *declaredVisual, source clip.MediaSource, index int) (captionLayer, error) {
	// A rapid phrase replaces its neighbour with no fade and no movement
	// whatever style it carries (CDS-4), so it has nothing to animate and takes
	// the one rasterisation its style would otherwise spend per frame.
	if visual.manifest.Role == "caption" && !visual.caption.Caption.Static() && visual.copy.Pace != "rapid" {
		sequence, err := r.captionSequence(ctx, ws, canvas, visual.copy, visual.caption, visual.manifest.StartMS, visual.manifest.EndMS, index)
		if err != nil {
			return captionLayer{}, err
		}
		return captionLayer{Sequence: &sequence}, nil
	}
	body, err := r.declaredSVG(canvas, *visual)
	if err != nil {
		return captionLayer{}, err
	}
	plate, err := r.rasterize(ctx, ws, canvas, body, fmt.Sprintf("declared-%04d", index))
	return captionLayer{Plate: plate}, err
}

// layerInput is what ffmpeg is told to read this layer back with. A sequence
// enters through `image2` at the output frame rate rather than through
// `image2pipe`: the chain already takes several inputs, feeding pipes
// concurrently makes scheduling and partial-failure retry much harder, and
// files let one failed caption re-render alone. Decoder threads stay limited on
// it exactly as they are on a single-frame input, because an unbounded image
// demuxer can leave the scheduler waiting once an overlay stops consuming.
func (r *Rendering) layerInput(args []string, layer captionLayer, window overlayWindow) []string {
	args = append(args, "-threads", strconv.Itoa(r.media.cfg.DecodeThreads), "-framerate", strconv.Itoa(r.cfg.FPS))
	if layer.Sequence != nil {
		// A caption that began before this window RESUMES at the frame the
		// window opens on rather than starting over: the sequence is the
		// caption's own motion through its interval, not a loop.
		first := 1 + max(0, min(window.StartFrame-layer.Sequence.StartFrame, layer.Sequence.Frames-1))
		return append(args, "-f", "image2", "-start_number", strconv.Itoa(first), "-i", layer.Sequence.Pattern)
	}
	return append(args, "-i", layer.Plate)
}

// Both export and preview shape exactly the same escaped, bundled templates.
// Only export can supply measured source luminance to this pure plate builder.
func (r *Rendering) declaredSVG(canvas clip.Canvas, visual declaredVisual) (string, error) {
	switch visual.manifest.Role {
	case "caption":
		return r.overlays.Render("copy."+visual.copy.Style, copyView(canvas, visual.copy, visual.caption, visual.ground))
	case "badge":
		return r.overlays.Render("furniture", furnitureView(canvas, visual.furniture))
	case "info":
		applyDeclaredGround(canvas, &visual)
		return r.overlays.Render("info", visual.info)
	case "hook", "ending":
		applyDeclaredGround(canvas, &visual)
		return r.overlays.Render("region", visual.region)
	default:
		return "", elementProblem(visual.text, "invalid_role")
	}
}

// Seeking past the final frame timestamp yields no image even though it is
// before the MP4 duration. Keep all three samples on an existing output frame.
func declaredSampleWindow(start, end, duration, fps int) [2]int {
	last := max(0, duration-(1000+fps-1)/fps)
	end = min(end, last+1)
	return [2]int{min(start, end-1), end}
}

// Only this window's small layer batch is opened. Subsequent batches read the
// preceding lossless window, not the original full-length composition.
func (r *Rendering) renderOverlayWindow(ctx context.Context, ws clip.MediaWorkspace, input string, window overlayWindow, visuals []declaredVisual, layers []captionLayer, firstPass bool, output string) error {
	args := r.baseArgs()
	if firstPass {
		args = append(args, "-ss", strconv.Itoa(window.StartFrame/r.cfg.FPS))
	}
	args = r.inputArgs(args, input)
	for _, index := range window.Layers {
		args = r.layerInput(args, layers[index], window)
	}
	args = append(args, "-filter_complex", declaredOverlayGraph(r.cfg, window, visuals, layers, firstPass))
	args = append(args, r.encodeProfile(false, 0, "yuv444p")...)
	args = append(args, "-t", frameSeconds(window.EndFrame-window.StartFrame, r.cfg.FPS))
	return r.runRender(ctx, ws, output, args)
}

// deliveredOverlayArgs fuses the two passes a single-window clip used to take.
// The overlay's own output IS the delivered picture there, so the graph carries
// on into the delivery format and the already-measured track joins it here:
// no full-length lossless intermediate is written and read back (CLIP-124).
//
// The tail repeats what the windowed path's delivery pass does to its piece —
// the timebase reset, the trim and the conversion, in that order — so only the
// lossless round trip is removed and no delivered pixel moves (CLIP-125).
func (r *Rendering) deliveredOverlayArgs(input string, window overlayWindow, visuals []declaredVisual, layers []captionLayer, audio string, measured *loudness) []string {
	args := r.baseArgs()
	args = append(args, "-ss", strconv.Itoa(window.StartFrame/r.cfg.FPS))
	args = r.inputArgs(args, input)
	for _, index := range window.Layers {
		args = r.layerInput(args, layers[index], window)
	}
	frames := window.EndFrame - window.StartFrame
	graph := strings.TrimSuffix(declaredOverlayGraph(r.cfg, window, visuals, layers, true), "[v]")
	graph += fmt.Sprintf(",settb=AVTB,setpts=PTS-STARTPTS,trim=end_frame=%d,setpts=PTS-STARTPTS,format=yuv420p[v]", frames)
	if audio != "" {
		args = r.inputArgs(args, audio)
		graph += fmt.Sprintf(";[%d:a:0]%s,aresample=%d,aformat=sample_fmts=fltp:channel_layouts=stereo[a]", 1+len(window.Layers), loudnormFilter(measured), r.cfg.AudioRate)
	}
	return append(args, "-filter_complex", graph)
}

func (r *Rendering) overlayComposition(ctx context.Context, ws clip.MediaWorkspace, input string, windows []overlayWindow, visuals []declaredVisual, layers []captionLayer, cleanup *[]string) ([]string, []int, error) {
	paths, frames := []string{}, []int{}
	for i, window := range windows {
		previous := input
		for offset := 0; offset < len(window.Layers) || offset == 0; offset += r.cfg.OverlayBatchSize {
			batch := window
			batch.Layers = window.Layers[offset:min(len(window.Layers), offset+r.cfg.OverlayBatchSize)]
			path := filepath.Join(ws.Path, fmt.Sprintf("overlay-%04d-%04d.mp4", i, offset/r.cfg.OverlayBatchSize))
			*cleanup = append(*cleanup, path)
			if err := r.renderOverlayWindow(ctx, ws, previous, batch, visuals, layers, offset == 0, path); err != nil {
				return nil, nil, err
			}
			if previous != input {
				if err := removeIntermediate(previous); err != nil {
					return nil, nil, err
				}
			}
			previous = path
		}
		paths = append(paths, previous)
		frames = append(frames, window.EndFrame-window.StartFrame)
	}
	return paths, frames, nil
}

// Source contrast changes only the backdrop and its delivery notice. Region
// slot fills remain their declared white/alpha so V20 can verify them unchanged.
func applyDeclaredGround(canvas clip.Canvas, visual *declaredVisual) {
	if !visual.ground.Sampled() {
		return
	}
	visual.manifest.Parts = slices.DeleteFunc(slices.Clone(visual.manifest.Parts), func(p design.Element) bool { return p.Kind == "scrim" })
	anchor := visual.manifest.Position
	if anchor == "header" || anchor == "auto" {
		anchor = "top"
		bounds := regionBounds(*visual)
		if bounds.Y+bounds.Height/2 > float64(canvas.Height)/2 {
			anchor = "bottom"
		}
	}
	var view *overlay.CopyView
	switch visual.manifest.Role {
	case "info":
		view = &visual.info
	case "hook", "ending":
		view = &visual.region.CopyView
	}
	if view != nil {
		view.Scrim = nil
	}
	if visual.ground.Scrim() {
		if scrim, ok := scrimFor(canvas, anchor); ok {
			colour := design.Scrim[scrim.Edge]
			if view != nil {
				view.Scrim = &overlay.Scrim{Box: overlayBox(scrim.Region, 0, colour.Hex, ""), From: trimmed(colour.From), To: trimmed(colour.To)}
			}
			visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "scrim", Region: design.Bounds(scrim.Region), StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
		}
	}
	line := 0
	for i := range visual.manifest.Parts {
		p := &visual.manifest.Parts[i]
		if p.Kind != "copy" {
			continue
		}
		stroke := ""
		if visual.caption.Caption.DarkStroke() {
			stroke = visual.caption.Style.Stroke
		}
		if view != nil {
			stroke = ""
			if line < len(view.Lines) && view.Lines[line].StrokeWidth > 0 {
				stroke = "small"
			}
		}
		p.Background = visual.ground.Background(canvas, design.StyleRule{Stroke: stroke}, anchor, clip.Region(p.Region))
		p.ContrastNotice = !design.Legible(design.Manifest{*p})
		line++
	}
}

func (layout *declaredLayout) recordContrastNotices() {
	for _, visual := range layout.visuals {
		// A shortfall under an owner placement is named the same way and does
		// not block the render: the owner chose the spot, so the clip is
		// delivered with a notice about it rather than refused (CDS-52).
		if visual.ground.Sampled() && !design.Legible(visual.manifest.Parts) {
			clip.AddPlanNotice(&layout.plan, "composition_contrast", visual.manifest.CutID, visual.manifest.ElementID, "shortfall")
		}
		// A caption drawn in the default style because its own face could not
		// set one of its syllables says so by name (CDS-84, CLIP-108).
		if visual.glyphFallback {
			clip.AddPlanNotice(&layout.plan, "composition_caption_glyph", visual.manifest.CutID, visual.manifest.ElementID, "style_fallback")
		}
	}
}
