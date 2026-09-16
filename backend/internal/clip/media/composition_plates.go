package media

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
)

func (r *Rendering) declaredPlate(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, visual *declaredVisual, source clip.MediaSource, index int) (string, error) {
	var body string
	var err error
	switch visual.manifest.Role {
	case "caption", "info", "hook", "ending":
		bounds := regionBounds(*visual)
		if bounds.Width > 0 && bounds.Height > 0 {
			cut := clip.Cut{EndMS: source.Info.DurationMS, Focal: clip.Point{X: .5, Y: .5}}
			window := declaredSampleWindow(visual.manifest.StartMS, visual.manifest.EndMS, source.Info.DurationMS, r.cfg.FPS)
			visual.ground, err = r.sample(ctx, ws, canvas, source, cut, window, bounds, index)
			if err != nil {
				return "", err
			}
			applyDeclaredGround(canvas, visual)
		}
	}
	body, err = r.declaredSVG(canvas, *visual)
	if err != nil {
		return "", err
	}
	return r.rasterize(ctx, ws, canvas, body, fmt.Sprintf("declared-%04d", index))
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
func (r *Rendering) renderOverlayWindow(ctx context.Context, ws clip.MediaWorkspace, input string, window overlayWindow, visuals []declaredVisual, plates []string, firstPass bool, output string) error {
	args := r.baseArgs()
	if firstPass {
		args = append(args, "-ss", strconv.Itoa(window.StartFrame/r.cfg.FPS))
	}
	args = r.inputArgs(args, input)
	for _, index := range window.Layers {
		args = append(args, "-threads", strconv.Itoa(r.media.cfg.DecodeThreads), "-framerate", strconv.Itoa(r.cfg.FPS), "-i", plates[index])
	}
	args = append(args, "-filter_complex", declaredOverlayGraph(r.cfg, window, visuals, firstPass))
	args = append(args, r.encodeProfile(false, 0, "yuv444p")...)
	args = append(args, "-t", frameSeconds(window.EndFrame-window.StartFrame, r.cfg.FPS))
	return r.runRender(ctx, ws, output, args)
}

func (r *Rendering) overlayComposition(ctx context.Context, ws clip.MediaWorkspace, input string, visuals []declaredVisual, plates []string, totalFrames int, cleanup *[]string) ([]string, []int, error) {
	windows := compositionWindows(visuals, totalFrames, r.cfg.FPS, r.cfg.OverlayBatchSize)
	paths, frames := []string{}, []int{}
	for i, window := range windows {
		previous := input
		for offset := 0; offset < len(window.Layers) || offset == 0; offset += r.cfg.OverlayBatchSize {
			batch := window
			batch.Layers = window.Layers[offset:min(len(window.Layers), offset+r.cfg.OverlayBatchSize)]
			path := filepath.Join(ws.Path, fmt.Sprintf("overlay-%04d-%04d.mp4", i, offset/r.cfg.OverlayBatchSize))
			*cleanup = append(*cleanup, path)
			if err := r.renderOverlayWindow(ctx, ws, previous, batch, visuals, plates, offset == 0, path); err != nil {
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
		stroke := design.Caption().Stroke
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
		if visual.ground.Sampled() && !design.Legible(visual.manifest.Parts) {
			clip.AddPlanNotice(&layout.plan, "composition_contrast", visual.manifest.CutID, visual.manifest.ElementID, "shortfall")
		}
	}
}
